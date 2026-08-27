package library

import (
	"context"
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
	l, err := Open(root, WithClock(fixedClock()), WithActor("test"))
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

	opID, err := l.write(ctx, oplog.ActionCreateBox, raw, oplog.SourceUser, nil)
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
	if len(ops) != 1 || ops[0].Action != oplog.ActionCreateBox || ops[0].Source != oplog.SourceUser {
		t.Fatalf("ops = %+v", ops)
	}
	if ops[0].Actor != "test" {
		t.Errorf("Actor = %q, want %q", ops[0].Actor, "test")
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
		BoxID:  domain.NewBoxID(), // no such box
		Before: oplog.BoxFields{Name: "whatever"},
		After:  oplog.BoxFields{Name: "nope"},
	}
	raw, _ := oplog.Marshal(stale)

	if _, err := l.write(ctx, oplog.ActionUpdateBox, raw, oplog.SourceUser, nil); err == nil {
		t.Fatal("writing an op whose Apply fails succeeded")
	}

	ops, _ := l.Ops(ctx, store.ListOpsOptions{Limit: 10})
	if len(ops) != 0 {
		t.Errorf("a failed write left %d op(s) behind", len(ops))
	}
}

func TestWriteTruncatesTimestampsToMilliseconds(t *testing.T) {
	// The storage format has exactly three fractional digits. A ts carrying
	// nanoseconds would not survive the round trip, and every comparison
	// against it would fail in ways that look like a clock bug.
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
	if _, err := l.write(context.Background(), oplog.ActionCreateBox, raw, oplog.SourceUser, nil); err != nil {
		t.Fatal(err)
	}

	ops, _ := l.Ops(context.Background(), store.ListOpsOptions{Limit: 1})
	want := odd.Truncate(time.Millisecond)
	if !ops[0].TS.Equal(want) {
		t.Errorf("stored ts = %v, want %v", ops[0].TS, want)
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
	opID, _ := l.write(ctx, oplog.ActionCreateBox, raw, oplog.SourceUser, nil)

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
	l.write(ctx, oplog.ActionCreateBox, create, oplog.SourceUser, nil)
	first, _, _ := l.Box(ctx, box)

	update, _ := oplog.Marshal(oplog.UpdateBoxPayload{
		BoxID:  box,
		Before: oplog.BoxFields{Name: "before"},
		After:  oplog.BoxFields{Name: "after"},
	})
	if _, err := l.write(ctx, oplog.ActionUpdateBox, update, oplog.SourceUser, nil); err != nil {
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
	l.write(ctx, oplog.ActionCreateBox, create, oplog.SourceUser, nil)
	first, _, _ := l.Box(ctx, box)

	update, _ := oplog.Marshal(oplog.UpdateBoxPayload{
		BoxID:  box,
		Before: oplog.BoxFields{Name: "before"},
		After:  oplog.BoxFields{Name: "after"},
	})
	if _, err := l.write(ctx, oplog.ActionUpdateBox, update, oplog.SourceMaintenance, nil); err != nil {
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
	_, err := l.write(context.Background(), oplog.Action("teleport"), []byte(`{}`), oplog.SourceUser, nil)
	if err == nil {
		t.Error("writing an unregistered action succeeded")
	}
}
