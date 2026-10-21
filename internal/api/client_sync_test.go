package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

func TestClientHTTPConnectSyncAndConflict(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	if _, err := library.Create(ctx, root); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(filepath.Join(root, "kist.toml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Library.Timezone = "UTC"
	if err := config.Save(filepath.Join(root, "kist.toml"), cfg); err != nil {
		t.Fatal(err)
	}
	nas, err := library.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer nas.Close()
	credential, err := nas.AuthorizeSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	remote := httptest.NewUnstartedServer(nil)
	remote.Config.Handler, err = New(nas, Options{AllowedHosts: []string{remote.Listener.Addr().String()}})
	if err != nil {
		t.Fatal(err)
	}
	remote.Start()
	defer remote.Close()
	head, err := nas.SyncHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c, err := library.CreateClient(ctx, t.TempDir(), library.ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	s, err := NewClient(c, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"server": remote.URL, "token": credential.Token, "allow_http": false})
	connected := call(s, "PUT", Prefix+"/client/connection", "", string(body))
	if connected.Code != 200 || strings.Contains(connected.Body.String(), credential.Token) {
		t.Fatalf("connection status=%d (body omitted)", connected.Code)
	}
	box, err := c.CreateBox(ctx, "Travel", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	sync := call(s, "POST", Prefix+"/client/sync", "", "{}")
	if sync.Code != 200 {
		t.Fatal(sync.Code, sync.Body)
	}
	var first clientSyncResponse
	if err := json.Unmarshal(sync.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if !first.Result.Complete || first.Result.Accepted != 1 || len(first.Status.Pending) != 0 {
		t.Fatal(first)
	}
	if _, err := c.UpdateBox(ctx, box.ID, library.BoxFields{Name: "Laptop"}); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.UpdateBox(ctx, box.ID, library.BoxFields{Name: "NAS"}); err != nil {
		t.Fatal(err)
	}
	conflict := call(s, "POST", Prefix+"/client/sync", "", "{}")
	if conflict.Code != 200 {
		t.Fatal(conflict.Code, conflict.Body)
	}
	var result clientSyncResponse
	if err := json.Unmarshal(conflict.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Result.Complete || result.Status.BlockedID == "" || len(result.Status.Pending) != 1 {
		t.Fatal(result)
	}
	current, found, err := nas.Box(ctx, box.ID)
	if err != nil || !found || current.Name != "NAS" {
		t.Fatal(current, err)
	}
	retry := call(s, "POST", Prefix+"/client/sync", "", "{}")
	if retry.Code != 200 {
		t.Fatal(retry.Code, retry.Body)
	}
	decision := `{"reason":"Keep the NAS name"}`
	resolved := call(s, "POST", Prefix+"/client/pending/"+result.Status.BlockedID+"/resolve", "", decision)
	if resolved.Code != 200 {
		t.Fatal(resolved.Code, resolved.Body)
	}
	duplicate := call(s, "POST", Prefix+"/client/pending/"+result.Status.BlockedID+"/resolve", "", decision)
	if duplicate.Code != 200 {
		t.Fatal(duplicate.Code, duplicate.Body)
	}
	final := call(s, "POST", Prefix+"/client/sync", "", "{}")
	if final.Code != 200 {
		t.Fatal(final.Code, final.Body)
	}
	var converged clientSyncResponse
	if err := json.Unmarshal(final.Body.Bytes(), &converged); err != nil {
		t.Fatal(err)
	}
	if !converged.Result.Complete || len(converged.Status.Pending) != 0 {
		t.Fatal(converged)
	}
	count, err := nas.OpCount(ctx)
	if err != nil || count != 2 {
		t.Fatal(count, err)
	}
}
