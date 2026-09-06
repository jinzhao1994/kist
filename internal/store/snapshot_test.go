package store

import (
	"regexp"
	"testing"
)

// authorityTables are the tables a replay reads instead of reproducing, so
// snapshotTables must not list them.
//
// There are none left in the data model: since the log moved to ops/ the ops
// table is rebuilt from it like every other view (invariant 1). goose's
// bookkeeping was never part of the data model at all.
var authorityTables = map[string]bool{
	"goose_db_version": true,
}

// TestSnapshotCoversEveryTable pins snapshotTables to the schema.
//
// snapshotTables defines what "the replay reproduces the library" means, for
// both the rebuild snapshot test and verify --deep. A table missing from it is
// not compared by either — and both stay green while saying nothing about it,
// which is the worst available outcome for the one invariant they exist to
// check.
func TestSnapshotCoversEveryTable(t *testing.T) {
	db := migrated(t)

	rows, err := db.Query(`SELECT name FROM sqlite_master
	                        WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatalf("read sqlite_master: %v", err)
	}
	defer rows.Close()

	inSchema := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		inSchema[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	listed := map[string]bool{}
	for _, tb := range snapshotTables {
		listed[tb.name] = true
	}

	for name := range inSchema {
		if !listed[name] && !authorityTables[name] {
			t.Errorf("table %q exists but snapshotTables does not list it; "+
				"neither the rebuild snapshot nor verify --deep looks at it", name)
		}
	}
	for name := range listed {
		if !inSchema[name] {
			t.Errorf("snapshotTables lists %q, which is not in the schema", name)
		}
	}
}

// bareColumn strips the rendering wrappers off a snapshot column expression.
var bareColumn = regexp.MustCompile(`[a-z_][a-z0-9_]*`)

// TestSnapshotCoversEveryColumn is the same guard one level down.
//
// Every column of a rebuildable table has to be reproduced by a replay — there
// is no such thing as a column that is derived but exempt. So the column list
// encodes no decision, and the only thing it can do is fall behind.
//
// The reverse direction needs no test: a column listed here that no longer
// exists makes Snapshot's own SELECT fail.
func TestSnapshotCoversEveryColumn(t *testing.T) {
	db := migrated(t)

	for _, tb := range snapshotTables {
		rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, tb.name)
		if err != nil {
			t.Fatalf("read the columns of %s: %v", tb.name, err)
		}
		var columns []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			columns = append(columns, name)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(columns) == 0 {
			t.Fatalf("%s has no columns", tb.name)
		}

		for _, col := range columns {
			// Word boundaries, so hex(id) counts as id but box_id does not.
			word := regexp.MustCompile(`\b` + regexp.QuoteMeta(col) + `\b`)
			if !word.MatchString(tb.columns) {
				t.Errorf("%s.%s is not in the snapshot; a replay could get it wrong "+
					"and nothing would notice", tb.name, col)
			}
		}
		if !bareColumn.MatchString(tb.order) {
			t.Errorf("%s has no ORDER BY; its snapshot is not deterministic", tb.name)
		}
	}
}
