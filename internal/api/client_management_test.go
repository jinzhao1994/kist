package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/library"
)

func TestClientManagementPersistsLocalSettingsWithoutNASWrites(t *testing.T) {
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
	server, err := NewClient(c, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	info := decodeBody[libraryInfo](t, server, "/info")
	if !info.Client {
		t.Fatal("client mode not advertised")
	}
	initial := decodeBody[library.ClientStatus](t, server, "/client/status")
	if initial.Catalog.Complete || len(initial.Pending) != 0 || initial.Connection != nil {
		t.Fatal(initial)
	}
	changed := call(server, "PUT", Prefix+"/client/cache", "", `{"max_age":"2d","max_bytes":1024}`)
	if changed.Code != 200 {
		t.Fatal(changed.Code, changed.Body)
	}
	var budget library.ClientCacheStatus
	if err := json.Unmarshal(changed.Body.Bytes(), &budget); err != nil {
		t.Fatal(err)
	}
	if budget.LimitBytes != 1024 {
		t.Fatal(budget)
	}
	for _, bad := range []string{`{"max_age":"-1h","max_bytes":1024}`, `{"max_age":"invalid","max_bytes":1024}`, `{"max_age":"48h","max_bytes":0}`} {
		rejected := call(server, "PUT", Prefix+"/client/cache", "", bad)
		if rejected.Code != 400 && rejected.Code != 422 {
			t.Fatal(rejected.Code, rejected.Body)
		}
	}
	reopened, err := library.OpenClient(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	status, err := reopened.Status(ctx)
	if err != nil || status.Settings.CacheMaxBytes != 1024 {
		t.Fatal(status, err)
	}
	age, err := time.ParseDuration(status.Settings.CacheMaxAge)
	if err != nil || age != 48*time.Hour {
		t.Fatal(age, err)
	}
	pruned := call(server, "POST", Prefix+"/client/cache/prune", "", "{}")
	if pruned.Code != 200 {
		t.Fatal(pruned.Code, pruned.Body)
	}
	empty := decodeBody[map[string]clientOfflineSelection](t, server, "/client/offline")
	if len(empty) != 0 {
		t.Fatal(empty)
	}
	bad := call(server, "PUT", Prefix+"/client/offline/travel", "", `{"files":["not-an-id"],"filesets":[]}`)
	if bad.Code != 400 {
		t.Fatal(bad.Code, bad.Body)
	}
	cleared := call(server, "DELETE", Prefix+"/client/offline/travel", "", "")
	if cleared.Code != 200 {
		t.Fatal(cleared.Code, cleared.Body)
	}
	final := decodeBody[library.ClientStatus](t, server, "/client/status")
	count, err := nas.OpCount(ctx)
	if err != nil || count != 0 || len(final.Pending) != 0 {
		t.Fatal(count, final, err)
	}
}
