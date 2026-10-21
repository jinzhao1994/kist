package library

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/store"
)

func TestClientTagCommandsPreserveTreeConstraintsAndSync(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	root, err := c.CreateTag(ctx, "Travel", nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := c.CreateTag(ctx, "Photos", &root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateTag(ctx, "travel", nil); err == nil {
		t.Fatal("duplicate tag accepted")
	}
	if err := c.MoveTag(ctx, root.ID, &child.ID); !errors.Is(err, ErrTagCycle) {
		t.Fatalf("cycle=%v", err)
	}
	if err := c.DeleteTag(ctx, root.ID); err == nil {
		t.Fatal("non-leaf deleted")
	}
	if err := c.RenameTag(ctx, child.ID, "Pictures"); err != nil {
		t.Fatal(err)
	}
	result, err := c.ExecuteRequest(ctx, Request{ID: "unchanged-tag-name", Digest: strings.Repeat("a", 64)}, func(scoped context.Context) error { return c.RenameTag(scoped, child.ID, "Pictures") })
	if err != nil || !result.Noop {
		t.Fatalf("noop=%+v %v", result, err)
	}
	if err := c.MoveTag(ctx, child.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteTag(ctx, root.ID); err != nil {
		t.Fatal(err)
	}
	synced, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if err != nil || synced.Accepted != 5 {
		t.Fatalf("sync=%+v %v", synced, err)
	}
	got, found, err := store.TagByID(nas.db, child.ID)
	if err != nil || !found || got.Name != "Pictures" || got.ParentID != nil {
		t.Fatalf("child=%+v %v", got, err)
	}
	if _, found, err := store.TagByID(nas.db, root.ID); err != nil || found {
		t.Fatalf("deleted root remains: %v", err)
	}
}

func TestClientTagMoveMergesIndependentRename(t *testing.T) {
	nas := openTestLibrary(t)
	ctx := t.Context()
	parent, err := nas.CreateTag(ctx, "Parent", nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := nas.CreateTag(ctx, "Child", nil)
	if err != nil {
		t.Fatal(err)
	}
	c := newSyncClient(t, nas)
	if err := c.MoveTag(ctx, child.ID, &parent.ID); err != nil {
		t.Fatal(err)
	}
	if err := nas.RenameTag(ctx, child.ID, "Renamed on NAS"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	got, found, err := store.TagByID(nas.db, child.ID)
	if err != nil || !found || got.Name != "Renamed on NAS" || got.ParentID == nil || *got.ParentID != parent.ID {
		t.Fatalf("lost independent edit=%+v %v", got, err)
	}
	missing := domain.NewTagID()
	if err := c.MoveTag(ctx, child.ID, &missing); !errors.Is(err, ErrNoSuchTag) {
		t.Fatalf("missing parent=%v", err)
	}
}

func TestClientTagAssignmentsDeduplicateAndPreserveNoopReceipts(t *testing.T) {
	nas, box := openWithBox(t)
	f := importOneFile(t, nas, box, "photo.txt", "tagged content")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	tag, err := c.CreateTag(ctx, "Offline tag", nil)
	if err != nil {
		t.Fatal(err)
	}
	count, err := c.TagFiles(ctx, tag.ID, []domain.FileID{f.ID, f.ID})
	if err != nil || count != 1 {
		t.Fatalf("tag count=%d %v", count, err)
	}
	noop, err := c.ExecuteRequest(ctx, Request{ID: "client-noop-tagging", Digest: strings.Repeat("b", 64)}, func(scoped context.Context) error {
		count, err := c.TagFiles(scoped, tag.ID, []domain.FileID{f.ID})
		if count != 0 {
			t.Fatal("duplicate assignment")
		}
		return err
	})
	if err != nil || !noop.Noop {
		t.Fatalf("noop=%+v %v", noop, err)
	}
	if count, err := c.UntagFiles(ctx, tag.ID, []domain.FileID{f.ID, f.ID}); err != nil || count != 1 {
		t.Fatalf("untag count=%d %v", count, err)
	}
	if count, err := c.TagFiles(ctx, tag.ID, []domain.FileID{f.ID}); err != nil || count != 1 {
		t.Fatalf("retag count=%d %v", count, err)
	}
	if count, err := c.TagBoxes(ctx, tag.ID, []domain.BoxID{box, box}); err != nil || count != 1 {
		t.Fatalf("box count=%d %v", count, err)
	}
	if count, err := c.UntagBoxes(ctx, tag.ID, []domain.BoxID{box, box}); err != nil || count != 1 {
		t.Fatalf("box removal=%d %v", count, err)
	}
	synced, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if err != nil || synced.Accepted != 6 {
		t.Fatalf("sync=%+v %v", synced, err)
	}
	tags, err := nas.FileTags(ctx, f.ID)
	if err != nil || len(tags) != 1 || tags[0] != tag.ID {
		t.Fatalf("NAS tags=%+v %v", tags, err)
	}
	// Duplicate-free payloads preserve the existing strict inverse semantics.
	entries, err := c.queue.snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nas.Undo(ctx, entries[3].Receipt.OpID); err != nil {
		t.Fatal(err)
	}
	tags, err = nas.FileTags(ctx, f.ID)
	if err != nil || len(tags) != 0 {
		t.Fatalf("undo tags=%+v %v", tags, err)
	}
}
