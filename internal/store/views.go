package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

// ViewTx is the materialized-view side of one transaction.
//
// It structurally satisfies oplog.Tx. store does not import oplog and oplog
// does not import store; library is what connects them. The ts it carries is
// the op's ts, and TS() is the only clock a handler can reach — that is how
// "timestamps come from ops.ts, never time.Now()" (spec 2.3.1) stops being a
// rule people have to remember.
type ViewTx struct {
	tx *sql.Tx
	ts time.Time
}

// NewViewTx binds a transaction to the op being applied inside it.
func NewViewTx(tx *sql.Tx, ts time.Time) *ViewTx {
	return &ViewTx{tx: tx, ts: ts.UTC()}
}

// TS is the timestamp of the op being applied.
func (v *ViewTx) TS() time.Time { return v.ts }

// Queryer exposes the transaction to the package's read helpers, so a caller
// already inside a transaction sees its own uncommitted writes.
func (v *ViewTx) Queryer() Queryer { return v.tx }

// SQL exposes the underlying transaction to library, which needs it for the
// ops table itself. Handlers only ever see the oplog.Tx interface, which does
// not include this method.
func (v *ViewTx) SQL() *sql.Tx { return v.tx }

// ── boxes ────────────────────────────────────────────────────────────────

func (v *ViewTx) InsertBox(b domain.Box) error {
	_, err := v.tx.Exec(
		`INSERT INTO boxes (id, name, name_fold, description,
		                    created_at, updated_at, start_at, end_at, root_folder_id, cover_file_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.Name, b.NameFold, b.Description,
		FormatTime(v.ts), FormatTime(v.ts), b.StartAt, b.EndAt, b.RootFolderID, b.CoverFileID)
	if err != nil {
		return fmt.Errorf("insert box %s: %w", b.ID, err)
	}
	return nil
}

// UpdateBox rewrites the semantic fields, and only those.
//
// Neither timestamp is touched. created_at is set once by InsertBox and must
// never move; updated_at belongs entirely to the write path's propagation step
// (spec 1.5.1 ②), because whether it moves depends on the op's source, and a
// handler has no business knowing that. Writing it here would let a
// maintenance op — a re-extraction, a relink — stamp the row as freshly
// modified, which is precisely the failure that rule exists to prevent.
func (v *ViewTx) UpdateBox(b domain.Box) error {
	res, err := v.tx.Exec(
		`UPDATE boxes SET name = ?, name_fold = ?, description = ?,
		                  start_at = ?, end_at = ?, cover_file_id = ?
		 WHERE id = ?`,
		b.Name, b.NameFold, b.Description,
		b.StartAt, b.EndAt, b.CoverFileID, b.ID)
	if err != nil {
		return fmt.Errorf("update box %s: %w", b.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("update box %s: no such box", b.ID)
	}
	return nil
}

func (v *ViewTx) DeleteBox(id domain.BoxID) error {
	if _, err := v.tx.Exec(`DELETE FROM boxes WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete box %s: %w", id, err)
	}
	return nil
}

func (v *ViewTx) GetBox(id domain.BoxID) (domain.Box, bool, error) {
	return scanBox(v.tx.QueryRow(`SELECT `+boxColumns+` FROM boxes WHERE id = ?`, id).Scan)
}

// boxDisplayColumns leaves semantic op snapshots independent of derived totals.
const boxDisplayColumns = boxColumns + `, (SELECT total_size FROM folders WHERE id = boxes.root_folder_id)`

func scanDisplayBox(scan func(...any) error) (domain.Box, bool, error) {
	var size int64
	b, found, err := scanBox(func(args ...any) error { return scan(append(args, &size)...) })
	b.TotalSize = size
	return b, found, err
}

// DisplayBox includes the root folder's materialized size in the same read.
func DisplayBox(q Queryer, id domain.BoxID) (domain.Box, bool, error) {
	return scanDisplayBox(q.QueryRow(`SELECT `+boxDisplayColumns+` FROM boxes WHERE id = ?`, id).Scan)
}

const boxColumns = `id, root_folder_id, name, name_fold, description, created_at, updated_at, start_at, end_at, cover_file_id`

func scanBox(scan func(...any) error) (domain.Box, bool, error) {
	var (
		b                domain.Box
		created, updated int64
		start, end       sql.NullString
	)
	err := scan(&b.ID, &b.RootFolderID, &b.Name, &b.NameFold, &b.Description,
		&created, &updated, &start, &end, &b.CoverFileID)
	if err == sql.ErrNoRows {
		return domain.Box{}, false, nil
	}
	if err != nil {
		return domain.Box{}, false, fmt.Errorf("read box: %w", err)
	}
	b.CreatedAt = ParseTime(created)
	b.UpdatedAt = ParseTime(updated)
	if start.Valid {
		if b.StartAt, err = domain.ParseLocalDate(start.String); err != nil {
			return domain.Box{}, false, err
		}
	}
	if end.Valid {
		if b.EndAt, err = domain.ParseLocalDate(end.String); err != nil {
			return domain.Box{}, false, err
		}
	}
	return b, true, nil
}

// TouchBoxes bumps updated_at on a deduplicated set of boxes.
//
// Deduplication is the point (spec 1.5.1 ③): tagging 500 files across 12 boxes
// must write 12 rows, not 500. Missing rows are ignored — undoing a create
// deletes the box, and the propagation pass that follows should not fail
// because its target is gone.
func (v *ViewTx) TouchBoxes(ids []domain.BoxID) error {
	seen := make(map[domain.BoxID]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		if _, err := v.tx.Exec(`UPDATE boxes SET updated_at = ? WHERE id = ?`, FormatTime(v.ts), id); err != nil {
			return fmt.Errorf("touch box %s: %w", id, err)
		}
	}
	return nil
}

func (v *ViewTx) CountBoxTags(box domain.BoxID) (int, error) {
	var n int
	if err := v.tx.QueryRow(`SELECT count(*) FROM box_tags WHERE box_id = ?`, box).Scan(&n); err != nil {
		return 0, fmt.Errorf("count tags on box %s: %w", box, err)
	}
	return n, nil
}

// ── folders ──────────────────────────────────────────────────────────────

func (v *ViewTx) InsertFolder(f domain.Folder) error {
	_, err := v.tx.Exec(
		`INSERT INTO folders (id, parent_id, name, name_fold, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		f.ID, f.ParentID, f.Name, f.NameFold, FormatTime(v.ts), FormatTime(v.ts))
	if err != nil {
		return fmt.Errorf("insert folder %s: %w", f.ID, err)
	}
	return nil
}

// ── tags ─────────────────────────────────────────────────────────────────

func (v *ViewTx) InsertTag(t domain.Tag) error {
	if _, err := v.tx.Exec(
		`INSERT INTO tags (id, parent_id, name, name_fold) VALUES (?, ?, ?, ?)`,
		t.ID, t.ParentID, t.Name, t.NameFold); err != nil {
		return fmt.Errorf("insert tag %q: %w", t.Name, err)
	}
	return nil
}

func (v *ViewTx) UpdateTag(t domain.Tag) error {
	res, err := v.tx.Exec(
		`UPDATE tags SET parent_id = ?, name = ?, name_fold = ? WHERE id = ?`,
		t.ParentID, t.Name, t.NameFold, t.ID)
	if err != nil {
		return fmt.Errorf("update tag %s: %w", t.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("update tag %s: no such tag", t.ID)
	}
	return nil
}

func (v *ViewTx) DeleteTag(id domain.TagID) error {
	if _, err := v.tx.Exec(`DELETE FROM tags WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete tag %s: %w", id, err)
	}
	return nil
}

func (v *ViewTx) GetTag(id domain.TagID) (domain.Tag, bool, error) {
	var t domain.Tag
	err := v.tx.QueryRow(`SELECT id, parent_id, name, name_fold FROM tags WHERE id = ?`, id).
		Scan(&t.ID, &t.ParentID, &t.Name, &t.NameFold)
	if err == sql.ErrNoRows {
		return domain.Tag{}, false, nil
	}
	if err != nil {
		return domain.Tag{}, false, fmt.Errorf("read tag %s: %w", id, err)
	}
	return t, true, nil
}

func (v *ViewTx) CountTagChildren(id domain.TagID) (int, error) {
	var n int
	if err := v.tx.QueryRow(`SELECT count(*) FROM tags WHERE parent_id = ?`, id).Scan(&n); err != nil {
		return 0, fmt.Errorf("count children of tag %s: %w", id, err)
	}
	return n, nil
}

// CountTagUsage counts everything carrying this tag, across boxes and files.
// spec 1.6: deleting a tag that is still in use is refused outright — cascade
// delete in archiving software can wipe a whole classification with one
// misclick, and the op payload needed to undo it would be enormous.
func (v *ViewTx) CountTagUsage(id domain.TagID) (int, error) {
	var n int
	err := v.tx.QueryRow(
		`SELECT (SELECT count(*) FROM box_tags WHERE tag_id = ?)
		      + (SELECT count(*) FROM file_tags WHERE tag_id = ?)`, id, id).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count usage of tag %s: %w", id, err)
	}
	return n, nil
}

// ── box_tags ─────────────────────────────────────────────────────────────

// AddBoxTag is idempotent: a batch op may name a box that already carries the
// tag, and one op means one outcome, not a failure.
func (v *ViewTx) AddBoxTag(box domain.BoxID, tag domain.TagID) error {
	_, err := v.tx.Exec(
		`INSERT INTO box_tags (box_id, tag_id) VALUES (?, ?)
		 ON CONFLICT (box_id, tag_id) DO NOTHING`,
		box, tag)
	if err != nil {
		return fmt.Errorf("tag box %s with %s: %w", box, tag, err)
	}
	return nil
}

func (v *ViewTx) RemoveBoxTag(box domain.BoxID, tag domain.TagID) error {
	if _, err := v.tx.Exec(`DELETE FROM box_tags WHERE box_id = ? AND tag_id = ?`, box, tag); err != nil {
		return fmt.Errorf("untag box %s: %w", box, err)
	}
	return nil
}

func (v *ViewTx) HasBoxTag(box domain.BoxID, tag domain.TagID) (bool, error) {
	var one int
	err := v.tx.QueryRow(`SELECT 1 FROM box_tags WHERE box_id = ? AND tag_id = ?`, box, tag).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check tag %s on box %s: %w", tag, box, err)
	}
	return true, nil
}

// ClearViews empties every rebuildable table.
//
// The order is dictated by the foreign keys: children before parents. ops and
// blobs are absent on purpose — ops is the authority, and blobs is rebuilt by
// walking the blobs/ directory (invariant 1). A version of this function that
// touched either would turn `kist rebuild` into data loss.
func ClearViews(tx *sql.Tx) error {
	for _, table := range []string{
		"fileset_members", "filesets", "box_tags", "file_tag_sources", "file_tags", "tag_rejections",
		"file_metadata", "file_extractions",
		// boxes before folders: a box points at its root folder, so the
		// dependency runs the opposite way from files → folders.
		"files", "boxes", "folders", "tags", "saved_searches",
	} {
		if _, err := tx.Exec(`DELETE FROM ` + table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	return nil
}

// ── blobs ────────────────────────────────────────────────────────────────

// UpsertBlob records a blob in the index.
//
// The blobs table is an index over the blobs/ directory and is rebuildable by
// scanning it (invariant 1), so this is deliberately idempotent: Rebuild
// replays import ops against a blobs table it never cleared, and writing the
// same row again must be a no-op rather than a constraint violation. Purity is
// untouched — identical payload, identical row.
func (v *ViewTx) UpsertBlob(key, sha256 string, size int64) error {
	_, err := v.tx.Exec(
		`INSERT INTO blobs (key, sha256, size) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET sha256 = excluded.sha256, size = excluded.size`,
		key, sha256, size)
	if err != nil {
		return fmt.Errorf("upsert blob %s: %w", key, err)
	}
	return nil
}

// ── files ────────────────────────────────────────────────────────────────

// fileColumns is every column of a file, in the order scanFile expects.
const fileColumns = `id, folder_id, name, name_fold, blob_key, captured_at, created_at, updated_at`

func scanFile(scan func(...any) error) (domain.File, error) {
	var (
		f                domain.File
		created, updated int64
	)
	if err := scan(&f.ID, &f.FolderID, &f.Name, &f.NameFold, &f.BlobKey,
		&f.CapturedAt, &created, &updated); err != nil {
		return domain.File{}, err
	}
	f.CreatedAt = ParseTime(created)
	f.UpdatedAt = ParseTime(updated)
	return f, nil
}

func (v *ViewTx) InsertFile(f domain.File) error {
	_, err := v.tx.Exec(
		`INSERT INTO files (`+fileColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.FolderID, f.Name, f.NameFold, f.BlobKey, f.CapturedAt,
		FormatTime(v.ts), FormatTime(v.ts))
	if err != nil {
		return fmt.Errorf("insert file %s: %w", f.Name, err)
	}
	return nil
}

func (v *ViewTx) GetFile(id domain.FileID) (domain.File, bool, error) {
	f, err := scanFile(v.tx.QueryRow(`SELECT `+fileColumns+` FROM files WHERE id = ?`, id).Scan)
	if err == sql.ErrNoRows {
		return domain.File{}, false, nil
	}
	if err != nil {
		return domain.File{}, false, fmt.Errorf("read file %s: %w", id, err)
	}
	return f, true, nil
}

// DeleteFile removes a file and everything hanging off it.
//
// Metadata and extraction rows are removed too. The library guards new import
// undo commands against losing later metadata (including user tombstones).
// This historical apply behavior remains unchanged so old logs still replay.
//
// The cascade is written out rather than left to ON DELETE CASCADE, because
// the schema states its foreign keys plainly and one clause that behaves
// differently from the others is a thing to trip over later.
func (v *ViewTx) DeleteFile(id domain.FileID) error {
	for _, table := range []string{"file_metadata", "file_extractions"} {
		if _, err := v.tx.Exec(`DELETE FROM `+table+` WHERE file_id = ?`, id); err != nil {
			return fmt.Errorf("delete %s of file %s: %w", table, id, err)
		}
	}
	if _, err := v.tx.Exec(`DELETE FROM files WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete file %s: %w", id, err)
	}
	return nil
}

func (v *ViewTx) CountFileTags(id domain.FileID) (int, error) {
	var n int
	if err := v.tx.QueryRow(`SELECT count(*) FROM file_tags WHERE file_id = ?`, id).Scan(&n); err != nil {
		return 0, fmt.Errorf("count tags on file %s: %w", id, err)
	}
	return n, nil
}

func (v *ViewTx) CountBoxFiles(id domain.BoxID) (int, error) {
	var n int
	if err := v.tx.QueryRow(`SELECT count(*) FROM files WHERE folder_id IN (`+FoldersOfBoxSQL+`)`, id).Scan(&n); err != nil {
		return 0, fmt.Errorf("count files in box %s: %w", id, err)
	}
	return n, nil
}

// ── folders, continued ───────────────────────────────────────────────────

const folderColumns = `id, parent_id, name, name_fold, created_at, updated_at`

func scanFolder(scan func(...any) error) (domain.Folder, error) {
	var (
		f                domain.Folder
		created, updated int64
	)
	if err := scan(&f.ID, &f.ParentID, &f.Name, &f.NameFold, &created, &updated); err != nil {
		return domain.Folder{}, err
	}
	f.CreatedAt = ParseTime(created)
	f.UpdatedAt = ParseTime(updated)
	return f, nil
}

func (v *ViewTx) GetFolder(id domain.FolderID) (domain.Folder, bool, error) {
	f, err := scanFolder(v.tx.QueryRow(`SELECT `+folderColumns+` FROM folders WHERE id = ?`, id).Scan)
	if err == sql.ErrNoRows {
		return domain.Folder{}, false, nil
	}
	if err != nil {
		return domain.Folder{}, false, fmt.Errorf("read folder %s: %w", id, err)
	}
	return f, true, nil
}

func (v *ViewTx) DeleteFolder(id domain.FolderID) error {
	if _, err := v.tx.Exec(`DELETE FROM folders WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete folder %s: %w", id, err)
	}
	return nil
}

func (v *ViewTx) CountFolderFiles(id domain.FolderID) (int, error) {
	var n int
	if err := v.tx.QueryRow(`SELECT count(*) FROM files WHERE folder_id = ?`, id).Scan(&n); err != nil {
		return 0, fmt.Errorf("count files in folder %s: %w", id, err)
	}
	return n, nil
}

func (v *ViewTx) CountFolderChildren(id domain.FolderID) (int, error) {
	var n int
	if err := v.tx.QueryRow(`SELECT count(*) FROM folders WHERE parent_id = ?`, id).Scan(&n); err != nil {
		return 0, fmt.Errorf("count subfolders of %s: %w", id, err)
	}
	return n, nil
}

// FolderChainOfFiles returns every folder on the path from these files up to
// their box roots.
//
// It exists so the write path can bump updated_at all the way up (spec 1.5.1)
// without any handler having to remember to. The recursion walks parent_id
// upward and stops at the root, whose parent_id is NULL.
func (v *ViewTx) FolderChainOfFiles(files []domain.FileID) ([]domain.FolderID, error) {
	if len(files) == 0 {
		return nil, nil
	}
	args := make([]any, len(files))
	for i, id := range files {
		args[i] = id
	}
	rows, err := v.tx.Query(`
		WITH RECURSIVE chain(id) AS (
			SELECT folder_id FROM files WHERE id IN (`+placeholders(len(files))+`)
			UNION
			SELECT f.parent_id FROM folders f JOIN chain c ON f.id = c.id
			WHERE f.parent_id IS NOT NULL
		)
		SELECT id FROM chain`, args...)
	if err != nil {
		return nil, fmt.Errorf("folder chain: %w", err)
	}
	defer rows.Close()

	var out []domain.FolderID
	for rows.Next() {
		var id domain.FolderID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("folder chain: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// TouchFolders stamps updated_at on the given folders. Same rule and same
// caller as TouchBoxes: the write path, never a handler.
func (v *ViewTx) TouchFolders(ids []domain.FolderID) error {
	for _, id := range ids {
		if _, err := v.tx.Exec(`UPDATE folders SET updated_at = ? WHERE id = ?`, FormatTime(v.ts), id); err != nil {
			return fmt.Errorf("touch folder %s: %w", id, err)
		}
	}
	return nil
}

// ── file_tags ────────────────────────────────────────────────────────────

// AddFileTag writes the claim and, if this is the first one, the tag.
//
// The tag row is an upsert rather than an insert because a second source
// claiming a tag that is already there must not fail — that is the whole point
// of splitting the two tables.
func (v *ViewTx) AddFileTag(file domain.FileID, tag domain.TagID, source string) error {
	if _, err := v.tx.Exec(
		`INSERT INTO file_tags (file_id, tag_id) VALUES (?, ?)
		 ON CONFLICT (file_id, tag_id) DO NOTHING`, file, tag); err != nil {
		return fmt.Errorf("tag file %s: %w", file, err)
	}
	if _, err := v.tx.Exec(
		`INSERT INTO file_tag_sources (file_id, tag_id, source) VALUES (?, ?, ?)`,
		file, tag, source); err != nil {
		return fmt.Errorf("record who tagged file %s: %w", file, err)
	}
	return nil
}

// RemoveFileTag withdraws one claim, and drops the tag once none is left.
func (v *ViewTx) RemoveFileTag(file domain.FileID, tag domain.TagID, source string) error {
	if _, err := v.tx.Exec(
		`DELETE FROM file_tag_sources WHERE file_id = ? AND tag_id = ? AND source = ?`,
		file, tag, source); err != nil {
		return fmt.Errorf("untag file %s: %w", file, err)
	}
	if _, err := v.tx.Exec(
		`DELETE FROM file_tags WHERE file_id = ? AND tag_id = ?
		   AND NOT EXISTS (SELECT 1 FROM file_tag_sources WHERE file_id = ? AND tag_id = ?)`,
		file, tag, file, tag); err != nil {
		return fmt.Errorf("untag file %s: %w", file, err)
	}
	return nil
}

func (v *ViewTx) HasFileTag(file domain.FileID, tag domain.TagID) (bool, error) {
	var n int
	err := v.tx.QueryRow(`SELECT count(*) FROM file_tags WHERE file_id = ? AND tag_id = ?`, file, tag).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check tag on file %s: %w", file, err)
	}
	return n > 0, nil
}

// placeholders renders "?, ?, ?" for an IN list.
//
// spec 4.3 measured SQLite's MAX_VARIABLE_NUMBER at 32766, which is far above
// anything a tag tree or a folder chain will produce, so there is no chunking
// here.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}
