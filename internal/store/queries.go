package store

import (
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
)

// ListBoxesOptions controls the box listing.
type ListBoxesOptions struct {
	// SortByName orders by name instead of by recent activity. The ordering is
	// byte order, deliberately (spec 4.8): the same library must list in the
	// same order on every platform and on either engine.
	SortByName bool
	// Tag restricts results to boxes carrying this tag or a descendant.
	Tag   *domain.TagID
	Limit int
}

// ListBoxes returns boxes, most recently active first by default —
// box.updated_at is literally "something happened under here lately"
// (spec 1.5.1).
func ListBoxes(q Queryer, opt ListBoxesOptions) ([]domain.Box, error) {
	order := `updated_at DESC, id DESC`
	if opt.SortByName {
		order = `name, id`
	}
	query := `SELECT ` + boxColumns + ` FROM boxes`
	args := []any{}
	if opt.Tag != nil {
		query = `WITH RECURSIVE sub(id) AS (
   SELECT id FROM tags WHERE id = ?
   UNION SELECT t.id FROM tags t JOIN sub ON t.parent_id = sub.id
  ) ` + query + ` WHERE EXISTS (SELECT 1 FROM box_tags bt JOIN sub ON bt.tag_id = sub.id WHERE bt.box_id = boxes.id)`
		args = append(args, *opt.Tag)
	}
	query += ` ORDER BY ` + order
	if opt.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, opt.Limit)
	}

	rows, err := q.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list boxes: %w", err)
	}
	defer rows.Close()

	var out []domain.Box
	for rows.Next() {
		b, _, err := scanBox(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListTags returns every tag, for the one caller that wants the whole tree to
// render it. Asking about one tag or one subtree goes through TagByName,
// TagByID or TagDescendants instead.
//
// There is no paging and no path column: the tree is a few thousand nodes, and
// spec 1.6 trades the path column for a recursive query so that moving a
// subtree stays one row.
func ListTags(q Queryer) ([]domain.Tag, error) {
	rows, err := q.Query(`SELECT id, parent_id, name, name_fold FROM tags ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	var out []domain.Tag
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.ID, &t.ParentID, &t.Name, &t.NameFold); err != nil {
			return nil, fmt.Errorf("list tags: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// BoxTags returns the tags attached directly to a box.
func BoxTags(q Queryer, box domain.BoxID) ([]domain.TagID, error) {
	rows, err := q.Query(`SELECT tag_id FROM box_tags WHERE box_id = ? ORDER BY tag_id`, box)
	if err != nil {
		return nil, fmt.Errorf("read tags of box %s: %w", box, err)
	}
	defer rows.Close()

	var out []domain.TagID
	for rows.Next() {
		var id domain.TagID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("read tags of box %s: %w", box, err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// FileTags returns the tags attached to a file.
func FileTags(q Queryer, file domain.FileID) ([]domain.TagID, error) {
	rows, err := q.Query(`SELECT tag_id FROM file_tags WHERE file_id = ? ORDER BY tag_id`, file)
	if err != nil {
		return nil, fmt.Errorf("read tags of file %s: %w", file, err)
	}
	defer rows.Close()

	var out []domain.TagID
	for rows.Next() {
		var id domain.TagID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("read tags of file %s: %w", file, err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// TagDirectCounts returns how many things hang directly off each tag.
//
// The counts are deliberately not a column on tags: they are derived on
// demand, so there is nothing to drift (spec 1.6). This is the one query in
// the tag path that scales with the size of file_tags rather than with the
// tree, which is why only the tag selector asks for it.
func TagDirectCounts(q Queryer) (map[domain.TagID]int, error) {
	rows, err := q.Query(
		`SELECT tag_id, count(*) FROM box_tags GROUP BY tag_id
		 UNION ALL
		 SELECT tag_id, count(*) FROM file_tags GROUP BY tag_id`)
	if err != nil {
		return nil, fmt.Errorf("count tag usage: %w", err)
	}
	defer rows.Close()

	out := map[domain.TagID]int{}
	for rows.Next() {
		var id domain.TagID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, fmt.Errorf("count tag usage: %w", err)
		}
		out[id] += n
	}
	return out, rows.Err()
}
