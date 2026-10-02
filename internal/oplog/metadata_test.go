package oplog

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
)

func seedFile(t *testing.T, tx *fakeTx) domain.FileID {
	t.Helper()
	box, root := seedBox(t, tx)
	imp := importFixture(box, root)
	mustApply(t, tx, ActionImportFiles, imp)
	return imp.Files[0].FileID
}

func text(key, value string) extract.KV {
	return extract.KV{Key: key, Value: value, Type: extract.TypeText}
}

func metaOf(tx *fakeTx, file domain.FileID, key string) (metaRow, bool) {
	r, ok := tx.meta[[2]string{file.String(), key}]
	return r, ok
}

// spec 2.3: Inverse restores the previous values, which is only possible
// because they travel in the payload.
func TestInverseRestoresOverwrittenValues(t *testing.T) {
	tx := newFakeTx(opTime)
	file := seedFile(t, tx)

	p := ExtractMetadataPayload{
		Extractor: "exif", Version: 2, PreviousVersion: 1,
		Files: []FileExtraction{{
			FileID: file,
			Set:    []extract.KV{text("exif:Make", "Apple")},
			Was:    []extract.KV{text("exif:Make", "APPLE")},
		}},
	}
	seedExtractionBefore(t, tx, p)
	mustApply(t, tx, ActionExtractMetadata, p)
	if got, _ := metaOf(tx, file, "exif:Make"); got.kv.Value != "Apple" {
		t.Fatalf("forward apply wrote %q", got.kv.Value)
	}

	action, inverse := inverseOf(t, ActionExtractMetadata, mustRaw(t, p))
	if err := apply(t, tx, action, inverse); err != nil {
		t.Fatal(err)
	}
	if got, _ := metaOf(tx, file, "exif:Make"); got.kv.Value != "APPLE" {
		t.Errorf("after undo the value is %q, want the old one back", got.kv.Value)
	}
}

// A key this op created has no previous value, so undoing it must delete the
// row rather than leave an empty one behind.
func TestUndoingAFirstExtractionRemovesTheKey(t *testing.T) {
	tx := newFakeTx(opTime)
	file := seedFile(t, tx)

	p := ExtractMetadataPayload{
		Extractor: "exif", Version: 1,
		Files: []FileExtraction{{FileID: file, Set: []extract.KV{text("exif:Make", "Apple")}}},
	}
	mustApply(t, tx, ActionExtractMetadata, p)
	action, inverse := inverseOf(t, ActionExtractMetadata, mustRaw(t, p))
	if err := apply(t, tx, action, inverse); err != nil {
		t.Fatal(err)
	}
	if _, ok := metaOf(tx, file, "exif:Make"); ok {
		t.Error("undoing a first extraction left the row behind")
	}
}

// The one that is easy to miss and impossible to notice.
//
// Without the version swap, an undo restores the rows but leaves
// file_extractions saying this extractor already ran at the new version — so
// the next extraction skips these files. Nothing errors, and the data is
// simply never recomputed.
func TestInverseAlsoRollsBackTheRecordedVersion(t *testing.T) {
	tx := newFakeTx(opTime)
	file := seedFile(t, tx)
	key := [2]string{file.String(), "exif"}

	p := ExtractMetadataPayload{
		Extractor: "exif", Version: 2, PreviousVersion: 1,
		Files: []FileExtraction{{
			FileID: file,
			Set:    []extract.KV{text("exif:Make", "Apple")},
			Was:    []extract.KV{text("exif:Make", "APPLE")},
		}},
	}
	seedExtractionBefore(t, tx, p)
	mustApply(t, tx, ActionExtractMetadata, p)
	if tx.extracts[key] != 2 {
		t.Fatalf("forward apply recorded version %d, want 2", tx.extracts[key])
	}

	action, inverse := inverseOf(t, ActionExtractMetadata, mustRaw(t, p))
	if err := apply(t, tx, action, inverse); err != nil {
		t.Fatal(err)
	}
	if got := tx.extracts[key]; got != 1 {
		t.Errorf("after undo file_extractions says version %d, want 1 — "+
			"the next extraction would skip this file and never say why", got)
	}
}

// Undoing a first extraction has to leave the file looking untouched, or the
// extractor would never come back to it.
func TestUndoingAFirstExtractionClearsTheExtractionRow(t *testing.T) {
	tx := newFakeTx(opTime)
	file := seedFile(t, tx)
	key := [2]string{file.String(), "exif"}

	p := ExtractMetadataPayload{
		Extractor: "exif", Version: 1,
		Files: []FileExtraction{{FileID: file, Set: []extract.KV{text("exif:Make", "Apple")}}},
	}
	mustApply(t, tx, ActionExtractMetadata, p)
	action, inverse := inverseOf(t, ActionExtractMetadata, mustRaw(t, p))
	if err := apply(t, tx, action, inverse); err != nil {
		t.Fatal(err)
	}
	if _, ok := tx.extracts[key]; ok {
		t.Errorf("after undoing a first extraction the file still counts as extracted (version %d)", tx.extracts[key])
	}
}

// captured_at rides in the same op because it is computed from these rows
// (spec 1.5.2), so undoing the rows has to undo the conclusion too.
func TestInverseAlsoRestoresCapturedAt(t *testing.T) {
	tx := newFakeTx(opTime)
	file := seedFile(t, tx)
	before := domain.NewInstant(1000)
	after := domain.NewInstant(2000)

	p := ExtractMetadataPayload{
		Extractor: "exif", Version: 2, PreviousVersion: 1,
		Files: []FileExtraction{{
			FileID:      file,
			Set:         []extract.KV{text("exif:DateTimeOriginal", "2019:08:12 14:03:22")},
			Was:         []extract.KV{text("exif:DateTimeOriginal", "2019:08:12 14:03:00")},
			CapturedAt:  after,
			CapturedWas: before,
		}},
	}
	seedExtractionBefore(t, tx, p)
	mustApply(t, tx, ActionExtractMetadata, p)
	if got := tx.files[file].CapturedAt; got != after {
		t.Fatalf("forward apply set captured_at to %v", got)
	}
	action, inverse := inverseOf(t, ActionExtractMetadata, mustRaw(t, p))
	if err := apply(t, tx, action, inverse); err != nil {
		t.Fatal(err)
	}
	if got := tx.files[file].CapturedAt; got != before {
		t.Errorf("after undo captured_at is %v, want %v", got, before)
	}
}

// A key the extractor no longer produces is removed going forward and written
// back going in reverse.
func TestRemovedKeysRoundTrip(t *testing.T) {
	tx := newFakeTx(opTime)
	file := seedFile(t, tx)
	mustApply(t, tx, ActionExtractMetadata, ExtractMetadataPayload{
		Extractor: "exif", Version: 1,
		Files: []FileExtraction{{FileID: file, Set: []extract.KV{text("exif:Junk", "x")}}},
	})

	p := ExtractMetadataPayload{
		Extractor: "exif", Version: 2, PreviousVersion: 1,
		Files: []FileExtraction{{
			FileID:  file,
			Removed: []string{"exif:Junk"},
			Was:     []extract.KV{text("exif:Junk", "x")},
		}},
	}
	mustApply(t, tx, ActionExtractMetadata, p)
	if _, ok := metaOf(tx, file, "exif:Junk"); ok {
		t.Fatal("the removed key is still there")
	}
	action, inverse := inverseOf(t, ActionExtractMetadata, mustRaw(t, p))
	if err := apply(t, tx, action, inverse); err != nil {
		t.Fatal(err)
	}
	if got, ok := metaOf(tx, file, "exif:Junk"); !ok || got.kv.Value != "x" {
		t.Errorf("undo did not restore the removed key: %+v, %v", got, ok)
	}
}

// source is the extractor's id, not a constant. Without it, re-running exif
// would have no way to leave OCR's rows alone (spec 1.7.6 ①).
func TestSourceIsTheExtractorID(t *testing.T) {
	tx := newFakeTx(opTime)
	file := seedFile(t, tx)
	mustApply(t, tx, ActionExtractMetadata, ExtractMetadataPayload{
		Extractor: "ocr", Version: 1,
		Files: []FileExtraction{{FileID: file, Set: []extract.KV{text("ocr:text", "hello")}}},
	})
	got, _ := metaOf(tx, file, "ocr:text")
	if got.source != "ocr" {
		t.Errorf("source = %q, want the extractor id", got.source)
	}
}

// An op with no targets is an undo that does nothing (spec 1.7.1).
func TestExtractMetadataRefusesAnEmptyBatch(t *testing.T) {
	tx := newFakeTx(opTime)
	err := applyPayload(t, tx, ActionExtractMetadata, ExtractMetadataPayload{Extractor: "exif", Version: 1})
	if !errors.Is(err, errEmptyBatch) {
		t.Errorf("err = %v, want errEmptyBatch", err)
	}
}

// A payload from a newer kist can carry a type this build does not know.
// Half-applying it would be worse than refusing it (spec 2.2).
func TestApplyRefusesAnUnknownType(t *testing.T) {
	tx := newFakeTx(opTime)
	file := seedFile(t, tx)
	err := applyPayload(t, tx, ActionExtractMetadata, ExtractMetadataPayload{
		Extractor: "exif", Version: 1,
		Files: []FileExtraction{{FileID: file, Set: []extract.KV{
			{Key: "exif:X", Value: "1", Type: "rational"},
		}}},
	})
	if err == nil {
		t.Error("an unknown value type was applied")
	}
}

// An event is applied once per history. Catch-up skips committed IDs; a
// second application against a changed before-state must be refused.
func TestExtractionRefusesRepeatedApply(t *testing.T) {
	tx := newFakeTx(opTime)
	file := seedFile(t, tx)
	p := ExtractMetadataPayload{
		Extractor: "exif", Version: 1,
		Files: []FileExtraction{{
			FileID:     file,
			Set:        []extract.KV{text("exif:Make", "Apple")},
			CapturedAt: domain.NewInstant(1565600000),
		}},
	}
	mustApply(t, tx, ActionExtractMetadata, p)
	first, _ := metaOf(tx, file, "exif:Make")
	firstCaptured := tx.files[file].CapturedAt

	if err := applyPayload(t, tx, ActionExtractMetadata, p); !errors.Is(err, ErrStaleBefore) {
		t.Fatalf("repeated apply = %v, want ErrStaleBefore", err)
	}
	second, _ := metaOf(tx, file, "exif:Make")
	if first != second || tx.files[file].CapturedAt != firstCaptured {
		t.Error("applying the same payload twice changed the result")
	}
}

// Invariant 4: op_files has to index this op, or "everything that happened to
// this file" would be missing a chapter.
func TestTargetsListsEveryFile(t *testing.T) {
	tx := newFakeTx(opTime)
	box, root := seedBox(t, tx)
	imp := importFixture(box, root)
	mustApply(t, tx, ActionImportFiles, imp)

	h, err := Lookup(ActionExtractMetadata)
	if err != nil {
		t.Fatal(err)
	}
	targets, err := h.Targets(mustRaw(t, ExtractMetadataPayload{
		Extractor: "exif", Version: 1,
		Files: []FileExtraction{
			{FileID: imp.Files[0].FileID, Set: []extract.KV{text("exif:Make", "Apple")}},
			{FileID: imp.Files[1].FileID, Set: []extract.KV{text("exif:Make", "Nikon")}},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(targets.Files) != 2 {
		t.Errorf("Targets reported %d files, want 2", len(targets.Files))
	}
	if len(targets.Boxes) != 0 {
		t.Errorf("Targets reported %d boxes; metadata changes nothing about a box", len(targets.Boxes))
	}
}

// seedExtractionBefore prepares the actual state an update payload asserts.
func seedExtractionBefore(t *testing.T, tx *fakeTx, p ExtractMetadataPayload) {
	t.Helper()
	for _, f := range p.Files {
		for _, kv := range f.Was {
			if err := tx.SetFileMetadata(f.FileID, kv, p.Extractor); err != nil {
				t.Fatal(err)
			}
		}
		if err := tx.SetCapturedAt(f.FileID, f.CapturedWas); err != nil {
			t.Fatal(err)
		}
		if err := tx.SetFileExtraction(f.FileID, p.Extractor, p.PreviousVersion); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExtractionUndoRefusesChangedState(t *testing.T) {
	for _, kind := range []string{"value", "source", "tombstone", "capture time"} {
		t.Run(kind, func(t *testing.T) {
			tx := newFakeTx(opTime)
			file := seedFile(t, tx)
			p := ExtractMetadataPayload{Extractor: "exif", Version: 1,
				Files: []FileExtraction{{FileID: file,
					Set:        []extract.KV{text("exif:Make", "Apple")},
					CapturedAt: domain.NewInstant(2000)}}}
			mustApply(t, tx, ActionExtractMetadata, p)
			switch kind {
			case "value":
				tx.SetFileMetadata(file, text("exif:Make", "Nikon"), "exif")
			case "source":
				tx.SetFileMetadata(file, text("exif:Make", "Apple"), "user")
			case "tombstone":
				tx.TombstoneFileMetadata(file, "exif:Make")
			case "capture time":
				tx.SetCapturedAt(file, domain.NewInstant(3000))
			}
			action, inverse := inverseOf(t, ActionExtractMetadata, mustRaw(t, p))
			if err := apply(t, tx, action, inverse); !errors.Is(err, ErrStaleBefore) {
				t.Fatalf("undo = %v, want ErrStaleBefore", err)
			}
		})
	}
}
