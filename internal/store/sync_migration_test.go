package store

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

func TestSyncMigrationPreservesOldHistoryAndIndexesNewReceipts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kist.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpTo(db, "migrations", 7); err != nil {
		t.Fatal(err)
	}
	old := sampleOp()
	old.Origin = "sync" // Legacy origin is historical data, not a receipt.
	old.RequestID, old.RequestDigest = "old-request", strings.Repeat("a", 64)
	appendOp(t, db, old)
	before, found, err := GetOp(db, 1)
	if err != nil || !found {
		t.Fatal(before, found, err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	after, found, err := GetOp(db, 1)
	if err != nil || !found || after.Origin != before.Origin || !after.TS.Equal(before.TS) || string(after.Payload) != string(before.Payload) {
		t.Fatal("migration changed old history", after, err)
	}
	if accepted, err := OpAcceptedAt(db, 1); err != nil || accepted != nil {
		t.Fatal("legacy origin acquired a fabricated receipt", accepted, err)
	}
	if digest, id, found, err := RequestOp(db, old.RequestID); err != nil || !found || id != 1 || digest != old.RequestDigest {
		t.Fatal("lost old request index", digest, id, found, err)
	}
	current := sampleOp()
	current.TS = old.TS.Add(-72 * time.Hour)
	current.SyncID, current.SyncDigest = "offline-operation", strings.Repeat("b", 64)
	current.AcceptedAt = old.TS.Add(time.Hour)
	appendOp(t, db, current)
	snapshot, err := Snapshot(db)
	if err != nil || !strings.Contains(snapshot, current.SyncID) {
		t.Fatal("snapshot omitted receipt", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if digest, id, at, found, err := SyncOp(db, current.SyncID); err != nil || !found || id != 2 || digest != current.SyncDigest || !at.Equal(current.AcceptedAt) {
		t.Fatal("receipt did not survive reopen", digest, id, at, found, err)
	}
	if got, err := Snapshot(db); err != nil || got != snapshot {
		t.Fatal("repeated migration changed snapshot", err)
	}
	first, err := ListOps(db, ListOpsOptions{Limit: 1})
	if err != nil || len(first) != 1 || first[0].ID != 2 || !first[0].TS.Equal(current.TS) {
		t.Fatal("history sorted by occurrence time", first, err)
	}
	second, err := ListOps(db, ListOpsOptions{Before: 2, Limit: 1})
	if err != nil || len(second) != 1 || second[0].ID != 1 {
		t.Fatal("history cursor skipped older ID", second, err)
	}
}
