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

	// One op at a time. Reading the whole log into a slice would hold every
	// payload in memory at once, and after a hundred thousand imported files
	// that is tens of megabytes for no reason (spec 2.2.1).
	err = store.IterOps(l.db, func(row store.OpRow) error {
		action := oplog.Action(row.Action)
		handler, err := oplog.Lookup(action)
		if err != nil {
			return fmt.Errorf("replaying op %d: %w", row.ID, err)
		}

		targets, err := handler.Targets(row.Payload)
		if err != nil {
			return fmt.Errorf("replaying op %d: %w", row.ID, err)
		}

		vtx := store.NewViewTx(tx, row.TS)
		chainBefore, err := vtx.FolderChainOfFiles(targets.Files)
		if err != nil {
			return err
		}

		if err := handler.Apply(vtx, row.Payload); err != nil {
			return fmt.Errorf("replaying op %d (%s): %w", row.ID, action, err)
		}
		if err := store.InsertOpBoxes(tx, row.ID, targets.Boxes); err != nil {
			return err
		}
		if err := store.InsertOpFiles(tx, row.ID, targets.Files); err != nil {
			return err
		}

		// Same rule as the live path — literally the same function, with this
		// op's own ts. That is why replayed updated_at values come out
		// identical instead of all being the moment of the rebuild
		// (spec 1.5.1 ①).
		if err := touchAfter(vtx, oplog.Source(row.Source), targets, chainBefore); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
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
