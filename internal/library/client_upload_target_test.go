package library

import (
	"reflect"
	"testing"
)

func TestClientUploadDestinationIncludesUncachedAndPendingFiles(t *testing.T) {
	nas, box := openWithBox(t)
	ctx := t.Context()
	file := importOneFile(t, nas, box, "existing.txt", "uncached original")
	c := newSyncClient(t, nas)
	target, err := c.UploadTarget(ctx, box, file.FolderID)
	if err != nil {
		t.Fatal(err)
	}
	want, err := nas.UploadTarget(ctx, box, file.FolderID)
	if err != nil || target != want {
		t.Fatalf("target=%s NAS=%s %v", target, want, err)
	}
	paths := []string{"existing.txt", "new.txt"}
	preview, err := c.PreviewUpload(ctx, box, file.FolderID, paths)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := nas.PreviewUpload(ctx, box, file.FolderID, paths)
	if err != nil || !reflect.DeepEqual(preview, expected) {
		t.Fatalf("preview=%+v NAS=%+v %v", preview, expected, err)
	}
	if len(preview.Paths) != 1 || !preview.CanOverwrite {
		t.Fatalf("uncached conflict lost: %+v", preview)
	}
	if _, err := c.RenameEntry(ctx, box, file.ID.ID, false, file.Name, "renamed.txt"); err != nil {
		t.Fatal(err)
	}
	changed, err := c.PreviewUpload(ctx, box, file.FolderID, paths)
	if err != nil || len(changed.Paths) != 0 || changed.Token == preview.Token {
		t.Fatalf("pending rename ignored=%+v %v", changed, err)
	}
	stable, err := c.UploadTarget(ctx, box, file.FolderID)
	if err != nil || stable != target {
		t.Fatalf("descendant change invalidated target=%s %v", stable, err)
	}
	before, _, err := c.Box(ctx, box)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.UpdateBox(ctx, box, BoxFields{Name: "Renamed destination", Description: before.Description, StartAt: before.StartAt, EndAt: before.EndAt}); err != nil {
		t.Fatal(err)
	}
	changedTarget, err := c.UploadTarget(ctx, box, file.FolderID)
	if err != nil || changedTarget == target {
		t.Fatalf("renamed destination not detected=%s %v", changedTarget, err)
	}
}
