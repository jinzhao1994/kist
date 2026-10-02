package store

import (
	"database/sql"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/query/querytest"
)

// TestSQLiteConformance runs spec 4.2's contract against the only backend
// there is (spec 4.1).
//
// The suite lives in its own package rather than here because the contract
// belongs to internal/query, not to whatever translates it: the assertions are
// about which rows come back, and store is one way of answering that.
func TestSQLiteConformance(t *testing.T) {
	querytest.Run(t, func(t *testing.T) querytest.Backend {
		return &sqliteBackend{db: migrated(t)}
	})
}

type sqliteBackend struct{ db *sql.DB }

// Seed writes the fixture straight into the tables.
//
// It goes through store's own write side rather than through library on
// purpose: the conformance suite is about the query contract, and dragging the
// op log into it would mean a second implementation had to have an op log
// before it could be tested against a query.
func (b *sqliteBackend) Seed(t *testing.T, f querytest.Fixture) {
	t.Helper()
	tx, err := b.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	vtx := NewViewTx(tx, testTime)

	// Folders first: boxes.root_folder_id points at one.
	for _, folder := range f.Folders {
		if err := vtx.InsertFolder(folder); err != nil {
			t.Fatal(err)
		}
	}
	for _, box := range f.Boxes {
		if err := vtx.InsertBox(box); err != nil {
			t.Fatal(err)
		}
	}
	for _, tag := range f.Tags {
		if err := vtx.InsertTag(tag); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range f.Files {
		if err := vtx.UpsertBlob(file.BlobKey, file.BlobKey, 1); err != nil {
			t.Fatal(err)
		}
		if err := vtx.InsertFile(file); err != nil {
			t.Fatal(err)
		}
		if err := setCapturedAt(tx, file); err != nil {
			t.Fatal(err)
		}
	}
	for id, kind := range f.Kinds {
		if err := vtx.SetFileMetadata(id, extract.KV{Key: "media:kind", Value: kind, Type: extract.TypeText}, "media"); err != nil {
			t.Fatal(err)
		}
	}
	for file, tags := range f.FileTags {
		for _, tag := range tags {
			if err := vtx.AddFileTag(file, tag, "user"); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

// setCapturedAt writes the capture time separately, because InsertFile does
// not carry one — no file has one until M3's extractor runs (spec 4.2.1), and
// the suite needs them to test the time and sort halves of the contract at all.
func setCapturedAt(tx *sql.Tx, f domain.File) error {
	if f.CapturedAt.IsZero() {
		return nil
	}
	_, err := tx.Exec(`UPDATE files SET captured_at = ? WHERE id = ?`, f.CapturedAt, f.ID)
	return err
}

// The zone is fixed to UTC, matching what querytest's fixtures mean by a date
// (spec 1.5.2). Which zone it is does not matter to any case — they all assert
// relative order or containment — but a backend that read the machine's zone
// would make the suite's answers depend on where it runs.
func (b *sqliteBackend) Files(q query.Query) ([]domain.File, string, error) {
	return ListFiles(b.db, q, time.UTC)
}
