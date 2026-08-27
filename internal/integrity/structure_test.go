package integrity

import (
	"database/sql"
	"testing"

	"github.com/jinzhao1994/kist/internal/store"
)

func emptyLibrary(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

// The whole layer must be silent on a healthy library. It is worth a test of
// its own: the individual queries are tested in store, but nothing else says
// "and CheckStructure runs all five of them".
func TestCheckStructureIsSilentOnAHealthyLibrary(t *testing.T) {
	db := emptyLibrary(t)

	r, err := CheckStructure(db)
	if err != nil {
		t.Fatal(err)
	}
	if r.Total() != 0 {
		t.Errorf("an empty library has %d structural findings", r.Total())
	}
}

// Every finding from this layer must name a command that fixes it. These are
// all "the views disagree with the log", and rebuild is the answer to all of
// them — a finding with no way out is a finding that just worries people.
func TestStructuralFindingsSayHowToFixThem(t *testing.T) {
	db := emptyLibrary(t)

	// Go around the foreign keys, which is how a corrupt database looks from
	// the outside. OpenMemory pins the pool to one connection, so the pragma
	// applies to the statement that follows it.
	if _, err := db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO files (id, box_id, folder_id, name, name_fold, blob_key, created_at, updated_at)
	                      VALUES (randomblob(16), randomblob(16), randomblob(16), 'a.txt', 'a.txt', 'sha256:gone', 't', 't')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatal(err)
	}

	r, err := CheckStructure(db)
	if err != nil {
		t.Fatal(err)
	}
	if r.Total() == 0 {
		t.Fatal("a file with no folder and no blob produced no findings")
	}
	if r.fixes[FileOrphanFolder] == "" {
		t.Error("the finding does not say what to run")
	}
}
