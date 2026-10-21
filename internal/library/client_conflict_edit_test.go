package library

import (
	"errors"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
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

func TestClientConflictDateEditPreservesPartialPrecision(t *testing.T) {
	nas, box := openWithBox(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	year, _ := domain.ParseLocalDate("2026")
	month, _ := domain.ParseLocalDate("2026-10")
	day, _ := domain.ParseLocalDate("2026-10-22")
	before, _, err := c.Box(ctx, box)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.UpdateBox(ctx, box, BoxFields{Name: before.Name, StartAt: year}); err != nil {
		t.Fatal(err)
	}
	if _, err = nas.UpdateBox(ctx, box, BoxFields{Name: before.Name, StartAt: day}); err != nil {
		t.Fatal(err)
	}
	result, err := c.Synchronize(ctx, &testSyncPeer{nas: nas})
	if !errors.Is(err, ErrSyncConflict) {
		t.Fatal(err)
	}
	draft, err := c.ConflictEdit(ctx, result.BlockedID)
	if err != nil || len(draft.Fields) != 1 || draft.Fields[0].Proposed != "2026" || draft.Fields[0].Current != "2026-10-22" {
		t.Fatal(draft, err)
	}
	p, err := oplog.Payload[oplog.UpdateBoxPayload](draft.Replacement.Payload)
	if err != nil {
		t.Fatal(err)
	}
	p.StartAt.After = month
	draft.Replacement.Payload, _ = oplog.Marshal(p)
	if err = c.Resolve(ctx, result.BlockedID, draft.Replacement, "choose month precision"); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	got, _, err := nas.Box(ctx, box)
	if err != nil || got.StartAt.String() != "2026-10" {
		t.Fatal(got, err)
	}
}

func TestClientConflictCoverReviewAndMissingTarget(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		t.Run(map[bool]string{false: "competing-cover", true: "deleted-local-cover"}[deleted], func(t *testing.T) {
			ctx := t.Context()
			nas, box := openWithBox(t)
			first := importOneFile(t, nas, box, "before.png", "before")
			current := importOneFile(t, nas, box, "current.png", "current")
			proposed := importOneFile(t, nas, box, "proposed.png", "proposed")
			if err := nas.SetBoxCover(ctx, box, first.ID); err != nil {
				t.Fatal(err)
			}
			client := newSyncClient(t, nas)
			if err := client.SetBoxCover(ctx, box, proposed.ID); err != nil {
				t.Fatal(err)
			}
			status, err := client.Status(ctx)
			if err != nil || len(status.Pending) != 1 {
				t.Fatal(status, err)
			}
			id := status.Pending[0].ID
			if err = nas.SetBoxCover(ctx, box, current.ID); err != nil {
				t.Fatal(err)
			}
			if deleted {
				preview, err := nas.PreviewEntryDeletion(ctx, box, proposed.ID.ID, false)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = nas.DeleteEntry(ctx, box, proposed.ID.ID, false, preview.Revision); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = client.Synchronize(ctx, &testSyncPeer{nas: nas}); !errors.Is(err, ErrSyncConflict) {
				t.Fatal(err)
			}
			draft, err := client.ConflictEdit(ctx, id)
			if err != nil || draft.Cover == nil || draft.Replacement == nil {
				t.Fatal(draft, err)
			}
			if draft.Cover.Before.ID != first.ID || draft.Cover.Before.Name != "before.png" || draft.Cover.Current.ID != current.ID || draft.Cover.Proposed.ID != proposed.ID || draft.Cover.Proposed.Available == deleted {
				t.Fatal(draft.Cover)
			}
			// A second NAS edit must invalidate the exact review, including a choice to
			// clear the cover. Refreshing is an explicit new review.
			if err = nas.SetBoxCover(ctx, box, domain.FileID{}); err != nil {
				t.Fatal(err)
			}
			if _, err = client.Synchronize(ctx, &testSyncPeer{nas: nas}); !errors.Is(err, ErrSyncConflict) {
				t.Fatal(err)
			}
			if err = client.Resolve(ctx, id, draft.Replacement, "reviewed cover"); err == nil {
				t.Fatal("stale cover review accepted")
			}
			draft, err = client.ConflictEdit(ctx, id)
			if err != nil || !draft.Cover.Current.ID.IsZero() || !draft.Cover.Current.Available {
				t.Fatal(draft, err)
			}
			p, err := oplog.Payload[oplog.UpdateBoxPayload](draft.Replacement.Payload)
			if err != nil || !p.CoverFileID.Before.IsZero() {
				t.Fatal(p, err)
			}
			if deleted {
				if err = client.Resolve(ctx, id, draft.Replacement, "missing local target"); err == nil {
					t.Fatal("deleted cover target accepted")
				}
				p.CoverFileID.After = first.ID
				draft.Replacement.Payload, _ = oplog.Marshal(p)
			}
			if err = client.Resolve(ctx, id, draft.Replacement, "reviewed cover"); err != nil {
				t.Fatal(err)
			}
			if err = client.Resolve(ctx, id, draft.Replacement, "reviewed cover"); err != nil {
				t.Fatal("retry", err)
			}
			if _, err = client.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
				t.Fatal(err)
			}
			got, _, err := nas.Box(ctx, box)
			if err != nil || got.CoverFileID != p.CoverFileID.After {
				t.Fatal(got, err)
			}
			detail, err := client.LocalHistoryOp(ctx, id)
			if err != nil || detail.State != "replaced" {
				t.Fatal(detail, err)
			}
			original, err := oplog.Payload[oplog.UpdateBoxPayload](detail.Proposal.Payload)
			if err != nil || original.CoverFileID.Before != first.ID || original.CoverFileID.After != proposed.ID {
				t.Fatal(original, err)
			}
			if _, err = client.Undo(ctx, SyncReference{LocalID: draft.Replacement.ID}); err != nil {
				t.Fatal(err)
			}
			if _, err = client.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
				t.Fatal(err)
			}
			got, _, err = nas.Box(ctx, box)
			if err != nil || !got.CoverFileID.IsZero() {
				t.Fatal("undo did not restore reviewed current value", got, err)
			}
		})
	}
}
