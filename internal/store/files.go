package store

import (
	"fmt"
	"strings"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

// fileOrder is the ordering used by the listings that do not take a Query:
// directory browsing and the projection's whole-box reads.
const fileOrder = ` ORDER BY captured_at DESC NULLS LAST, id DESC`

// orderBy renders one ordering, with the tie-breaker that makes keyset paging
// possible at all.
//
// NULLS LAST is written out in both directions. SQLite puts nulls last under
// DESC anyway and first under ASC, and Postgres does the opposite for DESC —
// so leaving it implicit means the two engines disagree and no test run on
// SQLite can ever notice (spec 4.7). Unknown belongs at the end whichever way
// you are reading, so both directions say LAST.
//
// The id tie-breaker is not decoration: without it two files sharing a capture
// time can swap places between pages, and keyset paging then repeats one and
// drops the other (spec 4.5).
func orderBy(s query.SortSpec) string {
	dir := " DESC"
	if s.Asc {
		dir = " ASC"
	}
	switch s.Key {
	case query.SortImported:
		// The id is the key, so there is nothing left to break ties between.
		return ` ORDER BY id` + dir
	case query.SortName:
		// name_fold is NOT NULL, so no NULLS clause is needed or emitted.
		return ` ORDER BY name_fold` + dir + `, id` + dir
	default:
		return ` ORDER BY captured_at` + dir + ` NULLS LAST, id` + dir
	}
}

// cursorPredicate is "strictly after this row, in this order".
//
// The nulls block is the part that is easy to get wrong, and it is not
// symmetric with intuition: nulls sort last in BOTH directions (see orderBy),
// so from a non-null cursor the remainder always still contains the whole
// nulls block, and from a null cursor it contains nothing but the rest of that
// block.
//
// Getting this wrong does not error. It silently drops or repeats a page,
// which is why TestCursorWalksEveryRowExactlyOnceInEveryOrder steps one row at
// a time across the boundary in all six orderings.
func cursorPredicate(s query.SortSpec, key string, id domain.FileID) (string, []any) {
	cmp := " < ?"
	if s.Asc {
		cmp = " > ?"
	}

	switch s.Key {
	case query.SortImported:
		return ` AND id` + cmp, []any{id}

	case query.SortName:
		return ` AND (name_fold` + cmp + ` OR (name_fold = ? AND id` + cmp + `))`,
			[]any{key, key, id}

	default:
		if key == "" {
			// The cursor is inside the nulls block, which is last in both
			// directions — so everything not yet visited is inside it too.
			return ` AND captured_at IS NULL AND id` + cmp, []any{id}
		}
		return ` AND (captured_at IS NULL OR captured_at` + cmp +
				` OR (captured_at = ? AND id` + cmp + `))`,
			[]any{key, key, id}
	}
}

// ListFiles answers a query and returns the page plus the cursor for the next
// one. An empty next cursor means this was the last page.
//
// It reads one row more than the page size to decide whether there is a next
// page, rather than assuming a full page has more behind it. The assumption is
// wrong exactly when the last page is exactly full, and the symptom is a "more"
// control that leads to an empty page.
func ListFiles(q Queryer, qry query.Query) ([]domain.File, string, error) {
	where, args, err := translate(qry)
	if err != nil {
		return nil, "", err
	}

	var sb strings.Builder
	sb.WriteString(`SELECT ` + fileColumns + ` FROM files WHERE 1 = 1`)
	sb.WriteString(where)

	if qry.Cursor != "" {
		key, id, err := query.DecodeCursor(qry.Sort, qry.Cursor)
		if err != nil {
			return nil, "", err
		}
		pred, pargs := cursorPredicate(qry.Sort, key, id)
		sb.WriteString(pred)
		args = append(args, pargs...)
	}

	size := qry.PageSize()
	sb.WriteString(orderBy(qry.Sort))
	sb.WriteString(` LIMIT ?`)
	args = append(args, size+1)

	out, err := queryFiles(q, sb.String(), args...)
	if err != nil {
		return nil, "", err
	}
	if len(out) <= size {
		return out, "", nil
	}
	out = out[:size]
	return out, query.EncodeCursor(qry.Sort, out[len(out)-1]), nil
}

// escapeLike neutralizes LIKE's wildcards so a pattern matches literally.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func queryFiles(q Queryer, query string, args ...any) ([]domain.File, error) {
	rows, err := q.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	defer rows.Close()

	var out []domain.File
	for rows.Next() {
		f, err := scanFile(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("list files: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FilesInFolder lists one directory's files, in the standard order.
func FilesInFolder(q Queryer, folder domain.FolderID) ([]domain.File, error) {
	return queryFiles(q, `SELECT `+fileColumns+` FROM files WHERE folder_id = ?`+fileOrder, folder)
}

// FilesOfBox lists every file in a box.
//
// relink needs the whole set to compute the desired projection, so memory is
// bounded by the largest box rather than by the library (spec 3.4.1). The
// order is by id — creation order — because the projection resolves name
// collisions that way and a stable input makes that resolution reproducible.
func FilesOfBox(q Queryer, box domain.BoxID) ([]domain.File, error) {
	return queryFiles(q, `SELECT `+fileColumns+` FROM files WHERE box_id = ? ORDER BY id`, box)
}

// FoldersOfBox returns every folder of a box, root first, then the rest in id
// order — which is creation order, so a parent always precedes its children.
func FoldersOfBox(q Queryer, box domain.BoxID) ([]domain.Folder, error) {
	rows, err := q.Query(
		`SELECT `+folderColumns+` FROM folders WHERE box_id = ?
		 ORDER BY (parent_id IS NOT NULL), id`, box)
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	defer rows.Close()

	var out []domain.Folder
	for rows.Next() {
		f, err := scanFolder(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("list folders: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ExistingNames maps folder → folded name → blob key, for one box.
//
// This is what makes import idempotent (spec 3.7.1 ②): a name already taken by
// the same blob is skipped, a name taken by different content is numbered.
// Loading it up front costs one scan of the box and turns a per-file query
// into a map lookup — which matters at 5000 files, and matters more because
// the alternative is a query inside the loop that would also have to see the
// files this same import added a moment ago.
func ExistingNames(q Queryer, box domain.BoxID) (map[domain.FolderID]map[string]string, error) {
	rows, err := q.Query(`SELECT folder_id, name_fold, blob_key FROM files WHERE box_id = ?`, box)
	if err != nil {
		return nil, fmt.Errorf("read existing names: %w", err)
	}
	defer rows.Close()

	out := map[domain.FolderID]map[string]string{}
	for rows.Next() {
		var (
			folder domain.FolderID
			fold   string
			key    string
		)
		if err := rows.Scan(&folder, &fold, &key); err != nil {
			return nil, fmt.Errorf("read existing names: %w", err)
		}
		if out[folder] == nil {
			out[folder] = map[string]string{}
		}
		out[folder][fold] = key
	}
	return out, rows.Err()
}
