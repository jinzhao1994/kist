package oplog

import (
	"errors"
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
)

func init() {
	Register(ActionTagBoxes, tagBoxes{})
	Register(ActionUntagBoxes, untagBoxes{})
}

// TagBoxesPayload attaches one tag to many boxes in a single op.
//
// The granularity is the batch (spec 2.2.1): tagging twelve boxes is one op
// with twelve targets, not twelve ops, or undoing it would take twelve
// clicks.
//
// BoxIDs names only the boxes that actually change. library filters out any
// box that already carries the tag before writing the op — otherwise the
// inverse would strip a tag this op never added, and an undo that does more
// than the op did is a bug you find out about much later.
type TagBoxesPayload struct {
	TagID  domain.TagID   `json:"tag_id"`
	BoxIDs []domain.BoxID `json:"box_ids"`
}

// UntagBoxesPayload is the same shape, so the two are exact inverses.
type UntagBoxesPayload = TagBoxesPayload

var errEmptyBatch = errors.New("op has no targets")

type tagBoxes struct{}

func (tagBoxes) Apply(tx Tx, payload []byte) error {
	p, err := Payload[TagBoxesPayload](payload)
	if err != nil {
		return err
	}
	if len(p.BoxIDs) == 0 {
		return fmt.Errorf("tag_boxes: %w", errEmptyBatch)
	}
	if err := checkBoxesExist(tx, p.BoxIDs); err != nil {
		return err
	}
	for _, box := range p.BoxIDs {
		if err := tx.AddBoxTag(box, p.TagID); err != nil {
			return err
		}
	}
	return nil
}

func (tagBoxes) Inverse(payload []byte) (Action, []byte, error) {
	return ActionUntagBoxes, payload, nil
}

func (tagBoxes) Targets(payload []byte) (Targets, error) { return boxTagTargets(payload) }

type untagBoxes struct{}

func (untagBoxes) Apply(tx Tx, payload []byte) error {
	p, err := Payload[UntagBoxesPayload](payload)
	if err != nil {
		return err
	}
	if len(p.BoxIDs) == 0 {
		return fmt.Errorf("untag_boxes: %w", errEmptyBatch)
	}
	if err := checkBoxesExist(tx, p.BoxIDs); err != nil {
		return err
	}
	for _, box := range p.BoxIDs {
		if err := tx.RemoveBoxTag(box, p.TagID); err != nil {
			return err
		}
	}
	return nil
}

func (untagBoxes) Inverse(payload []byte) (Action, []byte, error) {
	return ActionTagBoxes, payload, nil
}

func (untagBoxes) Targets(payload []byte) (Targets, error) { return boxTagTargets(payload) }

// checkBoxesExist is the strict-mode assertion for a batch.
//
// Without it, undoing an op whose boxes were deleted in the meantime fails
// deep inside the database and the user is shown "FOREIGN KEY constraint
// failed (787)". That is true, useless, and frightening. The foreign key is
// still the backstop; this is the sentence that gets read (spec 2.5).
func checkBoxesExist(tx Tx, boxes []domain.BoxID) error {
	for _, box := range boxes {
		_, ok, err := tx.GetBox(box)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("box %s no longer exists: %w", box, ErrStaleBefore)
		}
	}
	return nil
}

func boxTagTargets(payload []byte) (Targets, error) {
	p, err := Payload[TagBoxesPayload](payload)
	if err != nil {
		return Targets{}, err
	}
	return Targets{Boxes: p.BoxIDs}, nil
}
