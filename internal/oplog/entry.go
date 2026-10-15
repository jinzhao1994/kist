package oplog

import (
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
	"reflect"
	"time"
)

// RenameEntryPayload records exact location and name preconditions.
type RenameEntryPayload struct {
	BoxID     domain.BoxID      `json:"box_id"`
	ID        domain.ID         `json:"id"`
	Folder    bool              `json:"folder"`
	Parent    domain.FolderID   `json:"parent"`
	Before    string            `json:"before"`
	After     string            `json:"after"`
	Ancestors []domain.FolderID `json:"ancestors"`
}
type renameEntry struct{}
type entryContents struct{ deleting bool }

func init() {
	Register(ActionRenameEntry, renameEntry{})
	Register(ActionDeleteEntryContents, entryContents{deleting: true})
	Register(ActionRestoreEntryContents, entryContents{})
}
func (renameEntry) Apply(tx Tx, raw []byte) error {
	p, err := Payload[RenameEntryPayload](raw)
	if err != nil {
		return err
	}
	current, found, err := tx.EntryContents(p.BoxID, p.ID, p.Folder)
	if err != nil {
		return err
	}
	if !found || current.Name != p.Before || current.Parent != p.Parent {
		return ErrStaleBefore
	}
	if err := domain.ValidateName(p.After); err != nil {
		return err
	}
	available, err := tx.EntryNameAvailable(p.Parent, p.After, p.ID)
	if err != nil {
		return err
	}
	if !available {
		return ErrStaleBefore
	}
	return tx.RenameEntry(p.ID, p.Folder, p.After)
}
func (renameEntry) Inverse(raw []byte) (Action, []byte, error) {
	p, err := Payload[RenameEntryPayload](raw)
	if err != nil {
		return "", nil, err
	}
	p.Before, p.After = p.After, p.Before
	b, err := Marshal(p)
	return ActionRenameEntry, b, err
}
func (renameEntry) Targets(raw []byte) (Targets, error) {
	p, err := Payload[RenameEntryPayload](raw)
	t := Targets{Boxes: []domain.BoxID{p.BoxID}, FolderAncestors: p.Ancestors}
	if !p.Folder {
		t.Files = []domain.FileID{{ID: p.ID}}
	}
	return t, err
}
func (h entryContents) Apply(tx Tx, raw []byte) error {
	p, err := Payload[domain.EntryContents](raw)
	if err != nil {
		return err
	}
	if !h.deleting {
		box, ok, err := tx.GetBox(p.BoxID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrStaleBefore
		}
		at := p.Parent
		for i, id := range p.Ancestors {
			if id != at {
				return ErrStaleBefore
			}
			f, found, err := tx.GetFolder(id)
			if err != nil {
				return err
			}
			if !found {
				return ErrStaleBefore
			}
			if i == len(p.Ancestors)-1 {
				if id != box.RootFolderID || f.ParentID != nil {
					return ErrStaleBefore
				}
			} else {
				if f.ParentID == nil {
					return ErrStaleBefore
				}
				at = *f.ParentID
			}
		}
		if len(p.Ancestors) == 0 {
			return ErrStaleBefore
		}
		for _, f := range p.Folders {
			if f.ParentID == nil {
				return ErrStaleBefore
			}
			ok, err := tx.EntryNameAvailable(*f.ParentID, f.Name, domain.ID{})
			if err != nil {
				return err
			}
			if !ok {
				return ErrStaleBefore
			}
		}
		for _, f := range p.Files {
			ok, err := tx.EntryNameAvailable(f.File.FolderID, f.File.Name, domain.ID{})
			if err != nil {
				return err
			}
			if !ok {
				return ErrStaleBefore
			}
		}
		return tx.RestoreEntryContents(p)
	}
	current, found, err := tx.EntryContents(p.BoxID, p.ID, p.Folder)
	if err != nil {
		return err
	}
	for i := range current.Folders {
		current.Folders[i].UpdatedAt = time.Time{}
	}
	for i := range p.Folders {
		p.Folders[i].UpdatedAt = time.Time{}
	}
	if !found || !reflect.DeepEqual(current, p) {
		return fmt.Errorf("entry contents changed: %w", ErrStaleBefore)
	}
	return tx.DeleteEntryContents(p)
}
func (h entryContents) Inverse(raw []byte) (Action, []byte, error) {
	if _, err := Payload[domain.EntryContents](raw); err != nil {
		return "", nil, err
	}
	if h.deleting {
		return ActionRestoreEntryContents, raw, nil
	}
	return ActionDeleteEntryContents, raw, nil
}
func (entryContents) Targets(raw []byte) (Targets, error) {
	p, err := Payload[domain.EntryContents](raw)
	t := Targets{Boxes: []domain.BoxID{p.BoxID}, FolderAncestors: p.Ancestors}
	for _, f := range p.Files {
		t.Files = append(t.Files, f.File.ID)
		t.Blobs = append(t.Blobs, f.File.BlobKey)
	}
	return t, err
}
