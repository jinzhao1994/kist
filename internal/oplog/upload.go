package oplog

import (
	"reflect"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

// FileReplacement keeps both complete versions under the same file identity.
type FileReplacement struct {
	Before domain.EntryContents `json:"before"`
	After  domain.EntryContents `json:"after"`
}

// UploadFilesPayload commits additions and replacements as one atomic operation.
type UploadFilesPayload struct {
	Import       ImportFilesPayload `json:"import"`
	Replacements []FileReplacement  `json:"replacements"`
}
type uploadFiles struct{ undo bool }

func init() {
	Register(ActionUploadFiles, uploadFiles{})
	Register(ActionUndoUploadFiles, uploadFiles{undo: true})
}
func (h uploadFiles) Apply(tx Tx, raw []byte) error {
	p, err := Payload[UploadFilesPayload](raw)
	if err != nil {
		return err
	}
	for _, r := range p.Replacements {
		before, after := r.Before, r.After
		if h.undo {
			before, after = after, before
		}
		if before.Folder || after.Folder || len(before.Files) != 1 || len(after.Files) != 1 || before.ID != after.ID || before.Files[0].File.ID.ID != before.ID || after.Files[0].File.ID.ID != after.ID || before.Parent != after.Parent || before.BoxID != p.Import.BoxID || after.BoxID != before.BoxID {
			return ErrStaleBefore
		}
		current, found, err := tx.EntryContents(before.BoxID, before.ID, false)
		if err != nil {
			return err
		}
		if !found || len(current.Files) != 1 {
			return ErrStaleBefore
		}
		current.Files[0].File.UpdatedAt = time.Time{}
		before.Files[0].File.UpdatedAt = time.Time{}
		if !reflect.DeepEqual(current, before) {
			return ErrStaleBefore
		}
		if err := tx.DeleteEntryContents(before); err != nil {
			return err
		}
		after.Files[0].File.UpdatedAt = tx.TS()
		if err := tx.RestoreEntryContents(after); err != nil {
			return err
		}
	}
	rawImport, err := Marshal(p.Import)
	if err != nil {
		return err
	}
	if h.undo {
		return (deleteFiles{}).Apply(tx, rawImport)
	}
	return (importFiles{}).Apply(tx, rawImport)
}
func (h uploadFiles) Inverse(raw []byte) (Action, []byte, error) {
	if _, err := Payload[UploadFilesPayload](raw); err != nil {
		return "", nil, err
	}
	if h.undo {
		return ActionUploadFiles, raw, nil
	}
	return ActionUndoUploadFiles, raw, nil
}
func (uploadFiles) Targets(raw []byte) (Targets, error) {
	p, err := Payload[UploadFilesPayload](raw)
	if err != nil {
		return Targets{}, err
	}
	b, err := Marshal(p.Import)
	if err != nil {
		return Targets{}, err
	}
	t, err := fileTargets(b)
	if err != nil {
		return t, err
	}
	for _, r := range p.Replacements {
		if len(r.Before.Files) != 1 || len(r.After.Files) != 1 {
			return Targets{}, ErrStaleBefore
		}
		t.FolderAncestors = append(t.FolderAncestors, r.Before.Ancestors...)
		t.Files = append(t.Files, r.Before.Files[0].File.ID)
		t.Blobs = append(t.Blobs, r.Before.Files[0].File.BlobKey, r.After.Files[0].File.BlobKey)
	}
	return t, nil
}
