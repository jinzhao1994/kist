package library

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// write is the single write path. Every change to a kist library goes through
// this function; there is no other way in (invariant 2), which is why it is
// unexported.
//
// One transaction, four steps, in this order:
//
//  1. append the op — its id is the library's new version number
//  2. Apply it to the materialized views, with tx.TS() carrying the op's ts
//  3. write the index tables from the payload via Targets (invariant 4)
//  4. propagate updated_at, but only for sources that represent human intent
//
// If any step fails the whole thing rolls back. An op that survived a failed
// Apply would be worse than a lost change: a rebuild would replay it and make
// the claimed change real.
func (l *Library) write(
	ctx context.Context,
	action oplog.Action,
	payload []byte,
	source oplog.Source,
	revertsID *int64,
) (int64, error) {
	// Everything below — the transaction and the incremental projection at
	// the end — is one logical write, and the lock covers both (spec 2.7).
	// Holding it across the projection is the point: two processes renaming
	// directories under boxes/ at once is the silent failure it exists for.
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer release()

	// What boxes/ currently reflects, read under the lock. The incremental
	// projection at the end needs to tell a rename from a first projection,
	// and that answer is a property of the shared library rather than of this
	// process — another process may have renamed a box since we opened
	// (spec 2.7).
	before, _, err := l.boxDirs()
	if err != nil {
		return 0, err
	}

	handler, err := oplog.Lookup(action)
	if err != nil {
		return 0, err
	}
	targets, err := handler.Targets(payload)
	if err != nil {
		return 0, fmt.Errorf("extract targets for %s: %w", action, err)
	}

	ts := l.now().UTC()

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	opID, err := store.AppendOp(tx, store.OpRow{
		TS:        ts,
		Action:    string(action),
		Source:    string(source),
		Payload:   payload,
		RevertsID: revertsID,
	})
	if err != nil {
		return 0, err
	}

	vtx := store.NewViewTx(tx, ts)

	// The folder chain has to be sampled on both sides of Apply.
	//
	// An import's files do not exist before Apply, and a delete's files do not
	// exist after it, so either sample alone misses one direction. Taking the
	// union also gets a future move-file op right for free: the first sample
	// names the old chain, the second the new one, and both need bumping.
	chainBefore, err := vtx.FolderChainOfFiles(targets.Files)
	if err != nil {
		return 0, err
	}

	if err := handler.Apply(vtx, payload); err != nil {
		return 0, fmt.Errorf("apply %s: %w", action, err)
	}

	if err := store.InsertOpIndexes(tx, opID, targets.Boxes, targets.Files, targets.Blobs); err != nil {
		return 0, err
	}

	if err := touchAfter(vtx, action, source, targets, chainBefore); err != nil {
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
	source oplog.Source,
	targets oplog.Targets,
	chainBefore []domain.FolderID,
) error {
	// Two axes, and both are needed: source excludes what a machine did,
	// action excludes what left nothing to see. A future sync op is a person's
	// work from another device (propagates, source != user); a gc that removed
	// files changes content but is housekeeping (does not propagate).
	if !source.PropagatesUpdatedAt() || !action.ChangesContent() {
		return nil
	}
	if err := vtx.TouchBoxes(targets.Boxes); err != nil {
		return err
	}
	chainAfter, err := vtx.FolderChainOfFiles(targets.Files)
	if err != nil {
		return err
	}
	return vtx.TouchFolders(union(chainBefore, chainAfter))
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
