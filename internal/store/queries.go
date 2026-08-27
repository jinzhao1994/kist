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
	Limit      int
}

// ListBoxes returns boxes, most recently active first by default —
// box.updated_at is literally "something happened under here lately"
// (spec 1.5.1).
func ListBoxes(q Queryer, opt ListBoxesOptions) ([]domain.Box, error) {
	order := `updated_at DESC, id DESC`
	if opt.SortByName {
		order = `name, id`
	}
	query := `SELECT ` + boxColumns + ` FROM boxes ORDER BY ` + order
	args := []any{}
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

// ListTags returns every tag. The tree is a few thousand nodes and lives in
// memory (spec 1.6), so there is no paging and no path column.
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

// TagDirectCounts returns how many things hang directly off each tag.
//
// One GROUP BY at startup, then library keeps it up to date incrementally
// (spec 1.6). The counts are deliberately not a column on tags: a restart
// re-derives them, so there is nothing to drift.
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
