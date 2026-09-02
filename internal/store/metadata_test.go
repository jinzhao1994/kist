package store

import (
	"database/sql"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
)

// seedFile puts one file in a fresh library and returns its id.
func seedFile(t *testing.T, db *sql.DB, name string) domain.FileID {
	t.Helper()
	box := mustBox(t, "云南")
	root := domain.Folder{ID: domain.NewFolderID(), BoxID: box.ID}
	f, err := domain.NewFile(domain.NewFileID(), box.ID, root.ID, name, "sha256:"+name)
	if err != nil {
		t.Fatal(err)
	}
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.InsertBox(box); err != nil {
			t.Fatal(err)
		}
		if err := vtx.InsertFolder(root); err != nil {
			t.Fatal(err)
		}
		if err := vtx.UpsertBlob(f.BlobKey, "aa", 1); err != nil {
			t.Fatal(err)
		}
		if err := vtx.InsertFile(f); err != nil {
			t.Fatal(err)
		}
	})
	return f.ID
}

func setMeta(t *testing.T, db *sql.DB, file domain.FileID, key, value string, typ extract.Type, source string) {
	t.Helper()
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.SetFileMetadata(file, extract.KV{Key: key, Value: value, Type: typ}, source); err != nil {
			t.Fatal(err)
		}
	})
}

// spec 1.7.6 ③: this is the query file_extractions exists for. Answering it
// from payloads would mean parsing every op just to discover there is nothing
// to do — and payload is not queryable anyway (invariant 4).
func TestFilesNeedingExtractionFindsNewAndStale(t *testing.T) {
	db := migrated(t)
	never := seedFile(t, db, "never.jpg")
	old := seedFile(t, db, "old.jpg")
	current := seedFile(t, db, "current.jpg")

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.SetFileExtraction(old, "exif", 1); err != nil {
			t.Fatal(err)
		}
		if err := vtx.SetFileExtraction(current, "exif", 2); err != nil {
			t.Fatal(err)
		}
	})

	got, err := FilesNeedingExtraction(db, "exif", 2, domain.FileID{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	want := map[domain.FileID]bool{never: true, old: true}
	if len(got) != 2 {
		t.Fatalf("got %d files, want 2 (never extracted, and stale)", len(got))
	}
	for _, id := range got {
		if !want[id] {
			t.Errorf("unexpected file %s; the one already at version 2 must not be listed", id)
		}
	}
}

// A different extractor's record must not make a file look done. Without the
// extractor in the join condition, running OCR would silently mark every file
// as having been seen by exif too.
func TestFilesNeedingExtractionIsPerExtractor(t *testing.T) {
	db := migrated(t)
	file := seedFile(t, db, "a.jpg")
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.SetFileExtraction(file, "ocr", 1); err != nil {
			t.Fatal(err)
		}
	})
	got, err := FilesNeedingExtraction(db, "exif", 1, domain.FileID{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("got %d files, want 1 — an OCR record says nothing about exif", len(got))
	}
}

func TestFilesNeedingExtractionRespectsTheLimit(t *testing.T) {
	db := migrated(t)
	for _, name := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		seedFile(t, db, name)
	}
	got, err := FilesNeedingExtraction(db, "exif", 1, domain.FileID{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("got %d files, want the limit of 2", len(got))
	}
}

// Writing the same key twice replaces rather than duplicating. Apply has to be
// able to run one payload twice and land in the same place (spec 2.3.1).
func TestSetFileMetadataReplaces(t *testing.T) {
	db := migrated(t)
	file := seedFile(t, db, "a.jpg")
	setMeta(t, db, file, "exif:Make", "APPLE", extract.TypeText, "exif")
	setMeta(t, db, file, "exif:Make", "Apple", extract.TypeText, "exif")

	rows, err := FileMetadata(db, file)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("%d rows, want 1", len(rows))
	}
	if rows[0].Value != "Apple" {
		t.Errorf("value = %q, want the second write", rows[0].Value)
	}
}

// The type and the source come back as written. They are what a later value_int
// column and a later re-run would each depend on (spec 1.7.5, 1.7.6 ①).
func TestFileMetadataRoundTripsTypeAndSource(t *testing.T) {
	db := migrated(t)
	file := seedFile(t, db, "a.jpg")
	setMeta(t, db, file, "exif:ISO", "400", extract.TypeInt, "exif")
	setMeta(t, db, file, "user:有效期", "2029-08-12", extract.TypeText, extract.UserSource)

	rows, err := FileMetadata(db, file)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("%d rows, want 2", len(rows))
	}
	// Ordered by key, so exif: comes before user:.
	if rows[0].Type != extract.TypeInt || rows[0].Source != "exif" {
		t.Errorf("row 0 = %+v", rows[0])
	}
	if rows[1].Source != extract.UserSource {
		t.Errorf("row 1 source = %q, want %q", rows[1].Source, extract.UserSource)
	}
}

// captured_at is written separately from the file row because it is resolved
// long after the import and re-resolved whenever its inputs change (spec 1.5.2).
func TestSetCapturedAtRoundTrips(t *testing.T) {
	db := migrated(t)
	file := seedFile(t, db, "a.jpg")

	// A 1960 photograph: negative, and ordinary.
	want := domain.NewInstantFromTime(time.Date(1960, 1, 1, 0, 0, 0, 0, time.UTC))
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.SetCapturedAt(file, want); err != nil {
			t.Fatal(err)
		}
	})
	var got domain.File
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		var ok bool
		var err error
		if got, ok, err = vtx.GetFile(file); err != nil || !ok {
			t.Fatal(err)
		}
	})
	if got.CapturedAt != want {
		t.Errorf("captured_at = %v, want %v", got.CapturedAt, want)
	}

	// And back to unset, which is what undoing a first extraction does.
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.SetCapturedAt(file, domain.Instant{}); err != nil {
			t.Fatal(err)
		}
	})
	viewTx(t, db, testTime, func(vtx *ViewTx) { got, _, _ = vtx.GetFile(file) })
	if !got.CapturedAt.IsZero() {
		t.Errorf("captured_at = %v, want unset", got.CapturedAt)
	}
}
