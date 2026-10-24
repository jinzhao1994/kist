package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func TestClientServerCatalogAndAccessProtection(t *testing.T) {
	_, nas := testServer(t)
	ctx := t.Context()
	head, err := nas.SyncHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	client, err := library.CreateClient(ctx, root, library.ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	box, err := client.CreateBox(ctx, "Offline trip", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewClient(client, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	boxes := decodeBody[wire.BoxList](t, server, "/boxes")
	if len(boxes.Boxes) != 1 || boxes.Boxes[0].ID != box.ID.String() {
		t.Fatalf("boxes=%+v", boxes)
	}
	decodeBody[libraryInfo](t, server, "/info")
	status := decodeBody[library.ClientCatalogStatus](t, server, "/client/catalog")
	if status.Complete {
		t.Fatal("unreceived catalog reported complete")
	}
	if result := call(server, "GET", Prefix+"/sync/capabilities", "", ""); result.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated relay route: %d", result.Code)
	}
	for _, dir := range []string{"ops", "blobs"} {
		if _, err := os.Stat(filepath.Join(root, dir)); !os.IsNotExist(err) {
			t.Fatalf("NAS directory %s: %v", dir, err)
		}
	}
	req := httptest.NewRequest("GET", "http://attacker.invalid"+Prefix+"/boxes", nil)
	result := httptest.NewRecorder()
	server.ServeHTTP(result, req)
	if result.Code != http.StatusForbidden {
		t.Fatalf("host protection=%d", result.Code)
	}
	req = httptest.NewRequest("GET", "http://localhost:8473"+Prefix+"/boxes", nil)
	req.Header.Set("Origin", "https://attacker.invalid")
	result = httptest.NewRecorder()
	server.ServeHTTP(result, req)
	if result.Code != http.StatusForbidden {
		t.Fatalf("origin protection=%d", result.Code)
	}
	if _, err := NewClient(nil, Options{AllowedHosts: []string{"localhost:8473"}}); err == nil {
		t.Fatal("nil client accepted")
	}
}

func clientServerFromNAS(t *testing.T, nas *library.Library) *Server {
	t.Helper()
	ctx := t.Context()
	head, err := nas.SyncHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	client, err := library.CreateClient(ctx, t.TempDir(), library.ClientSettings{LibraryID: head.LibraryID, Timezone: nas.Zone().String()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	var after int64
	for {
		page, err := nas.SyncCatalog(ctx, head, after, 2, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.ReceiveCatalogPage(ctx, page); err != nil {
			t.Fatal(err)
		}
		if page.Done {
			break
		}
		after = page.Next
	}
	server, err := NewClient(client, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	return server
}
