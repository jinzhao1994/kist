package library

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
)

func TestClientBoxCoverSyncUndoAndOwnership(t *testing.T) {
	nas, box := openWithBox(t)
	first := importOneFile(t, nas, box, "first.png", "first")
	second := importOneFile(t, nas, box, "second.png", "second")
	ctx := t.Context()
	other, err := nas.CreateBox(ctx, "Other", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	foreign := importOneFile(t, nas, other.ID, "foreign.png", "foreign")
	if err := nas.SetBoxCover(ctx, box, first.ID); err != nil {
		t.Fatal(err)
	}
	c := newSyncClient(t, nas)
	if err := c.SetBoxCover(ctx, box, foreign.ID); !errors.Is(err, oplog.ErrStaleBefore) {
		t.Fatalf("cross-box cover=%v", err)
	}
	request := Request{ID: "offline-cover", Digest: strings.Repeat("b", 64)}
	result, err := c.ExecuteRequest(ctx, request, func(ctx context.Context) error { return c.SetBoxCover(ctx, box, second.ID) })
	if err != nil {
		t.Fatal(err)
	}
	cover, found, err := c.BoxCover(ctx, box)
	if err != nil || !found || cover.ID != second.ID {
		t.Fatalf("local cover=%+v %v %v", cover, found, err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	cover, found, err = nas.BoxCover(ctx, box)
	if err != nil || !found || cover.ID != second.ID {
		t.Fatalf("NAS cover=%+v %v %v", cover, found, err)
	}
	if _, err := c.Undo(ctx, SyncReference{LocalID: result.LocalID}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	cover, found, err = nas.BoxCover(ctx, box)
	if err != nil || !found || cover.ID != first.ID {
		t.Fatalf("undo cover=%+v %v %v", cover, found, err)
	}
}
