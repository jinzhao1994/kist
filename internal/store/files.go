package store

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/jinzhao1994/kist/internal/domain"
)

// fileOrder is the one sort order for file listings (spec 4.5.1).
//
// NULLS LAST is written out even though SQLite would do it anyway: Postgres
// defaults DESC to NULLS FIRST, so leaving it implicit means the two engines
// return opposite results and no test run on SQLite can ever notice
// (spec 4.7).
//
// id DESC is not only a tie-breaker. Ids are UUIDv7, so it is also created_at
// DESC — which is why files with no captured_at fall into import order without
// a COALESCE, and without the expression index a COALESCE would have needed.
const fileOrder = ` ORDER BY captured_at DESC NULLS LAST, id DESC`

// EncodeCursor renders the sort tuple of the last row on a page.
//
// The cursor carries the whole tuple, not just the id, because the predicate
// has to reconstruct "everything after this row in this order". It is opaque
// base64 so that changing the sort key later invalidates old cursors loudly
// instead of paging through a different order (spec 4.5).
func EncodeCursor(f domain.File) string {
	return base64.RawURLEncoding.EncodeToString([]byte(f.CapturedAt.String() + "\x00" + f.ID.String()))
}

func decodeCursor(s string) (captured string, id domain.FileID, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", domain.FileID{}, fmt.Errorf("bad cursor: %w", err)
	}
	before, after, ok := strings.Cut(string(raw), "\x00")
	if !ok {
		return "", domain.FileID{}, fmt.Errorf("bad cursor: missing separator")
	}
	parsed, err := domain.ParseID(after)
	if err != nil {
		return "", domain.FileID{}, fmt.Errorf("bad cursor: %w", err)
	}
	return before, domain.FileID{ID: parsed}, nil
}

// cursorPredicate is "strictly after this row, in fileOrder".
//
// The NULL block is the part that is easy to get wrong. Under DESC NULLS LAST
// every null sorts after every non-null, so:
//
//   - from a non-null cursor, the remainder is "smaller captured_at, or equal
//     captured_at with smaller id, or null captured_at"
//   - from a null cursor, the remainder is null captured_at with smaller id
//     and nothing else — the non-null block is already behind us
//
// Getting this wrong does not error; it silently drops or repeats a page,
// which is why TestListFilesCursorWalksEveryRowExactlyOnce steps one row at a
// time across the boundary.
func cursorPredicate(captured string, id domain.FileID) (string, []any) {
	if captured == "" {
		return ` AND captured_at IS NULL AND id < ?`, []any{id}
	}
	return ` AND (captured_at IS NULL OR captured_at < ? OR (captured_at = ? AND id < ?))`,
		[]any{captured, captured, id}
}

// ListFilesOptions is the M1c subset of spec 4.2's Query.
//
// The full AST — time ranges, media kinds, arbitrary AND/OR/NOT nesting —
// arrives with the conformance suite in M2. What is here is what the CLI can
// actually ask for, which is also the shape the UI exposes in v0.1: several
// tags intersected, plus an exclusion list (spec 4.3).
type ListFilesOptions struct {
	Box    *domain.BoxID
	Folder *domain.FolderID
	// Text is matched as a folded substring against name_fold. Callers pass it
	// raw; ListFiles folds it with the same function that built the column.
	Text string
	// AllOf is one entry per condition; a file must carry at least one tag
	// from each. library expands each condition to its descendants first
	// (spec 4.3 step 1) — that expansion is a memory-tree walk, not a join.
	AllOf [][]domain.TagID
	// NoneOf excludes files carrying any of these.
	NoneOf []domain.TagID
	Cursor string
	Limit  int
}

// ListFiles runs the query and returns the page plus the cursor for the next
// one. An empty next cursor means this was the last page.
func ListFiles(q Queryer, opt ListFilesOptions) ([]domain.File, string, error) {
	var (
		sb   strings.Builder
		args []any
	)
	sb.WriteString(`SELECT ` + fileColumns + ` FROM files WHERE 1 = 1`)

	if opt.Box != nil {
		sb.WriteString(` AND box_id = ?`)
		args = append(args, *opt.Box)
	}
	if opt.Folder != nil {
		sb.WriteString(` AND folder_id = ?`)
		args = append(args, *opt.Folder)
	}
	if opt.Text != "" {
		// ESCAPE is not optional: without it a search for "100%" matches
		// everything, and a search for "a_b" matches "axb".
		sb.WriteString(` AND name_fold LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(domain.Fold(opt.Text))+"%")
	}
	for _, cond := range opt.AllOf {
		if len(cond) == 0 {
			continue
		}
		sb.WriteString(` AND EXISTS (SELECT 1 FROM file_tags WHERE file_id = files.id AND tag_id IN (` +
			placeholders(len(cond)) + `))`)
		for _, id := range cond {
			args = append(args, id)
		}
	}
	if len(opt.NoneOf) > 0 {
		sb.WriteString(` AND NOT EXISTS (SELECT 1 FROM file_tags WHERE file_id = files.id AND tag_id IN (` +
			placeholders(len(opt.NoneOf)) + `))`)
		for _, id := range opt.NoneOf {
			args = append(args, id)
		}
	}
	if opt.Cursor != "" {
		captured, id, err := decodeCursor(opt.Cursor)
		if err != nil {
			return nil, "", err
		}
		pred, pargs := cursorPredicate(captured, id)
		sb.WriteString(pred)
		args = append(args, pargs...)
	}

	sb.WriteString(fileOrder)
	if opt.Limit > 0 {
		sb.WriteString(` LIMIT ?`)
		args = append(args, opt.Limit)
	}

	out, err := queryFiles(q, sb.String(), args...)
	if err != nil {
		return nil, "", err
	}

	// A short page is the last page; only a full one can have more behind it.
	next := ""
	if opt.Limit > 0 && len(out) == opt.Limit {
		next = EncodeCursor(out[len(out)-1])
	}
	return out, next, nil
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
