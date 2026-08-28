package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
)

// TagByID reads one tag.
func TagByID(q Queryer, id domain.TagID) (domain.Tag, bool, error) {
	return scanTag(q.QueryRow(
		`SELECT id, parent_id, name, name_fold FROM tags WHERE id = ?`, id))
}

// TagByName reads one tag by name, folded the same way the unique index is so
// that "Passport" and "passport" find the same tag (spec 1.6, 4.7).
func TagByName(q Queryer, name string) (domain.Tag, bool, error) {
	return scanTag(q.QueryRow(
		`SELECT id, parent_id, name, name_fold FROM tags WHERE name_fold = ?`,
		domain.Fold(name)))
}

func scanTag(row *sql.Row) (domain.Tag, bool, error) {
	var t domain.Tag
	err := row.Scan(&t.ID, &t.ParentID, &t.Name, &t.NameFold)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Tag{}, false, nil
	}
	if err != nil {
		return domain.Tag{}, false, fmt.Errorf("read tag: %w", err)
	}
	return t, true, nil
}

// TagDescendants returns the given tags plus everything under them: the id set
// a tag filter expands to (spec 4.3).
//
// It walks the tree in SQL rather than reading every tag and walking in Go.
// The cost then scales with the subtree asked about instead of with the size
// of the library's tag tree, which matters more on Postgres — a round trip
// there is not free — than it does on a local SQLite file.
//
// UNION rather than UNION ALL. The deduplication is what makes the recursion
// terminate: two roots in the same subtree would otherwise revisit the shared
// part, and a cycle — which MoveTag refuses to create, but which a corrupt
// views table could still hold — would recurse until the query is killed.
// This form is identical on SQLite and Postgres (spec 4.7).
func TagDescendants(q Queryer, roots []domain.TagID) ([]domain.TagID, error) {
	if len(roots) == 0 {
		return nil, nil
	}
	args := make([]any, len(roots))
	for i, id := range roots {
		args[i] = id
	}
	rows, err := q.Query(`
		WITH RECURSIVE sub(id) AS (
			SELECT id FROM tags WHERE id IN (`+placeholders(len(roots))+`)
			UNION
			SELECT t.id FROM tags t JOIN sub ON t.parent_id = sub.id
		)
		SELECT id FROM sub ORDER BY id`, args...)
	if err != nil {
		return nil, fmt.Errorf("resolve tag descendants: %w", err)
	}
	defer rows.Close()
	return scanTagIDs(rows, "resolve tag descendants")
}

// TagAncestors returns a tag and every tag above it, as a set — the order is
// not the root-to-leaf path.
//
// MoveTag uses it to refuse a cycle. Walking up rather than down is what makes
// that check cheap: it costs the depth of the tree, where asking "is the new
// parent somewhere under this subtree" would cost the whole subtree.
func TagAncestors(q Queryer, id domain.TagID) ([]domain.TagID, error) {
	rows, err := q.Query(`
		WITH RECURSIVE up(id, parent_id) AS (
			SELECT id, parent_id FROM tags WHERE id = ?
			UNION
			SELECT t.id, t.parent_id FROM tags t JOIN up ON t.id = up.parent_id
		)
		SELECT id FROM up ORDER BY id`, id)
	if err != nil {
		return nil, fmt.Errorf("resolve tag ancestors: %w", err)
	}
	defer rows.Close()
	return scanTagIDs(rows, "resolve tag ancestors")
}

func scanTagIDs(rows *sql.Rows, what string) ([]domain.TagID, error) {
	var out []domain.TagID
	for rows.Next() {
		var id domain.TagID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("%s: %w", what, err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
