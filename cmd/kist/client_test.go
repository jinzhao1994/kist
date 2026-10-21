package main

import (
	"encoding/json"
	"github.com/jinzhao1994/kist/internal/query"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/api"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/oplog"
)

func clientNAS(t *testing.T, timezone ...string) (*library.Library, *httptest.Server, library.NewSyncCredential) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "nas")
	if _, err := library.Create(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	if len(timezone) > 0 {
		path := filepath.Join(root, "kist.toml")
		cfg, err := config.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Library.Timezone = timezone[0]
		if err := config.Save(path, cfg); err != nil {
			t.Fatal(err)
		}
	}
	nas, err := library.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nas.Close() })
	credential, err := nas.AuthorizeSync(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(nil)
	server.Config.Handler, err = api.New(nas, api.Options{AllowedHosts: []string{server.Listener.Addr().String()}})
	if err != nil {
		t.Fatal(err)
	}
	server.Start()
	t.Cleanup(server.Close)
	return nas, server, credential
}

func TestClientCLIConnectionAndSynchronization(t *testing.T) {
	nas, server, credential := clientNAS(t)
	root := filepath.Join(t.TempDir(), "laptop")
	out, err := runKist(t, credential.Token+"\n", "--json", "client", "init", root, "--server", server.URL, "--token-stdin")
	var status library.ClientStatus
	if err != nil || json.Unmarshal([]byte(out), &status) != nil || status.Connection == nil || status.Catalog.Complete || strings.Contains(out, credential.Token) {
		t.Fatalf("init output invalid or exposed secret: err=%v", err)
	}
	client, err := library.OpenClient(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	id := domain.NewBoxID()
	raw, _ := oplog.Marshal(oplog.CreateBoxPayload{BoxID: id, RootFolderID: domain.NewFolderID(), BoxFields: oplog.BoxFields{Name: "offline from CLI"}})
	if err := client.Stage(t.Context(), library.SyncProposal{Version: 1, ID: "cli-pending-box", OccurredAt: time.Now().UTC(), Action: string(oplog.ActionCreateBox), Origin: "user", Payload: raw}); err != nil {
		t.Fatal(err)
	}
	client.Close()
	out, err = runKist(t, "", "--json", "client", "sync", root)
	var result library.ClientSyncResult
	if err != nil || json.Unmarshal([]byte(out), &result) != nil || !result.Complete || result.Accepted != 1 {
		t.Fatalf("sync=%s err=%v", out, err)
	}
	if _, found, err := nas.Box(t.Context(), id); err != nil || !found {
		t.Fatalf("NAS missing offline edit: %v", err)
	}
	_, otherServer, otherCredential := clientNAS(t)
	if _, err := runKist(t, otherCredential.Token, "--json", "client", "connect", root, "--server", otherServer.URL, "--token-stdin"); err == nil {
		t.Fatal("connected existing client to another NAS")
	}
	replacement, err := nas.AuthorizeSync(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	out, err = runKist(t, replacement.Token, "--json", "client", "connect", root, "--server", server.URL, "--token-stdin")
	if err != nil {
		t.Fatal(err)
	}
	mustJSON(t, out)
	if strings.Contains(out, replacement.Token) {
		t.Fatal("connection output exposed secret")
	}
	server.Close()
	for _, action := range []string{"status", "prune", "rebuild"} {
		out, err := runKist(t, "", "--json", "client", "cache", action, root)
		if err != nil {
			t.Fatalf("offline cache %s: %v", action, err)
		}
		mustJSON(t, out)
	}
	out, err = runKist(t, "", "--json", "client", "cache", "configure", root, "--max-bytes", "1024", "--max-age", "24h")
	var cache library.ClientCacheStatus
	if err != nil || json.Unmarshal([]byte(out), &cache) != nil || cache.LimitBytes != 1024 {
		t.Fatalf("cache configure=%s %v", out, err)
	}
	out, err = runKist(t, "", "--json", "client", "status", root)
	status = library.ClientStatus{}
	if err != nil || json.Unmarshal([]byte(out), &status) != nil || !status.Catalog.Complete || len(status.Pending) != 0 {
		t.Fatalf("offline status=%s err=%v", out, err)
	}
	if strings.Contains(out, replacement.Token) {
		t.Fatal("status output exposed secret")
	}
	if status.Settings.CacheMaxAge != "24h0m0s" || status.Settings.CacheMaxBytes != 1024 {
		t.Fatal("cache configuration was not persisted")
	}
	if _, err := runKist(t, "", "client", "sync", root); err == nil {
		t.Fatal("offline sync unexpectedly succeeded")
	}
}

func TestClientOfflineCLIRealDownloadAndDisconnectedStatus(t *testing.T) {
	nas, server, credential := clientNAS(t)
	ctx := t.Context()
	box, err := nas.CreateBox(ctx, "offline CLI files", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(source, []byte("download this original"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.Import(ctx, source, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files, _, err := nas.Files(ctx, query.Query{})
	if err != nil || len(files) != 1 {
		t.Fatalf("files=%v %v", files, err)
	}
	root := filepath.Join(t.TempDir(), "client")
	if _, err := runKist(t, credential.Token, "--json", "client", "init", root, "--server", server.URL, "--token-stdin"); err != nil {
		t.Fatal(err)
	}
	if _, err := runKist(t, "", "--json", "client", "sync", root); err != nil {
		t.Fatal(err)
	}
	if _, err := runKist(t, "", "--json", "client", "offline", "select", root, "trip", "--file", files[0].ID.String()); err != nil {
		t.Fatal(err)
	}
	out, err := runKist(t, "", "--json", "client", "offline", "status", root, "trip")
	var status library.ClientOfflineStatus
	if err != nil || json.Unmarshal([]byte(out), &status) != nil || status.Ready || len(status.Missing) != 1 {
		t.Fatalf("before=%s %v", out, err)
	}
	out, err = runKist(t, "", "--json", "client", "offline", "prepare", root, "trip")
	if err != nil || json.Unmarshal([]byte(out), &status) != nil || !status.Ready {
		t.Fatalf("prepare=%s %v", out, err)
	}
	server.Close()
	for _, action := range []string{"status", "prepare"} {
		out, err = runKist(t, "", "--json", "client", "offline", action, root, "trip")
		if err != nil || json.Unmarshal([]byte(out), &status) != nil || !status.Ready {
			t.Fatalf("disconnected %s=%s %v", action, out, err)
		}
	}
	if _, err := runKist(t, "", "--json", "client", "offline", "remove", root, "trip"); err != nil {
		t.Fatal(err)
	}
	out, err = runKist(t, "", "--json", "client", "offline", "list", root)
	var result struct {
		Selections map[string]library.ClientOfflineSelection `json:"selections"`
	}
	if err != nil || json.Unmarshal([]byte(out), &result) != nil || len(result.Selections) != 0 {
		t.Fatalf("remaining=%s %v", out, err)
	}
}

func TestClientCLIInitializesWithNASTimezone(t *testing.T) {
	_, server, credential := clientNAS(t, "Asia/Tokyo")
	root := filepath.Join(t.TempDir(), "client")
	out, err := runKist(t, credential.Token, "--json", "client", "init", root, "--server", server.URL, "--token-stdin")
	var status library.ClientStatus
	if err != nil || json.Unmarshal([]byte(out), &status) != nil || status.Settings.Timezone != "Asia/Tokyo" {
		t.Fatalf("init timezone=%s %v", out, err)
	}
}
