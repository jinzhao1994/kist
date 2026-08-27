package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

// TimeLayout is how every machine timestamp is written: UTC, milliseconds
// zero-padded, fixed width.
//
// Fixed width is the whole point. The log relies on the lexicographic order of
// ts being its chronological order, and dropping a millisecond digit breaks
// that silently — ORDER BY ts keeps working, it just stops being right.
const TimeLayout = "2006-01-02T15:04:05.000Z"

// FormatTime renders t for storage, converting to UTC first.
func FormatTime(t time.Time) string { return t.UTC().Format(TimeLayout) }

// ParseTime reads a stored timestamp.
func ParseTime(s string) (time.Time, error) {
	t, err := time.Parse(TimeLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp %q: %w", s, err)
	}
	return t.UTC(), nil
}

// Queryer is satisfied by both *sql.DB and *sql.Tx, so read helpers can be
// called inside or outside a transaction.
type Queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// OpRow is one row of the ops table.
//
// store deliberately defines its own shape rather than using oplog.Op: store
// must not import oplog, and oplog must not import store. library converts
// between the two. The duplicated struct is the price of a dependency
// direction that keeps *sql.Tx out of reach of Apply.
type OpRow struct {
	ID        int64
	TS        time.Time
	Action    string
	Source    string
	Actor     string
	Payload   []byte
	RevertsID *int64
}

// AppendOp inserts an op and returns its id, which is the library's version
// number. Ops are never updated and never deleted.
func AppendOp(tx *sql.Tx, r OpRow) (int64, error) {
	res, err := tx.Exec(
		`INSERT INTO ops (ts, action, source, actor, payload, reverts_id) VALUES (?, ?, ?, ?, ?, ?)`,
		FormatTime(r.TS), r.Action, r.Source, nullString(r.Actor), string(r.Payload), r.RevertsID)
	if err != nil {
		return 0, fmt.Errorf("append op %q: %w", r.Action, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("append op %q: %w", r.Action, err)
	}
	return id, nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// opColumns is everything about an op. GetOp and replay use it; both need the
// payload.
const opColumns = `id, ts, action, source, coalesce(actor, ''), payload, reverts_id`

// opListColumns is the same, minus the payload.
//
// The history view renders id, time, action, source and whether the op was
// undone — never the payload. Once imports exist, selecting it anyway means
// `kist log -n 20` reads twenty megabytes to print twenty lines: a 5000-file
// import's payload is about a megabyte (spec 2.2.1).
const opListColumns = `id, ts, action, source, coalesce(actor, ''), reverts_id`

func scanOp(scan func(...any) error) (OpRow, error) {
	var r OpRow
	var ts, payload string
	if err := scan(&r.ID, &ts, &r.Action, &r.Source, &r.Actor, &payload, &r.RevertsID); err != nil {
		return OpRow{}, err
	}
	t, err := ParseTime(ts)
	if err != nil {
		return OpRow{}, err
	}
	r.TS = t
	r.Payload = []byte(payload)
	return r, nil
}

// scanOpHeader reads everything but the payload.
func scanOpHeader(scan func(...any) error) (OpRow, error) {
	var r OpRow
	var ts string
	if err := scan(&r.ID, &ts, &r.Action, &r.Source, &r.Actor, &r.RevertsID); err != nil {
		return OpRow{}, err
	}
	t, err := ParseTime(ts)
	if err != nil {
		return OpRow{}, err
	}
	r.TS = t
	return r, nil
}

// GetOp reads one op.
func GetOp(q Queryer, id int64) (OpRow, bool, error) {
	r, err := scanOp(q.QueryRow(`SELECT `+opColumns+` FROM ops WHERE id = ?`, id).Scan)
	if err == sql.ErrNoRows {
		return OpRow{}, false, nil
	}
	if err != nil {
		return OpRow{}, false, fmt.Errorf("read op %d: %w", id, err)
	}
	return r, true, nil
}

// ListOpsOptions filters the history view.
type ListOpsOptions struct {
	// Box restricts to ops that touched this box, via the op_boxes index.
	Box *domain.BoxID
	// Before, when non-zero, returns only ops with a smaller id — the cursor
	// for paging back through history.
	Before int64
	Limit  int
}

// ListOps returns ops newest first, without their payloads.
//
// OpRow.Payload comes back empty on purpose; GetOp is how you ask for the
// whole thing. See opListColumns.
func ListOps(q Queryer, opt ListOpsOptions) ([]OpRow, error) {
	var (
		sb   strings.Builder
		args []any
	)
	sb.WriteString(`SELECT ` + opListColumns + ` FROM ops WHERE 1 = 1`)
	if opt.Box != nil {
		// The index table, not json_each over the payload: 0.11ms versus
		// 94.63ms at 200k ops, and the gap widens forever (spec 2.2.1).
		sb.WriteString(` AND id IN (SELECT op_id FROM op_boxes WHERE box_id = ?)`)
		args = append(args, *opt.Box)
	}
	if opt.Before > 0 {
		sb.WriteString(` AND id < ?`)
		args = append(args, opt.Before)
	}
	sb.WriteString(` ORDER BY id DESC`)
	if opt.Limit > 0 {
		sb.WriteString(` LIMIT ?`)
		args = append(args, opt.Limit)
	}

	rows, err := q.Query(sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list ops: %w", err)
	}
	defer rows.Close()

	var out []OpRow
	for rows.Next() {
		r, err := scanOpHeader(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("list ops: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// IterOps calls fn for every op, oldest first, one row at a time.
//
// Rebuild is the only caller and it is the reason this exists: the
// slice-returning ListOps would hold every payload in the library in memory at
// once, which after a hundred thousand imported files is tens of megabytes for
// no reason. A cursor keeps it to one op.
func IterOps(q Queryer, fn func(OpRow) error) error {
	rows, err := q.Query(`SELECT ` + opColumns + ` FROM ops ORDER BY id`)
	if err != nil {
		return fmt.Errorf("read the op log: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		r, err := scanOp(rows.Scan)
		if err != nil {
			return fmt.Errorf("read the op log: %w", err)
		}
		if err := fn(r); err != nil {
			return err
		}
	}
	return rows.Err()
}

// RevertedBy reports which op, if any, undid the given one. The history panel
// asks this for every row it renders, which is why reverts_id is a column with
// an index rather than a payload field (spec 2.2).
func RevertedBy(q Queryer, id int64) (int64, bool, error) {
	var by int64
	err := q.QueryRow(`SELECT id FROM ops WHERE reverts_id = ? ORDER BY id LIMIT 1`, id).Scan(&by)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("check whether op %d was reverted: %w", id, err)
	}
	return by, true, nil
}

// LastOpTouchingBox finds the most recent op after afterOpID that touched this
// box. Strict undo uses it to name the op that is in the way rather than
// reporting a bare failure (spec 2.5).
func LastOpTouchingBox(q Queryer, box domain.BoxID, afterOpID int64) (int64, bool, error) {
	var id int64
	err := q.QueryRow(
		`SELECT op_id FROM op_boxes WHERE box_id = ? AND op_id > ? ORDER BY op_id DESC LIMIT 1`,
		box, afterOpID).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("find the op blocking an undo: %w", err)
	}
	return id, true, nil
}

// InsertOpBoxes records which boxes an op touched. Repeated ids are collapsed:
// a batch op names each target once.
func InsertOpBoxes(tx *sql.Tx, opID int64, boxes []domain.BoxID) error {
	seen := make(map[domain.BoxID]bool, len(boxes))
	for _, b := range boxes {
		if seen[b] {
			continue
		}
		seen[b] = true
		if _, err := tx.Exec(`INSERT INTO op_boxes (op_id, box_id) VALUES (?, ?)`, opID, b); err != nil {
			return fmt.Errorf("index op %d against box %s: %w", opID, b, err)
		}
	}
	return nil
}

// InsertOpFiles records which files an op touched. Nothing writes files until
// M1c; the function exists now so the write path is complete and Targets has
// somewhere to land.
func InsertOpFiles(tx *sql.Tx, opID int64, files []domain.FileID) error {
	seen := make(map[domain.FileID]bool, len(files))
	for _, f := range files {
		if seen[f] {
			continue
		}
		seen[f] = true
		if _, err := tx.Exec(`INSERT INTO op_files (op_id, file_id) VALUES (?, ?)`, opID, f); err != nil {
			return fmt.Errorf("index op %d against file %s: %w", opID, f, err)
		}
	}
	return nil
}

// DeleteOpIndexes clears the index tables so Rebuild can regenerate them from
// the payloads (spec 2.2.1). It does NOT touch ops itself — that table is the
// authority and is never rewritten.
func DeleteOpIndexes(tx *sql.Tx) error {
	for _, table := range []string{"op_boxes", "op_files"} {
		if _, err := tx.Exec(`DELETE FROM ` + table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	return nil
}
