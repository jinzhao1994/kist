package api

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSavedSearchRequests(t *testing.T) {
	s, l := testServer(t)
	body := `{"name":"Pictures","definition":{"version":1,"entity":"files","filters":{"sort":"updated","asc":false,"kind":["image"]}}}`
	first := call(s, "POST", Prefix+"/saved-searches", "save-search", body)
	retry := call(s, "POST", Prefix+"/saved-searches", "save-search", body)
	if first.Code != 200 || retry.Code != 200 || !bytes.Equal(first.Body.Bytes(), retry.Body.Bytes()) {
		t.Fatalf("save: %d %s / %d %s", first.Code, first.Body, retry.Code, retry.Body)
	}
	items, err := l.SavedSearches(t.Context())
	if err != nil || len(items) != 1 {
		t.Fatalf("items: %v %v", items, err)
	}
	listed := call(s, "GET", Prefix+"/saved-searches", "", "")
	if listed.Code != 200 || !bytes.Contains(listed.Body.Bytes(), []byte(`"Pictures"`)) {
		t.Fatalf("list: %d %s", listed.Code, listed.Body)
	}
	path := Prefix + "/saved-searches/" + items[0].ID.String()
	var original savedSearchBody
	if err := json.Unmarshal([]byte(body), &original); err != nil {
		t.Fatal(err)
	}
	updateBody, _ := json.Marshal(updateSavedSearchBody{Name: "Photos", Definition: original.Definition, Before: original})
	renamed := call(s, "PUT", path, "rename-search", string(updateBody))
	if renamed.Code != 200 {
		t.Fatalf("update: %d %s", renamed.Code, renamed.Body)
	}
	stale := call(s, "PUT", path, "stale-search", string(updateBody))
	if stale.Code != 409 {
		t.Fatalf("stale update: %d %s", stale.Code, stale.Body)
	}
	removed := call(s, "DELETE", path, "remove-search", "")
	if removed.Code != 200 {
		t.Fatalf("delete: %d %s", removed.Code, removed.Body)
	}
	absent := call(s, "DELETE", path, "remove-missing-search", "")
	if absent.Code != 404 {
		t.Fatalf("missing: %d %s", absent.Code, absent.Body)
	}
	invalid := call(s, "POST", Prefix+"/saved-searches", "bad-search", string(bytes.ReplaceAll([]byte(body), []byte(`"version":1`), []byte(`"version":2`))))
	if invalid.Code != 400 {
		t.Fatalf("invalid: %d %s", invalid.Code, invalid.Body)
	}
}
