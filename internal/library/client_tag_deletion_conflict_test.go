package library

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestClientTagDeletionConcurrentUseRemainsResolvable(t *testing.T) {
	for _, usage := range []string{"child", "box"} {
		t.Run(usage, func(t *testing.T) {
			nas, box := openWithBox(t)
			ctx := t.Context()
			tag, err := nas.CreateTag(ctx, "Travel", nil)
			if err != nil {
				t.Fatal(err)
			}
			c := newSyncClient(t, nas)
			if err := c.DeleteTag(ctx, tag.ID); err != nil {
				t.Fatal(err)
			}
			pending, err := c.queue.snapshot(ctx)
			if err != nil || len(pending) != 1 {
				t.Fatal(pending, err)
			}
			if usage == "child" {
				_, err = nas.CreateTag(ctx, "Photos", &tag.ID)
			} else {
				_, err = nas.TagBoxes(ctx, tag.ID, []domain.BoxID{box})
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := nas.OpCount(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := nas.AcceptSync(ctx, pending[0].Proposal); !errors.Is(err, ErrSyncConflict) {
				t.Fatalf("concurrent usage was not classified as a conflict: %v", err)
			}
			if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); !errors.Is(err, ErrSyncConflict) {
				t.Fatal("client conflict", err)
			}
			status, err := c.Status(ctx)
			if err != nil || status.BlockedID != pending[0].Proposal.ID {
				t.Fatal("conflict made client status unavailable", status, err)
			}
			if err := c.Resolve(ctx, pending[0].Proposal.ID, nil, "keep the NAS tag and its new uses"); err != nil {
				t.Fatal(err)
			}
			if result, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil || !result.Complete {
				t.Fatal(result, err)
			}
			if after, err := nas.OpCount(ctx); err != nil || after != before {
				t.Fatal("discarding a conflicting deletion changed NAS history", after, before, err)
			}
		})
	}
}
