package library

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// ClientImportOptions names a local destination and an optional stable operation
// group. It never accepts a temporary client-view operation ordinal.
type ClientImportOptions struct {
	Box          domain.BoxID
	Into         domain.FolderID
	Hidden       bool
	ConflictMode string
	Group        *SyncReference
}

type ClientImportResult struct {
	Reference   SyncReference
	Files       []FileResult
	Interrupted bool
	Remaining   int
}

// PreparedClientImport owns immutable intent, while original bytes are already
// durable in protected local storage. Abandoning preparation cannot evict them.
type PreparedClientImport struct {
	client    *Client
	result    ClientImportResult
	action    oplog.Action
	payload   []byte
	group     *SyncReference
	box       domain.BoxID
	into      domain.FolderID
	target    string
	conflicts string
	paths     []string
	files     []SyncFileState
}

func (p *PreparedClientImport) Result() ClientImportResult {
	result := p.result
	result.Files = append([]FileResult(nil), result.Files...)
	return result
}

func (c *Client) PrepareFiles(ctx context.Context, sources []ImportSource, opt ClientImportOptions) (*PreparedClientImport, error) {
	if ctx.Value(clientRequestContextKey{}) != nil || ctx.Value(clientViewContextKey{}) != nil {
		return nil, ErrRequestScope
	}
	candidates, err := importCandidates(sources)
	if err != nil {
		return nil, err
	}
	return c.prepareImport(ctx, candidates, opt)
}

func (c *Client) PrepareImport(ctx context.Context, src string, opt ClientImportOptions) (*PreparedClientImport, error) {
	if ctx.Value(clientRequestContextKey{}) != nil || ctx.Value(clientViewContextKey{}) != nil {
		return nil, ErrRequestScope
	}
	candidates, err := scan(src, opt.Hidden)
	if err != nil {
		return nil, err
	}
	return c.prepareImport(ctx, candidates, opt)
}

func (c *Client) Import(ctx context.Context, src string, opt ClientImportOptions) (*ClientImportResult, error) {
	prepared, err := c.PrepareImport(ctx, src, opt)
	if err != nil {
		return nil, err
	}
	return prepared.Commit(context.WithoutCancel(ctx))
}

func (c *Client) prepareImport(ctx context.Context, candidates []scanned, opt ClientImportOptions) (*PreparedClientImport, error) {
	release, leaseErr := c.localPreparationLease(ctx)
	if leaseErr != nil {
		return nil, leaseErr
	}
	defer release()
	if opt.ConflictMode != "" && opt.ConflictMode != "keep_both" && opt.ConflictMode != "overwrite" {
		return nil, ErrInvalidRequest
	}
	prepared := &PreparedClientImport{client: c, box: opt.Box, action: oplog.ActionImportFiles}
	if opt.Group != nil {
		group := *opt.Group
		prepared.group = &group
	}
	var tree *importTree
	var taken map[domain.FolderID]map[string]string
	var conflicts UploadConflicts
	var zone *time.Location
	for _, candidate := range candidates {
		prepared.paths = append(prepared.paths, candidate.rel)
	}
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		_, found, err := tx.GetBox(opt.Box)
		if err != nil {
			return err
		}
		if !found {
			return ErrNoSuchBox
		}
		folders, err := store.FoldersOfBox(tx.Queryer(), opt.Box)
		if err != nil {
			return err
		}
		tree, err = newImportTree(opt.Box, folders, opt.Into)
		if err != nil {
			return err
		}
		prepared.into = tree.base
		taken, err = store.ExistingNames(tx.Queryer(), opt.Box)
		if err != nil {
			return err
		}
		zone, err = c.readZone()
		if err != nil {
			return err
		}
		read := func(fn func(*store.ViewTx) error) error { return fn(tx) }
		prepared.target, err = uploadTarget(read, opt.Box, tree.base)
		if err != nil {
			return err
		}
		if opt.ConflictMode != "" {
			conflicts, err = previewUpload(read, opt.Box, tree.base, prepared.paths)
			if err != nil {
				return err
			}
			prepared.conflicts = conflicts.Token
			if opt.ConflictMode == "overwrite" && !conflicts.CanOverwrite {
				return ErrUploadTargetChanged
			}
			for _, folder := range folders {
				if folder.ParentID != nil {
					if taken[*folder.ParentID] == nil {
						taken[*folder.ParentID] = map[string]string{}
					}
					taken[*folder.ParentID][domain.Fold(folder.Name)] = "folder"
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	payload := oplog.ImportFilesPayload{BoxID: opt.Box}
	var replacements []oplog.FileReplacement
	strict := false
	for _, candidate := range candidates {
		strict = strict || candidate.expected != nil
	}
	verified := map[blob.Key]ImportIntegrity{}
	for i, candidate := range candidates {
		if ctx.Err() != nil {
			prepared.result.Interrupted = true
			prepared.result.Remaining = len(candidates) - i
			break
		}
		before, replacing := conflicts.entries[candidate.rel]
		replacing = replacing && opt.ConflictMode == "overwrite"
		if replacing {
			delete(taken[before.Parent], domain.Fold(before.Name))
		}
		result := prepareImportFile(ctx, c.blobs, zone, candidate, tree, taken, &payload, opt.ConflictMode != "")
		if replacing && result.Status == FileImported {
			file := payload.Files[len(payload.Files)-1]
			payload.Files = payload.Files[:len(payload.Files)-1]
			replacements = append(replacements, oplog.FileReplacement{Before: before, After: replacementAfter(before, file, func(facts []extract.KV) domain.Instant { return capturedFrom(facts, zone) })})
			prepared.files = append(prepared.files, SyncFileState{ID: before.Files[0].File.ID, BlobKey: before.Files[0].File.BlobKey})
			result.Name = before.Name
		}
		if strict && result.Err != nil {
			return nil, result.Err
		}
		if candidate.expected != nil {
			verified[result.blobKey] = *candidate.expected
		}
		if result.Status == FileFailed && errors.Is(result.Err, context.Canceled) {
			prepared.result.Interrupted = true
			prepared.result.Remaining = len(candidates) - i
			break
		}
		prepared.result.Files = append(prepared.result.Files, result)
	}
	if strict {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for key, expected := range verified {
			digest, size, err := c.blobs.Rehash(ctx, key)
			if err != nil {
				return nil, err
			}
			if digest != expected.SHA256 || size != expected.Size {
				return nil, ErrImportIntegrity
			}
		}
	}
	if len(payload.Files) > 0 || len(payload.Folders) > 0 || len(replacements) > 0 {
		var body any = payload
		if len(replacements) > 0 {
			prepared.action = oplog.ActionUploadFiles
			body = oplog.UploadFilesPayload{Import: payload, Replacements: replacements}
		}
		prepared.payload, err = oplog.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	return prepared, nil
}

// Commit revalidates the destination under the client writer lock, then records
// one durable pending intent. Retry protection is supplied by ExecuteRequest.
func (p *PreparedClientImport) Commit(ctx context.Context) (*ClientImportResult, error) {
	result := p.Result()
	scope, release, err := p.client.commandLock(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	tx, err := p.client.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	vtx := store.NewViewTx(tx, time.Time{})
	read := func(fn func(*store.ViewTx) error) error { return fn(vtx) }
	target, err := uploadTarget(read, p.box, p.into)
	if err == nil && target != p.target {
		err = ErrUploadTargetChanged
	}
	if err == nil && p.conflicts != "" {
		var current UploadConflicts
		current, err = previewUpload(read, p.box, p.into, p.paths)
		if err == nil && current.Token != p.conflicts {
			err = ErrUploadTargetChanged
		}
	}
	rollbackErr := tx.Rollback()
	if err != nil {
		return nil, err
	}
	if rollbackErr != nil {
		return nil, rollbackErr
	}
	if len(p.payload) == 0 {
		return &result, nil
	}
	proposal := SyncProposal{Version: 1, ID: domain.NewID().String(), OccurredAt: time.Now().UTC(), Action: string(p.action), Origin: "user", Payload: p.payload, Group: p.group, Files: p.files}
	result.Reference, err = p.client.stageCommandLocked(ctx, scope, proposal)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
