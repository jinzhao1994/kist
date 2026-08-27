package library

import (
	"context"
	"fmt"

	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// Rebuild discards every materialized view and replays the log.
//
// This is what invariant 1 buys: ops and blobs/ are the authority, so a
// corrupted view is an inconvenience rather than data loss (spec 2.1). It is
// also the assertion that keeps invariant 3 honest — if any Apply generated an
// id, read a clock, or looked at the current state to decide something, the
// rebuilt library differs from the original and the test in this package says
// so.
//
// ops and blobs are not touched. The whole point is that they are the input.
func (l *Library) Rebuild(ctx context.Context) error {
	ops, err := store.ListOps(l.db, store.ListOpsOptions{})
	if err != nil {
		return err
	}

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin rebuild: %w", err)
	}
	defer tx.Rollback()

	if err := store.ClearViews(tx); err != nil {
		return err
	}
	if err := store.DeleteOpIndexes(tx); err != nil {
		return err
	}

	// ListOps returns newest first; replay wants oldest first.
	for i := len(ops) - 1; i >= 0; i-- {
		row := ops[i]
		action := oplog.Action(row.Action)
		handler, err := oplog.Lookup(action)
		if err != nil {
			return fmt.Errorf("replaying op %d: %w", row.ID, err)
		}

		vtx := store.NewViewTx(tx, row.TS)
		if err := handler.Apply(vtx, row.Payload); err != nil {
			return fmt.Errorf("replaying op %d (%s): %w", row.ID, action, err)
		}

		targets, err := handler.Targets(row.Payload)
		if err != nil {
			return fmt.Errorf("replaying op %d: %w", row.ID, err)
		}
		if err := store.InsertOpBoxes(tx, row.ID, targets.Boxes); err != nil {
			return err
		}
		if err := store.InsertOpFiles(tx, row.ID, targets.Files); err != nil {
			return err
		}

		// Same rule as the live path, with this op's own ts — which is why the
		// replayed updated_at values come out identical instead of all being
		// the moment of the rebuild (spec 1.5.1 ①).
		if oplog.Source(row.Source).PropagatesUpdatedAt() {
			if err := vtx.TouchBoxes(targets.Boxes); err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rebuild: %w", err)
	}

	tags, err := LoadTagTree(l.db)
	if err != nil {
		return fmt.Errorf("reload the tag tree after rebuild: %w", err)
	}
	l.tags = tags
	return nil
}
