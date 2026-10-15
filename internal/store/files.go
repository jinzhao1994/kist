package store

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

// fileOrder is the ordering used by the listings that do not take a Query:
// directory browsing and the projection's whole-box reads.
const fileOrder = ` ORDER BY captured_at DESC NULLS LAST, id DESC`

// orderBy renders one ordering, with the tie-breaker that makes keyset paging
// possible at all.
//
// NULLS LAST is written out in both directions. Unknown belongs at the end
// whichever way you are reading, and SQLite does not agree on its own: it
// treats NULL as the smallest value, so ASC would lead with the files whose
// capture time is unknown. DESC happens to come out right without it, which is
// the reason to write it anyway — one ordering spelled two ways is the shape
// that rots (spec 4.7).
//
// The id tie-breaker is not decoration: without it two files sharing a capture
// time can swap places between pages, and keyset paging then repeats one and
// drops the other (spec 4.5).
func orderBy(s query.SortSpec) string {
	dir := " DESC"
	if s.Asc {
		dir = " ASC"
	}
	if key := extraFileSortKey(s.Key); key != "" {
		return ` ORDER BY ` + key + dir + `, id` + dir
	}
	switch s.Key {
	case query.SortUpdated:
		return ` ORDER BY updated_at` + dir + `, id` + dir
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

	if expression := extraFileSortKey(s.Key); expression != "" {
		var value any = key
		if s.Key == query.SortSize {
			value, _ = strconv.ParseInt(key, 10, 64)
		}
		return ` AND (` + expression + cmp + ` OR (` + expression + ` = ? AND id` + cmp + `))`, []any{value, value, id}
	}
	switch s.Key {
	case query.SortUpdated:
		return ` AND (updated_at` + cmp + ` OR (updated_at = ? AND id` + cmp + `))`, []any{key, key, id}
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
func ListFiles(q Queryer, qry query.Query, zone *time.Location) ([]domain.File, string, error) {
	where, args, err := translate(qry, zone)
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
	last := out[len(out)-1]
	if expression := extraFileSortKey(qry.Sort.Key); expression != "" {
		var value string
		if err := q.QueryRow(`SELECT `+expression+` FROM files WHERE id=?`, last.ID).Scan(&value); err != nil {
			return nil, "", err
		}
		return out, query.EncodeCursorValue(qry.Sort, value, last.ID), nil
	}
	return out, query.EncodeCursor(qry.Sort, last), nil
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
	return queryFiles(q, `SELECT `+fileColumns+` FROM files
		WHERE folder_id IN (`+FoldersOfBoxSQL+`) ORDER BY id`, box)
}

// FolderChain returns the box a folder belongs to, and the names from the box
// root down to that folder — the root's own empty name excluded.
//
// It walks up, so it reads only the folder's ancestors. The old shape read
// every folder of the box, because a file carried a box_id and nothing else
// pointed anywhere; that cost the whole box to answer a question about one
// path (see open-questions "find 每一行都单独解析一次路径").
func FolderChain(q Queryer, folder domain.FolderID) (domain.BoxID, []string, error) {
	rows, err := q.Query(`
		WITH RECURSIVE up(id, parent_id, name, depth) AS (
			SELECT id, parent_id, name, 0 FROM folders WHERE id = ?
		  UNION
			SELECT d.id, d.parent_id, d.name, up.depth + 1
			FROM folders d JOIN up ON d.id = up.parent_id
		)
		SELECT up.name, up.parent_id IS NULL, b.id
		FROM up LEFT JOIN boxes b ON b.root_folder_id = up.id
		ORDER BY up.depth DESC`, folder)
	if err != nil {
		return domain.BoxID{}, nil, fmt.Errorf("resolve folder chain: %w", err)
	}
	defer rows.Close()

	var (
		box   domain.BoxID
		names []string
	)
	for rows.Next() {
		var (
			name   string
			isRoot bool
			owner  []byte // NULL when the chain ends at a root no box claims
		)
		if err := rows.Scan(&name, &isRoot, &owner); err != nil {
			return domain.BoxID{}, nil, fmt.Errorf("resolve folder chain: %w", err)
		}
		if isRoot {
			if owner != nil {
				if err := box.Scan(owner); err != nil {
					return domain.BoxID{}, nil, fmt.Errorf("resolve folder chain: %w", err)
				}
			}
			continue // the root's name is "" and contributes nothing to the path
		}
		names = append(names, name)
	}
	return box, names, rows.Err()
}

// FoldersOfBoxSQL is a subquery yielding every folder id of one box, root
// included. Its single "?" is the box id, so callers embed it wherever they
// would once have written "box_id = ?".
//
// A folder's box is the box whose root it descends from, and since folders and
// files stopped carrying a box_id that is the only place the answer lives
// (spec 1.4). The walk therefore costs one recursive descent per query instead
// of one denormalised column per row — and the column that used to be here
// could disagree with the tree, which this cannot.
//
// UNION rather than UNION ALL, for the reason TagDescendants gives: the
// deduplication is what makes a corrupt cycle terminate rather than recurse
// until the query is killed.
const FoldersOfBoxSQL = `
	WITH RECURSIVE box_folders(id) AS (
		SELECT root_folder_id FROM boxes WHERE id = ?
		UNION
		SELECT f.id FROM folders f JOIN box_folders ON f.parent_id = box_folders.id
	)
	SELECT id FROM box_folders`

// FoldersOfBox returns every folder of a box, root first, then the rest in id
// order — which is creation order, so a parent always precedes its children.
func FoldersOfBox(q Queryer, box domain.BoxID) ([]domain.Folder, error) {
	rows, err := q.Query(
		`SELECT `+folderColumns+` FROM folders WHERE id IN (`+FoldersOfBoxSQL+`)
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
	rows, err := q.Query(
		`SELECT folder_id, name_fold, blob_key FROM files
		 WHERE folder_id IN (`+FoldersOfBoxSQL+`)`, box)
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
