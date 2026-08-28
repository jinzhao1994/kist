package library

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/store"
)

// fakeExtractor produces a fixed set of tags, so the library-level behaviour
// under test is diffing and batching rather than EXIF parsing.
type fakeExtractor struct {
	id      string
	version int
	ns      []string
	mimes   []string
	tags    []extract.KV
	err     error
	calls   int
}

func (f *fakeExtractor) ID() string           { return f.id }
func (f *fakeExtractor) Version() int         { return f.version }
func (f *fakeExtractor) Namespaces() []string { return f.ns }

func (f *fakeExtractor) Wants(mime string) bool {
	if f.mimes == nil {
		return true
	}
	for _, m := range f.mimes {
		if m == mime {
			return true
		}
	}
	return false
}

func (f *fakeExtractor) Extract(context.Context, io.ReadSeeker, string) ([]extract.KV, error) {
	f.calls++
	return f.tags, f.err
}

func extractWith(t *testing.T, l *Library, e extract.Extractor) *ExtractResult {
	t.Helper()
	res, err := l.extractUsing(t.Context(), []extract.Extractor{e}, ExtractOptions{BatchSize: 100}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func metaOf(t *testing.T, l *Library, file domain.FileID) map[string]store.MetadataRow {
	t.Helper()
	rows, err := store.FileMetadata(l.db, file)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]store.MetadataRow{}
	for _, r := range rows {
		out[r.Key] = r
	}
	return out
}

func text(key, value string) extract.KV {
	return extract.KV{Key: key, Value: value, Type: extract.TypeText}
}

// spec 1.7.6 ④ and the "only what changed" rule: re-running the same
// extractor at the same version writes nothing at all.
func TestReExtractingWithTheSameVersionWritesNoOp(t *testing.T) {
	l, box := openWithBox(t)
	importOneFile(t, l, box, "a.jpg", "x")
	e := &fakeExtractor{id: "fake", version: 1, ns: []string{"fake"},
		tags: []extract.KV{text("fake:Make", "Apple")}}

	if res := extractWith(t, l, e); len(res.OpIDs) != 1 {
		t.Fatalf("first run wrote %d ops, want 1", len(res.OpIDs))
	}
	before, _ := l.Ops(t.Context(), ListOpsOptions{})

	res := extractWith(t, l, e)
	if len(res.OpIDs) != 0 {
		t.Errorf("a second run at the same version wrote %d ops", len(res.OpIDs))
	}
	after, _ := l.Ops(t.Context(), ListOpsOptions{})
	if len(after) != len(before) {
		t.Errorf("the op log grew from %d to %d", len(before), len(after))
	}
	if e.calls != 1 {
		t.Errorf("the extractor ran %d times; the second run should not have read the file at all", e.calls)
	}
}

// A bumped version re-reads, but only the keys that actually differ reach the
// payload — otherwise an undo would restore values this op never changed.
func TestABumpedVersionRewritesOnlyWhatChanged(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "a.jpg", "x")
	e := &fakeExtractor{id: "fake", version: 1, ns: []string{"fake"},
		tags: []extract.KV{text("fake:Make", "Apple"), text("fake:Model", "iPhone")}}
	extractWith(t, l, e)

	e.version = 2
	e.tags = []extract.KV{text("fake:Make", "Apple"), text("fake:Model", "iPhone 15")}
	res := extractWith(t, l, e)
	if len(res.OpIDs) != 1 {
		t.Fatalf("wrote %d ops, want 1", len(res.OpIDs))
	}

	// spec 2.2.1: payload does not come back from a list query, so ask for the
	// one op directly.
	row, ok, err := store.GetOp(l.db, res.OpIDs[0])
	if err != nil || !ok {
		t.Fatal(err)
	}
	payload := string(row.Payload)
	if !strings.Contains(payload, "fake:Model") {
		t.Error("the changed key is not in the payload")
	}
	if strings.Contains(payload, `"fake:Make"`) {
		t.Errorf("the unchanged key is in the payload:\n%s", payload)
	}
	if got := metaOf(t, l, file.ID)["fake:Model"].Value; got != "iPhone 15" {
		t.Errorf("stored value = %q, want the new one", got)
	}
}

// spec 1.7 rule ②: rows a person wrote are never touched. v0.1 has no UI for
// writing them, so the rule is pinned by inserting one directly — it has to
// hold now, because by the time v0.2 can write them it is too late to find out.
func TestExtractionNeverOverwritesAUserRow(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "a.jpg", "x")

	tx, err := l.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	vtx := store.NewViewTx(tx, l.now())
	if err := vtx.SetFileMetadata(file.ID, text("fake:Make", "handwritten"), extract.UserSource); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	e := &fakeExtractor{id: "fake", version: 1, ns: []string{"fake"},
		tags: []extract.KV{text("fake:Make", "Apple")}}
	extractWith(t, l, e)

	got := metaOf(t, l, file.ID)["fake:Make"]
	if got.Value != "handwritten" || got.Source != extract.UserSource {
		t.Errorf("a user row was overwritten: %+v", got)
	}
}

// spec 3.7.1 ③: one file failing does not stop the batch, and the failed file
// is not recorded as extracted — so a later run picks it up again.
func TestAFailedFileIsNotRecordedAsExtracted(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "a.jpg", "x")
	e := &fakeExtractor{id: "fake", version: 1, ns: []string{"fake"},
		err: errFake}
	res := extractWith(t, l, e)

	if len(res.Files) != 1 || res.Files[0].Status != FileFailed {
		t.Fatalf("results = %+v, want one failure", res.Files)
	}
	got, err := store.Extractions(l.db, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, recorded := got["fake"]; recorded {
		t.Error("a file that failed extraction was recorded as extracted; it would never be retried")
	}
}

// A file the extractor does not want is not a failure, and it does count as
// seen — otherwise every run would re-read every PDF in the library.
func TestAnUnwantedFileIsRecordedAsSeen(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "notes.txt", "hello")
	e := &fakeExtractor{id: "fake", version: 1, ns: []string{"fake"},
		mimes: []string{"image/jpeg"}}
	res := extractWith(t, l, e)

	if len(res.Files) != 1 || res.Files[0].Status != FileSkipped {
		t.Fatalf("results = %+v, want one skip", res.Files)
	}
	got, _ := store.Extractions(l.db, file.ID)
	if got["fake"] != 1 {
		t.Errorf("a skipped file was not recorded as seen: %v", got)
	}
	if e.calls != 0 {
		t.Errorf("the extractor was called %d times for a file it does not want", e.calls)
	}
}

// spec 1.5.2: captured_at is the kernel's conclusion, written in the same op
// as the rows it was drawn from.
func TestExtractSetsCapturedAt(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "a.jpg", "x")
	e := &fakeExtractor{id: "fake", version: 1, ns: []string{"fake", "exif"},
		tags: []extract.KV{text("exif:DateTimeOriginal", "2019:08:12 14:03:22")}}
	extractWith(t, l, e)

	got, ok, err := l.File(t.Context(), file.ID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if got.CapturedAt.IsZero() {
		t.Fatal("captured_at was not set")
	}
	sec, _ := got.CapturedAt.Unix()
	if sec == 0 {
		t.Error("captured_at is the epoch")
	}
}

// "Inputs changed" means recomputing, and a new extractor is exactly that
// case: geo:timezone from a GPS lookup has to be able to move a conclusion
// EXIF alone had already drawn (spec 1.5.2).
func TestANewExtractorRecomputesCapturedAt(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "a.jpg", "x")

	exif := &fakeExtractor{id: "exif-ish", version: 1, ns: []string{"exif"},
		tags: []extract.KV{text("exif:DateTimeOriginal", "2019:08:12 14:03:22")}}
	extractWith(t, l, exif)
	first, _, _ := l.File(t.Context(), file.ID)

	geo := &fakeExtractor{id: "geo-ish", version: 1, ns: []string{"geo"},
		tags: []extract.KV{text("geo:timezone", "America/Los_Angeles")}}
	extractWith(t, l, geo)
	second, _, _ := l.File(t.Context(), file.ID)

	// Asserting only that it moved is not enough: wiping it would satisfy
	// that too, and wiping is exactly what a recompute that looked at one
	// extractor's output in isolation would do — the EXIF wall clock would be
	// invisible to it. Name the answer.
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skip("no tzdata on this system")
	}
	want := time.Date(2019, 8, 12, 14, 3, 22, 0, la).Unix()
	got, ok := second.CapturedAt.Unix()
	if !ok {
		t.Fatal("captured_at was cleared; the recompute lost the EXIF wall clock")
	}
	if got != want {
		t.Errorf("captured_at = %d, want %d (the same wall clock, read in Los Angeles)", got, want)
	}
	if first.CapturedAt == second.CapturedAt {
		t.Error("adding a timezone did not move captured_at at all")
	}
}

var errFake = errFakeType{}

type errFakeType struct{}

func (errFakeType) Error() string { return "fake extractor failure" }
