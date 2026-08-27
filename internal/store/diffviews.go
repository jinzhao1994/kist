package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Drift is one table that does not match what replaying the log produces.
type Drift struct {
	Table         string
	OnlyInLive    int
	OnlyInRebuilt int
	Examples      []string
}

// driftExamples bounds how many rows are pulled back per direction. The count
// is the finding; the rows are evidence, and evidence stops helping at about
// five lines.
const driftExamples = 5

// DiffViews compares every rebuildable table in live against the same table in
// a database at rebuiltPath, in both directions.
//
// This is `verify --deep` (spec 3.8), and it is the only check in production
// of invariant 1: everything but ops can be rebuilt from ops. Until now that
// was asserted only in a test.
//
// The comparison happens in SQL rather than by diffing two Snapshot strings,
// because a snapshot pulls the whole library into memory — a hundred thousand
// files is tens of megabytes of text on each side. EXCEPT does the work in the
// engine, and only the differing rows come back to Go.
//
// snapshotTables is reused deliberately: it already names the rebuildable
// tables and a byte-stable column list for each. A second such list would mean
// two places to update when a column is added, with the failure mode being a
// check that quietly stops looking at the new column.
func DiffViews(ctx context.Context, live *sql.DB, rebuiltPath string) ([]Drift, error) {
	// ATTACH applies per connection, and database/sql hands out whichever
	// connection it likes — the same trap Open documents for PRAGMA. Pin one
	// for the whole comparison.
	conn, err := live.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("open a connection for the comparison: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS rebuilt`, rebuiltPath); err != nil {
		return nil, fmt.Errorf("attach the rebuilt copy: %w", err)
	}
	// Detaching matters beyond tidiness: Windows will not unlink a file that
	// is still open, so the caller could not delete its scratch copy while it
	// remained attached. WithoutCancel because this has to happen even when
	// the comparison was interrupted — that is exactly when a leftover copy
	// would be most annoying.
	defer conn.ExecContext(context.WithoutCancel(ctx), `DETACH DATABASE rebuilt`)

	var out []Drift
	for _, t := range snapshotTables {
		liveOnly := fmt.Sprintf(
			`SELECT %[1]s FROM main.%[2]s EXCEPT SELECT %[1]s FROM rebuilt.%[2]s`, t.columns, t.name)
		rebuiltOnly := fmt.Sprintf(
			`SELECT %[1]s FROM rebuilt.%[2]s EXCEPT SELECT %[1]s FROM main.%[2]s`, t.columns, t.name)

		nLive, exLive, err := countAndSample(ctx, conn, liveOnly)
		if err != nil {
			return nil, fmt.Errorf("compare %s: %w", t.name, err)
		}
		nRebuilt, exRebuilt, err := countAndSample(ctx, conn, rebuiltOnly)
		if err != nil {
			return nil, fmt.Errorf("compare %s: %w", t.name, err)
		}
		if nLive == 0 && nRebuilt == 0 {
			continue
		}

		d := Drift{Table: t.name, OnlyInLive: nLive, OnlyInRebuilt: nRebuilt}
		for _, e := range exLive {
			d.Examples = append(d.Examples, "in the library but not in the replay: "+e)
		}
		for _, e := range exRebuilt {
			d.Examples = append(d.Examples, "in the replay but not in the library: "+e)
		}
		out = append(out, d)
	}
	return out, nil
}

// RebuildableTables names the tables a replay is expected to reproduce. It is
// what a report means by "tables compared".
func RebuildableTables() []string {
	out := make([]string, len(snapshotTables))
	for i, t := range snapshotTables {
		out[i] = t.name
	}
	return out
}

// countAndSample runs one EXCEPT twice: once to count, once for a few rows.
// Two queries is cheaper than the alternative, which is streaming every
// differing row back into Go on a library that has drifted badly.
func countAndSample(ctx context.Context, conn *sql.Conn, except string) (int, []string, error) {
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM (`+except+`)`).Scan(&n); err != nil {
		return 0, nil, err
	}
	if n == 0 {
		return 0, nil, nil
	}

	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`SELECT * FROM (%s) LIMIT %d`, except, driftExamples))
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return 0, nil, err
	}
	var out []string
	for rows.Next() {
		cells := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return 0, nil, err
		}
		parts := make([]string, len(cells))
		for i, c := range cells {
			parts[i] = fmt.Sprint(c)
		}
		out = append(out, strings.Join(parts, " | "))
	}
	return n, out, rows.Err()
}
