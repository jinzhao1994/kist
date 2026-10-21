package library

import (
	"errors"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/oplog"
)

func TestClientLocalHistoryRetainsIntentUndoAndAcceptance(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	original := syncBox(t, "local-history-create")
	if err := c.Stage(t.Context(), original); err != nil {
		t.Fatal(err)
	}
	inverse, err := c.Undo(t.Context(), SyncReference{LocalID: original.ID})
	if err != nil {
		t.Fatal(err)
	}
	page, next, err := c.LocalHistory(t.Context(), "", 1)
	if err != nil || len(page) != 1 || page[0].ID != inverse.LocalID || page[0].State != "pending" || page[0].Action != "undo" || page[0].Undo.LocalID != original.ID || next != inverse.LocalID {
		t.Fatal(page, next, err)
	}
	older, end, err := c.LocalHistory(t.Context(), next, 1)
	if err != nil || len(older) != 1 || older[0].ID != original.ID || end != "" || older[0].Receipt != nil {
		t.Fatal(older, end, err)
	}
	detail, err := c.LocalHistoryOp(t.Context(), original.ID)
	if err != nil || string(detail.Proposal.Payload) != string(original.Payload) || !detail.OccurredAt.Equal(original.OccurredAt) {
		t.Fatal(detail, err)
	}
	if _, err := c.Synchronize(t.Context(), &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	accepted, _, err := c.LocalHistory(t.Context(), "", 10)
	if err != nil || len(accepted) != 2 {
		t.Fatal(accepted, err)
	}
	for _, entry := range accepted {
		if entry.State != "accepted" || entry.Receipt == nil || entry.Receipt.OpID == 0 {
			t.Fatal(entry)
		}
		formal, err := nas.HistoryOp(t.Context(), entry.Receipt.OpID)
		if err != nil || !formal.TS.Equal(entry.OccurredAt) {
			t.Fatal(formal, entry, err)
		}
	}
	reopened, err := OpenClient(t.Context(), c.root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.LocalHistoryOp(t.Context(), original.ID)
	if err != nil || got.State != "accepted" || string(got.Proposal.Payload) != string(original.Payload) {
		t.Fatal(got, err)
	}
	if _, _, err := c.LocalHistory(t.Context(), "unknown", 1); !errors.Is(err, ErrNoSuchOp) {
		t.Fatal(err)
	}
}

func TestClientLocalHistoryKeepsReplacedIntentAndDecision(t *testing.T) {
	nas, box := openWithBox(t)
	c := newSyncClient(t, nas)
	b, _, err := c.Box(t.Context(), box)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := oplog.Marshal(oplog.UpdateBoxPayload{BoxID: box, Name: &oplog.TextChange{Before: b.Name, After: "local name"}})
	original := SyncProposal{Version: 1, ID: "local-history-conflict", OccurredAt: time.Now().UTC(), Action: string(oplog.ActionUpdateBox), Origin: "user", Payload: raw}
	if err := c.Stage(t.Context(), original); err != nil {
		t.Fatal(err)
	}
	peer := &testSyncPeer{nas: nas, beforeSubmit: func() {
		if _, err := nas.UpdateBox(t.Context(), box, BoxFields{Name: "NAS name"}); err != nil {
			t.Fatal(err)
		}
	}}
	_, err = c.Synchronize(t.Context(), peer)
	var rejected *SyncRejectedError
	if !errors.As(err, &rejected) {
		t.Fatal(err)
	}
	replacement := original
	replacement.ID = "local-history-replacement"
	replacement.Payload, _ = oplog.Marshal(oplog.UpdateBoxPayload{BoxID: box, Name: &oplog.TextChange{Before: "NAS name", After: "chosen name"}})
	if err := c.Resolve(t.Context(), original.ID, &replacement, "choose another name"); err != nil {
		t.Fatal(err)
	}
	page, cursor, err := c.LocalHistory(t.Context(), "", 1)
	if err != nil || len(page) != 1 || page[0].ID != replacement.ID || cursor != replacement.ID {
		t.Fatal(page, cursor, err)
	}
	old, err := c.LocalHistoryOp(t.Context(), original.ID)
	if err != nil || old.State != "replaced" || old.ReplacedBy != replacement.ID || old.Reason != "choose another name" || string(old.Proposal.Payload) != string(original.Payload) {
		t.Fatal(old, err)
	}
	older, _, err := c.LocalHistory(t.Context(), cursor, 1)
	if err != nil || len(older) != 1 || older[0].ID != original.ID {
		t.Fatal(older, err)
	}
}
