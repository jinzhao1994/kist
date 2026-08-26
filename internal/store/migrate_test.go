package store

import (
	"crypto/rand"
	"database/sql"
	"strings"
	"testing"
)

const testTS = "2026-08-22T00:00:00.000Z"

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

func TestMigrateCreatesEveryTable(t *testing.T) {
	db := migrated(t)

	want := []string{
		"blobs", "box_tags", "boxes", "file_tags", "files", "folders",
		"metadata", "op_boxes", "op_files", "ops", "tag_rejections", "tags",
	}
	for _, table := range want {
		var n int
		err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n)
		if err != nil {
			t.Fatalf("query sqlite_master: %v", err)
		}
		if n != 1 {
			t.Errorf("table %q is missing", table)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := migrated(t)
	if err := Migrate(db); err != nil {
		t.Errorf("second Migrate: %v", err)
	}
}

func TestOneRootFolderPerBox(t *testing.T) {
	// The naive UNIQUE(parent_id, name) does not bite for root folders: their
	// parent_id is NULL and NULLs are never equal, so a box could grow two
	// roots and fork the whole projection (spec 1.4).
	db := migrated(t)
	box := insertBox(t, db, "trip")

	insertRootFolder(t, db, box)
	if err := tryInsertRootFolder(db, box); err == nil {
		t.Error("a second root folder was accepted for the same box")
	}
}

func TestFolderNamesUniqueWithinParent(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "trip")
	root := insertRootFolder(t, db, box)

	insertFolder(t, db, box, root, "receipts")
	if err := tryInsertFolder(db, box, root, "receipts"); err == nil {
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
	root := insertRootFolder(t, db, box)
	key := insertBlob(t, db, "sha256:"+strings.Repeat("ab", 32))

	insertFile(t, db, box, root, "receipt.jpg", key)
	if err := tryInsertFile(db, box, root, "receipt.jpg", key); err == nil {
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

	res, err := db.Exec(`INSERT INTO ops (ts, action, source, payload) VALUES (?, ?, ?, ?)`,
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

func TestOpIDsAreMonotonic(t *testing.T) {
	// ops.id is the version number; the entire history model assumes it only
	// goes up.
	db := migrated(t)

	var ids []int64
	for i := 0; i < 3; i++ {
		res, err := db.Exec(`INSERT INTO ops (ts, action, source, payload) VALUES (?, ?, ?, ?)`,
			"2026-08-22T00:00:00.000Z", "noop", "user", `{}`)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		ids = append(ids, id)
	}
	if _, err := db.Exec(`DELETE FROM ops WHERE id = ?`, ids[2]); err != nil {
		t.Fatal(err)
	}
	res, err := db.Exec(`INSERT INTO ops (ts, action, source, payload) VALUES (?, ?, ?, ?)`,
		"2026-08-22T00:00:00.000Z", "noop", "user", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := res.LastInsertId()
	if next <= ids[2] {
		t.Errorf("op id %d was reused after a delete; AUTOINCREMENT is missing", next)
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

func insertBox(t *testing.T, db *sql.DB, name string) []byte {
	t.Helper()
	id := randomID(t)
	_, err := db.Exec(`INSERT INTO boxes (id, name, name_fold, description, created_at, updated_at)
	                   VALUES (?, ?, ?, '', ?, ?)`,
		id, name, strings.ToLower(name), testTS, testTS)
	if err != nil {
		t.Fatalf("insert box: %v", err)
	}
	return id
}

func insertRootFolder(t *testing.T, db *sql.DB, box []byte) []byte {
	t.Helper()
	id := randomID(t)
	if _, err := db.Exec(`INSERT INTO folders (id, box_id, parent_id, name, name_fold, created_at, updated_at)
	                      VALUES (?, ?, NULL, '', '', ?, ?)`, id, box, testTS, testTS); err != nil {
		t.Fatalf("insert root folder: %v", err)
	}
	return id
}

func tryInsertRootFolder(db *sql.DB, box []byte) error {
	b := make([]byte, 16)
	rand.Read(b)
	_, err := db.Exec(`INSERT INTO folders (id, box_id, parent_id, name, name_fold, created_at, updated_at)
	                   VALUES (?, ?, NULL, '', '', ?, ?)`, b, box, testTS, testTS)
	return err
}

func insertFolder(t *testing.T, db *sql.DB, box, parent []byte, name string) []byte {
	t.Helper()
	id := randomID(t)
	if _, err := db.Exec(`INSERT INTO folders (id, box_id, parent_id, name, name_fold, created_at, updated_at)
	                      VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, box, parent, name, strings.ToLower(name), testTS, testTS); err != nil {
		t.Fatalf("insert folder: %v", err)
	}
	return id
}

func tryInsertFolder(db *sql.DB, box, parent []byte, name string) error {
	b := make([]byte, 16)
	rand.Read(b)
	_, err := db.Exec(`INSERT INTO folders (id, box_id, parent_id, name, name_fold, created_at, updated_at)
	                   VALUES (?, ?, ?, ?, ?, ?, ?)`,
		b, box, parent, name, strings.ToLower(name), testTS, testTS)
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

func insertFile(t *testing.T, db *sql.DB, box, folder []byte, name, key string) []byte {
	t.Helper()
	id := randomID(t)
	if _, err := db.Exec(`INSERT INTO files (id, box_id, folder_id, name, name_fold, blob_key, created_at, updated_at)
	                      VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, box, folder, name, strings.ToLower(name), key, testTS, testTS); err != nil {
		t.Fatalf("insert file: %v", err)
	}
	return id
}

func tryInsertFile(db *sql.DB, box, folder []byte, name, key string) error {
	b := make([]byte, 16)
	rand.Read(b)
	_, err := db.Exec(`INSERT INTO files (id, box_id, folder_id, name, name_fold, blob_key, created_at, updated_at)
	                   VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		b, box, folder, name, strings.ToLower(name), key, testTS, testTS)
	return err
}
