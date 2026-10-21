package api

import (
	"encoding/json"
	"testing"

	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func TestClientHTTPWritesDeduplicateAndRecoverReceipts(t *testing.T) {
	_, nas := testServer(t)
	ctx := t.Context()
	head, err := nas.SyncHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	c, err := library.CreateClient(ctx, root, library.ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	s, err := NewClient(c, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"name":"Trip","description":"offline"}`
	first := call(s, "POST", Prefix+"/boxes", "create-trip-key", body)
	if first.Code != 200 {
		t.Fatalf("create=%d %s", first.Code, first.Body)
	}
	var receipt library.ClientRequestResult
	if err := json.Unmarshal(first.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.LocalID == "" || receipt.Noop {
		t.Fatalf("receipt=%+v", receipt)
	}
	retry := call(s, "POST", Prefix+"/boxes", "create-trip-key", `{ "description":"offline", "name":"Trip" }`)
	if retry.Code != 200 || retry.Body.String() != first.Body.String() {
		t.Fatalf("retry=%d %s", retry.Code, retry.Body)
	}
	changed := call(s, "POST", Prefix+"/boxes", "create-trip-key", `{"name":"Different"}`)
	if changed.Code != 409 {
		t.Fatalf("changed=%d %s", changed.Code, changed.Body)
	}
	other, err := library.OpenClient(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	reopened, err := NewClient(other, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	got := decodeBody[library.ClientRequestResult](t, reopened, "/requests/create-trip-key")
	if got != receipt {
		t.Fatalf("reopened receipt=%+v want=%+v", got, receipt)
	}
	boxes := decodeBody[wire.BoxList](t, reopened, "/boxes")
	if len(boxes.Boxes) != 1 {
		t.Fatalf("boxes=%+v", boxes)
	}
	mustWrite(t, reopened, "POST", "/tags", "create-tag-key", `{"name":"Travel"}`)
	tags := decodeBody[wire.TagTree](t, reopened, "/tags")
	if len(tags.Tags) != 1 {
		t.Fatalf("tags=%+v", tags)
	}
	attach := `{"ids":["` + boxes.Boxes[0].ID + `"]}`
	mustWrite(t, reopened, "PUT", "/tags/"+tags.Tags[0].ID+"/boxes", "attach-box-tag", attach)
	attached := decodeBody[tagList](t, reopened, "/boxes/"+boxes.Boxes[0].ID+"/tags")
	if len(attached.Tags) != 1 || attached.Tags[0] != tags.Tags[0].ID {
		t.Fatalf("attached=%+v", attached)
	}
	boxPath := "/boxes/" + boxes.Boxes[0].ID
	preview := decodeBody[library.ClientDeletionPreview](t, reopened, boxPath+"/deletion-preview")
	if preview.Revision == "" {
		t.Fatal("missing deletion revision")
	}
	mustWrite(t, reopened, "PUT", boxPath, "rename-before-delete", `{"name":"Changed trip","description":"offline"}`)
	oldBody, _ := json.Marshal(map[string]string{"revision": preview.Revision})
	stale := call(reopened, "DELETE", Prefix+boxPath, "stale-delete-key", string(oldBody))
	if stale.Code != 409 {
		t.Fatalf("stale delete=%d %s", stale.Code, stale.Body)
	}
	preview = decodeBody[library.ClientDeletionPreview](t, reopened, boxPath+"/deletion-preview")
	newBody, _ := json.Marshal(map[string]string{"revision": preview.Revision})
	mustWrite(t, reopened, "DELETE", boxPath, "confirmed-delete-key", string(newBody))
	mustWrite(t, reopened, "DELETE", boxPath, "confirmed-delete-key", string(newBody))
	if got := decodeBody[wire.BoxList](t, reopened, "/boxes"); len(got.Boxes) != 0 {
		t.Fatalf("deleted box visible=%+v", got)
	}
	missing := call(reopened, "GET", Prefix+"/requests/missing-key", "", "")
	if missing.Code != 404 {
		t.Fatalf("missing=%d", missing.Code)
	}
}
