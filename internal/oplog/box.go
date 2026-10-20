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
	CoverFileID domain.FileID    `json:"cover_file_id,omitzero"`
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
	b, err := domain.NewBox(p.BoxID, p.RootFolderID, p.Name, p.Description)
	if err != nil {
		return err
	}
	b.StartAt, b.EndAt = p.StartAt, p.EndAt
	b.CoverFileID = p.CoverFileID
	// The folder first: boxes.root_folder_id points at it.
	if err := tx.InsertFolder(domain.NewRootFolder(p.RootFolderID)); err != nil {
		return err
	}
	return tx.InsertBox(b)
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
	if err := tx.DeleteBox(p.BoxID); err != nil {
		return err
	}
	return tx.DeleteFolder(p.RootFolderID)
}

func (deleteBox) Inverse(payload []byte) (Action, []byte, error) {
	return ActionCreateBox, payload, nil
}

func (deleteBox) Targets(payload []byte) (Targets, error) { return boxTargets(payload) }

// TextChange and DateChange are one field's two sides. Both are here so
// Inverse is a swap and needs no database access at all (spec 2.3).
type TextChange struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

// DateChange is the same for the variable-precision dates (spec 1.3).
type DateChange struct {
	Before domain.LocalDate `json:"before"`
	After  domain.LocalDate `json:"after"`
}

// CoverChange records a soft reference; zero denotes no selected cover.
type CoverChange struct {
	Before domain.FileID `json:"before"`
	After  domain.FileID `json:"after"`
}

// UpdateBoxPayload records only the fields this op actually changes. A field
// it left alone is absent, not a pair of equal values.
//
// The absence is what keeps Apply honest: it asserts exactly what it writes.
// A payload carrying all four fields would have to assert all four, and then
// undoing a rename would fail because somebody edited the description in
// between — or, worse, succeed and silently revert that edit too, which is
// precisely the half-restored archive spec 2.5 exists to prevent.
type UpdateBoxPayload struct {
	CoverFileID *CoverChange `json:"cover_file_id,omitempty"`
	BoxID       domain.BoxID `json:"box_id"`
	Name        *TextChange  `json:"name,omitempty"`
	Description *TextChange  `json:"description,omitempty"`
	StartAt     *DateChange  `json:"start_at,omitempty"`
	EndAt       *DateChange  `json:"end_at,omitempty"`
}

// Empty reports whether the payload changes nothing. library skips writing
// such an op; Apply refuses one, because a stored op that does nothing would
// still have to be undoable.
func (p UpdateBoxPayload) Empty() bool {
	return p.CoverFileID == nil && p.Name == nil && p.Description == nil && p.StartAt == nil && p.EndAt == nil
}

func (c *TextChange) swapped() *TextChange {
	if c == nil {
		return nil
	}
	return &TextChange{Before: c.After, After: c.Before}
}

func (c *DateChange) swapped() *DateChange {
	if c == nil {
		return nil
	}
	return &DateChange{Before: c.After, After: c.Before}
}

type updateBox struct{}

func (updateBox) Apply(tx Tx, payload []byte) error {
	p, err := Payload[UpdateBoxPayload](payload)
	if err != nil {
		return err
	}
	if p.Empty() {
		return fmt.Errorf("update_box: %w", errEmptyBatch)
	}
	current, ok, err := tx.GetBox(p.BoxID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("box %s no longer exists: %w", p.BoxID, ErrStaleBefore)
	}

	// One assertion per field the op touches, and none for the fields it does
	// not. A later edit to a different field is not in the way of this one.
	next := BoxFields{
		Name: current.Name, Description: current.Description,
		StartAt: current.StartAt, EndAt: current.EndAt,
	}
	if c := p.Name; c != nil {
		if current.Name != c.Before {
			return staleField(p.BoxID, "name", current.Name, c.Before)
		}
		next.Name = c.After
	}
	if c := p.Description; c != nil {
		if current.Description != c.Before {
			return staleField(p.BoxID, "description", current.Description, c.Before)
		}
		next.Description = c.After
	}
	if c := p.StartAt; c != nil {
		if current.StartAt != c.Before {
			return staleField(p.BoxID, "start_at", current.StartAt, c.Before)
		}
		next.StartAt = c.After
	}
	if c := p.EndAt; c != nil {
		if current.EndAt != c.Before {
			return staleField(p.BoxID, "end_at", current.EndAt, c.Before)
		}
		next.EndAt = c.After
	}

	// Through NewBox rather than writing the fields straight out: validation
	// and folding live there, and an update must not be able to store a name
	// a create would have rejected. The root folder is structure, not a
	// user-editable field, so it comes from the box as it stands.
	updated, err := domain.NewBox(p.BoxID, current.RootFolderID, next.Name, next.Description)
	if err != nil {
		return err
	}
	updated.StartAt, updated.EndAt = next.StartAt, next.EndAt
	updated.CoverFileID = current.CoverFileID
	if c := p.CoverFileID; c != nil {
		if current.CoverFileID != c.Before {
			return staleField(p.BoxID, "cover_file_id", current.CoverFileID, c.Before)
		}
		updated.CoverFileID = c.After
	}
	return tx.UpdateBox(updated)
}

func staleField(id domain.BoxID, field string, got, want any) error {
	return fmt.Errorf("box %s: %s reads %v but this op expected %v: %w",
		id, field, got, want, ErrStaleBefore)
}

func (updateBox) Inverse(payload []byte) (Action, []byte, error) {
	p, err := Payload[UpdateBoxPayload](payload)
	if err != nil {
		return "", nil, err
	}
	p.Name, p.Description = p.Name.swapped(), p.Description.swapped()
	if c := p.CoverFileID; c != nil {
		p.CoverFileID = &CoverChange{Before: c.After, After: c.Before}
	}
	p.StartAt, p.EndAt = p.StartAt.swapped(), p.EndAt.swapped()
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

// checkBoxFields is delete_box's assertion: every semantic field at once.
//
// Total, unlike update_box's per-field checks, because the two ops touch
// different amounts. An update writes only the columns it declared; a delete
// takes the whole row away, so it has to be right about the whole row.
//
// created_at is immutable and updated_at is derived, so neither belongs in the
// comparison.
func checkBoxFields(id domain.BoxID, want BoxFields, got domain.Box) error {
	actual := BoxFields{
		CoverFileID: got.CoverFileID,
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
