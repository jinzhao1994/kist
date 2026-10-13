package oplog

import (
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
)

// FolderPayload records the new directory and its ancestor timestamp targets.
// Keeping ancestors in the event makes timestamp propagation self-contained.
type FolderPayload struct {
	BoxID     domain.BoxID      `json:"box_id"`
	Folder    ImportedFolder    `json:"folder"`
	Ancestors []domain.FolderID `json:"ancestors"`
}

type folderAction struct{ remove bool }

func init() {
	Register(ActionCreateFolder, folderAction{})
	Register(ActionDeleteFolder, folderAction{remove: true})
}
func (h folderAction) Apply(tx Tx, raw []byte) error {
	p, err := Payload[FolderPayload](raw)
	if err != nil {
		return err
	}
	if h.remove {
		f, ok, err := tx.GetFolder(p.Folder.FolderID)
		if err != nil {
			return err
		}
		if !ok || f.ParentID == nil || *f.ParentID != p.Folder.ParentID || f.Name != p.Folder.Name {
			return fmt.Errorf("directory changed: %w", ErrStaleBefore)
		}
	}
	payload, err := Marshal(ImportFilesPayload{BoxID: p.BoxID, Folders: []ImportedFolder{p.Folder}})
	if err != nil {
		return err
	}
	if h.remove {
		return (deleteFiles{}).Apply(tx, payload)
	}
	return (importFiles{}).Apply(tx, payload)
}
func (h folderAction) Inverse(raw []byte) (Action, []byte, error) {
	if h.remove {
		return ActionCreateFolder, raw, nil
	}
	return ActionDeleteFolder, raw, nil
}
func (h folderAction) Targets(raw []byte) (Targets, error) {
	p, err := Payload[FolderPayload](raw)
	if err != nil {
		return Targets{}, err
	}
	return Targets{Boxes: []domain.BoxID{p.BoxID}, FolderAncestors: p.Ancestors}, nil
}
