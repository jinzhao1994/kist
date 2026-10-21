package api

import (
	"testing"

	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func TestClientLocalHistoryAndUndoKeepIndependentIdentities(t *testing.T) {
	_, nas := testServer(t)
	head, err := nas.SyncHead(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	c, err := library.CreateClient(t.Context(), t.TempDir(), library.ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	s, err := NewClient(c, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	created := call(s, "POST", Prefix+"/boxes", "local-history-create", `{"name":"Offline box"}`)
	if created.Code != 200 {
		t.Fatal(created.Code, created.Body)
	}
	history := decodeBody[clientLocalHistoryList](t, s, "/client/history")
	if len(history.Entries) != 1 || history.Entries[0].State != "pending" || history.Entries[0].Receipt != nil {
		t.Fatal(history)
	}
	localID := history.Entries[0].ID
	detail := decodeBody[library.ClientLocalHistoryDetail](t, s, "/client/history/"+localID)
	if detail.Proposal.ID != localID || detail.Proposal.Action != "create_box" {
		t.Fatal(detail)
	}
	undoPath := Prefix + "/client/history/" + localID + "/undo"
	first := call(s, "POST", undoPath, "local-history-undo", `{}`)
	second := call(s, "POST", undoPath, "local-history-undo", `{}`)
	if first.Code != 200 || second.Code != 200 || first.Body.String() != second.Body.String() {
		t.Fatal(first.Code, first.Body, second.Code, second.Body)
	}
	boxes := decodeBody[wire.BoxList](t, s, "/boxes")
	if len(boxes.Boxes) != 0 {
		t.Fatal(boxes)
	}
	history = decodeBody[clientLocalHistoryList](t, s, "/client/history?limit=1")
	if len(history.Entries) != 1 || history.Entries[0].Action != "undo" || history.Entries[0].Undo.LocalID != localID || history.Before == "" {
		t.Fatal(history)
	}
	count, err := nas.OpCount(t.Context())
	if err != nil || count != 0 {
		t.Fatal(count, err)
	}
}
