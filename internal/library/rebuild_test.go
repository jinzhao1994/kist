package library

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

// busyLibrary exercises every op type, including an undo, so the rebuild proof
// covers all of them.
func busyLibrary(t *testing.T) *Library {
	t.Helper()
	l := openTestLibrary(t)
	ctx := context.Background()

	places, err := l.CreateTag(ctx, "地点", nil)
	if err != nil {
		t.Fatal(err)
	}
	yunnan, err := l.CreateTag(ctx, "云南", &places.ID)
	if err != nil {
		t.Fatal(err)
	}
	people, _ := l.CreateTag(ctx, "人物", nil)
	doomed, _ := l.CreateTag(ctx, "要删掉的", nil)
	if err := l.DeleteTag(ctx, doomed.ID); err != nil {
		t.Fatal(err)
	}
	if err := l.RenameTag(ctx, yunnan.ID, "云南省"); err != nil {
		t.Fatal(err)
	}
	if err := l.MoveTag(ctx, yunnan.ID, &people.ID); err != nil {
		t.Fatal(err)
	}

	start, _ := domain.ParseLocalDate("2019-08")
	var boxes []domain.BoxID
	for _, n := range []string{"一", "二", "三"} {
		b, err := l.CreateBox(ctx, n, "note "+n, start, domain.LocalDate{})
		if err != nil {
			t.Fatal(err)
		}
		boxes = append(boxes, b.ID)
	}
	if _, err := l.UpdateBox(ctx, boxes[0], oplog.BoxFields{Name: "一号", Description: "renamed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.TagBoxes(ctx, yunnan.ID, boxes); err != nil {
		t.Fatal(err)
	}
	if _, err := l.UntagBoxes(ctx, yunnan.ID, boxes[2:]); err != nil {
		t.Fatal(err)
	}

	// Files, in a two-level tree, so the proof covers files / folders / blobs
	// as well. Before M1c those four tables were empty in this fixture and
	// invariant 3 had never actually been checked along the file path.
	src := writeTree(t, map[string]string{
		"封面.jpg":         "cover",
		"day1/IMG_1.jpg": "one",
		"day1/IMG_2.jpg": "two",
		"day2/IMG_3.jpg": "three",
		"day2/dup.jpg":   "one", // same content as IMG_1: one blob, two rows
	})
	if _, err := l.Import(ctx, src, ImportOptions{Box: boxes[1]}); err != nil {
		t.Fatal(err)
	}

	// Tag some of those files, so file_tags is covered too — and untag one, so
	// the replay handles both directions.
	imported, _, err := store.ListFiles(l.db, query.Query{Boxes: []domain.BoxID{boxes[1]}}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]domain.FileID, len(imported))
	for i, f := range imported {
		ids[i] = f.ID
	}
	if _, err := l.TagFiles(ctx, yunnan.ID, ids); err != nil {
		t.Fatal(err)
	}
	if _, err := l.UntagFiles(ctx, yunnan.ID, ids[:1]); err != nil {
		t.Fatal(err)
	}

	// An import that gets undone, so the replay has to handle delete_files too.
	undone := writeTree(t, map[string]string{"temp/scratch.txt": "throwaway"})
	if _, err := l.Import(ctx, undone, ImportOptions{Box: boxes[2]}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Undo(ctx, lastOpID(t, l)); err != nil {
		t.Fatal(err)
	}

	// A box that gets created and then undone, so the replay has to handle an
	// inverse op too.
	if _, err := l.CreateBox(ctx, "临时", "", domain.LocalDate{}, domain.LocalDate{}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Undo(ctx, lastOpID(t, l)); err != nil {
		t.Fatal(err)
	}

	seedExtractions(t, l)
	return l
}

// seedExtractions puts M3's tables into the proof.
//
// Four shapes, because each one replays differently:
//
//   - a first extraction, where the before state is empty
//   - a second at a higher version, where Set and Was are both populated and
//     one key disappears
//   - an extraction that is then undone, so the replay has to run the inverse
//     including its version rollback
//   - a second extractor's rows alongside the first's, and the import's own
//     file:size / file:mtime rows, so the replay has to keep three sources
//     apart inside one file's row set
//
// A negative captured_at is in there too: a 1960 photograph is exactly what an
// archive holds, and it is the value a TEXT column used to get wrong.
func seedExtractions(t *testing.T, l *Library) {
	t.Helper()
	ctx := context.Background()

	first := &fakeExtractor{id: "seed", version: 1, ns: []string{"seed"}, tags: []extract.KV{
		{Key: "seed:Make", Value: "Apple", Type: extract.TypeText},
		{Key: "seed:Junk", Value: "x", Type: extract.TypeText},
		{Key: "exif:DateTimeOriginal", Value: "1960:03:04 09:00:00", Type: extract.TypeText},
	}}
	first.ns = []string{"seed", "exif"}
	if _, err := l.extractUsing(ctx, []extract.Extractor{first}, ExtractOptions{BatchSize: 2}, nil); err != nil {
		t.Fatal(err)
	}

	second := &fakeExtractor{id: "seed", version: 2, ns: []string{"seed", "exif"}, tags: []extract.KV{
		{Key: "seed:Make", Value: "APPLE", Type: extract.TypeText},
		{Key: "seed:ISO", Value: "400", Type: extract.TypeInt},
		{Key: "exif:DateTimeOriginal", Value: "1960:03:04 09:00:00", Type: extract.TypeText},
	}}
	if _, err := l.extractUsing(ctx, []extract.Extractor{second}, ExtractOptions{BatchSize: 100}, nil); err != nil {
		t.Fatal(err)
	}

	undone := &fakeExtractor{id: "undone", version: 1, ns: []string{"undone"}, tags: []extract.KV{
		{Key: "undone:x", Value: "1", Type: extract.TypeReal},
	}}
	if _, err := l.extractUsing(ctx, []extract.Extractor{undone}, ExtractOptions{BatchSize: 100}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Undo(ctx, lastOpID(t, l)); err != nil {
		t.Fatal(err)
	}

	// A second extractor, so every file ends up with rows from two sources.
	// The replay has to put each row back under the source that wrote it, and
	// a mix-up would be invisible in any test that only ever had one.
	other := &fakeExtractor{id: "other", version: 1, ns: []string{"other"}, tags: []extract.KV{
		{Key: "other:note", Value: "2029-08-12", Type: extract.TypeText},
	}}
	if _, err := l.extractUsing(ctx, []extract.Extractor{other}, ExtractOptions{BatchSize: 100}, nil); err != nil {
		t.Fatal(err)
	}

	// A row a person wrote — the sharper case, because it is the one an
	// extraction must never touch. It is also the only NULL-valued row the
	// schema has, so it is the only thing that would catch a replay writing
	// an empty string where the library holds NULL.
	files, _, err := store.ListFiles(l.db, query.Query{Limit: 1}, l.zone())
	if err != nil || len(files) == 0 {
		t.Fatalf("list a file to edit: %v", err)
	}
	if _, err := l.RemoveMetadata(ctx, files[0].ID, "other:note"); err != nil {
		t.Fatal(err)
	}
}

// The fixture is only a proof if it actually populated the file tables.
//
// The counts are worth stating rather than just asserting non-zero:
//
//   - files 5: the four distinct files plus dup.jpg, which shares a blob with
//     IMG_1.jpg. Deduplication happens in blob storage, not in the file table
//   - folders 5: three box roots (临时 was undone, taking its root with it)
//     plus day1 and day2. temp/ went away with the undone import
//   - blobs 5: four distinct contents from the surviving import, plus the one
//     from the import that was undone. Undo does not reclaim blobs — that is
//     GC's job and v0.1 has none (spec 3.9), so an orphan blob costs space and
//     nothing else
func TestBusyLibraryCoversTheFileTables(t *testing.T) {
	l := busyLibrary(t)
	for table, want := range map[string]int{"files": 5, "folders": 5, "blobs": 5, "file_tags": 4,
		"file_metadata": 30, "file_extractions": 10} {
		var n int
		if err := l.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Errorf("%s has %d rows, want %d — the rebuild proof only covers what the fixture creates", table, n, want)
		}
	}
}

// This is the test the whole plan is built around (spec 2.3.1). Every claim
// about Apply being a pure function is only as good as this comparison.
func TestRebuildReproducesTheLibraryExactly(t *testing.T) {
	l := busyLibrary(t)
	ctx := context.Background()

	before, err := store.Snapshot(l.db)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if !strings.Contains(before, "== boxes ==") || len(before) < 100 {
		t.Fatalf("the snapshot looks empty:\n%s", before)
	}

	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	after, err := store.Snapshot(l.db)
	if err != nil {
		t.Fatalf("Snapshot after rebuild: %v", err)
	}
	if before != after {
		t.Errorf("rebuild did not reproduce the library.\n%s", firstDifference(before, after))
	}
}

// firstDifference points at the line that broke, because a diff of two
// thousand-line dumps is not a bug report.
func firstDifference(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(al) || i < len(bl); i++ {
		var x, y string
		if i < len(al) {
			x = al[i]
		}
		if i < len(bl) {
			y = bl[i]
		}
		if x != y {
			return fmt.Sprintf("line %d:\n  before: %q\n  after:  %q", i+1, x, y)
		}
	}
	return "the dumps differ but no line does; check trailing whitespace"
}

func TestRebuildIsIdempotent(t *testing.T) {
	l := busyLibrary(t)
	ctx := context.Background()

	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("first Rebuild: %v", err)
	}
	once, _ := store.Snapshot(l.db)
	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("second Rebuild: %v", err)
	}
	twice, _ := store.Snapshot(l.db)
	if once != twice {
		t.Error("rebuilding twice gave a different result than rebuilding once")
	}
}

func TestRebuildLeavesTheOpLogAlone(t *testing.T) {
	// Invariant 1: ops is the authority. A rebuild that rewrote it would not be
	// a repair, it would be the one thing this design exists to prevent.
	l := busyLibrary(t)
	ctx := context.Background()

	before, _ := l.Ops(ctx, store.ListOpsOptions{})
	payloads := map[int64][]byte{}
	for _, op := range before {
		row, ok, err := store.GetOp(l.db, op.ID)
		if err != nil || !ok {
			t.Fatalf("GetOp(%d): %v ok=%v", op.ID, err, ok)
		}
		payloads[op.ID] = row.Payload
	}

	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	after, _ := l.Ops(ctx, store.ListOpsOptions{})

	if len(before) != len(after) {
		t.Fatalf("op count went from %d to %d", len(before), len(after))
	}
	for i := range before {
		if before[i].ID != after[i].ID || before[i].Action != after[i].Action ||
			!before[i].TS.Equal(after[i].TS) {
			t.Errorf("op %d changed during the rebuild", before[i].ID)
		}
	}
	// ListOps deliberately leaves the payload out, so comparing it there would
	// only ever compare two empty slices. Read the real thing.
	for i := range before {
		row, ok, err := store.GetOp(l.db, before[i].ID)
		if err != nil || !ok {
			t.Fatalf("GetOp(%d): %v ok=%v", before[i].ID, err, ok)
		}
		if string(row.Payload) != string(payloads[before[i].ID]) {
			t.Errorf("the payload of op %d changed during the rebuild", before[i].ID)
		}
	}
}

func TestRebuildRegeneratesTheIndexTables(t *testing.T) {
	// spec 2.2.1: op_boxes is derived from the payload, so wiping it and
	// replaying must put it back exactly.
	l := busyLibrary(t)
	ctx := context.Background()

	var before int
	l.db.QueryRow(`SELECT count(*) FROM op_boxes`).Scan(&before)
	if before == 0 {
		t.Fatal("the fixture produced no op_boxes rows")
	}

	if _, err := l.db.Exec(`DELETE FROM op_boxes`); err != nil {
		t.Fatal(err)
	}
	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	var after int
	l.db.QueryRow(`SELECT count(*) FROM op_boxes`).Scan(&after)
	if after != before {
		t.Errorf("op_boxes has %d rows after the rebuild, want %d", after, before)
	}
}

func TestRebuildRepairsAWreckedView(t *testing.T) {
	// "物化视图坏了不是数据丢失" (spec 2.1) — the promise, tested.
	l := busyLibrary(t)
	ctx := context.Background()
	before, _ := store.Snapshot(l.db)

	if _, err := l.db.Exec(`DELETE FROM box_tags`); err != nil {
		t.Fatal(err)
	}
	if _, err := l.db.Exec(`UPDATE boxes SET name = 'corrupted', name_fold = 'corrupted'`); err != nil {
		t.Fatal(err)
	}

	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	after, _ := store.Snapshot(l.db)
	if before != after {
		t.Errorf("rebuild did not repair the damage.\n%s", firstDifference(before, after))
	}
}

func TestRebuildReloadsTheTagTree(t *testing.T) {
	l := busyLibrary(t)
	ctx := context.Background()
	before := tagTotal(t, l)

	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if got := tagTotal(t, l); got != before {
		t.Errorf("the tag tree has %d nodes after the rebuild, want %d", got, before)
	}
	if _, ok := tagByName(t, l, "云南省"); !ok {
		t.Error("the rebuilt tree lost a tag")
	}
}

// Rebuild must stream the log rather than read it into a slice.
//
// The first attempt at this test measured the heap across a real rebuild of
// 20 imports x 100 files. It does not work: the replay's own working set —
// two thousand file rows, their domain values, the driver's buffers — is
// larger than the payload it is supposed to be isolating, so the measurement
// says nothing. It also took 23 seconds. Measured and discarded.
//
// What is left is structural, and it encodes the decision exactly: the log is
// walked one line at a time. The source is now ops/ rather than the ops table
// (spec 2.1.1), so the call to watch is journal's Iter — and store.ListOps
// stays banned for the original reason, that it does not read payloads
// (spec 2.2.1) and so could not replay them anyway.
func TestRebuildStreamsTheLog(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "rebuild.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	calls := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if recv, ok := sel.X.(*ast.Ident); ok {
			calls[recv.Name+"."+sel.Sel.Name] = true
		}
		return true
	})

	if !calls["ops.Iter"] {
		t.Error("Rebuild does not walk the journal with Iter; it has to replay one op at a time")
	}
	if calls["store.ListOps"] {
		t.Error("Rebuild uses store.ListOps, which materializes every op — and, since\n" +
			"ListOps does not read payloads, could not replay them anyway")
	}
}
