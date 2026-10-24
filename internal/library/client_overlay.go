package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

type clientOverlay struct {
	// View IDs are private materialization positions. API callers must use
	// References rather than expose these changing integers as NAS identities.
	References map[int64]SyncReference
	BlockedID  string
	Conflict   error
}

// rebuildClientInto atomically replaces the derived client view. Confirmed
// history is immutable input; pending intents are applied in logical order.
// Blob bytes are intentionally not required to rebuild a metadata catalog.
// The caller holds the client lock and supplies a validated catalog prefix.
func rebuildClientInto(ctx context.Context, db *sql.DB, confirmed *journal.Dir, pending pendingState) (clientOverlay, error) {
	return rebuildClientPrefixInto(ctx, db, confirmed, pending, math.MaxInt64, "")
}

func rebuildClientPrefixInto(ctx context.Context, db *sql.DB, confirmed *journal.Dir, pending pendingState, through int64, sum string) (clientOverlay, error) {
	result := clientOverlay{References: map[int64]SyncReference{}}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	for _, clear := range []func(*sql.Tx) error{store.ClearViews, store.DeleteOpIndexes, store.ClearOps} {
		if err := clear(tx); err != nil {
			return result, err
		}
	}
	// All client blob facts come from history. This removes stale facts left
	// by abandoned local imports without touching any actual content file.
	if _, err := tx.Exec(`DELETE FROM blobs`); err != nil {
		return result, err
	}
	var last int64
	localIDs := map[string]int64{}
	end := errors.New("client catalog boundary reached")
	var lastSum string
	err = confirmed.Iter(func(line journal.Line) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if line.ID > through {
			return end
		}
		if line.ID != last+1 {
			return fmt.Errorf("%w: noncontiguous client catalog", journal.ErrCorrupt)
		}
		row, err := rowFromLine(line)
		if err != nil {
			return err
		}
		if _, err := applyOp(tx, row); err != nil {
			return err
		}
		last = row.ID
		lastSum = line.Sum
		result.References[row.ID] = SyncReference{OpID: row.ID}
		if line.Sync != nil {
			localIDs[line.Sync.ID] = row.ID
		}
		return nil
	})
	if err != nil && !errors.Is(err, end) {
		return result, err
	}
	if through != math.MaxInt64 && (last != through || lastSum != sum) {
		return result, fmt.Errorf("%w: incomplete client catalog prefix", journal.ErrCorrupt)
	}
	entries := slices.Clone(pending.entries)
	slices.SortStableFunc(entries, comparePending)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		p := entry.Proposal
		if id, found := localIDs[p.ID]; found {
			digest, _, at, _, err := store.SyncOp(tx, p.ID)
			if err != nil {
				return result, err
			}
			if digest != entry.Digest || entry.Discarded || entry.ReplacedBy != "" || (entry.Receipt != nil && (entry.Receipt.OpID != id || !entry.Receipt.AcceptedAt.Equal(at))) {
				return result, fmt.Errorf("%w: pending intent contradicts confirmed receipt", ErrInvariantViolated)
			}
			continue // Includes a NAS commit whose response was lost locally.
		}
		if entry.Discarded || entry.ReplacedBy != "" {
			continue
		}
		if entry.Conflict != "" {
			result.BlockedID, result.Conflict = p.ID, ErrSyncConflict
			break
		}
		// Acknowledged but not yet downloaded events remain visible as pending
		// overlays until their authoritative event actually reaches the catalog.
		if _, err := tx.Exec(`SAVEPOINT pending_apply`); err != nil {
			return result, err
		}
		row, applyErr := clientPendingRow(tx, p, last+1, localIDs, result.References)
		if applyErr == nil {
			vtx := store.NewViewTx(tx, row.TS)
			applyErr = validateWriteState(vtx, row)
			if applyErr == nil {
				applyErr = checkSyncFileStates(vtx, oplog.Action(row.Action), row.Payload, p.Files, p.Undo != nil)
			}
		}
		if applyErr == nil {
			_, applyErr = applyOpChecked(tx, row, true)
		}
		if applyErr != nil {
			if _, err := tx.Exec(`ROLLBACK TO pending_apply`); err != nil {
				return result, err
			}
			if !clientRebaseConflict(applyErr) {
				return result, applyErr
			}
			result.BlockedID, result.Conflict = p.ID, applyErr
			break
		}
		if _, err := tx.Exec(`RELEASE pending_apply`); err != nil {
			return result, err
		}
		last = row.ID
		localIDs[p.ID] = row.ID
		result.References[row.ID] = SyncReference{LocalID: p.ID}
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func clientRebaseConflict(err error) bool {
	return errors.Is(err, ErrSyncConflict) || errors.Is(err, ErrSyncDependency) || errors.Is(err, oplog.ErrStaleBefore) || errors.Is(err, oplog.ErrTagInUse) || errors.Is(err, ErrAlreadyReverted) || errors.Is(err, ErrUndoBlocked) || errors.Is(err, ErrTagCycle) || errors.Is(err, ErrNoSuchFile) || store.IsStateConflict(err)
}

func clientPendingRow(tx *sql.Tx, p SyncProposal, id int64, localIDs map[string]int64, references map[int64]SyncReference) (store.OpRow, error) {
	row := store.OpRow{ID: id, TS: p.OccurredAt, Action: p.Action, Origin: p.Origin, Payload: p.Payload}
	resolve := func(ref SyncReference) (int64, error) {
		if ref.LocalID != "" {
			if id, found := localIDs[ref.LocalID]; found {
				return id, nil
			}
		} else if ref.OpID > 0 && references[ref.OpID].OpID == ref.OpID {
			return ref.OpID, nil
		}
		return 0, ErrSyncDependency
	}
	for _, dep := range p.Requires {
		if _, err := resolve(SyncReference{LocalID: dep}); err != nil {
			return row, err
		}
	}
	if p.Group != nil {
		root, err := resolve(*p.Group)
		if err != nil {
			return row, err
		}
		target, _, err := store.GetOp(tx, root)
		if err != nil {
			return row, err
		}
		if target.RootOpID != nil {
			return row, ErrSyncConflict
		}
		row.RootOpID = &root
	}
	if p.Undo != nil {
		targetID, err := resolve(*p.Undo)
		if err != nil {
			return row, err
		}
		target, _, err := store.GetOp(tx, targetID)
		if err != nil {
			return row, err
		}
		h, err := oplog.Lookup(oplog.Action(target.Action))
		if err != nil {
			return row, err
		}
		action, raw, err := h.Inverse(target.Payload)
		if err != nil {
			return row, err
		}
		row.Action, row.Payload, row.RevertsID = string(action), raw, &targetID
	}
	if p.Deletion != nil {
		var err error
		row.Payload, err = rebaseSyncDeletion(store.NewViewTx(tx, row.TS), p.Deletion, oplog.Action(row.Action), row.Payload, references)
		if err != nil {
			return row, err
		}
	}
	return row, nil
}
