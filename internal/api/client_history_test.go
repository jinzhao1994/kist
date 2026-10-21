package api

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
)

func TestClientHistoricalAPIReadsConfirmedStateAndCachedContent(t *testing.T) {
	_, nas := testServer(t)
	ctx := t.Context()
	box, err := nas.CreateBox(ctx, "original box", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "original.txt")
	const data = "historical cached bytes"
	if err := os.WriteFile(source, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.Import(ctx, source, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files, _, err := nas.Files(ctx, query.Query{})
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	file := files[0]
	cutoff, err := nas.SyncHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nas.RenameEntry(ctx, box.ID, file.ID.ID, false, file.Name, "NAS.txt"); err != nil {
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
	if err != nil || !page.Done {
		t.Fatal(page, err)
	}
	if err := c.ReceiveCatalogPage(ctx, page); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RenameEntry(ctx, box.ID, file.ID.ID, false, "NAS.txt", "pending.txt"); err != nil {
		t.Fatal(err)
	}
	s, err := NewClient(c, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	old := decodeBody[historicalFile](t, s, fmt.Sprintf("/history/%d/files/%s", cutoff.OpID, file.ID))
	if old.File.File.Name != "original.txt" || len(old.File.File.ExtractionPending) != 0 {
		t.Fatal(old)
	}
	oldBox := decodeBody[historicalBox](t, s, fmt.Sprintf("/history/%d/boxes/%s", cutoff.OpID, box.ID))
	if len(oldBox.Files) != 1 || oldBox.Files[0].Name != "original.txt" {
		t.Fatal(oldBox)
	}
	current := decodeBody[fileDetail](t, s, "/files/"+file.ID.String())
	if current.File.Name != "pending.txt" {
		t.Fatal(current)
	}
	path := Prefix + fmt.Sprintf("/files/%s/content?at=%d", file.ID, cutoff.OpID)
	missing := call(s, "GET", path, "", "")
	if missing.Code != 503 || !strings.Contains(missing.Body.String(), `"code":"not_cached"`) {
		t.Fatal(missing.Code, missing.Body)
	}
	if err := c.CacheOriginal(ctx, file.BlobKey, previewOriginalSource{nas, data}); err != nil {
		t.Fatal(err)
	}
	content := call(s, "GET", path, "", "")
	if content.Code != 200 || content.Body.String() != data || !strings.Contains(content.Header().Get("Content-Disposition"), "original.txt") {
		t.Fatal(content.Code, content.Body, content.Header())
	}
	r := httptest.NewRequest("GET", "http://localhost:8473"+path, nil)
	r.Header.Set("Range", "bytes=0-3")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 206 || w.Body.String() != "hist" {
		t.Fatal(w.Code, w.Body)
	}
	for _, suffix := range []string{
		fmt.Sprintf("/history/%d/files/%s", head.OpID+1, file.ID),
		fmt.Sprintf("/files/%s/content?at=%d", file.ID, head.OpID+1),
	} {
		w := call(s, "GET", Prefix+suffix, "", "")
		if w.Code != 404 {
			t.Fatal(w.Code, w.Body)
		}
	}
	after, err := nas.SyncHead(ctx)
	if err != nil || after != head {
		t.Fatal(after, head, err)
	}
}
