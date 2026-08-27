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
	// Source is "user" or "derived" (spec 1.6.1). It lives in the payload
	// rather than being a constant inside Apply because a rebuild has to
	// reproduce it.
	Source string `json:"source"`
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
	for _, box := range p.BoxIDs {
		if err := tx.AddBoxTag(box, p.TagID, p.Source); err != nil {
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

func boxTagTargets(payload []byte) (Targets, error) {
	p, err := Payload[TagBoxesPayload](payload)
	if err != nil {
		return Targets{}, err
	}
	return Targets{Boxes: p.BoxIDs}, nil
}
