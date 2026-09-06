package library

import (
	"context"
	"errors"
	"fmt"

	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/store"
)

// ErrNoJournal reports a database that has history in it while ops/ has none.
//
// Two things reach this state and both need a person: a library created before
// the log moved out of the database (a pre-release format, spec 2.1.1), and
// one whose ops/ was deleted. There is no automatic answer to either, because
// the authority is the missing half.
var ErrNoJournal = errors.New("the op log is missing from ops/")

// catchUpLocked brings the database level with the journal. The caller holds
// the write lock.
//
// A crash between appending a line and applying it leaves the database one op
// behind (spec 2.2.2). This is the repair, and it is a replay rather than
// anything clever: Apply is a pure function of the payload, so re-running the
// tail produces exactly what the interrupted write would have.
//
// The comparison itself is two integers, so the common case — nothing to do —
// costs one seek to the end of the newest bucket and one MAX(id). Only a
// library that is actually behind pays for a scan.
func (l *Library) catchUpLocked(ctx context.Context) error {
	if _, err := l.ops.RecoverTail(); err != nil {
		return err
	}
	last, ok, err := l.ops.Last()
	if err != nil {
		return err
	}
	var logID int64
	if ok {
		logID = last.ID
	}
	dbID, err := store.MaxOpID(l.db)
	if err != nil {
		return err
	}

	switch {
	case dbID == logID:
		return nil
	case dbID > logID && !ok:
		return fmt.Errorf("%w: the database holds %d operation(s) but ops/ is empty; "+
			"this library predates the move of the log out of kist.db, or its ops/ was deleted",
			ErrNoJournal, dbID)
	case dbID > logID:
		// The database is ahead of the log. Nothing in the write path can
		// produce this — the line is flushed before anything applies it — so
		// it means the log lost entries that the views still remember.
		return fmt.Errorf("%w: the database is at op %d but ops/ ends at %d",
			ErrInvariantViolated, dbID, logID)
	}

	l.log.Info("the database is behind the op log; replaying the tail",
		"database", dbID, "log", logID)

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin catch-up: %w", err)
	}
	defer tx.Rollback()

	err = l.ops.Iter(func(line journal.Line) error {
		if line.ID <= dbID {
			return nil
		}
		row, err := rowFromLine(line)
		if err != nil {
			return err
		}
		_, err = applyOp(tx, row)
		return err
	})
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit catch-up: %w", err)
	}
	return nil
}

// catchUp takes the lock and catches up. Open calls it so that every command,
// including the read-only ones, sees a database that reflects the whole log
// (spec 2.2.2).
func (l *Library) catchUp(ctx context.Context) error {
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	return l.catchUpLocked(ctx)
}
