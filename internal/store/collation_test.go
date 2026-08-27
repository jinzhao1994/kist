package store

import (
	"sort"
	"strings"
	"testing"
)

// These seven characters separate the three plausible orderings completely
// (spec 4.8). Under pinyin the sequence is almost exactly reversed, so any
// locale-aware collation sneaking in fails loudly rather than subtly.
//
//	char  codepoint  pinyin  strokes
//	阿    U+963F     a       7
//	陈    U+9648     chen    7
//	李    U+674E     li      7
//	王    U+738B     wang    4
//	一    U+4E00     yi      1
//	张    U+5F20     zhang   7
//	重    U+91CD     chong   9
var (
	collationInput = []string{"阿", "陈", "李", "王", "一", "张", "重"}

	// Byte order, which is what v0.1 uses everywhere.
	wantByteOrder = []string{"一", "张", "李", "王", "重", "阿", "陈"}

	// What a zh-Hans collation would produce. Asserted against, never for.
	pinyinOrder = []string{"阿", "陈", "李", "王", "一", "张", "重"}
)

func joined(s []string) string { return strings.Join(s, " ") }

// TestSQLiteSortsChineseByByteOrder pins spec 4.8 to a test.
//
// v0.1 deliberately does not sort by pinyin or by stroke count: those are
// locale-dependent, they differ between platforms and between database
// engines, and an engine or ICU upgrade silently reorders indexes built on
// them. Byte order is the one answer that is identical on every platform, on
// either engine, and at any point in time.
//
// This test is the enforcement. Without it, 4.8 is only prose, and the first
// person who reaches for a collation to "fix" Chinese sorting gets a green
// build.
func TestSQLiteSortsChineseByByteOrder(t *testing.T) {
	db := migrated(t)
	for _, name := range collationInput {
		insertBox(t, db, name)
	}

	rows, err := db.Query(`SELECT name FROM boxes ORDER BY name`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	if joined(got) != joined(wantByteOrder) {
		t.Errorf("ORDER BY name gave\n  %s\nwant byte order\n  %s", joined(got), joined(wantByteOrder))
	}
	if joined(got) == joined(pinyinOrder) {
		t.Error("ORDER BY name produced pinyin order; a locale-aware collation is in play, which spec 4.8 rules out for v0.1")
	}
}

// TestGoAndSQLiteAgreeOnOrder is the property that lets sorting happen on
// either side of the wire.
//
// Some listings are ordered by the database and some in Go (the tag tree lives
// in memory, spec 1.6). If the two disagreed, the same library would present
// its contents differently depending on which code path produced the list.
func TestGoAndSQLiteAgreeOnOrder(t *testing.T) {
	db := migrated(t)
	for _, name := range collationInput {
		insertBox(t, db, name)
	}

	rows, err := db.Query(`SELECT name FROM boxes ORDER BY name`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var fromSQL []string
	for rows.Next() {
		var n string
		rows.Scan(&n)
		fromSQL = append(fromSQL, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	fromGo := append([]string(nil), collationInput...)
	sort.Strings(fromGo)

	if joined(fromSQL) != joined(fromGo) {
		t.Errorf("SQLite and Go disagree:\n  SQLite: %s\n  Go:     %s", joined(fromSQL), joined(fromGo))
	}
}

// TestNoCaseDoesNotTouchChinese records why NOCASE is not a shortcut to
// case-insensitive search for this project.
//
// SQLite's built-in NOCASE folds ASCII A-Z and nothing else, which is exactly
// the engine-specific behaviour spec 4.7 refuses to depend on. Case
// insensitivity is Go's job, through the generated fold columns.
func TestNoCaseDoesNotTouchChinese(t *testing.T) {
	db := migrated(t)
	for _, name := range collationInput {
		insertBox(t, db, name)
	}

	rows, err := db.Query(`SELECT name FROM boxes ORDER BY name COLLATE NOCASE`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var n string
		rows.Scan(&n)
		got = append(got, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	if joined(got) != joined(wantByteOrder) {
		t.Errorf("COLLATE NOCASE changed the order of Chinese text:\n  %s", joined(got))
	}
}

// TestOnlyBuiltinCollationsExist guards the other half of the same decision.
//
// A custom collation registered from Go would give real pinyin ordering, but
// it makes the database unreadable by standard tools: `sqlite3` cannot run
// ORDER BY on such a column, and PRAGMA integrity_check fails outright. For an
// archive whose whole promise is that the data outlives the software, that is
// disqualifying (spec 4.8).
func TestOnlyBuiltinCollationsExist(t *testing.T) {
	db := migrated(t)

	rows, err := db.Query(`PRAGMA collation_list`)
	if err != nil {
		t.Fatalf("collation_list: %v", err)
	}
	defer rows.Close()

	found := map[string]bool{}
	for rows.Next() {
		var seq int
		var name string
		if err := rows.Scan(&seq, &name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		found[strings.ToUpper(name)] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"BINARY", "NOCASE", "RTRIM"} {
		if !found[want] {
			t.Errorf("the built-in collation %s is missing", want)
		}
	}
	if len(found) != 3 {
		t.Errorf("this build registers %d collations (%v); v0.1 relies on byte comparison only", len(found), found)
	}
}
