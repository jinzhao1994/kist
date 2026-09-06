package library

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jinzhao1994/kist/internal/journal"
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
	// A rebuild truncates every materialized view before replaying, so a write
	// landing in the middle would be applied to tables that are about to be
	// discarded. It takes the same lock as an ordinary write (spec 2.7).
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()

	if err := rebuildInto(ctx, l.db, l.ops); err != nil {
		return err
	}
	return nil
}

// rebuildInto discards db's materialized views and replays ops into them.
//
// The log comes from the journal, not from db: since the authority moved to
// ops/ the ops table is a view like any other, so a rebuild refills it too
// (invariant 1). That is also what turned verify --deep into a real
// cross-check — its scratch database is built from the files, so a corrupted
// ops table now shows up as drift instead of being replayed into itself.
//
// It is a function rather than a method because verify --deep runs it against
// a database that is deliberately not this Library's: the point there is to
// produce a second, independent answer and compare the two.
func rebuildInto(ctx context.Context, db *sql.DB, ops *journal.Dir) error {
	tx, err := db.BeginTx(ctx, nil)
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
	// After the index tables, which reference it.
	if err := store.ClearOps(tx); err != nil {
		return err
	}

	// One op at a time. Reading the whole log into a slice would hold every
	// payload in memory at once, and after a hundred thousand imported files
	// that is tens of megabytes for no reason (spec 2.2.1).
	err = ops.Iter(func(line journal.Line) error {
		row, err := rowFromLine(line)
		if err != nil {
			return err
		}
		if _, err := applyOp(tx, row); err != nil {
			return fmt.Errorf("replaying %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rebuild: %w", err)
	}
	return nil
}

// OpCount is how many operations a rebuild would replay.
//
// It exists for the CLI, which has to tell the user how much work it is about
// to do and cannot reach store itself (invariant 2).
func (l *Library) OpCount(ctx context.Context) (int64, error) {
	var n int64
	if err := l.db.QueryRowContext(ctx, `SELECT count(*) FROM ops`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count operations: %w", err)
	}
	return n, nil
}
