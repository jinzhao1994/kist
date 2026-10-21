package library

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// ClientDeletionPreview binds confirmation to a snapshot, never a temporary
// operation number in the client's materialized view.
type ClientDeletionPreview struct {
	Name        string `json:"name"`
	FileCount   int    `json:"file_count"`
	FolderCount int    `json:"folder_count"`
	Revision    string `json:"revision"`
}

type clientDeletion struct {
	Basis   SyncDeletionBasis
	Preview ClientDeletionPreview
	Action  oplog.Action
	Payload json.RawMessage
}

// deletionLocked reads the complete deletion scope while the client edit lock
// prevents another process from changing the confirmed/pending overlay.
func (c *Client) deletionLocked(ctx context.Context, view clientOverlay, box domain.BoxID, id domain.ID, folder, wholeBox bool) (clientDeletion, error) {
	var result clientDeletion
	settings, err := c.settings()
	if err != nil {
		return result, err
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	vtx := store.NewViewTx(tx, time.Time{})
	if wholeBox {
		contents, found, err := vtx.BoxContents(box)
		if err != nil {
			return result, err
		}
		if !found {
			return result, ErrNoSuchBox
		}
		result.Preview.Name = contents.Box.Name
		result.Preview.FileCount = len(contents.Files)
		result.Action = oplog.ActionDeleteBoxContents
		result.Payload, err = oplog.Marshal(contents)
		if err != nil {
			return result, err
		}
	} else {
		contents, found, err := vtx.EntryContents(box, id, folder)
		if err != nil {
			return result, err
		}
		if !found {
			return result, ErrNoSuchFile
		}
		result.Preview.Name = contents.Name
		result.Preview.FileCount = len(contents.Files)
		result.Preview.FolderCount = len(contents.Folders)
		if folder && result.Preview.FolderCount > 0 {
			result.Preview.FolderCount--
		}
		changes, err := entryFilesetChanges(vtx, contents)
		if err != nil {
			return result, err
		}
		raw, err := oplog.Marshal(contents)
		if err != nil {
			return result, err
		}
		result.Action, result.Payload = oplog.ActionDeleteEntryContents, raw
		if len(changes) > 0 {
			result.Action = oplog.ActionDeleteFilesetEntry
			result.Payload, err = oplog.Marshal(oplog.FilesetEntryPayload{Entry: raw, Changes: changes})
			if err != nil {
				return result, err
			}
		}
	}
	revisionPayload, err := deletionSemanticPayload(result.Action, result.Payload)
	if err != nil {
		return result, err
	}
	last, _, err := store.LastOpTouchingBox(tx, box, 0)
	if err != nil {
		return result, err
	}
	_, found := view.References[last]
	if !found {
		return result, ErrInvariantViolated
	}
	identity, err := syncOperationIdentity(tx, last, view.References)
	if err != nil {
		return result, err
	}
	result.Basis = SyncDeletionBasis{Box: box, Last: identity}
	raw, err := json.Marshal(struct {
		Version  int
		Library  domain.ID
		Box      domain.BoxID
		Target   domain.ID
		Folder   bool
		WholeBox bool
		Last     string
		Action   oplog.Action
		Payload  json.RawMessage
	}{1, settings.LibraryID, box, id, folder, wholeBox, identity, result.Action, revisionPayload})
	if err != nil {
		return result, err
	}
	result.Preview.Revision = fmt.Sprintf("%x", sha256.Sum256(raw))
	return result, nil
}

func (c *Client) PreviewEntryDeletion(ctx context.Context, box domain.BoxID, id domain.ID, folder bool) (ClientDeletionPreview, error) {
	_, view, release, err := c.commandViewLock(ctx)
	if err != nil {
		return ClientDeletionPreview{}, err
	}
	defer release()
	deletion, err := c.deletionLocked(ctx, view, box, id, folder, false)
	return deletion.Preview, err
}

func (c *Client) PreviewBoxDeletion(ctx context.Context, box domain.BoxID) (ClientDeletionPreview, error) {
	_, view, release, err := c.commandViewLock(ctx)
	if err != nil {
		return ClientDeletionPreview{}, err
	}
	defer release()
	deletion, err := c.deletionLocked(ctx, view, box, box.ID, false, true)
	return deletion.Preview, err
}

func (c *Client) DeleteEntry(ctx context.Context, box domain.BoxID, id domain.ID, folder bool, revision string) (SyncReference, error) {
	return c.deleteConfirmed(ctx, box, id, folder, false, revision)
}

func (c *Client) DeleteBox(ctx context.Context, box domain.BoxID, revision string) (SyncReference, error) {
	return c.deleteConfirmed(ctx, box, box.ID, false, true, revision)
}

func (c *Client) deleteConfirmed(ctx context.Context, box domain.BoxID, id domain.ID, folder, wholeBox bool, revision string) (SyncReference, error) {
	scope, view, release, err := c.commandViewLock(ctx)
	if err != nil {
		return SyncReference{}, err
	}
	defer release()
	deletion, err := c.deletionLocked(ctx, view, box, id, folder, wholeBox)
	if err != nil {
		return SyncReference{}, err
	}
	if deletion.Preview.Revision != revision {
		return SyncReference{}, oplog.ErrStaleBefore
	}
	return c.stageCommandLocked(ctx, scope, SyncProposal{Version: 2, ID: domain.NewID().String(), OccurredAt: time.Now().UTC(), Origin: "user", Action: string(deletion.Action), Payload: deletion.Payload, Deletion: &deletion.Basis})
}

// These view timestamps change when a local intent receives a NAS acceptance
// time. The stable last-operation identity still invalidates real later edits.
func clearDeletionViewTimes(folders []domain.Folder, files []domain.ArchivedFile) {
	for i := range folders {
		folders[i].CreatedAt, folders[i].UpdatedAt = time.Time{}, time.Time{}
	}
	for i := range files {
		files[i].File.CreatedAt, files[i].File.UpdatedAt = time.Time{}, time.Time{}
	}
}
