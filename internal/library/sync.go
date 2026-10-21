package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

var (
	ErrSyncConflict   = errors.New("offline operation conflicts with the current library")
	ErrSyncDependency = errors.New("offline operation depends on an unaccepted operation")
	ErrSyncVersion    = errors.New("unsupported offline operation version")
)

// syncBlobClaims rejects conflicting declarations within one operation before
// an upsert can hide an earlier claim. It depends only on the event's facts.
type syncBlobClaims struct {
	oplog.Tx
	claims map[string]syncBlobClaim
}

type syncBlobClaim struct {
	digest string
	size   int64
}

func (tx *syncBlobClaims) UpsertBlob(key, digest string, size int64) error {
	if err := tx.record(key, digest, size); err != nil {
		return err
	}
	return tx.Tx.UpsertBlob(key, digest, size)
}

func (tx *syncBlobClaims) record(key, digest string, size int64) error {
	claim := syncBlobClaim{digest, size}
	if previous, ok := tx.claims[key]; ok && previous != claim {
		return fmt.Errorf("%w: inconsistent declarations for blob %s", ErrSyncConflict, key)
	}
	tx.claims[key] = claim
	return nil
}

func (tx *syncBlobClaims) archived(files []domain.ArchivedFile) error {
	for _, f := range files {
		if err := tx.record(f.File.BlobKey, f.SHA256, f.Size); err != nil {
			return err
		}
	}
	return nil
}

func (tx *syncBlobClaims) RestoreBoxContents(s domain.BoxContents) error {
	if err := tx.archived(s.Files); err != nil {
		return err
	}
	return tx.Tx.RestoreBoxContents(s)
}

func (tx *syncBlobClaims) RestoreEntryContents(s domain.EntryContents) error {
	if err := tx.archived(s.Files); err != nil {
		return err
	}
	return tx.Tx.RestoreEntryContents(s)
}

// SyncReference distinguishes authoritative IDs from independent pending IDs.
type SyncReference struct {
	OpID    int64  `json:"op_id,omitempty"`
	LocalID string `json:"local_id,omitempty"`
}

// SyncFileState binds extracted facts to the content that was actually read.
type SyncFileState struct {
	ID      domain.FileID `json:"id"`
	BlobKey string        `json:"blob_key"`
}

// SyncProposal is an immutable offline intent, not a copied NAS log line.
// Undo carries only a reference: the NAS computes the exact authoritative inverse.
type SyncProposal struct {
	Version    int             `json:"version"`
	ID         string          `json:"id"`
	OccurredAt time.Time       `json:"occurred_at"`
	Action     string          `json:"action,omitempty"`
	Origin     string          `json:"origin"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	Undo       *SyncReference  `json:"undo,omitempty"`
	Group      *SyncReference  `json:"group,omitempty"`
	Requires   []string        `json:"requires,omitempty"`
	Files      []SyncFileState `json:"files,omitempty"`
}

type SyncResult struct {
	OpID       int64     `json:"op_id"`
	AcceptedAt time.Time `json:"accepted_at"`
}

// syncActions explicitly gates the protocol. A registry comparison test makes
// new actions require an intentional protocol decision before they can sync.
var syncActions = []oplog.Action{
	oplog.ActionCreateBox, oplog.ActionDeleteBox, oplog.ActionUpdateBox,
	oplog.ActionDeleteBoxContents, oplog.ActionRestoreBoxContents,
	oplog.ActionCreateFolder, oplog.ActionDeleteFolder, oplog.ActionRenameEntry,
	oplog.ActionDeleteEntryContents, oplog.ActionRestoreEntryContents,
	oplog.ActionCreateTag, oplog.ActionDeleteTag, oplog.ActionRenameTag, oplog.ActionMoveTag,
	oplog.ActionTagBoxes, oplog.ActionUntagBoxes, oplog.ActionTagFiles, oplog.ActionUntagFiles,
	oplog.ActionImportFiles, oplog.ActionDeleteFiles, oplog.ActionUploadFiles, oplog.ActionUndoUploadFiles,
	oplog.ActionExtractMetadata, oplog.ActionSetMetadata,
	oplog.ActionCreateSavedSearch, oplog.ActionUpdateSavedSearch, oplog.ActionDeleteSavedSearch,
	oplog.ActionSetFileset, oplog.ActionDeleteFilesetEntry, oplog.ActionRestoreFilesetEntry,
}

// SyncActions returns the currently supported domain operations, sorted for
// deterministic capability negotiation. Request receipts are protocol-internal.
func SyncActions() []string {
	out := make([]string, len(syncActions))
	for i, a := range syncActions {
		out[i] = string(a)
	}
	slices.Sort(out)
	return out
}

func (p SyncProposal) digest() (string, error) {
	if p.Version != 1 {
		return "", ErrSyncVersion
	}
	if p.OccurredAt.IsZero() || !time.Unix(0, p.OccurredAt.UnixNano()).Equal(p.OccurredAt) {
		return "", ErrInvalidRequest
	}
	if p.Origin != string(oplog.OriginUser) && p.Origin != string(oplog.OriginExtractor) {
		return "", ErrInvalidRequest
	}
	if len(p.Payload) > 16<<20 || len(p.Requires) > 10000 || len(p.Files) > 10000 {
		return "", ErrInvalidRequest
	}
	if p.Undo != nil {
		if p.Action != "" || len(p.Payload) != 0 || p.Origin != string(oplog.OriginUser) {
			return "", ErrInvalidRequest
		}
	} else if !slices.Contains(syncActions, oplog.Action(p.Action)) || !json.Valid(p.Payload) {
		return "", ErrSyncVersion
	}
	if p.Origin == string(oplog.OriginExtractor) && p.Action != string(oplog.ActionExtractMetadata) {
		return "", ErrInvalidRequest
	}
	// A canonical UTC timestamp prevents local zone formatting from changing identity.
	p.OccurredAt = p.OccurredAt.UTC()
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	digest := hex.EncodeToString(sum[:])
	if !(journal.Request{ID: p.ID, Digest: digest}).Valid() {
		return "", ErrInvalidRequest
	}
	return digest, nil
}

// AcceptSync serializes receipt lookup, precondition checks and the durable
// write with all NAS writers. No network handler may bypass this path.
func (l *Library) AcceptSync(ctx context.Context, p SyncProposal) (SyncResult, error) {
	digest, err := p.digest()
	if err != nil {
		return SyncResult{}, err
	}
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return SyncResult{}, err
	}
	defer release()
	if err = l.catchUpLocked(ctx); err != nil {
		return SyncResult{}, err
	}
	old, id, at, found, err := store.SyncOp(l.db, p.ID)
	if err != nil {
		return SyncResult{}, err
	}
	if found {
		if old != digest {
			return SyncResult{}, ErrRequestConflict
		}
		return SyncResult{OpID: id, AcceptedAt: at}, nil
	}
	for _, dep := range p.Requires {
		if _, err := l.syncReference(SyncReference{LocalID: dep}); err != nil {
			return SyncResult{}, err
		}
	}
	action, raw := oplog.Action(p.Action), []byte(p.Payload)
	opt := writeOptions{sync: &syncWrite{ID: p.ID, Digest: digest, OccurredAt: p.OccurredAt}}
	if p.Group != nil {
		id, err := l.syncReference(*p.Group)
		if err != nil {
			return SyncResult{}, err
		}
		row, _, err := store.GetOp(l.db, id)
		if err != nil {
			return SyncResult{}, err
		}
		if row.RootOpID != nil {
			return SyncResult{}, ErrInvalidRequest
		}
		opt.RootOpID = &id
	}
	if p.Undo != nil {
		id, err := l.syncReference(*p.Undo)
		if err != nil {
			return SyncResult{}, err
		}
		row, _, err := store.GetOp(l.db, id)
		if err != nil {
			return SyncResult{}, err
		}
		h, err := oplog.Lookup(oplog.Action(row.Action))
		if err != nil {
			return SyncResult{}, err
		}
		action, raw, err = h.Inverse(row.Payload)
		if err != nil {
			return SyncResult{}, err
		}
		if !slices.Contains(syncActions, action) {
			return SyncResult{}, ErrSyncVersion
		}
		opt.RevertsID = &id
	}
	if err = l.checkSyncFileStates(ctx, action, raw, p.Files, p.Undo != nil); err != nil {
		return SyncResult{}, err
	}
	id, err = l.writeLocked(ctx, action, raw, oplog.Origin(p.Origin), opt)
	if err != nil {
		var outcome *WriteOutcomeError
		if errors.As(err, &outcome) {
			return SyncResult{OpID: id}, err
		}
		if errors.Is(err, oplog.ErrStaleBefore) || store.IsStateConflict(err) || errors.Is(err, ErrAlreadyReverted) {
			return SyncResult{}, fmt.Errorf("%w: %w", ErrSyncConflict, err)
		}
		return SyncResult{}, err
	}
	_, _, at, _, err = store.SyncOp(l.db, p.ID)
	return SyncResult{OpID: id, AcceptedAt: at}, err
}

func (l *Library) syncReference(ref SyncReference) (int64, error) {
	if (ref.OpID > 0) == (ref.LocalID != "") || ref.OpID < 0 {
		return 0, ErrInvalidRequest
	}
	if ref.LocalID != "" {
		_, id, _, found, err := store.SyncOp(l.db, ref.LocalID)
		if err != nil {
			return 0, err
		}
		if !found {
			return 0, ErrSyncDependency
		}
		return id, nil
	}
	_, found, err := store.GetOp(l.db, ref.OpID)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, ErrSyncDependency
	}
	return ref.OpID, nil
}

func (l *Library) checkSyncFileStates(ctx context.Context, action oplog.Action, raw []byte, states []SyncFileState, undo bool) error {
	return l.readTx(ctx, func(tx *store.ViewTx) error { return checkSyncFileStates(tx, action, raw, states, undo) })
}

func checkSyncFileStates(tx *store.ViewTx, action oplog.Action, raw []byte, states []SyncFileState, undo bool) error {
	seen := map[domain.FileID]bool{}
	for _, s := range states {
		if seen[s.ID] {
			return ErrInvalidRequest
		}
		seen[s.ID] = true
		f, found, err := tx.GetFile(s.ID)
		if err != nil {
			return err
		}
		if !found || f.BlobKey != s.BlobKey {
			return fmt.Errorf("%w: file content changed", ErrSyncConflict)
		}
	}
	if action == oplog.ActionExtractMetadata && !undo {
		h, _ := oplog.Lookup(action)
		targets, err := h.Targets(raw)
		if err != nil {
			return err
		}
		for _, id := range targets.Files {
			if !seen[id] {
				return fmt.Errorf("%w: extraction requires original content identity", ErrInvalidRequest)
			}
		}
	}
	return nil
}

// SyncStatus resolves durable acknowledgement after an interrupted request.
func (l *Library) SyncStatus(ctx context.Context, id string) (SyncResult, bool, error) {
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return SyncResult{}, false, err
	}
	defer release()
	if err := l.catchUpLocked(ctx); err != nil {
		return SyncResult{}, false, err
	}
	_, op, at, found, err := store.SyncOp(l.db, id)
	return SyncResult{OpID: op, AcceptedAt: at}, found, err
}

// checkSyncBlobs validates the exact materialized blob facts before append.
// It runs while the write/placement lock is held so a repair cannot swap bytes.
func (l *Library) checkSyncBlobs(ctx context.Context, tx *sql.Tx, targets oplog.Targets) error {
	seen := map[string]bool{}
	for _, name := range targets.Blobs {
		if seen[name] {
			continue
		}
		seen[name] = true
		key, err := blob.ParseKey(name)
		if err != nil {
			return err
		}
		var size int64
		var digest string
		if err := tx.QueryRow(`SELECT size,sha256 FROM blobs WHERE key=?`, name).Scan(&size, &digest); err != nil {
			return err
		}
		r, err := l.blobs.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("sync requires complete blob %s: %w", name, err)
		}
		hash := sha256.New()
		buf := make([]byte, 128*1024)
		var read int64
		for {
			if err = ctx.Err(); err != nil {
				break
			}
			var n int
			n, err = r.Read(buf)
			if n > 0 {
				hash.Write(buf[:n])
				read += int64(n)
			}
			if err != nil {
				break
			}
		}
		closeErr := r.Close()
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if read != size || hex.EncodeToString(hash.Sum(nil)) != key.Hex() || digest != key.Hex() {
			return fmt.Errorf("%w: blob %s does not match its declared content", ErrSyncConflict, name)
		}
	}
	return nil
}
