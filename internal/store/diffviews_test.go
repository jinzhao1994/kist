package store

import (
	"path/filepath"
	"testing"
)

// TestVacuumIntoAndAttachWork is not a test of our code. It is a test of the
// assumption the whole --deep check rests on: that modernc.org/sqlite supports
// VACUUM INTO (SQLite 3.27+) and ATTACH with a bound parameter. If a dependency
// bump ever takes either away, the failure should be this line rather than a
// mystery inside verify.
func TestVacuumIntoAndAttachWork(t *testing.T) {
	db := migrated(t)
	insertBox(t, db, "box")

	dst := filepath.Join(t.TempDir(), "copy.db")
	if _, err := db.Exec(`VACUUM INTO ?`, dst); err != nil {
		t.Fatalf("VACUUM INTO: %v", err)
	}

	// ATTACH applies per connection, and database/sql hands out whichever one
	// it likes — the same trap Open documents for PRAGMA. Pin one.
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(t.Context(), `ATTACH DATABASE ? AS copy`, dst); err != nil {
		t.Fatalf("ATTACH: %v", err)
	}
	// Detaching is not tidiness. Windows refuses to unlink a file that is
	// still open, so an attached database cannot be deleted — which is what
	// t.TempDir does on cleanup, and what verify --deep does to its scratch
	// copy. Leaving it attached failed on the windows runner only.
	defer conn.ExecContext(t.Context(), `DETACH DATABASE copy`)

	var n int
	if err := conn.QueryRowContext(t.Context(), `SELECT count(*) FROM copy.boxes`).Scan(&n); err != nil {
		t.Fatalf("query the attached database: %v", err)
	}
	if n != 1 {
		t.Errorf("the copy has %d boxes, want 1", n)
	}
}

// Two identical databases drift by nothing.
func TestDiffViewsFindsNothingBetweenIdenticalCopies(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	root := insertRootFolder(t, db, box)
	key := insertBlob(t, db, "sha256:aa")
	insertFile(t, db, box, root, "a.txt", key)

	copyPath := filepath.Join(t.TempDir(), "copy.db")
	if _, err := db.Exec(`VACUUM INTO ?`, copyPath); err != nil {
		t.Fatal(err)
	}

	drift, err := DiffViews(t.Context(), db, copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 0 {
		t.Errorf("identical databases drifted: %+v", drift)
	}
}

// One changed cell must be found, named, and shown — "the files table differs"
// is not an answer anybody can act on.
func TestDiffViewsNamesTheTableAndShowsTheRow(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	root := insertRootFolder(t, db, box)
	key := insertBlob(t, db, "sha256:aa")
	file := insertFile(t, db, box, root, "a.txt", key)

	copyPath := filepath.Join(t.TempDir(), "copy.db")
	if _, err := db.Exec(`VACUUM INTO ?`, copyPath); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE files SET name = 'renamed.txt' WHERE id = ?`, file); err != nil {
		t.Fatal(err)
	}

	drift, err := DiffViews(t.Context(), db, copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 1 {
		t.Fatalf("found %d drifting tables, want 1: %+v", len(drift), drift)
	}
	d := drift[0]
	if d.Table != "files" {
		t.Errorf("table = %q, want files", d.Table)
	}
	if d.OnlyInLive != 1 || d.OnlyInRebuilt != 1 {
		t.Errorf("counts = %d live / %d rebuilt, want 1 and 1", d.OnlyInLive, d.OnlyInRebuilt)
	}
	if len(d.Examples) == 0 {
		t.Error("no example rows — the report would be unactionable")
	}
}

// A row present on one side only, which is what a lost or spurious insert
// looks like.
func TestDiffViewsFindsAnExtraRow(t *testing.T) {
	db := migrated(t)
	copyPath := filepath.Join(t.TempDir(), "copy.db")
	if _, err := db.Exec(`VACUUM INTO ?`, copyPath); err != nil {
		t.Fatal(err)
	}
	insertBox(t, db, "invented")

	drift, err := DiffViews(t.Context(), db, copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 1 || drift[0].Table != "boxes" {
		t.Fatalf("drift = %+v, want one about boxes", drift)
	}
	if drift[0].OnlyInLive != 1 || drift[0].OnlyInRebuilt != 0 {
		t.Errorf("counts = %+v, want 1 live / 0 rebuilt", drift[0])
	}
}

// ops must never be compared: it is the input, not the output. Comparing it
// would make the check vacuously pass on a library whose views are wrong but
// whose log is intact — which is every library this check exists for.
func TestDiffViewsIgnoresOps(t *testing.T) {
	db := migrated(t)
	copyPath := filepath.Join(t.TempDir(), "copy.db")
	if _, err := db.Exec(`VACUUM INTO ?`, copyPath); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ops (ts, action, origin, payload) VALUES ('t','a','user','{}')`); err != nil {
		t.Fatal(err)
	}
	drift, err := DiffViews(t.Context(), db, copyPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drift {
		if d.Table == "ops" {
			t.Error("ops was compared; it is the authority, not a view")
		}
	}
}

// A badly drifted table must not stream every differing row back into Go.
func TestDiffViewsCountsEverythingButShowsAFew(t *testing.T) {
	db := migrated(t)
	copyPath := filepath.Join(t.TempDir(), "copy.db")
	if _, err := db.Exec(`VACUUM INTO ?`, copyPath); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		insertBox(t, db, "box")
	}

	drift, err := DiffViews(t.Context(), db, copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 1 {
		t.Fatalf("drift = %+v", drift)
	}
	if drift[0].OnlyInLive != 50 {
		t.Errorf("counted %d, want 50", drift[0].OnlyInLive)
	}
	if len(drift[0].Examples) > driftExamples {
		t.Errorf("pulled back %d example rows, want at most %d", len(drift[0].Examples), driftExamples)
	}
}
