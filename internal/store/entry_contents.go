package store

import (
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
)

// EntryContents captures a stable subtree with all file annotations.
func (v *ViewTx) EntryContents(box domain.BoxID, id domain.ID, folder bool) (domain.EntryContents, bool, error) {
	s := domain.EntryContents{BoxID: box, ID: id, Folder: folder}
	all, found, err := v.boxContents(box, &id, folder)
	if err != nil || !found {
		return s, false, err
	}
	byID := map[domain.FolderID]domain.Folder{}
	for _, f := range all.Folders {
		byID[f.ID] = f
	}
	selected := map[domain.FolderID]bool{}
	if folder {
		root, ok := byID[domain.FolderID{ID: id}]
		if !ok || root.ParentID == nil {
			return s, false, nil
		}
		s.Parent = *root.ParentID
		s.Name = root.Name
		for _, f := range all.Folders {
			if f.ID == root.ID || (f.ParentID != nil && selected[*f.ParentID]) {
				selected[f.ID] = true
				s.Folders = append(s.Folders, f)
			}
		}
	}
	for _, f := range all.Files {
		if (!folder && f.File.ID.ID == id) || (folder && selected[f.File.FolderID]) {
			s.Files = append(s.Files, f)
			if !folder {
				s.Name = f.File.Name
				s.Parent = f.File.FolderID
			}
		}
	}
	if !folder && len(s.Files) != 1 {
		return s, false, nil
	}
	seen := map[domain.FolderID]bool{}
	for at := s.Parent; ; {
		f, ok := byID[at]
		if !ok || seen[at] {
			return s, false, fmt.Errorf("invalid folder ancestry")
		}
		seen[at] = true
		s.Ancestors = append(s.Ancestors, at)
		if f.ParentID == nil {
			break
		}
		at = *f.ParentID
	}
	return s, true, nil
}

func (v *ViewTx) DeleteEntryContents(s domain.EntryContents) error {
	for _, f := range s.Files {
		for _, table := range []string{"file_tag_sources", "file_tags", "tag_rejections"} {
			if _, err := v.tx.Exec(`DELETE FROM `+table+` WHERE file_id=?`, f.File.ID); err != nil {
				return err
			}
		}
		if err := v.DeleteFile(f.File.ID); err != nil {
			return err
		}
	}
	for i := len(s.Folders) - 1; i >= 0; i-- {
		if err := v.DeleteFolder(s.Folders[i].ID); err != nil {
			return err
		}
	}
	return nil
}
func (v *ViewTx) RestoreEntryContents(s domain.EntryContents) error {
	for _, f := range s.Folders {
		if err := v.InsertFolder(f); err != nil {
			return err
		}
		if _, err := v.tx.Exec(`UPDATE folders SET created_at=?,updated_at=? WHERE id=?`, FormatTime(f.CreatedAt), FormatTime(f.UpdatedAt), f.ID); err != nil {
			return err
		}
	}
	return v.restoreArchivedFiles(s.Files)
}

func (v *ViewTx) RenameEntry(id domain.ID, folder bool, name string) error {
	table := "files"
	if folder {
		table = "folders"
	}
	_, err := v.tx.Exec(`UPDATE `+table+` SET name=?,name_fold=?,updated_at=? WHERE id=?`, name, domain.Fold(name), FormatTime(v.ts), id)
	return err
}

func (v *ViewTx) EntryNameAvailable(parent domain.FolderID, name string, except domain.ID) (bool, error) {
	var n int
	err := v.tx.QueryRow(`SELECT (SELECT count(*) FROM files WHERE folder_id=? AND name_fold=? AND id<>?) + (SELECT count(*) FROM folders WHERE parent_id=? AND name_fold=? AND id<>?)`, parent, domain.Fold(name), except, parent, domain.Fold(name), except).Scan(&n)
	return n == 0, err
}
