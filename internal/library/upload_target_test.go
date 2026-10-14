package library

import (
	"context"
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestUploadTargetSnapshotAndCheckedReceipt(t *testing.T) {
	l, box := openWithBox(t)
	b, _, err := l.Box(t.Context(), box)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := l.UploadTarget(t.Context(), box, b.RootFolderID)
	if err != nil {
		t.Fatal(err)
	}
	op, err := l.CreateFolder(t.Context(), box, b.RootFolderID, "destination")
	if err != nil {
		t.Fatal(err)
	}
	folders, err := l.Folders(t.Context(), box)
	if err != nil {
		t.Fatal(err)
	}
	var child domain.FolderID
	for _, f := range folders {
		if f.Name == "destination" {
			child = f.ID
		}
	}
	if child.IsZero() {
		t.Fatal("missing folder")
	}
	root, err := l.UploadTarget(t.Context(), box, b.RootFolderID)
	if err != nil {
		t.Fatal(err)
	}
	if root != initial {
		t.Fatal("unrelated folder or propagated timestamp invalidated root")
	}
	if _, err := l.UploadTarget(t.Context(), box, child); err != nil {
		t.Fatal(err)
	}
	foreign, err := l.CreateBox(t.Context(), "foreign", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.UploadTarget(t.Context(), box, foreign.RootFolderID); !errors.Is(err, ErrFolderNotInBox) {
		t.Fatal(err)
	}
	if _, err := l.Undo(t.Context(), op); err != nil {
		t.Fatal(err)
	}
	if _, err := l.UploadTarget(t.Context(), box, child); !errors.Is(err, ErrFolderNotInBox) {
		t.Fatal(err)
	}
	req := requestFor("upload-target-test", "rename box")
	if _, found, err := l.CheckRequest(t.Context(), req); err != nil || found {
		t.Fatal(found, err)
	}
	before, _ := l.OpCount(t.Context())
	result, err := l.ExecuteRequest(t.Context(), req, func(ctx context.Context) error {
		signature, err := l.UploadTarget(ctx, box, b.RootFolderID)
		if err != nil {
			return err
		}
		if signature != initial {
			return ErrUploadTargetChanged
		}
		_, err = l.UpdateBox(ctx, box, BoxFields{Name: "changed"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	now, err := l.UploadTarget(t.Context(), box, b.RootFolderID)
	if err != nil {
		t.Fatal(err)
	}
	if now == initial {
		t.Fatal("box rename did not invalidate target")
	}
	got, found, err := l.CheckRequest(t.Context(), req)
	if err != nil || !found || got != result {
		t.Fatal(got, found, err)
	}
	if _, _, err := l.CheckRequest(t.Context(), requestFor(req.ID, "different")); !errors.Is(err, ErrRequestConflict) {
		t.Fatal(err)
	}
	after, _ := l.OpCount(t.Context())
	if after != before+1 {
		t.Fatal("lookup wrote an op", before, after)
	}
}
