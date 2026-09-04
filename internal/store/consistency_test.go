package store

import (
	"database/sql"
	"testing"
)

// forceUpdate writes something the schema would normally refuse, the way a
// corrupted database looks from the outside.
//
// PRAGMA foreign_keys applies per connection, and database/sql hands out
// whichever connection it likes — the same trap Open documents. This works
// because migrated(t) uses OpenMemory, which pins the pool to one connection.
// On a file-backed database it would have to go through db.Conn first.
func forceUpdate(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(`PRAGMA foreign_keys = ON`)
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

// A file whose folder is gone. This cannot happen through library — the FK
// forbids it — which is exactly why verify should look: the check is for the
// day something got in another way (a hand-edited database, a restored file
// from a different point in time, a bug we have not written yet).
func TestOrphanFilesAreFound(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	root := boxRoot(t, db, box)
	key := insertBlob(t, db, "sha256:aa")
	file := insertFile(t, db, root, "a.txt", key)

	forceUpdate(t, db, `UPDATE files SET folder_id = randomblob(16) WHERE id = ?`, file)

	got, err := OrphanFiles(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("found %d orphans, want 1: %+v", len(got), got)
	}
	if got[0].Detail == "" {
		t.Error("the finding says nothing about which file it is")
	}
}

// A box's root_folder_id is the only place a folder's box is written down now,
// so it is the only place that answer can go wrong.
func TestBadBoxRootIsFound(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	other := insertBox(t, db, "other")

	// Point one box at the other box's root, which has no parent but is
	// already claimed — and then at a folder that does have a parent.
	day1 := insertFolder(t, db, boxRoot(t, db, other), "day1")
	forceUpdate(t, db, `UPDATE boxes SET root_folder_id = ? WHERE id = ?`, day1, box)

	got, err := BoxesWithBadRoot(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("found %d, want 1: %+v", len(got), got)
	}
}

// A parentless folder no box claims is invisible rather than missing: every
// read path starts at a box and walks down, so its whole subtree drops out of
// the library while every row is still there.
func TestUnclaimedRootFolderIsFound(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	stray := insertFolder(t, db, boxRoot(t, db, box), "day1")
	forceUpdate(t, db, `UPDATE folders SET parent_id = NULL WHERE id = ?`, stray)

	got, err := UnclaimedRootFolders(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("found %d, want 1: %+v", len(got), got)
	}
}

// A folder whose parent is gone, and a folder that is its own ancestor, are
// the same finding: not reachable from the box root. One recursive query
// covers both, which is why there is no separate cycle detector.
func TestUnreachableFoldersCoverGoneParentsAndCycles(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	root := boxRoot(t, db, box)
	day1 := insertFolder(t, db, root, "day1")
	deeper := insertFolder(t, db, day1, "deeper")

	if got, err := UnreachableFolders(db); err != nil || len(got) != 0 {
		t.Fatalf("a healthy library reports %v (err %v), want none", got, err)
	}

	// A cycle: day1's parent becomes its own child.
	forceUpdate(t, db, `UPDATE folders SET parent_id = ? WHERE id = ?`, deeper, day1)

	got, err := UnreachableFolders(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("found %d unreachable folders, want 2 (both members of the cycle): %+v", len(got), got)
	}
}

// The other half of the same query: a parent that simply is not there.
func TestUnreachableFoldersFindAGoneParent(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	root := boxRoot(t, db, box)
	day1 := insertFolder(t, db, root, "day1")

	forceUpdate(t, db, `UPDATE folders SET parent_id = randomblob(16) WHERE id = ?`, day1)

	got, err := UnreachableFolders(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("found %d, want 1: %+v", len(got), got)
	}
}

func TestUnknownBlobKeyIsFound(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	root := boxRoot(t, db, box)
	key := insertBlob(t, db, "sha256:aa")
	file := insertFile(t, db, root, "a.txt", key)

	forceUpdate(t, db, `UPDATE files SET blob_key = 'sha256:gone' WHERE id = ?`, file)

	got, err := FilesWithUnknownBlob(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("found %d, want 1: %+v", len(got), got)
	}
}

// A healthy library must come back completely silent, or the checks are
// useless — a report that always has something in it teaches people to ignore
// it.
func TestAHealthyLibraryHasNoStructuralFindings(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	root := boxRoot(t, db, box)
	day1 := insertFolder(t, db, root, "day1")
	key := insertBlob(t, db, "sha256:aa")
	insertFile(t, db, root, "a.txt", key)
	insertFile(t, db, day1, "b.txt", key)

	checks := map[string]func(Queryer) ([]Inconsistency, error){
		"OrphanFiles":          OrphanFiles,
		"FilesWithUnknownBlob": FilesWithUnknownBlob,
		"UnreachableFolders":   UnreachableFolders,
		"BoxesWithBadRoot":     BoxesWithBadRoot,
		"UnclaimedRootFolders": UnclaimedRootFolders,
	}
	for name, fn := range checks {
		got, err := fn(db)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(got) != 0 {
			t.Errorf("%s found %+v in a healthy library", name, got)
		}
	}
}
