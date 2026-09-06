package store

import (
	"crypto/rand"
	"database/sql"
	"strings"
	"testing"
)

// testTS is a unix nanosecond instant, matching the column type.
const testTS int64 = 1787356800000000000

func migrated(t *testing.T) *sql.DB {
	t.Helper()
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := migrated(t)
	if err := Migrate(db); err != nil {
		t.Errorf("second Migrate: %v", err)
	}
}

func TestOneRootFolderPerBox(t *testing.T) {
	// A box names its root in a single column, so "two roots for one box" is
	// not representable. What is still representable, and what the unique
	// index on boxes.root_folder_id forbids, is two boxes claiming the same
	// root — which would fork the projection the same way (spec 1.4).
	db := migrated(t)
	box := insertBox(t, db, "trip")

	if err := tryClaimRoot(db, boxRoot(t, db, box)); err == nil {
		t.Error("two boxes were allowed to share a root folder")
	}
}

func TestFolderNamesUniqueWithinParent(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "trip")
	root := boxRoot(t, db, box)

	insertFolder(t, db, root, "receipts")
	if err := tryInsertFolder(db, root, "receipts"); err == nil {
		t.Error("duplicate folder name accepted under the same parent")
	}
}

func TestTagNamesAreGloballyUnique(t *testing.T) {
	// spec 1.6: the hierarchy is not a namespace. Uniqueness is on the folded
	// name, or "Apple" and "apple" would be two tags and "unique" would be a
	// lie to the user.
	db := migrated(t)

	people := insertTag(t, db, nil, "People", "people")
	if err := tryInsertTag(db, &people, "People", "people"); err == nil {
		t.Error("the same tag name was accepted under a different parent")
	}
	if err := tryInsertTag(db, nil, "PEOPLE", "people"); err == nil {
		t.Error("a case variant was accepted as a second tag")
	}
}

func TestFileNamesUniqueWithinFolder(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "trip")
	root := boxRoot(t, db, box)
	key := insertBlob(t, db, "sha256:"+strings.Repeat("ab", 32))

	insertFile(t, db, root, "receipt.jpg", key)
	if err := tryInsertFile(db, root, "receipt.jpg", key); err == nil {
		t.Error("duplicate file name accepted in the same folder")
	}
}

func TestBlobSha256IsNotUnique(t *testing.T) {
	// A collision produces two rows with the same digest under different keys
	// (spec 3.3). A unique index here would make that unstorable.
	db := migrated(t)
	digest := strings.Repeat("cd", 32)

	insertBlob(t, db, "sha256:"+digest)
	if _, err := db.Exec(`INSERT INTO blobs (key, sha256, size) VALUES (?, ?, ?)`,
		"sha256:"+digest+".1", digest, 1); err != nil {
		t.Errorf("second row with the same digest was rejected: %v", err)
	}
}

func TestOpIndexTablesSurviveEntityDeletion(t *testing.T) {
	// History outlives its subject: op_boxes must not have a foreign key to
	// boxes, or deleting a box would take the record of that deletion with it
	// (spec 4.6).
	db := migrated(t)
	box := insertBox(t, db, "doomed")

	res, err := db.Exec(`INSERT INTO ops (ts, action, origin, payload) VALUES (?, ?, ?, ?)`,
		"2026-08-22T00:00:00.000Z", "delete_box", "user", `{}`)
	if err != nil {
		t.Fatalf("insert op: %v", err)
	}
	opID, _ := res.LastInsertId()
	if _, err := db.Exec(`INSERT INTO op_boxes (op_id, box_id) VALUES (?, ?)`, opID, box); err != nil {
		t.Fatalf("insert op_boxes: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM boxes WHERE id = ?`, box); err != nil {
		t.Fatalf("delete box: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT count(*) FROM op_boxes WHERE box_id = ?`, box).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("op_boxes rows after deleting the box = %d, want 1", n)
	}
}

// Op ids come from the journal now, not from the table (spec 2.2.2), so what
// this pins has changed: the table must store what it is handed, including an
// id it has seen before. The old guarantee — AUTOINCREMENT never reusing a
// rowid — moved to journal, where Last().ID+1 mints ids and Check refuses a
// gap or a repeat.
func TestOpIDsComeFromTheCaller(t *testing.T) {
	// ops.id is the version number; the entire history model assumes it only
	// goes up.
	db := migrated(t)

	for id := int64(1); id <= 3; id++ {
		if _, err := db.Exec(`INSERT INTO ops (id, ts, action, origin, payload) VALUES (?, ?, ?, ?, ?)`,
			id, "2026-08-22T00:00:00.000Z", "noop", "user", `{}`); err != nil {
			t.Fatal(err)
		}
	}
	// A rebuild clears the table and refills it from the journal, so the same
	// id has to be storable again and come back unchanged.
	if _, err := db.Exec(`DELETE FROM ops WHERE id = 3`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ops (id, ts, action, origin, payload) VALUES (?, ?, ?, ?, ?)`,
		3, "2026-08-22T00:00:00.000Z", "noop", "user", `{}`); err != nil {
		t.Fatalf("op 3 could not be written back after a clear: %v", err)
	}

	var got int64
	if err := db.QueryRow(`SELECT MAX(id) FROM ops`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Errorf("MAX(id) = %d, want 3 — the table must store the id it was handed", got)
	}
}

func randomID(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

// insertBox creates the root folder too: boxes.root_folder_id is NOT NULL, so
// a box without one is not a thing the schema can hold.
func insertBox(t *testing.T, db *sql.DB, name string) []byte {
	t.Helper()
	id, root := randomID(t), randomID(t)
	if _, err := db.Exec(`INSERT INTO folders (id, parent_id, name, name_fold, created_at, updated_at)
	                      VALUES (?, NULL, '', '', ?, ?)`, root, testTS, testTS); err != nil {
		t.Fatalf("insert root folder: %v", err)
	}
	_, err := db.Exec(`INSERT INTO boxes (id, name, name_fold, description, created_at, updated_at, root_folder_id)
	                   VALUES (?, ?, ?, '', ?, ?, ?)`,
		id, name, strings.ToLower(name), testTS, testTS, root)
	if err != nil {
		t.Fatalf("insert box: %v", err)
	}
	return id
}

func boxRoot(t *testing.T, db *sql.DB, box []byte) []byte {
	t.Helper()
	var root []byte
	if err := db.QueryRow(`SELECT root_folder_id FROM boxes WHERE id = ?`, box).Scan(&root); err != nil {
		t.Fatalf("read box root: %v", err)
	}
	return root
}

// tryClaimRoot points a second box at a folder that is already a root.
func tryClaimRoot(db *sql.DB, root []byte) error {
	b := make([]byte, 16)
	rand.Read(b)
	_, err := db.Exec(`INSERT INTO boxes (id, name, name_fold, description, created_at, updated_at, root_folder_id)
	                   VALUES (?, 'other', 'other', '', ?, ?, ?)`, b, testTS, testTS, root)
	return err
}

func insertFolder(t *testing.T, db *sql.DB, parent []byte, name string) []byte {
	t.Helper()
	id := randomID(t)
	if _, err := db.Exec(`INSERT INTO folders (id, parent_id, name, name_fold, created_at, updated_at)
	                      VALUES (?, ?, ?, ?, ?, ?)`,
		id, parent, name, strings.ToLower(name), testTS, testTS); err != nil {
		t.Fatalf("insert folder: %v", err)
	}
	return id
}

func tryInsertFolder(db *sql.DB, parent []byte, name string) error {
	b := make([]byte, 16)
	rand.Read(b)
	_, err := db.Exec(`INSERT INTO folders (id, parent_id, name, name_fold, created_at, updated_at)
	                   VALUES (?, ?, ?, ?, ?, ?)`,
		b, parent, name, strings.ToLower(name), testTS, testTS)
	return err
}

func insertTag(t *testing.T, db *sql.DB, parent *[]byte, name, fold string) []byte {
	t.Helper()
	id := randomID(t)
	var p any
	if parent != nil {
		p = *parent
	}
	if _, err := db.Exec(`INSERT INTO tags (id, parent_id, name, name_fold) VALUES (?, ?, ?, ?)`,
		id, p, name, fold); err != nil {
		t.Fatalf("insert tag: %v", err)
	}
	return id
}

func tryInsertTag(db *sql.DB, parent *[]byte, name, fold string) error {
	b := make([]byte, 16)
	rand.Read(b)
	var p any
	if parent != nil {
		p = *parent
	}
	_, err := db.Exec(`INSERT INTO tags (id, parent_id, name, name_fold) VALUES (?, ?, ?, ?)`, b, p, name, fold)
	return err
}

func insertBlob(t *testing.T, db *sql.DB, key string) string {
	t.Helper()
	digest := key[len("sha256:"):]
	if i := strings.IndexByte(digest, '.'); i >= 0 {
		digest = digest[:i]
	}
	if _, err := db.Exec(`INSERT INTO blobs (key, sha256, size) VALUES (?, ?, ?)`, key, digest, 1); err != nil {
		t.Fatalf("insert blob: %v", err)
	}
	return key
}

func insertFile(t *testing.T, db *sql.DB, folder []byte, name, key string) []byte {
	t.Helper()
	id := randomID(t)
	if _, err := db.Exec(`INSERT INTO files (id, folder_id, name, name_fold, blob_key, created_at, updated_at)
	                      VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, folder, name, strings.ToLower(name), key, testTS, testTS); err != nil {
		t.Fatalf("insert file: %v", err)
	}
	return id
}

func tryInsertFile(db *sql.DB, folder []byte, name, key string) error {
	b := make([]byte, 16)
	rand.Read(b)
	_, err := db.Exec(`INSERT INTO files (id, folder_id, name, name_fold, blob_key, created_at, updated_at)
	                   VALUES (?, ?, ?, ?, ?, ?, ?)`,
		b, folder, name, strings.ToLower(name), key, testTS, testTS)
	return err
}

// spec 1.7.3: metadata hangs off a file and carries a real foreign key.
//
// The first version of this table was polymorphic — (target_type, target_id)
// so it could describe a box too — and that shape cannot have a foreign key at
// all, because target_id points at two tables. Deleting a file would leave
// orphan rows nothing catches: not the database, and not integrity's fifteen
// checks either, which were written while this table was empty.
//
// 1.6 had already reached the opposite conclusion for the same shape: box_tags
// and file_tags are two tables precisely so each can name its own parent.
func TestFileMetadataHasARealForeignKey(t *testing.T) {
	db := migrated(t)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatal(err)
	}
	_, err := db.Exec(`INSERT INTO file_metadata (file_id, key, value, type, source)
	                   VALUES (?, 'exif:Make', 'Apple', 'text', 'exif')`, randomID(t))
	if err == nil {
		t.Fatal("a metadata row naming a file that does not exist was accepted")
	}
	// Any error would satisfy the check above — including "no such table",
	// which is what this test reported before the table existed. Name the
	// constraint, or the test passes hardest exactly when it is least true.
	if !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Errorf("the insert failed for the wrong reason: %v", err)
	}
}

// spec 1.7.6 ③: "which files has exif v3 not seen yet" has to be a query, so
// the extractor and its version are an index table rather than a field inside
// the payload — payload is not queryable (invariant 4).
//
// One row per (file, extractor): a re-run replaces the version, it does not
// append a second row. Without the constraint, "has this file been extracted"
// would depend on which of several rows you happened to read.
func TestFileExtractionsIsOnePerFilePerExtractor(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "trip")
	root := boxRoot(t, db, box)
	insertBlob(t, db, "sha256:aa")
	file := insertFile(t, db, root, "a.jpg", "sha256:aa")

	if _, err := db.Exec(`INSERT INTO file_extractions (file_id, extractor, version) VALUES (?, 'exif', 1)`, file); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO file_extractions (file_id, extractor, version) VALUES (?, 'exif', 2)`, file); err == nil {
		t.Error("a second row for the same (file, extractor) was accepted; a re-run must replace the version, not append")
	}
}

// spec 1.5.2: captured_at is a unix instant, not a variable-precision wall
// clock. The column type is load-bearing rather than cosmetic — as TEXT, a
// scanned 1960 photograph (a negative timestamp) sorts before every positive
// one, and scanning old photographs is exactly what an archive is for.
func TestCapturedAtIsAnInteger(t *testing.T) {
	db := migrated(t)
	var typ string
	if err := db.QueryRow(
		`SELECT type FROM pragma_table_info('files') WHERE name = 'captured_at'`).Scan(&typ); err != nil {
		t.Fatal(err)
	}
	if typ != "INTEGER" {
		t.Errorf("files.captured_at is %s, want INTEGER (spec 1.5.2)", typ)
	}
}
