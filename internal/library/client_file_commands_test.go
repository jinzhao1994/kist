package library

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
)

func TestClientMetadataRemovalSurvivesSyncAndExtraction(t *testing.T) {
	nas, box := openWithBox(t)
	f := importOneFile(t, nas, box, "photo.jpg", "original")
	extractor := &fakeExtractor{id: "fake", version: 1, ns: []string{"fake"}, tags: []extract.KV{text("fake:Make", "Camera")}}
	extractWith(t, nas, extractor)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	removed, err := c.RemoveMetadata(ctx, f.ID, "fake:Make")
	if err != nil || !removed {
		t.Fatalf("remove=%t %v", removed, err)
	}
	noop, err := c.ExecuteRequest(ctx, Request{ID: "metadata-remove-noop", Digest: strings.Repeat("a", 64)}, func(scoped context.Context) error {
		removed, err := c.RemoveMetadata(scoped, f.ID, "fake:Make")
		if removed {
			t.Fatal("tombstone removed twice")
		}
		return err
	})
	if err != nil || !noop.Noop {
		t.Fatalf("noop=%+v %v", noop, err)
	}
	// Metadata edits require known facts, not a downloaded original.
	if _, err := c.OpenOriginal(ctx, f.BlobKey); !errors.Is(err, ErrNotCached) {
		t.Fatalf("original unexpectedly needed=%v", err)
	}
	if _, err := nas.RenameEntry(ctx, box, f.ID.ID, false, "photo.jpg", "renamed.jpg"); err != nil {
		t.Fatal(err)
	}
	synced, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if err != nil || synced.Accepted != 1 {
		t.Fatalf("sync=%+v %v", synced, err)
	}
	row := metaOf(t, nas, f.ID)["fake:Make"]
	if !row.Tombstone || row.Source != extract.UserSource {
		t.Fatalf("not a user tombstone: %+v", row)
	}
	extractor.version = 2
	extractWith(t, nas, extractor)
	if !metaOf(t, nas, f.ID)["fake:Make"].Tombstone {
		t.Fatal("extraction resurrected suppressed key")
	}
	got, found, err := nas.File(ctx, f.ID)
	if err != nil || !found || got.Name != "renamed.jpg" {
		t.Fatalf("lost NAS rename: %+v %v", got, err)
	}
}

func TestClientRenameEntryPreservesBeforeStateAndUndo(t *testing.T) {
	nas, box := openWithBox(t)
	f := importOneFile(t, nas, box, "before.txt", "content")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	ref, err := c.RenameEntry(ctx, box, f.ID.ID, false, "before.txt", "after.txt")
	if err != nil || ref.LocalID == "" {
		t.Fatalf("rename=%+v %v", ref, err)
	}
	if _, err := c.RenameEntry(ctx, box, f.ID.ID, false, "before.txt", "stale.txt"); !errors.Is(err, oplog.ErrStaleBefore) {
		t.Fatalf("stale rename=%v", err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	receipt, found, err := nas.SyncStatus(ctx, ref.LocalID)
	if err != nil || !found {
		t.Fatal(err)
	}
	if _, err := nas.Undo(ctx, receipt.OpID); err != nil {
		t.Fatal(err)
	}
	got, found, err := nas.File(ctx, f.ID)
	if err != nil || !found || got.Name != "before.txt" {
		t.Fatalf("undo rename=%+v %v", got, err)
	}
}
