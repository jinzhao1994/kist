package store

import (
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
)

// BoxContents reads a complete box in the caller's snapshot. Queries never
// inspect operation payloads. UNION makes damaged recursive trees terminate.
func (v *ViewTx) BoxContents(id domain.BoxID) (domain.BoxContents, bool, error) {
	return v.boxContents(id, nil, false)
}

// boxContents can limit file annotations to a single entry or subtree.
func (v *ViewTx) boxContents(id domain.BoxID, entry *domain.ID, folder bool) (domain.BoxContents, bool, error) {
	var s domain.BoxContents
	b, found, err := v.GetBox(id)
	if err != nil || !found {
		return s, found, err
	}
	s.Box = b
	rows, err := v.tx.Query(`SELECT `+folderColumns+` FROM folders WHERE id IN (`+FoldersOfBoxSQL+`) ORDER BY id`, id)
	if err != nil {
		return s, false, err
	}
	var pending []domain.Folder
	for rows.Next() {
		f, e := scanFolder(rows.Scan)
		if e != nil {
			rows.Close()
			return s, false, e
		}
		pending = append(pending, f)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return s, false, err
	}
	// Topological ordering is independent of ID generation or wall-clock order.
	seen := map[domain.FolderID]bool{}
	for len(pending) > 0 {
		var rest []domain.Folder
		for _, f := range pending {
			if f.ParentID == nil || seen[*f.ParentID] {
				s.Folders = append(s.Folders, f)
				seen[f.ID] = true
			} else {
				rest = append(rest, f)
			}
		}
		if len(rest) == len(pending) {
			return s, false, fmt.Errorf("cyclic folder hierarchy")
		}
		pending = rest
	}
	fileQuery := `SELECT ` + fileColumns + ` FROM files WHERE folder_id IN (` + FoldersOfBoxSQL + `)`
	args := []any{id}
	if entry != nil {
		if folder {
			fileQuery += ` AND folder_id IN (WITH RECURSIVE subtree(id) AS (SELECT id FROM folders WHERE id=? UNION SELECT f.id FROM folders f JOIN subtree s ON f.parent_id=s.id) SELECT id FROM subtree)`
		} else {
			fileQuery += ` AND id=?`
		}
		args = append(args, *entry)
	}
	rows, err = v.tx.Query(fileQuery+` ORDER BY id`, args...)
	if err != nil {
		return s, false, err
	}
	for rows.Next() {
		f, e := scanFile(rows.Scan)
		if e != nil {
			rows.Close()
			return s, false, e
		}
		s.Files = append(s.Files, domain.ArchivedFile{File: f})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return s, false, err
	}
	rows, err = v.tx.Query(`SELECT tag_id FROM box_tags WHERE box_id=? ORDER BY tag_id`, id)
	if err != nil {
		return s, false, err
	}
	for rows.Next() {
		var tag domain.TagID
		if err = rows.Scan(&tag); err != nil {
			rows.Close()
			return s, false, err
		}
		s.Tags = append(s.Tags, tag)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return s, false, err
	}
	for i := range s.Files {
		f := &s.Files[i]
		if err = v.tx.QueryRow(`SELECT sha256,size FROM blobs WHERE key=?`, f.File.BlobKey).Scan(&f.SHA256, &f.Size); err != nil {
			return s, false, err
		}
		rows, err = v.tx.Query(`SELECT tag_id,source FROM file_tag_sources WHERE file_id=? ORDER BY tag_id,source`, f.File.ID)
		if err != nil {
			return s, false, err
		}
		for rows.Next() {
			var tag domain.ArchivedTag
			if err = rows.Scan(&tag.Tag, &tag.Source); err != nil {
				rows.Close()
				return s, false, err
			}
			f.Tags = append(f.Tags, tag)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return s, false, err
		}
		rows, err = v.tx.Query(`SELECT tag_id FROM tag_rejections WHERE file_id=? ORDER BY tag_id`, f.File.ID)
		if err != nil {
			return s, false, err
		}
		for rows.Next() {
			var tag domain.TagID
			if err = rows.Scan(&tag); err != nil {
				rows.Close()
				return s, false, err
			}
			f.Rejections = append(f.Rejections, tag)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return s, false, err
		}
		rows, err = v.tx.Query(`SELECT key,value,type,source FROM file_metadata WHERE file_id=? ORDER BY key`, f.File.ID)
		if err != nil {
			return s, false, err
		}
		for rows.Next() {
			var m domain.ArchivedMetadata
			if err = rows.Scan(&m.Key, &m.Value, &m.Type, &m.Source); err != nil {
				rows.Close()
				return s, false, err
			}
			f.Metadata = append(f.Metadata, m)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return s, false, err
		}
		f.Extractions, err = Extractions(v.tx, f.File.ID)
		if err != nil {
			return s, false, err
		}
	}
	sets, err := Filesets(v.tx)
	if err != nil {
		return s, false, err
	}
	for _, set := range sets {
		if seen[set.FolderID] {
			s.Filesets = append(s.Filesets, set)
		}
	}
	return s, true, nil
}

// DeleteBoxContents removes only the IDs recorded in the checked payload.
func (v *ViewTx) DeleteBoxContents(s domain.BoxContents) error {
	if err := v.DeleteFilesets(s.Filesets); err != nil {
		return err
	}
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
	for _, tag := range s.Tags {
		if err := v.RemoveBoxTag(s.Box.ID, tag); err != nil {
			return err
		}
	}
	if err := v.DeleteBox(s.Box.ID); err != nil {
		return err
	}
	for i := len(s.Folders) - 1; i >= 0; i-- {
		if err := v.DeleteFolder(s.Folders[i].ID); err != nil {
			return err
		}
	}
	return nil
}

// RestoreBoxContents inserts the recorded rows. Plain INSERT and foreign keys
// reject occupied IDs and missing tags; no existing state is overwritten.
func (v *ViewTx) RestoreBoxContents(s domain.BoxContents) error {
	for _, f := range s.Folders {
		if err := v.InsertFolder(f); err != nil {
			return err
		}
		if _, err := v.tx.Exec(`UPDATE folders SET created_at=?,updated_at=? WHERE id=?`, FormatTime(f.CreatedAt), FormatTime(f.UpdatedAt), f.ID); err != nil {
			return err
		}
	}
	if err := v.InsertBox(s.Box); err != nil {
		return err
	}
	if _, err := v.tx.Exec(`UPDATE boxes SET created_at=?,updated_at=? WHERE id=?`, FormatTime(s.Box.CreatedAt), FormatTime(s.Box.UpdatedAt), s.Box.ID); err != nil {
		return err
	}
	for _, tag := range s.Tags {
		if err := v.AddBoxTag(s.Box.ID, tag); err != nil {
			return err
		}
	}
	if err := v.restoreArchivedFiles(s.Files); err != nil {
		return err
	}
	return v.RestoreFilesets(s.Filesets)
}

func (v *ViewTx) restoreArchivedFiles(files []domain.ArchivedFile) error {
	for _, f := range files {
		if err := v.UpsertBlob(f.File.BlobKey, f.SHA256, f.Size); err != nil {
			return err
		}
		if err := v.InsertFile(f.File); err != nil {
			return err
		}
		if _, err := v.tx.Exec(`UPDATE files SET created_at=?,updated_at=? WHERE id=?`, FormatTime(f.File.CreatedAt), FormatTime(f.File.UpdatedAt), f.File.ID); err != nil {
			return err
		}
		for _, tag := range f.Tags {
			if err := v.AddFileTag(f.File.ID, tag.Tag, tag.Source); err != nil {
				return err
			}
		}
		for _, tag := range f.Rejections {
			if _, err := v.tx.Exec(`INSERT INTO tag_rejections(file_id,tag_id) VALUES (?,?)`, f.File.ID, tag); err != nil {
				return err
			}
		}
		for _, m := range f.Metadata {
			if _, err := v.tx.Exec(`INSERT INTO file_metadata(file_id,key,value,type,source) VALUES (?,?,?,?,?)`, f.File.ID, m.Key, m.Value, m.Type, m.Source); err != nil {
				return err
			}
		}
		for name, version := range f.Extractions {
			if err := v.SetFileExtraction(f.File.ID, name, version); err != nil {
				return err
			}
		}
	}
	return nil
}
