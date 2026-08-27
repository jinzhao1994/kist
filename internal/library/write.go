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
	handler, err := oplog.Lookup(action)
	if err != nil {
		return 0, err
	}
	targets, err := handler.Targets(payload)
	if err != nil {
		return 0, fmt.Errorf("extract targets for %s: %w", action, err)
	}

	// Truncated to milliseconds because that is the resolution the storage
	// format keeps. Without this, what is written and what is read back
	// differ, and every timestamp comparison becomes mysteriously flaky.
	ts := l.now().UTC().Truncate(time.Millisecond)

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	opID, err := store.AppendOp(tx, store.OpRow{
		TS:        ts,
		Action:    string(action),
		Source:    string(source),
		Actor:     l.actor,
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

	if err := store.InsertOpBoxes(tx, opID, targets.Boxes); err != nil {
		return 0, err
	}
	if err := store.InsertOpFiles(tx, opID, targets.Files); err != nil {
		return 0, err
	}

	if err := touchAfter(vtx, source, targets, chainBefore); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit %s: %w", action, err)
	}

	if err := l.refreshTagState(action, payload); err != nil {
		// The database is already committed and correct; only the in-memory
		// mirror is behind. Reloading it is always safe, so recover rather
		// than fail a write that actually succeeded.
		l.log.Error("the in-memory tag tree fell out of step; reloading it",
			"op", opID, "action", action, "err", err)
		tags, rerr := LoadTagTree(l.db)
		if rerr != nil {
			return opID, fmt.Errorf("op %d committed but the tag tree could not be reloaded: %w", opID, rerr)
		}
		l.tags = tags
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
	source oplog.Source,
	targets oplog.Targets,
	chainBefore []domain.FolderID,
) error {
	if !source.PropagatesUpdatedAt() {
		return nil
	}
	if err := vtx.TouchBoxes(targets.Boxes); err != nil {
		return err
	}
	chainAfter, err := vtx.FolderChainOfFiles(targets.Files)
	if err != nil {
		return err
	}
	return vtx.TouchFolders(unionFolders(chainBefore, chainAfter))
}

// unionFolders merges two folder-id lists without duplicates.
func unionFolders(a, b []domain.FolderID) []domain.FolderID {
	if len(a) == 0 {
		return b
	}
	seen := make(map[domain.FolderID]bool, len(a)+len(b))
	out := make([]domain.FolderID, 0, len(a)+len(b))
	for _, list := range [][]domain.FolderID{a, b} {
		for _, id := range list {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

// refreshTagState mirrors a committed op into the in-memory tag tree.
//
// It is driven by the action rather than by the command that produced it, so
// undo gets the same treatment for free: an inverse op is just another action
// arriving at this switch.
func (l *Library) refreshTagState(action oplog.Action, payload []byte) error {
	switch action {
	case oplog.ActionCreateTag:
		p, err := oplog.Payload[oplog.CreateTagPayload](payload)
		if err != nil {
			return err
		}
		tag, err := domain.NewTag(p.TagID, p.ParentID, p.Name)
		if err != nil {
			return err
		}
		return l.tags.Insert(tag)

	case oplog.ActionDeleteTag:
		p, err := oplog.Payload[oplog.DeleteTagPayload](payload)
		if err != nil {
			return err
		}
		return l.tags.Remove(p.TagID)

	case oplog.ActionRenameTag:
		p, err := oplog.Payload[oplog.RenameTagPayload](payload)
		if err != nil {
			return err
		}
		current, ok := l.tags.ByID(p.TagID)
		if !ok {
			return fmt.Errorf("tag %s is not in the tree", p.TagID)
		}
		renamed, err := domain.NewTag(p.TagID, current.ParentID, p.After)
		if err != nil {
			return err
		}
		return l.tags.Update(renamed)

	case oplog.ActionMoveTag:
		p, err := oplog.Payload[oplog.MoveTagPayload](payload)
		if err != nil {
			return err
		}
		current, ok := l.tags.ByID(p.TagID)
		if !ok {
			return fmt.Errorf("tag %s is not in the tree", p.TagID)
		}
		moved, err := domain.NewTag(p.TagID, p.NewParent, current.Name)
		if err != nil {
			return err
		}
		return l.tags.Update(moved)

	case oplog.ActionTagBoxes:
		p, err := oplog.Payload[oplog.TagBoxesPayload](payload)
		if err != nil {
			return err
		}
		l.tags.AddDirectCount(p.TagID, len(p.BoxIDs))

	case oplog.ActionUntagBoxes:
		p, err := oplog.Payload[oplog.UntagBoxesPayload](payload)
		if err != nil {
			return err
		}
		l.tags.AddDirectCount(p.TagID, -len(p.BoxIDs))

	case oplog.ActionTagFiles:
		p, err := oplog.Payload[oplog.TagFilesPayload](payload)
		if err != nil {
			return err
		}
		l.tags.AddDirectCount(p.TagID, len(p.FileIDs))

	case oplog.ActionUntagFiles:
		p, err := oplog.Payload[oplog.UntagFilesPayload](payload)
		if err != nil {
			return err
		}
		l.tags.AddDirectCount(p.TagID, -len(p.FileIDs))
	}
	return nil
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
