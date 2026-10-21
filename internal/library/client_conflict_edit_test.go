package library

import (
	"errors"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/oplog"
)

func TestClientConflictEditPreservesReviewAndOriginal(t *testing.T) {
	nas, box := openWithBox(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	b, _, err := c.Box(ctx, box)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := oplog.Marshal(oplog.UpdateBoxPayload{BoxID: box, Name: &oplog.TextChange{Before: b.Name, After: "local choice"}})
	original := SyncProposal{Version: 1, ID: "editable-conflict", OccurredAt: time.Now().UTC(), Action: string(oplog.ActionUpdateBox), Origin: "user", Payload: raw}
	if err = c.Stage(ctx, original); err != nil {
		t.Fatal(err)
	}
	if _, err = c.ConflictEdit(ctx, original.ID); !errors.Is(err, ErrSyncConflict) {
		t.Fatal("nonconflicting intent editable", err)
	}
	if _, err = nas.UpdateBox(ctx, box, BoxFields{Name: "NAS choice", Description: "independent NAS description"}); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Synchronize(ctx, &testSyncPeer{nas: nas}); err == nil {
		t.Fatal("expected conflict")
	}
	draft, err := c.ConflictEdit(ctx, original.ID)
	if err != nil || draft.Replacement == nil || len(draft.Fields) != 1 {
		t.Fatal(draft, err)
	}
	field := draft.Fields[0]
	if field.Before != b.Name || field.Current != "NAS choice" || field.Proposed != "local choice" {
		t.Fatal(field)
	}
	if draft.Replacement.ID == original.ID {
		t.Fatal("reused original identity")
	}
	// A newer competing edit invalidates the reviewed draft, not the user's old
	// intention. No replacement is persisted on failure.
	if _, err = nas.UpdateBox(ctx, box, BoxFields{Name: "newer NAS choice", Description: "independent NAS description"}); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Synchronize(ctx, &testSyncPeer{nas: nas}); err == nil {
		t.Fatal("expected conflict")
	}
	if err = c.Resolve(ctx, original.ID, draft.Replacement, "reviewed choice"); err == nil {
		t.Fatal("stale review overwritten")
	}
	draft, err = c.ConflictEdit(ctx, original.ID)
	if err != nil {
		t.Fatal(err)
	}
	p, err := oplog.Payload[oplog.UpdateBoxPayload](draft.Replacement.Payload)
	if err != nil {
		t.Fatal(err)
	}
	p.Name.After = "reconciled choice"
	draft.Replacement.Payload, _ = oplog.Marshal(p)
	if err = c.Resolve(ctx, original.ID, draft.Replacement, "reviewed choice"); err != nil {
		t.Fatal(err)
	}
	if err = c.Resolve(ctx, original.ID, draft.Replacement, "reviewed choice"); err != nil {
		t.Fatal("retry", err)
	}
	if _, err = c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	got, _, err := nas.Box(ctx, box)
	if err != nil || got.Name != "reconciled choice" || got.Description != "independent NAS description" {
		t.Fatal(got, err)
	}
	detail, err := c.LocalHistoryOp(ctx, original.ID)
	if err != nil || detail.State != "replaced" || string(detail.Proposal.Payload) != string(raw) {
		t.Fatal(detail, err)
	}
	if err = c.Resolve(ctx, original.ID, draft.Replacement, "reviewed choice"); err != nil {
		t.Fatal("retry after sync", err)
	}
}

func TestClientConflictEditRenameTargets(t *testing.T) {
	for _, kind := range []string{"file", "tag"} {
		t.Run(kind, func(t *testing.T) {
			nas, box := openWithBox(t)
			ctx := t.Context()
			var c *Client
			var id string
			if kind == "file" {
				f := importOneFile(t, nas, box, "original.txt", "content")
				c = newSyncClient(t, nas)
				ref, err := c.RenameEntry(ctx, box, f.ID.ID, false, f.Name, "local.txt")
				if err != nil {
					t.Fatal(err)
				}
				id = ref.LocalID
				if _, err = nas.RenameEntry(ctx, box, f.ID.ID, false, f.Name, "nas.txt"); err != nil {
					t.Fatal(err)
				}
			} else {
				tag, err := nas.CreateTag(ctx, "original", nil)
				if err != nil {
					t.Fatal(err)
				}
				c = newSyncClient(t, nas)
				if err = c.RenameTag(ctx, tag.ID, "local"); err != nil {
					t.Fatal(err)
				}
				status, err := c.Status(ctx)
				if err != nil {
					t.Fatal(err)
				}
				id = status.Pending[0].ID
				if err = nas.RenameTag(ctx, tag.ID, "nas"); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err == nil {
				t.Fatal("expected conflict")
			}
			draft, err := c.ConflictEdit(ctx, id)
			if err != nil || draft.Replacement == nil || len(draft.Fields) != 1 || draft.Fields[0].Path[0] != "after" {
				t.Fatal(draft, err)
			}
			if err = c.Resolve(ctx, id, draft.Replacement, "keep local name after review"); err != nil {
				t.Fatal(err)
			}
			if _, err = c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
