package library

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
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

	// A box that gets created and then undone, so the replay has to handle an
	// inverse op too.
	if _, err := l.CreateBox(ctx, "临时", "", domain.LocalDate{}, domain.LocalDate{}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Undo(ctx, lastOpID(t, l)); err != nil {
		t.Fatal(err)
	}
	return l
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
	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	after, _ := l.Ops(ctx, store.ListOpsOptions{})

	if len(before) != len(after) {
		t.Fatalf("op count went from %d to %d", len(before), len(after))
	}
	for i := range before {
		if before[i].ID != after[i].ID || before[i].Action != after[i].Action ||
			!before[i].TS.Equal(after[i].TS) || string(before[i].Payload) != string(after[i].Payload) {
			t.Errorf("op %d changed during the rebuild", before[i].ID)
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
	before := l.Tags().Len()

	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if got := l.Tags().Len(); got != before {
		t.Errorf("the tag tree has %d nodes after the rebuild, want %d", got, before)
	}
	if _, ok := l.Tags().ByName("云南省"); !ok {
		t.Error("the rebuilt tree lost a tag")
	}
}
