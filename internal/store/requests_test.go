package store

import (
	"github.com/pressly/goose/v3"
	"testing"
)

func TestRequestIndexMigrationPreservesExistingHistory(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpTo(db, "migrations", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ops (id, ts, action, origin, payload) VALUES (1, ?, 'create_tag', 'user', '{}')`, testTS); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if n, err := MaxOpID(db); err != nil || n != 1 {
		t.Fatalf("history=%d err=%v", n, err)
	}
	if _, _, found, err := RequestOp(db, "no-request"); err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
}
