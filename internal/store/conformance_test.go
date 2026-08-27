package store

import (
	"database/sql"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/query/querytest"
)

// TestSQLiteConformance is the SQLite implementation's half of spec 4.2's
// contract.
//
// When Postgres lands in v0.2 it gets a file exactly like this one and the
// same suite runs against it — which is the whole point of writing the suite
// now, with one implementation, rather than then, with two and no agreement
// about which was right.
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

	for _, box := range f.Boxes {
		if err := vtx.InsertBox(box); err != nil {
			t.Fatal(err)
		}
	}
	for _, folder := range f.Folders {
		if err := vtx.InsertFolder(folder); err != nil {
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

func (b *sqliteBackend) Files(q query.Query) ([]domain.File, string, error) {
	return ListFiles(b.db, q)
}
