package store

import (
	"database/sql"
	"github.com/jinzhao1994/kist/internal/domain"
)

func Filesets(q Queryer) ([]domain.Fileset, error) {
	rows, err := q.Query(`SELECT folder_id,kind,primary_file_id,layout FROM filesets ORDER BY folder_id`)
	if err != nil {
		return nil, err
	}
	var out []domain.Fileset
	for rows.Next() {
		var s domain.Fileset
		var primary []byte
		if err = rows.Scan(&s.FolderID, &s.Kind, &primary, &s.Layout); err != nil {
			rows.Close()
			return nil, err
		}
		if primary != nil {
			if err = s.Primary.Scan(primary); err != nil {
				rows.Close()
				return nil, err
			}
		}
		out = append(out, s)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range out {
		rows, err = q.Query(`SELECT file_id,folder_id,role,language,is_default FROM fileset_members WHERE fileset_id=? ORDER BY position`, out[i].FolderID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var m domain.FilesetMember
			var f, d []byte
			if err = rows.Scan(&f, &d, &m.Role, &m.Language, &m.Default); err != nil {
				rows.Close()
				return nil, err
			}
			if f != nil {
				if err = m.FileID.Scan(f); err != nil {
					rows.Close()
					return nil, err
				}
			}
			if d != nil {
				if err = m.FolderID.Scan(d); err != nil {
					rows.Close()
					return nil, err
				}
			}
			out[i].Members = append(out[i].Members, m)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func FilesetByID(q Queryer, id domain.FolderID) (*domain.Fileset, error) {
	// Configuration lists are small; callers needing several use Filesets once.
	all, err := Filesets(q)
	if err != nil {
		return nil, err
	}
	for _, s := range all {
		if s.FolderID == id {
			return &s, nil
		}
	}
	return nil, nil
}
func (v *ViewTx) GetFileset(id domain.FolderID) (*domain.Fileset, error) {
	return FilesetByID(v.tx, id)
}
func (v *ViewTx) SetFileset(id domain.FolderID, s *domain.Fileset) error {
	if _, err := v.tx.Exec(`DELETE FROM fileset_members WHERE fileset_id=?`, id); err != nil {
		return err
	}
	if s == nil {
		_, err := v.tx.Exec(`DELETE FROM filesets WHERE folder_id=?`, id)
		return err
	}
	var primary any
	if !s.Primary.IsZero() {
		primary = s.Primary
	}
	_, err := v.tx.Exec(`INSERT INTO filesets(folder_id,kind,primary_file_id,layout) VALUES(?,?,?,?) ON CONFLICT(folder_id) DO UPDATE SET kind=excluded.kind,primary_file_id=excluded.primary_file_id,layout=excluded.layout`, id, s.Kind, primary, s.Layout)
	if err != nil {
		return err
	}
	for i, m := range s.Members {
		var f, d any
		if !m.FileID.IsZero() {
			f = m.FileID
		}
		if !m.FolderID.IsZero() {
			d = m.FolderID
		}
		if _, err = v.tx.Exec(`INSERT INTO fileset_members VALUES(?,?,?,?,?,?,?)`, id, i, f, d, m.Role, m.Language, m.Default); err != nil {
			return err
		}
	}
	return nil
}

// RestoreFilesets runs after all folders and files exist, within the same transaction.
func (v *ViewTx) RestoreFilesets(sets []domain.Fileset) error {
	for _, s := range sets {
		var n int
		if err := v.tx.QueryRow(`SELECT count(*) FROM filesets WHERE folder_id=?`, s.FolderID).Scan(&n); err != nil {
			return err
		}
		if n != 0 {
			return sql.ErrNoRows
		}
		if err := v.SetFileset(s.FolderID, &s); err != nil {
			return err
		}
	}
	return nil
}
func (v *ViewTx) DeleteFilesets(sets []domain.Fileset) error {
	for _, s := range sets {
		if _, err := v.tx.Exec(`DELETE FROM fileset_members WHERE fileset_id=?`, s.FolderID); err != nil {
			return err
		}
	}
	for _, s := range sets {
		if _, err := v.tx.Exec(`DELETE FROM filesets WHERE folder_id=?`, s.FolderID); err != nil {
			return err
		}
	}
	return nil
}
