package store

import "github.com/jinzhao1994/kist/internal/domain"

// TagCounts counts distinct entities carrying a tag or one of its descendants.
type TagCounts struct {
	Boxes int
	Files int
}

// TagSubtreeCounts deduplicates each entity across all tags in a subtree.
// UNION makes the ancestry walk terminate even if a damaged view has a cycle.
func TagSubtreeCounts(q Queryer) (map[domain.TagID]TagCounts, error) {
	rows, err := q.Query(`WITH RECURSIVE sub(root, id) AS (
 SELECT id, id FROM tags
 UNION
 SELECT sub.root, t.id FROM tags t JOIN sub ON t.parent_id = sub.id
 ), matches(root, kind, entity) AS (
 SELECT sub.root, 0, bt.box_id FROM sub JOIN box_tags bt ON bt.tag_id = sub.id
 UNION
 SELECT sub.root, 1, ft.file_id FROM sub JOIN file_tags ft ON ft.tag_id = sub.id
 ) SELECT root, kind, COUNT(*) FROM matches GROUP BY root, kind`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[domain.TagID]TagCounts{}
	for rows.Next() {
		var id domain.TagID
		var kind, count int
		if err := rows.Scan(&id, &kind, &count); err != nil {
			return nil, err
		}
		c := out[id]
		if kind == 0 {
			c.Boxes = count
		} else {
			c.Files = count
		}
		out[id] = c
	}
	return out, rows.Err()
}
