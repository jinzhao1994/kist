package api

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
)

func TestClientDemandContentExtractionAndConnectionState(t *testing.T) {
	_, nas := testServer(t)
	ctx := t.Context()
	box, err := nas.CreateBox(ctx, "online", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, name := range []string{"first.txt", "second.txt", "offline.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("original content"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Distinct bytes ensure the extraction path cannot reuse the first download.
	if err := os.WriteFile(filepath.Join(dir, "second.txt"), []byte("second original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "offline.txt"), []byte("not downloaded"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.Import(ctx, dir, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files, _, err := nas.Files(ctx, query.Query{})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, file := range files {
		ids[file.Name] = file.ID.String()
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
	var after int64
	for {
		page, err := nas.SyncCatalog(ctx, head, after, 128, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		if err = c.ReceiveCatalogPage(ctx, page); err != nil {
			t.Fatal(err)
		}
		if page.Done {
			break
		}
		after = page.Next
	}
	local, err := NewClient(c, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	state := func() string { return decodeBody[clientConnectionStatus](t, local, "/client/connection-status").State }
	if state() != "unconfigured" {
		t.Fatal(state())
	}
	remote := httptest.NewUnstartedServer(nil)
	remote.Config.Handler, err = New(nas, Options{AllowedHosts: []string{remote.Listener.Addr().String()}})
	if err != nil {
		t.Fatal(err)
	}
	remote.Start()
	defer remote.Close()
	grant, err := nas.AuthorizeSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.SetConnection(ctx, head.LibraryID, library.ClientConnection{Server: remote.URL}, grant.Token); err != nil {
		t.Fatal(err)
	}
	if state() != "connected" {
		t.Fatal(state())
	}
	content := Prefix + "/files/" + ids["first.txt"] + "/content"
	if w := call(local, "GET", content, "", ""); w.Code != 200 || w.Body.String() != "original content" {
		t.Fatal(w.Code, w.Body)
	}
	extraction := Prefix + "/files/" + ids["second.txt"] + "/extractions/media"
	if w := call(local, "POST", extraction, "online-extraction-001", "{}"); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if _, err := nas.RevokeSync(ctx, grant.ID); err != nil {
		t.Fatal(err)
	}
	if state() != "unauthorized" {
		t.Fatal(state())
	}
	remote.Close()
	if state() != "unreachable" {
		t.Fatal(state())
	}
	if w := call(local, "GET", content, "", ""); w.Code != 200 || w.Body.String() != "original content" {
		t.Fatal("cached offline content", w.Code, w.Body)
	}
	if w := call(local, "GET", Prefix+"/files/"+ids["offline.txt"]+"/content", "", ""); w.Code != 503 || !strings.Contains(w.Body.String(), `"code":"not_cached"`) {
		t.Fatal("offline miss", w.Code, w.Body)
	}
}
