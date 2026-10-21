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

func TestClientContentRangeAndMissingCache(t *testing.T) {
	_, nas := testServer(t)
	ctx := t.Context()
	content := "0123456789"
	box, err := nas.CreateBox(ctx, "Content", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "notes.txt"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.Import(ctx, source, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files, _, err := nas.Files(ctx, query.Query{})
	if err != nil || len(files) != 1 {
		t.Fatalf("files=%+v %v", files, err)
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
		if err := c.ReceiveCatalogPage(ctx, page); err != nil {
			t.Fatal(err)
		}
		if page.Done {
			break
		}
		after = page.Next
	}
	s, err := NewClient(c, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	path := Prefix + "/files/" + files[0].ID.String() + "/content"
	missing := call(s, "GET", path, "", "")
	if missing.Code != 503 || !strings.Contains(missing.Body.String(), `"code":"not_cached"`) {
		t.Fatalf("uncached=%d %s", missing.Code, missing.Body)
	}
	if _, err := c.PutOriginal(ctx, strings.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "http://localhost:8473"+path, nil)
	req.Header.Set("Range", "bytes=2-5")
	result := httptest.NewRecorder()
	s.ServeHTTP(result, req)
	if result.Code != 206 || result.Body.String() != "2345" || result.Header().Get("Content-Range") != "bytes 2-5/10" {
		t.Fatalf("range=%d %s %+v", result.Code, result.Body, result.Header())
	}
	if !strings.HasPrefix(result.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("unsafe inline content")
	}
	if head := call(s, "HEAD", path, "", ""); head.Code != 200 || head.Body.Len() != 0 {
		t.Fatalf("HEAD=%d %s", head.Code, head.Body)
	}
}
