package library

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// TestARemovedKeyStaysRemoved is the whole reason a tombstone exists.
//
// Deleting the row instead would leave the key merely absent, and absent is
// exactly the state an extractor is there to fill — so the next run would put
// it back and the removal would look like it never happened (spec 1.7.7).
func TestARemovedKeyStaysRemoved(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "a.jpg", "x")
	e := &fakeExtractor{id: "fake", version: 1, ns: []string{"fake"},
		tags: []extract.KV{text("fake:Make", "Apple")}}
	extractWith(t, l, e)

	removed, err := l.RemoveMetadata(t.Context(), file.ID, "fake:Make")
	if err != nil || !removed {
		t.Fatalf("RemoveMetadata = %v, %v", removed, err)
	}

	// A fresh version, so the extractor really runs again.
	e.version = 2
	extractWith(t, l, e)

	row, ok := metaOf(t, l, file.ID)["fake:Make"]
	if !ok {
		t.Fatal("the tombstone is gone; nothing is left to keep the key out")
	}
	if !row.Tombstone {
		t.Errorf("fake:Make came back as %q from %q", row.Value, row.Source)
	}
}

func TestRemovedMetadataIsHiddenButUndoable(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "a.jpg", "x")
	extractWith(t, l, &fakeExtractor{id: "fake", version: 1, ns: []string{"fake"},
		tags: []extract.KV{text("fake:Make", "Apple")}})

	if _, err := l.RemoveMetadata(t.Context(), file.ID, "fake:Make"); err != nil {
		t.Fatal(err)
	}
	ops, err := l.Ops(t.Context(), store.ListOpsOptions{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Undo(t.Context(), ops[0].ID); err != nil {
		t.Fatal(err)
	}

	row := metaOf(t, l, file.ID)["fake:Make"]
	if row.Tombstone || row.Value != "Apple" || row.Source != "fake" {
		t.Errorf("undo did not put the row back: %+v", row)
	}
}

func TestRemovingTwiceWritesNoSecondOp(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "a.jpg", "x")
	extractWith(t, l, &fakeExtractor{id: "fake", version: 1, ns: []string{"fake"},
		tags: []extract.KV{text("fake:Make", "Apple")}})

	if _, err := l.RemoveMetadata(t.Context(), file.ID, "fake:Make"); err != nil {
		t.Fatal(err)
	}
	before, _ := l.Ops(t.Context(), store.ListOpsOptions{Limit: 100})

	removed, err := l.RemoveMetadata(t.Context(), file.ID, "fake:Make")
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Error("removing an already-removed key reported a change")
	}
	after, _ := l.Ops(t.Context(), store.ListOpsOptions{Limit: 100})
	if len(after) != len(before) {
		t.Errorf("a no-op removal wrote %d op(s)", len(after)-len(before))
	}
}

func TestRemovingAnAbsentKeyIsAnError(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "a.jpg", "x")
	if _, err := l.RemoveMetadata(t.Context(), file.ID, "fake:Nope"); !errors.Is(err, ErrNoSuchMetadata) {
		t.Errorf("err = %v, want ErrNoSuchMetadata", err)
	}
}

// spec 1.5.2: captured_at is the kernel's conclusion, so removing the key it
// was drawn from has to redraw it — otherwise the file keeps a timestamp
// sourced from an observation that is no longer there.
func TestRemovingTheCaptureKeyRedrawsCapturedAt(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "a.jpg", "x")
	extractWith(t, l, &fakeExtractor{id: "exif", version: 1, ns: []string{"exif"},
		tags: []extract.KV{text("exif:DateTimeOriginal", "2019:08:12 14:03:00")}})

	withExif, _, _ := l.File(t.Context(), file.ID)
	if _, ok := withExif.CapturedAt.Time(l.zone()); !ok {
		t.Fatal("the fixture did not set captured_at")
	}

	if _, err := l.RemoveMetadata(t.Context(), file.ID, "exif:DateTimeOriginal"); err != nil {
		t.Fatal(err)
	}
	after, _, _ := l.File(t.Context(), file.ID)
	if after.CapturedAt == withExif.CapturedAt {
		t.Errorf("captured_at still %v, drawn from a key that is gone", after.CapturedAt)
	}
	// file:mtime is still there, so it falls back rather than going unknown.
	if _, ok := after.CapturedAt.Time(l.zone()); !ok {
		t.Error("captured_at fell all the way to unknown; file:mtime should have caught it")
	}
}

// Apply asserts the recorded before-state, which is what makes an edit
// undoable at all (spec 2.5). A payload whose before no longer matches must
// fail rather than overwrite.
func TestSetMetadataRefusesAStalePayload(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "a.jpg", "x")

	raw, err := oplog.Marshal(oplog.SetMetadataPayload{Files: []oplog.FileMetadataChange{{
		FileID: file.ID,
		Keys: []oplog.KeyChange{{
			Key:    "file:size",
			Before: &oplog.MetadataState{Value: "999", Type: extract.TypeInt, Source: extract.ImportSource},
			After:  &oplog.MetadataState{Source: extract.UserSource, Tombstone: true},
		}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = l.write(t.Context(), oplog.ActionSetMetadata, raw, oplog.OriginUser, writeOptions{})
	if !errors.Is(err, oplog.ErrStaleBefore) {
		t.Errorf("err = %v, want ErrStaleBefore", err)
	}
}
