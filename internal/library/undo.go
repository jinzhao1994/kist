package library

import (
	"context"
	"errors"
	"fmt"

	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

var (
	// ErrAlreadyReverted means some other op has already undone this one.
	ErrAlreadyReverted = errors.New("this op has already been undone")
	// ErrUndoBlocked means later ops changed what this undo wanted to restore.
	ErrUndoBlocked = errors.New("later changes are in the way of this undo")
)

// Undo appends the inverse of an op.
//
// It is not a rollback (spec 2.4): op 47 stays in the log, op 48 says "this
// undoes 47", and op 48 can itself be undone by op 49. The history only ever
// grows, which is what makes it trustworthy.
//
// Strict mode (spec 2.5): if the state the inverse expects to find has since
// been changed, the undo is refused rather than partially applied. Half a
// restored archive that looks whole is the failure worth preventing.
func (l *Library) Undo(ctx context.Context, opID int64) (int64, error) {
	row, ok, err := store.GetOp(l.db, opID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("op %d does not exist", opID)
	}
	target := toOp(row)

	// reverts_id is an indexed column precisely so this stays cheap: the
	// history panel asks it for every row it renders (spec 2.2).
	if by, reverted, err := store.RevertedBy(l.db, opID); err != nil {
		return 0, err
	} else if reverted {
		return 0, fmt.Errorf("op %d was already undone by op %d: %w", opID, by, ErrAlreadyReverted)
	}

	handler, err := oplog.Lookup(target.Action)
	if err != nil {
		return 0, err
	}
	action, payload, err := handler.Inverse(target.Payload)
	if err != nil {
		return 0, fmt.Errorf("compute the inverse of op %d: %w", opID, err)
	}

	undoID, err := l.write(ctx, action, payload, oplog.SourceUser, &opID)
	if err != nil {
		if errors.Is(err, oplog.ErrStaleBefore) {
			return 0, l.explainBlockedUndo(target, err)
		}
		return 0, err
	}
	return undoID, nil
}

// explainBlockedUndo turns "the state does not match" into "op N is in the
// way".
//
// Without this the user gets a true but useless sentence. With it they can go
// look at op N, undo that first, or decide the change was wanted after all —
// which is the difference between an error message and a dead end (spec 2.5).
func (l *Library) explainBlockedUndo(target oplog.Op, cause error) error {
	handler, err := oplog.Lookup(target.Action)
	if err != nil {
		return cause
	}
	targets, err := handler.Targets(target.Payload)
	if err != nil {
		return cause
	}

	for _, box := range targets.Boxes {
		blocker, ok, err := store.LastOpTouchingBox(l.db, box, target.ID)
		if err != nil || !ok {
			continue
		}
		return fmt.Errorf(
			"cannot undo op %d: op %d changed the same box afterwards; undo that first if you want this one back: %w",
			target.ID, blocker, ErrUndoBlocked)
	}
	return fmt.Errorf("cannot undo op %d: the library has changed since it ran: %w", target.ID, ErrUndoBlocked)
}
