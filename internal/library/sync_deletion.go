package library

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// SyncDeletionBasis binds timestamp rebasing to the last operation touching the
// entire box. A matching snapshot alone cannot detect an intervening ABA edit.
type SyncDeletionBasis struct {
	Box  domain.BoxID `json:"box"`
	Last string       `json:"last"`
}

func syncOperationIdentity(q store.Queryer, id int64, refs map[int64]SyncReference) (string, error) {
	if ref, ok := refs[id]; ok && ref.LocalID != "" {
		return "local:" + ref.LocalID, nil
	}
	local, err := store.OpSyncIdentity(q, id)
	if err != nil {
		return "", err
	}
	if local != "" {
		return "local:" + local, nil
	}
	return fmt.Sprintf("nas:%d", id), nil
}

func deletionSemanticPayload(action oplog.Action, raw []byte) ([]byte, error) {
	switch action {
	case oplog.ActionDeleteBoxContents:
		p, err := oplog.Payload[domain.BoxContents](raw)
		if err != nil {
			return nil, err
		}
		p.Box.CreatedAt, p.Box.UpdatedAt = time.Time{}, time.Time{}
		clearDeletionViewTimes(p.Folders, p.Files)
		return oplog.Marshal(p)
	case oplog.ActionDeleteEntryContents:
		p, err := oplog.Payload[domain.EntryContents](raw)
		if err != nil {
			return nil, err
		}
		clearDeletionViewTimes(p.Folders, p.Files)
		return oplog.Marshal(p)
	case oplog.ActionDeleteFilesetEntry:
		p, err := oplog.Payload[oplog.FilesetEntryPayload](raw)
		if err != nil {
			return nil, err
		}
		p.Entry, err = deletionSemanticPayload(oplog.ActionDeleteEntryContents, p.Entry)
		if err != nil {
			return nil, err
		}
		return oplog.Marshal(p)
	default:
		return nil, ErrInvalidRequest
	}
}

// rebaseSyncDeletion changes only materialization times after proving both the
// stable operation frontier and all semantic snapshot facts still match.
// It runs before Apply; the generated authoritative payload stays self-contained.
func rebaseSyncDeletion(tx *store.ViewTx, basis *SyncDeletionBasis, action oplog.Action, raw []byte, refs map[int64]SyncReference) ([]byte, error) {
	if basis == nil {
		return raw, nil
	}
	last, found, err := store.LastOpTouchingBox(tx.Queryer(), basis.Box, 0)
	if err != nil {
		return nil, err
	}
	identity, err := syncOperationIdentity(tx.Queryer(), last, refs)
	if err != nil {
		return nil, err
	}
	if !found || identity != basis.Last {
		return nil, fmt.Errorf("%w: box changed since deletion confirmation", ErrSyncConflict)
	}
	var current json.RawMessage
	if action == oplog.ActionDeleteBoxContents {
		original, err := oplog.Payload[domain.BoxContents](raw)
		if err != nil {
			return nil, err
		}
		if original.Box.ID != basis.Box {
			return nil, ErrInvalidRequest
		}
		snapshot, found, err := tx.BoxContents(basis.Box)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, ErrSyncConflict
		}
		current, err = oplog.Marshal(snapshot)
		if err != nil {
			return nil, err
		}
	} else {
		entryRaw := raw
		if action == oplog.ActionDeleteFilesetEntry {
			p, err := oplog.Payload[oplog.FilesetEntryPayload](raw)
			if err != nil {
				return nil, err
			}
			entryRaw = p.Entry
		} else if action != oplog.ActionDeleteEntryContents {
			return nil, ErrInvalidRequest
		}
		original, err := oplog.Payload[domain.EntryContents](entryRaw)
		if err != nil {
			return nil, err
		}
		if original.BoxID != basis.Box {
			return nil, ErrInvalidRequest
		}
		snapshot, found, err := tx.EntryContents(basis.Box, original.ID, original.Folder)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, ErrSyncConflict
		}
		changes, err := entryFilesetChanges(tx, snapshot)
		if err != nil {
			return nil, err
		}
		current, err = oplog.Marshal(snapshot)
		if err != nil {
			return nil, err
		}
		if action == oplog.ActionDeleteFilesetEntry {
			current, err = oplog.Marshal(oplog.FilesetEntryPayload{Entry: current, Changes: changes})
			if err != nil {
				return nil, err
			}
		} else if len(changes) != 0 {
			return nil, ErrSyncConflict
		}
	}
	before, err := deletionSemanticPayload(action, raw)
	if err != nil {
		return nil, err
	}
	after, err := deletionSemanticPayload(action, current)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(before, after) {
		return nil, fmt.Errorf("%w: deletion contents changed", ErrSyncConflict)
	}
	return current, nil
}
