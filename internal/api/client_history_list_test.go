package api

import (
	"fmt"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

func TestClientHistoryListAndUndoUseFormalReferences(t *testing.T) {
	_, nas := testServer(t)
	ctx := t.Context()
	box, err := nas.CreateBox(ctx, "NAS box", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	head, err := nas.SyncHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c, err := library.CreateClient(ctx, t.TempDir(), library.ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	page, err := nas.SyncCatalog(ctx, head, 0, 128, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ReceiveCatalogPage(ctx, page); err != nil {
		t.Fatal(err)
	}
	s, err := NewClient(c, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	list := decodeBody[historyList](t, s, "/ops")
	if len(list.Ops) != 1 || list.Ops[0].ID != head.OpID {
		t.Fatal(list)
	}
	summary := decodeBody[library.HistorySummary](t, s, fmt.Sprintf("/ops/%d/summary", head.OpID))
	if summary.Name != "NAS box" {
		t.Fatal(summary)
	}
	detail := decodeBody[historyDetail](t, s, fmt.Sprintf("/ops/%d", head.OpID))
	if len(detail.Boxes) != 1 || detail.Boxes[0] != box.ID.String() {
		t.Fatal(detail)
	}
	path := Prefix + fmt.Sprintf("/ops/%d/undo", head.OpID)
	first := call(s, "POST", path, "client-history-undo", `{}`)
	if first.Code != 200 {
		t.Fatal(first.Code, first.Body)
	}
	second := call(s, "POST", path, "client-history-undo", `{}`)
	if second.Code != 200 || second.Body.String() != first.Body.String() {
		t.Fatal(second.Code, second.Body, first.Body)
	}
	if _, found, err := c.Box(ctx, box.ID); err != nil || found {
		t.Fatal(found, err)
	}
	list = decodeBody[historyList](t, s, "/ops")
	if len(list.Ops) != 1 || list.Ops[0].UndoneBy != nil {
		t.Fatal(list)
	}
	if _, found, err := nas.Box(ctx, box.ID); err != nil || !found {
		t.Fatal(found, err)
	}
	missing := call(s, "GET", Prefix+fmt.Sprintf("/ops/%d", head.OpID+1), "", "")
	if missing.Code != 404 {
		t.Fatal(missing.Code, missing.Body)
	}
}
