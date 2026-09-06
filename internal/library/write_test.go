package library

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// fixedClock hands out timestamps one second apart, so op order is visible in
// the data and every run produces the same values.
func fixedClock() func() time.Time {
	base := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	n := 0
	return func() time.Time {
		n++
		return base.Add(time.Duration(n) * time.Second)
	}
}

// openTestLibrary creates a library in a temp dir and opens it.
func openTestLibrary(t *testing.T) *Library {
	t.Helper()
	root := t.TempDir()
	if _, err := Create(context.Background(), root); err != nil {
		t.Fatalf("Create: %v", err)
	}
	l, err := Open(root, WithClock(fixedClock()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func TestOpenRejectsANonLibrary(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil {
		t.Error("Open on an empty directory = nil error, want a refusal")
	}
}

func TestWriteRecordsAnOpAndAppliesIt(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()

	p := oplog.CreateBoxPayload{
		BoxID:        domain.NewBoxID(),
		RootFolderID: domain.NewFolderID(),
		BoxFields:    oplog.BoxFields{Name: "云南"},
	}
	raw, _ := oplog.Marshal(p)

	opID, err := l.write(ctx, oplog.ActionCreateBox, raw, oplog.OriginUser, writeOptions{})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if opID <= 0 {
		t.Fatalf("write returned op id %d", opID)
	}

	ops, err := l.Ops(ctx, store.ListOpsOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Ops: %v", err)
	}
	if len(ops) != 1 || ops[0].Action != oplog.ActionCreateBox || ops[0].Origin != oplog.OriginUser {
		t.Fatalf("ops = %+v", ops)
	}

	// The materialized view moved in the same transaction.
	b, ok, err := l.Box(ctx, p.BoxID)
	if err != nil || !ok {
		t.Fatalf("Box: %v ok=%v", err, ok)
	}
	if b.Name != "云南" {
		t.Errorf("box name = %q", b.Name)
	}
	if !b.CreatedAt.Equal(ops[0].TS) {
		t.Errorf("CreatedAt = %v, want the op's ts %v", b.CreatedAt, ops[0].TS)
	}
}

func TestWriteIsAtomic(t *testing.T) {
	// If Apply fails, the op must not survive either — otherwise the log would
	// claim something happened that did not, and a rebuild would then make it
	// happen.
	l := openTestLibrary(t)
	ctx := context.Background()

	stale := oplog.UpdateBoxPayload{
		BoxID: domain.NewBoxID(), // no such box
		Name:  &oplog.TextChange{Before: "whatever", After: "nope"},
	}
	raw, _ := oplog.Marshal(stale)

	if _, err := l.write(ctx, oplog.ActionUpdateBox, raw, oplog.OriginUser, writeOptions{}); err == nil {
		t.Fatal("writing an op whose Apply fails succeeded")
	}

	ops, _ := l.Ops(ctx, store.ListOpsOptions{Limit: 10})
	if len(ops) != 0 {
		t.Errorf("a failed write left %d op(s) behind", len(ops))
	}
}

func TestWriteStoresTheClockExactly(t *testing.T) {
	// Rebuild's snapshot proof compares timestamps, so a write path that
	// rounded the clock off would produce a library its own replay cannot
	// reproduce.
	root := t.TempDir()
	if _, err := Create(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	odd := time.Date(2026, 8, 27, 9, 0, 0, 123_456_789, time.UTC)
	l, err := Open(root, WithClock(func() time.Time { return odd }))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	p := oplog.CreateBoxPayload{BoxID: domain.NewBoxID(), RootFolderID: domain.NewFolderID(),
		BoxFields: oplog.BoxFields{Name: "x"}}
	raw, _ := oplog.Marshal(p)
	if _, err := l.write(context.Background(), oplog.ActionCreateBox, raw, oplog.OriginUser, writeOptions{}); err != nil {
		t.Fatal(err)
	}

	ops, _ := l.Ops(context.Background(), store.ListOpsOptions{Limit: 1})
	if !ops[0].TS.Equal(odd) {
		t.Errorf("stored ts = %v, want %v", ops[0].TS, odd)
	}
}

func TestWriteFillsTheIndexTables(t *testing.T) {
	// Invariant 4: op_boxes is derived from the payload by Targets, and the
	// write path must actually call it — a handler that returns targets nobody
	// writes is a silent hole in the history view.
	l := openTestLibrary(t)
	ctx := context.Background()

	p := oplog.CreateBoxPayload{BoxID: domain.NewBoxID(), RootFolderID: domain.NewFolderID(),
		BoxFields: oplog.BoxFields{Name: "indexed"}}
	raw, _ := oplog.Marshal(p)
	opID, _ := l.write(ctx, oplog.ActionCreateBox, raw, oplog.OriginUser, writeOptions{})

	byBox, err := l.Ops(ctx, store.ListOpsOptions{Box: &p.BoxID, Limit: 10})
	if err != nil {
		t.Fatalf("Ops by box: %v", err)
	}
	if len(byBox) != 1 || byBox[0].ID != opID {
		t.Errorf("ops for the box = %+v, want just op %d", byBox, opID)
	}
}

func TestUserOpsPropagateUpdatedAt(t *testing.T) {
	// spec 1.5.1 ②: the rule lives on the write path, so a new op type cannot
	// forget it.
	l := openTestLibrary(t)
	ctx := context.Background()

	box := domain.NewBoxID()
	create, _ := oplog.Marshal(oplog.CreateBoxPayload{BoxID: box, RootFolderID: domain.NewFolderID(),
		BoxFields: oplog.BoxFields{Name: "before"}})
	l.write(ctx, oplog.ActionCreateBox, create, oplog.OriginUser, writeOptions{})
	first, _, _ := l.Box(ctx, box)

	update, _ := oplog.Marshal(oplog.UpdateBoxPayload{
		BoxID: box,
		Name:  &oplog.TextChange{Before: "before", After: "after"},
	})
	if _, err := l.write(ctx, oplog.ActionUpdateBox, update, oplog.OriginUser, writeOptions{}); err != nil {
		t.Fatal(err)
	}
	second, _, _ := l.Box(ctx, box)

	if !second.UpdatedAt.After(first.UpdatedAt) {
		t.Errorf("UpdatedAt did not move: %v then %v", first.UpdatedAt, second.UpdatedAt)
	}
	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Errorf("CreatedAt moved from %v to %v", first.CreatedAt, second.CreatedAt)
	}
}

func TestMaintenanceOpsDoNotPropagateUpdatedAt(t *testing.T) {
	// The failure this prevents: re-running an extractor over the library and
	// stamping ten years of archive as modified this morning.
	l := openTestLibrary(t)
	ctx := context.Background()

	box := domain.NewBoxID()
	create, _ := oplog.Marshal(oplog.CreateBoxPayload{BoxID: box, RootFolderID: domain.NewFolderID(),
		BoxFields: oplog.BoxFields{Name: "before"}})
	l.write(ctx, oplog.ActionCreateBox, create, oplog.OriginUser, writeOptions{})
	first, _, _ := l.Box(ctx, box)

	update, _ := oplog.Marshal(oplog.UpdateBoxPayload{
		BoxID: box,
		Name:  &oplog.TextChange{Before: "before", After: "after"},
	})
	if _, err := l.write(ctx, oplog.ActionUpdateBox, update, oplog.OriginMaintenance, writeOptions{}); err != nil {
		t.Fatal(err)
	}
	second, _, _ := l.Box(ctx, box)

	if !second.UpdatedAt.Equal(first.UpdatedAt) {
		t.Errorf("a maintenance op moved UpdatedAt from %v to %v", first.UpdatedAt, second.UpdatedAt)
	}
	if second.Name != "after" {
		t.Errorf("the maintenance op did not apply: name is %q", second.Name)
	}
}

func TestWriteRejectsAnUnknownAction(t *testing.T) {
	l := openTestLibrary(t)
	_, err := l.write(context.Background(), oplog.Action("teleport"), []byte(`{}`), oplog.OriginUser, writeOptions{})
	if err == nil {
		t.Error("writing an unregistered action succeeded")
	}
}

func TestTaggingIsHistoryNotContent(t *testing.T) {
	// spec 1.5.1: updated_at tracks content, and a tag is a label on content.
	// spec 2.2.1: op_boxes exists so `log --box` finds cross-box tagging, which
	// is the first reason that table is many-to-many. The two rules pull in
	// opposite directions on purpose.
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a/deep.txt": "x"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	before, _, err := l.Box(t.Context(), box)
	if err != nil {
		t.Fatal(err)
	}

	files := listAll(t, l, box)
	tag, err := l.CreateTag(t.Context(), "deep", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.TagFiles(t.Context(), tag.ID, []domain.FileID{files[0].ID}); err != nil {
		t.Fatal(err)
	}

	after, _, err := l.Box(t.Context(), box)
	if err != nil {
		t.Fatal(err)
	}
	if !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("tagging moved the box's updated_at from %v to %v; nothing under boxes/ changed",
			before.UpdatedAt, after.UpdatedAt)
	}

	ops, err := l.Ops(t.Context(), store.ListOpsOptions{Box: &box, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	var tagged bool
	for _, op := range ops {
		if op.Action == oplog.ActionTagFiles {
			tagged = true
		}
	}
	if !tagged {
		t.Errorf("the box's history has %d op(s) and none of them is the tagging", len(ops))
	}
}

// A group is bookkeeping about how ops were written, not about what they did.
//
// Nothing downstream of the op log may depend on it — which is another way of
// saying a rebuild must not be able to tell whether one was set.
func TestAGroupChangesNothingAboutTheState(t *testing.T) {
	ctx := context.Background()

	// Fixed ids: the two runs have to produce comparable snapshots, and a
	// freshly minted UUIDv7 would differ between them for reasons that are
	// not the group.
	boxPayload := func(t *testing.T, name string, n byte) []byte {
		t.Helper()
		boxID, err := domain.ParseID(fmt.Sprintf("01900000-0000-7000-8000-0000000000%02d", n))
		if err != nil {
			t.Fatal(err)
		}
		rootID, err := domain.ParseID(fmt.Sprintf("01900000-0000-7000-8000-0000000001%02d", n))
		if err != nil {
			t.Fatal(err)
		}
		raw, err := oplog.Marshal(oplog.CreateBoxPayload{
			BoxID:        domain.BoxID{ID: boxID},
			RootFolderID: domain.FolderID{ID: rootID},
			BoxFields:    oplog.BoxFields{Name: name},
		})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}

	snap := func(group bool) string {
		t.Helper()
		l := openTestLibrary(t)

		// The op that heads the group, written the way a real one is: with
		// no root of its own.
		first, err := l.write(ctx, oplog.ActionCreateBox,
			boxPayload(t, "云南", 1), oplog.OriginUser, writeOptions{})
		if err != nil {
			t.Fatal(err)
		}

		var opt writeOptions
		if group {
			opt.RootOpID = &first
		}
		if _, err := l.write(ctx, oplog.ActionCreateBox,
			boxPayload(t, "西藏", 2), oplog.OriginUser, opt); err != nil {
			t.Fatal(err)
		}

		got, err := store.Snapshot(l.db)
		if err != nil {
			t.Fatal(err)
		}
		return withoutOps(got)
	}

	if with, without := snap(true), snap(false); with != without {
		t.Error("setting a group changed the materialized state")
	}
}

// withoutOps drops the ops section of a snapshot.
//
// root_op_id is a column of ops, so of course the log differs — that is the
// whole point of writing it down. The claim being tested is narrower and is
// the one that matters for invariant 3: the group tag never reaches Apply, so
// nothing it produces may differ. Comparing the log too would test the
// opposite of the claim.
func withoutOps(snapshot string) string {
	const header = "== ops ==\n"
	start := strings.Index(snapshot, header)
	if start < 0 {
		return snapshot
	}
	rest := snapshot[start+len(header):]
	next := strings.Index(rest, "\n== ")
	if next < 0 {
		return snapshot[:start]
	}
	return snapshot[:start] + rest[next+1:]
}
