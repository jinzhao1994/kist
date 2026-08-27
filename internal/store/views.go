package store

import (
	"database/sql"
	"fmt"
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

// SQL exposes the underlying transaction to library, which needs it for the
// ops table itself. Handlers only ever see the oplog.Tx interface, which does
// not include this method.
func (v *ViewTx) SQL() *sql.Tx { return v.tx }

// ── boxes ────────────────────────────────────────────────────────────────

func (v *ViewTx) InsertBox(b domain.Box) error {
	_, err := v.tx.Exec(
		`INSERT INTO boxes (id, name, name_fold, description, description_fold,
		                    created_at, updated_at, start_at, end_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.Name, b.NameFold, b.Description, b.DescriptionFold,
		FormatTime(v.ts), FormatTime(v.ts), b.StartAt, b.EndAt)
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
		`UPDATE boxes SET name = ?, name_fold = ?, description = ?, description_fold = ?,
		                  start_at = ?, end_at = ?
		 WHERE id = ?`,
		b.Name, b.NameFold, b.Description, b.DescriptionFold,
		b.StartAt, b.EndAt, b.ID)
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

const boxColumns = `id, name, name_fold, description, description_fold, created_at, updated_at, start_at, end_at`

func scanBox(scan func(...any) error) (domain.Box, bool, error) {
	var (
		b                domain.Box
		created, updated string
		start, end       sql.NullString
	)
	err := scan(&b.ID, &b.Name, &b.NameFold, &b.Description, &b.DescriptionFold,
		&created, &updated, &start, &end)
	if err == sql.ErrNoRows {
		return domain.Box{}, false, nil
	}
	if err != nil {
		return domain.Box{}, false, fmt.Errorf("read box: %w", err)
	}
	if b.CreatedAt, err = ParseTime(created); err != nil {
		return domain.Box{}, false, err
	}
	if b.UpdatedAt, err = ParseTime(updated); err != nil {
		return domain.Box{}, false, err
	}
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
		`INSERT INTO folders (id, box_id, parent_id, name, name_fold, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.BoxID, f.ParentID, f.Name, f.NameFold, FormatTime(v.ts), FormatTime(v.ts))
	if err != nil {
		return fmt.Errorf("insert folder %s: %w", f.ID, err)
	}
	return nil
}

// DeleteFoldersOfBox removes every folder of a box. Foreign keys are on, so a
// box cannot be deleted while its folders still reference it — the delete_box
// handler calls this first, explicitly, rather than relying on a cascade
// nobody can see in the schema.
func (v *ViewTx) DeleteFoldersOfBox(id domain.BoxID) error {
	if _, err := v.tx.Exec(`DELETE FROM folders WHERE box_id = ?`, id); err != nil {
		return fmt.Errorf("delete folders of box %s: %w", id, err)
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
func (v *ViewTx) AddBoxTag(box domain.BoxID, tag domain.TagID, source string) error {
	_, err := v.tx.Exec(
		`INSERT INTO box_tags (box_id, tag_id, source) VALUES (?, ?, ?)
		 ON CONFLICT (box_id, tag_id) DO UPDATE SET source = excluded.source`,
		box, tag, source)
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
		"box_tags", "file_tags", "tag_rejections", "metadata",
		"files", "folders", "boxes", "tags",
	} {
		if _, err := tx.Exec(`DELETE FROM ` + table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	return nil
}
