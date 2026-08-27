package oplog

import (
	"errors"
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
)

// ErrTagInUse means a tag cannot be deleted because something still hangs off
// it. This is a refusal by policy (spec 1.6), not a stale-state conflict, and
// the user needs a different message: cascade delete in archiving software can
// wipe a whole classification with one misclick.
var ErrTagInUse = errors.New("tag is still in use")

func init() {
	Register(ActionCreateTag, createTagOp{})
	Register(ActionDeleteTag, deleteTagOp{})
	Register(ActionRenameTag, renameTagOp{})
	Register(ActionMoveTag, moveTagOp{})
}

// CreateTagPayload carries the id minted by library (invariant 3).
type CreateTagPayload struct {
	TagID    domain.TagID  `json:"tag_id"`
	ParentID *domain.TagID `json:"parent_id,omitempty"`
	Name     string        `json:"name"`
}

// DeleteTagPayload is the same shape, so undoing a delete rebuilds the node
// exactly where it was.
type DeleteTagPayload = CreateTagPayload

type createTagOp struct{}

func (createTagOp) Apply(tx Tx, payload []byte) error {
	p, err := Payload[CreateTagPayload](payload)
	if err != nil {
		return err
	}
	tag, err := domain.NewTag(p.TagID, p.ParentID, p.Name)
	if err != nil {
		return err
	}
	return tx.InsertTag(tag)
}

func (createTagOp) Inverse(payload []byte) (Action, []byte, error) {
	return ActionDeleteTag, payload, nil
}

func (createTagOp) Targets([]byte) (Targets, error) { return Targets{}, nil }

type deleteTagOp struct{}

func (deleteTagOp) Apply(tx Tx, payload []byte) error {
	p, err := Payload[DeleteTagPayload](payload)
	if err != nil {
		return err
	}
	current, ok, err := tx.GetTag(p.TagID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("tag %s is already gone: %w", p.TagID, ErrStaleBefore)
	}
	if current.Name != p.Name {
		return fmt.Errorf("tag %s is now named %q but this op expected %q: %w",
			p.TagID, current.Name, p.Name, ErrStaleBefore)
	}

	children, err := tx.CountTagChildren(p.TagID)
	if err != nil {
		return err
	}
	if children > 0 {
		return fmt.Errorf("tag %q has %d child tag(s); move or delete them first: %w",
			p.Name, children, ErrTagInUse)
	}
	usage, err := tx.CountTagUsage(p.TagID)
	if err != nil {
		return err
	}
	if usage > 0 {
		return fmt.Errorf("tag %q is attached to %d item(s); remove it from them first: %w",
			p.Name, usage, ErrTagInUse)
	}
	return tx.DeleteTag(p.TagID)
}

func (deleteTagOp) Inverse(payload []byte) (Action, []byte, error) {
	return ActionCreateTag, payload, nil
}

func (deleteTagOp) Targets([]byte) (Targets, error) { return Targets{}, nil }

// RenameTagPayload records both names so Inverse is a swap.
type RenameTagPayload struct {
	TagID  domain.TagID `json:"tag_id"`
	Before string       `json:"before"`
	After  string       `json:"after"`
}

type renameTagOp struct{}

func (renameTagOp) Apply(tx Tx, payload []byte) error {
	p, err := Payload[RenameTagPayload](payload)
	if err != nil {
		return err
	}
	current, ok, err := tx.GetTag(p.TagID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("tag %s no longer exists: %w", p.TagID, ErrStaleBefore)
	}
	if current.Name != p.Before {
		return fmt.Errorf("tag %s is named %q but this op expected %q: %w",
			p.TagID, current.Name, p.Before, ErrStaleBefore)
	}

	renamed, err := domain.NewTag(p.TagID, current.ParentID, p.After)
	if err != nil {
		return err
	}
	return tx.UpdateTag(renamed)
}

func (renameTagOp) Inverse(payload []byte) (Action, []byte, error) {
	p, err := Payload[RenameTagPayload](payload)
	if err != nil {
		return "", nil, err
	}
	p.Before, p.After = p.After, p.Before
	raw, err := Marshal(p)
	return ActionRenameTag, raw, err
}

func (renameTagOp) Targets([]byte) (Targets, error) { return Targets{}, nil }

// MoveTagPayload is three fields, and that is the whole point.
//
// Global name uniqueness (spec 1.6) means a move can never collide, so there
// is no rename to record and no lookup to perform. Per-parent uniqueness would
// have forced this payload to grow a before/after name pair and Inverse to
// grow with it.
type MoveTagPayload struct {
	TagID     domain.TagID  `json:"tag_id"`
	OldParent *domain.TagID `json:"old_parent,omitempty"`
	NewParent *domain.TagID `json:"new_parent,omitempty"`
}

type moveTagOp struct{}

func (moveTagOp) Apply(tx Tx, payload []byte) error {
	p, err := Payload[MoveTagPayload](payload)
	if err != nil {
		return err
	}
	current, ok, err := tx.GetTag(p.TagID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("tag %s no longer exists: %w", p.TagID, ErrStaleBefore)
	}
	if !sameParent(current.ParentID, p.OldParent) {
		return fmt.Errorf("tag %s hangs off %v but this op expected %v: %w",
			p.TagID, current.ParentID, p.OldParent, ErrStaleBefore)
	}

	// One row, one column. There is no materialized path to cascade, which is
	// why moving a subtree of any size is O(1) (spec 1.6).
	current.ParentID = p.NewParent
	return tx.UpdateTag(current)
}

func (moveTagOp) Inverse(payload []byte) (Action, []byte, error) {
	p, err := Payload[MoveTagPayload](payload)
	if err != nil {
		return "", nil, err
	}
	p.OldParent, p.NewParent = p.NewParent, p.OldParent
	raw, err := Marshal(p)
	return ActionMoveTag, raw, err
}

func (moveTagOp) Targets([]byte) (Targets, error) { return Targets{}, nil }

func sameParent(a, b *domain.TagID) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}
