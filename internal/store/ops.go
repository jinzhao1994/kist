package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

// FormatTime renders a machine timestamp for storage: unix nanoseconds, UTC.
//
// Nanoseconds are time.Time's own resolution, so the round trip is exact and
// nothing has to be truncated first.
//
// It is not a claim that ts is unique or monotonic — it is a wall clock, and
// NTP moves those backwards. ops.id is the ordering authority.
func FormatTime(t time.Time) int64 { return t.UTC().UnixNano() }

// ParseTime reads a stored machine timestamp. It cannot fail: the column is
// INTEGER, so the driver has already done the only parsing there is.
func ParseTime(ns int64) time.Time { return time.Unix(0, ns).UTC() }

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
	Origin    string
	Payload   []byte
	RevertsID *int64
	// RootOpID is the first op of the group this one belongs to, set when a
	// single command wrote several. It is bookkeeping about the write, not
	// about what was written, which is why Apply never sees it and a rebuild
	// does not care. The first op of a group carries nil: it is the root.
	RootOpID *int64
	// RequestID and RequestDigest are projected into op_requests, not ops.
	RequestID     string
	RequestDigest string
}

// InsertOp records an op that is already in the journal.
//
// The id comes from the caller because the journal assigned it: the line is
// appended and flushed before anything touches the database (spec 2.2.2), so
// by the time this runs the version number is already durable. That is also
// why there is no AppendOp any more — nothing may mint an op id here, or the
// table and the log would each have an opinion about what the version is.
func InsertOp(tx *sql.Tx, r OpRow) error {
	_, err := tx.Exec(
		`INSERT INTO ops (id, ts, action, origin, payload, reverts_id, root_op_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.ID, FormatTime(r.TS), r.Action, r.Origin, string(r.Payload), r.RevertsID, r.RootOpID)
	if err != nil {
		return fmt.Errorf("insert op %d (%s): %w", r.ID, r.Action, err)
	}
	if r.RequestID != "" {
		if _, err := tx.Exec(`INSERT INTO op_requests (request_id, digest, op_id) VALUES (?, ?, ?)`, r.RequestID, r.RequestDigest, r.ID); err != nil {
			return fmt.Errorf("index request: %w", err)
		}
	}
	return nil
}

// MaxOpID is the newest op the database knows about, or 0 if it knows none.
//
// It is compared against the journal's last id to find out whether a crash
// left the database behind the log (spec 2.2.2).
func MaxOpID(q Queryer) (int64, error) {
	var id sql.NullInt64
	if err := q.QueryRow(`SELECT MAX(id) FROM ops`).Scan(&id); err != nil {
		return 0, fmt.Errorf("read the newest op id: %w", err)
	}
	return id.Int64, nil
}

// ClearOps empties the ops table so a rebuild can refill it from the journal.
//
// defer_foreign_keys is needed for exactly one reason: ops references itself
// through reverts_id and root_op_id, so deleting the rows one at a time
// violates a constraint halfway through even though the end state is fine. It
// resets at commit, so its scope is this statement's transaction.
func ClearOps(tx *sql.Tx) error {
	if _, err := tx.Exec(`PRAGMA defer_foreign_keys = ON`); err != nil {
		return fmt.Errorf("defer foreign keys: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM ops`); err != nil {
		return fmt.Errorf("clear ops: %w", err)
	}
	return nil
}

// opColumns is everything about an op. GetOp and replay use it; both need the
// payload.
const opColumns = `id, ts, action, origin, payload, reverts_id, root_op_id`

// opListColumns is the same, minus the payload.
//
// The history view renders id, time, action, origin and whether the op was
// undone — never the payload. Once imports exist, selecting it anyway means
// `kist log -n 20` reads twenty megabytes to print twenty lines: a 5000-file
// import's payload is about a megabyte (spec 2.2.1).
const opListColumns = `id, ts, action, origin, reverts_id, root_op_id`

func scanOp(scan func(...any) error) (OpRow, error) {
	var r OpRow
	var ts int64
	var payload string
	if err := scan(&r.ID, &ts, &r.Action, &r.Origin, &payload, &r.RevertsID, &r.RootOpID); err != nil {
		return OpRow{}, err
	}
	r.TS = ParseTime(ts)
	r.Payload = []byte(payload)
	return r, nil
}

// scanOpHeader reads everything but the payload.
func scanOpHeader(scan func(...any) error) (OpRow, error) {
	var r OpRow
	var ts int64
	if err := scan(&r.ID, &ts, &r.Action, &r.Origin, &r.RevertsID, &r.RootOpID); err != nil {
		return OpRow{}, err
	}
	r.TS = ParseTime(ts)
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
	// File restricts to the complete history of one file, including deletion.
	File *domain.FileID
	// Action restricts to an exact action discriminator.
	Action string
	// Group restricts to the ops one command wrote, addressed by any op in
	// it: nobody wants to type a UUID that is printed nowhere.
	Group *int64
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
	if opt.File != nil {
		sb.WriteString(` AND id IN (SELECT op_id FROM op_files WHERE file_id = ?)`)
		args = append(args, *opt.File)
	}
	if opt.Action != "" {
		sb.WriteString(` AND action = ?`)
		args = append(args, opt.Action)
	}
	if opt.Group != nil {
		// A group is named by any op in it, root or not: the group has no id
		// of its own to type. The coalesce turns a member into its root and
		// leaves a root — or an op in no group at all — as itself, so
		// "--group 7" always says something rather than nothing.
		const root = `(SELECT coalesce(root_op_id, id) FROM ops WHERE id = ?)`
		sb.WriteString(` AND (id = ` + root + ` OR root_op_id = ` + root + `)`)
		args = append(args, *opt.Group, *opt.Group)
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

// InsertOpIndexes writes every index table for one op.
//
// One function rather than three calls, because there are two call sites — the
// write path and Rebuild — and they must stay identical. A table one of them
// forgets is a difference only `verify --deep` would find.
func InsertOpIndexes(tx *sql.Tx, opID int64, boxes []domain.BoxID, files []domain.FileID, blobs []string) error {
	if err := InsertOpBoxes(tx, opID, boxes); err != nil {
		return err
	}
	if err := InsertOpFiles(tx, opID, files); err != nil {
		return err
	}
	return InsertOpBlobs(tx, opID, blobs)
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

// InsertOpFiles records which files an op touched.
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

// InsertOpBlobs records which content an op touched. files.blob_key answers the
// same question only while the file exists.
func InsertOpBlobs(tx *sql.Tx, opID int64, blobs []string) error {
	seen := make(map[string]bool, len(blobs))
	for _, b := range blobs {
		if seen[b] {
			continue
		}
		seen[b] = true
		if _, err := tx.Exec(`INSERT INTO op_blobs (op_id, blob_key) VALUES (?, ?)`, opID, b); err != nil {
			return fmt.Errorf("index op %d against blob %s: %w", opID, b, err)
		}
	}
	return nil
}

// DeleteOpIndexes clears the index tables so Rebuild can regenerate them from
// the payloads (spec 2.2.1). It does NOT touch ops itself — that table is the
// authority and is never rewritten.
func DeleteOpIndexes(tx *sql.Tx) error {
	for _, table := range []string{"op_boxes", "op_files", "op_blobs", "op_requests"} {
		if _, err := tx.Exec(`DELETE FROM ` + table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	return nil
}

// RequestOp returns the durable result for a client request, if indexed.
func RequestOp(q Queryer, id string) (digest string, opID int64, found bool, err error) {
	err = q.QueryRow(`SELECT digest, op_id FROM op_requests WHERE request_id = ?`, id).Scan(&digest, &opID)
	if err == sql.ErrNoRows {
		return "", 0, false, nil
	}
	return digest, opID, err == nil, err
}
