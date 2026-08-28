package library

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

func lastOpID(t *testing.T, l *Library) int64 {
	t.Helper()
	ops, err := l.Ops(context.Background(), store.ListOpsOptions{Limit: 1})
	if err != nil || len(ops) == 0 {
		t.Fatalf("Ops: %v (%d rows)", err, len(ops))
	}
	return ops[0].ID
}

func TestUndoCreateBox(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	b, _ := l.CreateBox(ctx, "regrettable", "", domain.LocalDate{}, domain.LocalDate{})
	opID := lastOpID(t, l)

	undoID, err := l.Undo(ctx, opID)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if _, ok, _ := l.Box(ctx, b.ID); ok {
		t.Error("the box survived the undo")
	}

	// spec 2.4: undo appends, it does not erase. Both ops are still there.
	ops, _ := l.Ops(ctx, store.ListOpsOptions{Limit: 10})
	if len(ops) != 2 {
		t.Fatalf("history has %d ops, want 2", len(ops))
	}
	if ops[0].ID != undoID || ops[0].RevertsID == nil || *ops[0].RevertsID != opID {
		t.Errorf("the undo op = %+v", ops[0])
	}
}

func TestUndoIsItselfUndoable(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	b, _ := l.CreateBox(ctx, "back and forth", "", domain.LocalDate{}, domain.LocalDate{})
	createOp := lastOpID(t, l)

	undoOp, err := l.Undo(ctx, createOp)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if _, err := l.Undo(ctx, undoOp); err != nil {
		t.Fatalf("undoing the undo: %v", err)
	}

	got, ok, _ := l.Box(ctx, b.ID)
	if !ok {
		t.Fatal("the box did not come back")
	}
	if got.Name != "back and forth" {
		t.Errorf("name came back as %q", got.Name)
	}
}

func TestUndoUpdateBox(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	b, _ := l.CreateBox(ctx, "before", "", domain.LocalDate{}, domain.LocalDate{})
	l.UpdateBox(ctx, b.ID, oplog.BoxFields{Name: "after", Description: "noted"})
	updateOp := lastOpID(t, l)

	if _, err := l.Undo(ctx, updateOp); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	got, _, _ := l.Box(ctx, b.ID)
	if got.Name != "before" || got.Description != "" {
		t.Errorf("after undo: %+v", got)
	}
}

func TestUndoTagBoxesRestoresTheWholeBatch(t *testing.T) {
	// spec 2.2.1: one batch, one op, one undo.
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "云南", nil)
	var boxes []domain.BoxID
	for _, n := range []string{"一", "二", "三"} {
		b, _ := l.CreateBox(ctx, n, "", domain.LocalDate{}, domain.LocalDate{})
		boxes = append(boxes, b.ID)
	}
	l.TagBoxes(ctx, tag.ID, boxes)
	tagOp := lastOpID(t, l)

	if _, err := l.Undo(ctx, tagOp); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	for _, id := range boxes {
		tags, _ := l.BoxTags(ctx, id)
		if len(tags) != 0 {
			t.Errorf("box %s still carries %d tag(s)", id, len(tags))
		}
	}
	if got := tagCount(t, l, tag.ID); got != 0 {
		t.Errorf("the count cache says %d after the undo, want 0", got)
	}
}

func TestUndoTagOpsKeepTheTreeInStep(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "临时", nil)
	createOp := lastOpID(t, l)

	if _, err := l.Undo(ctx, createOp); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if _, ok := tagByID(t, l, tag.ID); ok {
		t.Error("the in-memory tree still holds a tag that was undone")
	}
	// The name is free again, which is only true if the tree really dropped it.
	if _, err := l.CreateTag(ctx, "临时", nil); err != nil {
		t.Errorf("recreating the tag after undo: %v", err)
	}
}

func TestUndoRefusesToRunTwice(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	l.CreateBox(ctx, "once", "", domain.LocalDate{}, domain.LocalDate{})
	opID := lastOpID(t, l)

	if _, err := l.Undo(ctx, opID); err != nil {
		t.Fatalf("first undo: %v", err)
	}
	_, err := l.Undo(ctx, opID)
	if !errors.Is(err, ErrAlreadyReverted) {
		t.Errorf("second undo = %v, want ErrAlreadyReverted", err)
	}
}

func TestUndoOfAMissingOp(t *testing.T) {
	l := openTestLibrary(t)
	if _, err := l.Undo(context.Background(), 9999); err == nil {
		t.Error("undoing an op that does not exist succeeded")
	}
}

func TestUndoAcrossTimeIsRefusedAndNamesTheBlocker(t *testing.T) {
	// spec 2.5, the whole point of strict mode: "I thought the undo worked but
	// it only restored half of it" is the worse outcome in an archive, and a
	// bare "undo failed" leaves the user with nothing to do about it.
	l := openTestLibrary(t)
	ctx := context.Background()
	b, _ := l.CreateBox(ctx, "v1", "", domain.LocalDate{}, domain.LocalDate{})

	l.UpdateBox(ctx, b.ID, oplog.BoxFields{Name: "v2"})
	firstUpdate := lastOpID(t, l)
	l.UpdateBox(ctx, b.ID, oplog.BoxFields{Name: "v3"})
	secondUpdate := lastOpID(t, l)

	_, err := l.Undo(ctx, firstUpdate)
	if !errors.Is(err, ErrUndoBlocked) {
		t.Fatalf("error = %v, want ErrUndoBlocked", err)
	}
	if !strings.Contains(err.Error(), fmt.Sprint(secondUpdate)) {
		t.Errorf("error = %q, want it to name op %d as the blocker", err, secondUpdate)
	}

	// Nothing changed, and nothing was written.
	got, _, _ := l.Box(ctx, b.ID)
	if got.Name != "v3" {
		t.Errorf("the box is now %q; a refused undo must change nothing", got.Name)
	}
	if lastOpID(t, l) != secondUpdate {
		t.Error("a refused undo still appended an op")
	}
}

func TestUndoBlockedByALaterTag(t *testing.T) {
	// Undoing the creation of a box that has since been tagged would take the
	// tagging with it.
	l := openTestLibrary(t)
	ctx := context.Background()
	b, _ := l.CreateBox(ctx, "tagged later", "", domain.LocalDate{}, domain.LocalDate{})
	createOp := lastOpID(t, l)
	tag, _ := l.CreateTag(ctx, "云南", nil)
	l.TagBoxes(ctx, tag.ID, []domain.BoxID{b.ID})

	if _, err := l.Undo(ctx, createOp); !errors.Is(err, ErrUndoBlocked) {
		t.Errorf("error = %v, want ErrUndoBlocked", err)
	}
	if _, ok, _ := l.Box(ctx, b.ID); !ok {
		t.Error("the box was deleted anyway")
	}
}
