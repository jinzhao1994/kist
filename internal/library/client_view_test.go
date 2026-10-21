package library

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jinzhao1994/kist/internal/query"
)

func TestClientViewSharesSnapshotAndExpires(t *testing.T) {
	nas, box := openWithBox(t)
	file := importOneFile(t, nas, box, "photo.png", "not cached")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	var saved *ClientView
	err := c.View(ctx, func(view *ClientView) error {
		saved = view
		got, found, err := view.File(ctx, file.ID)
		if err != nil || !found || got.ID != file.ID {
			t.Fatalf("file=%+v %v %v", got, found, err)
		}
		// These reads all reuse the active SQLite transaction; reacquiring the client
		// edit lock here would deadlock instead of finishing the callback.
		if _, err := view.FileMetadata(ctx, file.ID); err != nil {
			return err
		}
		if _, err := view.MediaInfo(ctx, file.ID); err != nil {
			return err
		}
		if _, err := view.CapturedInfo(ctx, got); err != nil {
			return err
		}
		if _, err := view.Folders(ctx, box); err != nil {
			return err
		}
		if _, _, err := view.FilesWithTags(ctx, query.Query{}); err != nil {
			return err
		}
		if view.Zone().String() != nas.Zone().String() {
			t.Fatalf("zone=%s", view.Zone())
		}
		var group sync.WaitGroup
		for range 4 {
			group.Go(func() {
				_, found, err := view.Box(ctx, box)
				if err != nil || !found {
					t.Errorf("concurrent read: %v %v", found, err)
				}
			})
		}
		group.Wait()
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, _, err := view.File(cancelled, file.ID); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled=%v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := saved.File(ctx, file.ID); !errors.Is(err, ErrRequestScope) {
		t.Fatalf("expired view=%v", err)
	}
	if err := c.View(ctx, nil); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("nil callback=%v", err)
	}
	sentinel := errors.New("callback failure")
	if err := c.View(ctx, func(*ClientView) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("callback error=%v", err)
	}
	// A failed callback must still release the edit lock.
	if _, _, err := c.File(ctx, file.ID); err != nil {
		t.Fatal(err)
	}
}
