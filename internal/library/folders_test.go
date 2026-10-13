package library

import (
	"errors"
	"github.com/jinzhao1994/kist/internal/domain"
	"testing"
)

func TestCreateFolderLifecycle(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	b, _, _ := l.Box(ctx, box)
	op, err := l.CreateFolder(ctx, box, b.RootFolderID, "photos")
	if err != nil {
		t.Fatal(err)
	}
	parent, err := l.ResolveFolder(ctx, box, "photos")
	if err != nil {
		t.Fatal(err)
	}
	childOp, err := l.CreateFolder(ctx, box, parent.ID, "raw")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.ViewAt(ctx, op, func(view *HistoricalView) error {
		folders, err := view.Folders(ctx, box)
		if err != nil {
			return err
		}
		if len(folders) != 2 {
			t.Fatalf("historical folders: %v", folders)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	after, _ := l.ResolveFolder(ctx, box, "photos")
	if !after.UpdatedAt.After(parent.UpdatedAt) {
		t.Fatal("ancestor timestamp did not advance")
	}
	if _, err := l.CreateFolder(ctx, box, parent.ID, "RAW"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := l.CreateFolder(ctx, box, domain.NewFolderID(), "bad"); !errors.Is(err, ErrFolderNotInBox) {
		t.Fatalf("foreign folder: %v", err)
	}
	if _, err := l.Undo(ctx, op); !errors.Is(err, ErrUndoBlocked) {
		t.Fatalf("nonempty undo: %v", err)
	}
	if err := l.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := l.ResolveFolder(ctx, box, "photos/raw"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{childOp, op} {
		if _, err := l.Undo(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	folders, _ := l.Folders(ctx, box)
	if len(folders) != 1 {
		t.Fatalf("undo left folders: %v", folders)
	}
	r, err := l.Verify(ctx, VerifyOptions{Deep: true})
	if err != nil || r.Total() != 0 {
		t.Fatalf("verify: %v, %v", r, err)
	}
}

func TestCreateFolderDepthLimit(t *testing.T) {
	l, box := openWithBox(t)
	b, _, _ := l.Box(t.Context(), box)
	parent := b.RootFolderID
	for depth := 1; depth <= 32; depth++ {
		if _, err := l.CreateFolder(t.Context(), box, parent, "nested"); err != nil {
			t.Fatalf("depth %d: %v", depth, err)
		}
		folders, _ := l.Folders(t.Context(), box)
		for _, f := range folders {
			if f.ParentID != nil && *f.ParentID == parent {
				parent = f.ID
				break
			}
		}
	}
	if _, err := l.CreateFolder(t.Context(), box, parent, "too-deep"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("depth 33: %v", err)
	}
}
