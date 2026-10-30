package store

import "github.com/jinzhao1994/kist/internal/domain"

// FileLocation is the current display context of a file, including its box.
type FileLocation struct {
	Name     string
	BoxName  string
	BoxStart string
}

// FileLocations resolves a bounded batch in one snapshot. UNION also terminates
// the ancestor walk if a damaged folder tree contains a cycle.
func FileLocations(q Queryer, ids []domain.FileID) (map[domain.FileID]FileLocation, error) {
	out := make(map[domain.FileID]FileLocation)
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := q.Query(`WITH RECURSIVE ancestors(file_id,folder_id) AS (
	 SELECT id,folder_id FROM files WHERE id IN (`+placeholders(len(ids))+`)
	 UNION
	 SELECT a.file_id,f.parent_id FROM ancestors a JOIN folders f ON f.id=a.folder_id WHERE f.parent_id IS NOT NULL
	) SELECT f.id,f.name,b.name,COALESCE(b.start_at,'') FROM ancestors a
	 JOIN boxes b ON b.root_folder_id=a.folder_id JOIN files f ON f.id=a.file_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id domain.FileID
		var location FileLocation
		if err := rows.Scan(&id, &location.Name, &location.BoxName, &location.BoxStart); err != nil {
			return nil, err
		}
		out[id] = location
	}
	return out, rows.Err()
}
