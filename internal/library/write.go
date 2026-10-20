package library

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// writeOptions are the two things that describe how an op came about rather
// than what it does. Both are columns on ops and neither reaches Apply, so
// neither can change what a rebuild produces.
//
// A struct rather than two more parameters: the pair grows — a third axis is
// already imaginable — and a call site reading `..., nil, nil)` says nothing
// about which nil is which.
type writeOptions struct {
	// RevertsID names the op this one undoes.
	RevertsID *int64
	// RootOpID is the first op of the group this one joins. The first op of a
	// group carries nil — ops.id comes from AUTOINCREMENT, so it is not known
	// until the row is in, and pointing it at itself afterwards would mean
	// updating an op.
	RootOpID *int64
	request  *Request
}

// write is the single write path. Every change to a kist library goes through
// this function; there is no other way in (invariant 2), which is why it is
// unexported.
//
// Two phases, and the order between them is the invariant (spec 2.2.2):
//
//  1. append the line to ops/ and fsync — the op is now durable, and its id
//     is the library's new version number
//  2. in one transaction: insert the ops row, Apply to the materialized
//     views, write the index tables, propagate updated_at
//
// Phase 2 rolling back leaves an op in the log that the database has not
// applied, which is the recoverable direction: the next Open replays it. The
// reverse — applying before the log is durable — would leave the views
// claiming a change no log entry justifies, and nothing could put that right.
//
// Everything that can fail without writing anything is checked before phase 1,
// so a rejected action never gets a line in the log.
func (l *Library) write(
	ctx context.Context,
	action oplog.Action,
	payload []byte,
	origin oplog.Origin,
	opt writeOptions,
) (resultID int64, writeErr error) {
	if scope, ok := ctx.Value(requestContextKey{}).(*requestScope); ok {
		scope.mu.Lock()
		defer scope.mu.Unlock()
		if !scope.active || scope.library != l || scope.attempted {
			return 0, ErrRequestScope
		}
		scope.attempted = true
		opt.request = &scope.request
		id, err := l.writeLocked(ctx, action, payload, origin, opt)
		scope.opID, scope.err = id, err
		return id, err
	}
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer release()
	return l.writeLocked(ctx, action, payload, origin, opt)
}

// writeLocked includes validation, append, apply and projection under one lock.
func (l *Library) writeLocked(ctx context.Context, action oplog.Action, payload []byte, origin oplog.Origin, opt writeOptions) (resultID int64, writeErr error) {

	// What boxes/ currently reflects, read under the lock. The incremental
	// projection at the end needs to tell a rename from a first projection,
	// and that answer is a property of the shared library rather than of this
	// process — another process may have renamed a box since we opened
	// (spec 2.7).
	before, _, err := l.boxDirs()
	if err != nil {
		return 0, err
	}

	if _, err := oplog.Lookup(action); err != nil {
		return 0, err
	}
	if !origin.Valid() {
		return 0, fmt.Errorf("op origin %q is not one of %v", origin, oplog.AllOrigins())
	}

	// Another process may have crashed between its append and its apply since
	// this one opened. Catching up here as well as in Open is what keeps the
	// next id correct: taking it from the log while the database is behind
	// would skip the op nobody applied.
	if err := l.catchUpLocked(ctx); err != nil {
		return 0, err
	}

	if action == oplog.ActionUpdateBox && opt.RevertsID == nil {
		p, err := oplog.Payload[oplog.UpdateBoxPayload](payload)
		if err != nil {
			return 0, err
		}
		if p.CoverFileID != nil {
			if err := l.checkBoxCover(ctx, p.BoxID, p.CoverFileID.After); err != nil {
				return 0, err
			}
		}
	}

	// Recheck state-dependent constraints after recovery, while no writer can
	// invalidate them. This also covers inverse ops, which bypass commands.
	if opt.RevertsID != nil {
		if by, done, err := store.RevertedBy(l.db, *opt.RevertsID); err != nil {
			return 0, err
		} else if done {
			return 0, &AlreadyRevertedError{OpID: *opt.RevertsID, RevertedBy: by}
		}
		if action == oplog.ActionUndoUploadFiles {
			p, err := oplog.Payload[oplog.UploadFilesPayload](payload)
			if err != nil {
				return 0, err
			}
			raw, err := oplog.Marshal(p.Import)
			if err != nil {
				return 0, err
			}
			if err := l.checkImportUndoMetadata(ctx, raw, *opt.RevertsID); err != nil {
				return 0, err
			}
		}
		if action == oplog.ActionDeleteFiles {
			if err := l.checkImportUndoMetadata(ctx, payload, *opt.RevertsID); err != nil {
				return 0, err
			}
		}
	}
	if action == oplog.ActionMoveTag {
		p, err := oplog.Payload[oplog.MoveTagPayload](payload)
		if err != nil {
			return 0, err
		}
		if p.NewParent != nil {
			above, err := store.TagAncestors(l.db, *p.NewParent)
			if err != nil {
				return 0, err
			}
			if slices.Contains(above, p.TagID) {
				return 0, ErrTagCycle
			}
		}
	}

	last, ok, err := l.ops.Last()
	if err != nil {
		return 0, err
	}
	var opID int64 = 1
	if ok {
		opID = last.ID + 1
	}

	row := store.OpRow{
		ID:        opID,
		TS:        l.now().UTC(),
		Action:    string(action),
		Origin:    string(origin),
		Payload:   payload,
		RevertsID: opt.RevertsID,
		RootOpID:  opt.RootOpID,
	}

	if opt.request != nil {
		row.RequestID = opt.request.ID
		row.RequestDigest = opt.request.Digest
	}

	// Validate the exact replay path before making the event authoritative.
	// The lock stays held across rollback, append and apply, so another
	// library writer cannot invalidate the checked state in between.
	check, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin validation: %w", err)
	}
	if _, err := applyOp(check, row); err != nil {
		check.Rollback()
		return 0, err
	}
	if err := check.Rollback(); err != nil {
		return 0, fmt.Errorf("rollback validation: %w", err)
	}

	// Once Append starts, an error is not proof that nothing was written.
	// In particular, fsync can fail after a complete line reached the file.
	durable := false
	defer func() {
		if writeErr != nil {
			outcome := &WriteOutcomeError{Cause: writeErr}
			if durable {
				outcome.OpID = opID
				resultID = opID
			}
			writeErr = outcome
		}
	}()
	// Phase 1. After this returns the op exists, whatever happens next.
	if err := l.ops.Append(lineFromRow(row)); err != nil {
		return 0, err
	}
	durable = true
	if l.hooks.afterAppend != nil {
		if err := l.hooks.afterAppend(); err != nil {
			return 0, err
		}
	}

	// Phase 2.
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	targets, err := applyOp(tx, row)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit %s: %w", action, err)
	}

	// The transaction is committed and the library is correct; only the
	// human-readable mirror can still be behind, and relink fixes that
	// (spec 3.4.1). Failing the caller here would tell them their change did
	// not happen when it did, and the next thing they would do is repeat it.
	if err := l.project(ctx, targets.Boxes, before); err != nil {
		l.log.Error("boxes/ could not be updated; run `kist relink`",
			"op", opID, "action", action, "err", err)
		l.markDirty(err)
	}
	return opID, nil
}

// touchAfter propagates updated_at once an op has been applied.
//
// spec 1.5.1 ②: updated_at records human intent, not system activity. The rule
// lives here rather than in each handler so that a new op type cannot forget
// it, and it is shared by the live write path and Rebuild so that a replayed
// library carries the same values as the original — which the byte-for-byte
// snapshot test would otherwise catch immediately.
func touchAfter(
	vtx *store.ViewTx,
	action oplog.Action,
	origin oplog.Origin,
	targets oplog.Targets,
	chainBefore []domain.FolderID,
) error {
	// Two axes, and both are needed: origin excludes what a machine did,
	// action excludes what left nothing to see. A future sync op is a person's
	// work from another device (propagates, origin != user); a gc that removed
	// files changes content but is housekeeping (does not propagate).
	if !origin.PropagatesUpdatedAt() || !action.ChangesContent() {
		return nil
	}
	if err := vtx.TouchBoxes(targets.Boxes); err != nil {
		return err
	}
	chainAfter, err := vtx.FolderChainOfFiles(targets.Files)
	if err != nil {
		return err
	}
	return vtx.TouchFolders(union(union(chainBefore, chainAfter), targets.FolderAncestors))
}

// union merges two id lists without duplicates.
func union[T comparable](a, b []T) []T {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	seen := make(map[T]bool, len(a)+len(b))
	out := make([]T, 0, len(a)+len(b))
	for _, list := range [][]T{a, b} {
		for _, id := range list {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

// readTx runs fn against a read-only transaction.
func (l *Library) readTx(ctx context.Context, fn func(*store.ViewTx) error) error {
	tx, err := l.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return fn(store.NewViewTx(tx, time.Time{}))
}
