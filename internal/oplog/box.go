package oplog

import (
	"errors"
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
)

// ErrStaleBefore means the payload's recorded "before" state does not match
// what is actually in the library.
//
// This is strict undo (spec 2.5). Undoing op 3 while sitting at op 500 can
// mean restoring a box that later ops have already changed; kist refuses
// rather than half-restoring, because "I thought the undo worked but it only
// brought back part of it" is the worse failure in an archive.
var ErrStaleBefore = errors.New("the library has moved on since this op")

func init() {
	Register(ActionCreateBox, createBox{})
	Register(ActionDeleteBox, deleteBox{})
	Register(ActionUpdateBox, updateBox{})
}

// BoxFields are the user-editable fields of a box. updated_at is absent on
// purpose: it is maintained derived state and does not take part in Inverse
// (spec 1.5.1 ④).
type BoxFields struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	StartAt     domain.LocalDate `json:"start_at"`
	EndAt       domain.LocalDate `json:"end_at"`
}

// CreateBoxPayload carries every decision the command made, so that Apply
// makes none (invariant 3): both ids are minted by library before the op is
// written.
type CreateBoxPayload struct {
	BoxID        domain.BoxID    `json:"box_id"`
	RootFolderID domain.FolderID `json:"root_folder_id"`
	BoxFields
}

// DeleteBoxPayload is the same shape: deleting must record enough to rebuild
// the box exactly, including the root folder's id — a re-created folder with a
// fresh id would orphan every files.folder_id pointing at it.
type DeleteBoxPayload = CreateBoxPayload

type createBox struct{}

func (createBox) Apply(tx Tx, payload []byte) error {
	p, err := Payload[CreateBoxPayload](payload)
	if err != nil {
		return err
	}
	b, err := domain.NewBox(p.BoxID, p.Name, p.Description)
	if err != nil {
		return err
	}
	b.StartAt, b.EndAt = p.StartAt, p.EndAt
	if err := tx.InsertBox(b); err != nil {
		return err
	}
	return tx.InsertFolder(domain.NewRootFolder(p.RootFolderID, p.BoxID))
}

func (createBox) Inverse(payload []byte) (Action, []byte, error) {
	// The payload already describes the whole box, so the inverse is the same
	// bytes under a different action.
	return ActionDeleteBox, payload, nil
}

func (createBox) Targets(payload []byte) (Targets, error) { return boxTargets(payload) }

type deleteBox struct{}

func (deleteBox) Apply(tx Tx, payload []byte) error {
	p, err := Payload[DeleteBoxPayload](payload)
	if err != nil {
		return err
	}

	current, ok, err := tx.GetBox(p.BoxID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("box %s is already gone: %w", p.BoxID, ErrStaleBefore)
	}
	if err := checkBoxFields(p.BoxID, p.BoxFields, current); err != nil {
		return err
	}
	// Refusing to delete a box that has since been tagged is the same rule as
	// refusing to delete a tag that is in use: an undo must not quietly take
	// unrelated work with it.
	n, err := tx.CountBoxTags(p.BoxID)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("box %s carries %d tag(s) added after this op: %w", p.BoxID, n, ErrStaleBefore)
	}
	// Same rule for files, and the same reason it is checked here rather than
	// left to the foreign key: "FOREIGN KEY constraint failed (787)" is true,
	// useless and frightening.
	files, err := tx.CountBoxFiles(p.BoxID)
	if err != nil {
		return err
	}
	if files > 0 {
		return fmt.Errorf("box %q still holds %d file(s); undo the imports first: %w",
			p.Name, files, ErrStaleBefore)
	}

	// The root folder, by id, rather than every folder of the box: the
	// payload records exactly one folder id, so deleting more than that would
	// be an op its own inverse cannot undo.
	subs, err := tx.CountFolderChildren(p.RootFolderID)
	if err != nil {
		return err
	}
	if subs > 0 {
		return fmt.Errorf("box %q still has %d subfolder(s): %w", p.Name, subs, ErrStaleBefore)
	}
	if err := tx.DeleteFolder(p.RootFolderID); err != nil {
		return err
	}
	return tx.DeleteBox(p.BoxID)
}

func (deleteBox) Inverse(payload []byte) (Action, []byte, error) {
	return ActionCreateBox, payload, nil
}

func (deleteBox) Targets(payload []byte) (Targets, error) { return boxTargets(payload) }

// UpdateBoxPayload records both sides so Inverse is a swap and needs no
// database access at all (spec 2.3).
type UpdateBoxPayload struct {
	BoxID  domain.BoxID `json:"box_id"`
	Before BoxFields    `json:"before"`
	After  BoxFields    `json:"after"`
}

type updateBox struct{}

func (updateBox) Apply(tx Tx, payload []byte) error {
	p, err := Payload[UpdateBoxPayload](payload)
	if err != nil {
		return err
	}
	current, ok, err := tx.GetBox(p.BoxID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("box %s no longer exists: %w", p.BoxID, ErrStaleBefore)
	}
	if err := checkBoxFields(p.BoxID, p.Before, current); err != nil {
		return err
	}

	updated, err := domain.NewBox(p.BoxID, p.After.Name, p.After.Description)
	if err != nil {
		return err
	}
	updated.StartAt, updated.EndAt = p.After.StartAt, p.After.EndAt
	return tx.UpdateBox(updated)
}

func (updateBox) Inverse(payload []byte) (Action, []byte, error) {
	p, err := Payload[UpdateBoxPayload](payload)
	if err != nil {
		return "", nil, err
	}
	p.Before, p.After = p.After, p.Before
	raw, err := Marshal(p)
	return ActionUpdateBox, raw, err
}

func (updateBox) Targets(payload []byte) (Targets, error) {
	p, err := Payload[UpdateBoxPayload](payload)
	if err != nil {
		return Targets{}, err
	}
	return Targets{Boxes: []domain.BoxID{p.BoxID}}, nil
}

// checkBoxFields is the strict-mode assertion. It compares semantic fields
// only — created_at is immutable and updated_at is derived, so neither belongs
// in the comparison.
func checkBoxFields(id domain.BoxID, want BoxFields, got domain.Box) error {
	actual := BoxFields{
		Name:        got.Name,
		Description: got.Description,
		StartAt:     got.StartAt,
		EndAt:       got.EndAt,
	}
	if actual != want {
		return fmt.Errorf("box %s reads %+v but this op expected %+v: %w", id, actual, want, ErrStaleBefore)
	}
	return nil
}

func boxTargets(payload []byte) (Targets, error) {
	p, err := Payload[CreateBoxPayload](payload)
	if err != nil {
		return Targets{}, err
	}
	return Targets{Boxes: []domain.BoxID{p.BoxID}}, nil
}
