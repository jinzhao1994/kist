package store

import (
	"database/sql"
	"errors"

	"github.com/jinzhao1994/kist/internal/domain"
)

// EntryNamed resolves one logical child without reading the disk projection.
func EntryNamed(q Queryer, parent domain.FolderID, name string) (domain.ID, bool, bool, error) {
	var id domain.ID
	var folder bool
	err := q.QueryRow(`SELECT id, 1 FROM folders WHERE parent_id=? AND name_fold=?
 UNION ALL SELECT id, 0 FROM files WHERE folder_id=? AND name_fold=?`, parent, domain.Fold(name), parent, domain.Fold(name)).Scan(&id, &folder)
	if errors.Is(err, sql.ErrNoRows) {
		return id, false, false, nil
	}
	return id, folder, err == nil, err
}
