package library

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestClientUndoLocalThenRedoAfterNASAcceptance(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	var box domain.Box
	created, err := c.ExecuteRequest(ctx, Request{ID: "undo-local-create", Digest: strings.Repeat("a", 64)}, func(scoped context.Context) error {
		var err error
		box, err = c.CreateBox(scoped, "Offline", "", domain.LocalDate{}, domain.LocalDate{})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	inverse, err := c.Undo(ctx, SyncReference{LocalID: created.LocalID})
	if err != nil || inverse.LocalID == "" || inverse.OpID != 0 {
		t.Fatalf("undo=%+v %v", inverse, err)
	}
	if _, found, err := c.Box(ctx, box.ID); err != nil || found {
		t.Fatalf("local undo missing: %v", err)
	}
	synced, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if err != nil || synced.Accepted != 2 {
		t.Fatalf("sync=%+v %v", synced, err)
	}
	if _, found, err := nas.Box(ctx, box.ID); err != nil || found {
		t.Fatalf("NAS undo missing: %v", err)
	}
	// The independent local identity remains resolvable after NAS acceptance.
	if _, err := c.Undo(ctx, inverse); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	if got, found, err := nas.Box(ctx, box.ID); err != nil || !found || got.Name != "Offline" {
		t.Fatalf("redo=%+v %v", got, err)
	}
}

func TestClientUndoRejectsLaterChangesAndAcceptsNASReference(t *testing.T) {
	nas, box := openWithBox(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	original, _, err := nas.Box(ctx, box)
	if err != nil {
		t.Fatal(err)
	}
	rename, err := c.ExecuteRequest(ctx, Request{ID: "undo-local-rename", Digest: strings.Repeat("b", 64)}, func(scoped context.Context) error {
		_, err := c.UpdateBox(scoped, box, BoxFields{Name: "First", Description: original.Description})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.UpdateBox(ctx, box, BoxFields{Name: "Second", Description: original.Description}); err != nil {
		t.Fatal(err)
	}
	before, err := c.queue.snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ExecuteRequest(ctx, Request{ID: "blocked-client-undo", Digest: strings.Repeat("c", 64)}, func(scoped context.Context) error {
		_, err := c.Undo(scoped, SyncReference{LocalID: rename.LocalID})
		return err
	}); !errors.Is(err, ErrUndoBlocked) {
		t.Fatalf("undo did not report later changes: %v", err)
	}
	after, err := c.queue.snapshot(ctx)
	if err != nil || len(after) != len(before) {
		t.Fatalf("failed undo persisted: %v", err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	after, err = c.queue.snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	last := after[len(after)-1].Receipt.OpID
	if _, err := c.Undo(ctx, SyncReference{OpID: last}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	if got, _, err := nas.Box(ctx, box); err != nil || got.Name != "First" {
		t.Fatalf("NAS referenced undo=%+v %v", got, err)
	}
}
