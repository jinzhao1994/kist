package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
)

func TestClientHTTPExtractionUsesLocalReceipt(t *testing.T) {
	_, nas := testServer(t)
	ctx := t.Context()
	head, err := nas.SyncHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c, err := library.CreateClient(ctx, t.TempDir(), library.ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	box, err := c.CreateBox(ctx, "offline", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte("some text"), 0600); err != nil {
		t.Fatal(err)
	}
	prepared, err := c.PrepareFiles(ctx, []library.ImportSource{{Path: path, RelativePath: "sample.txt"}}, library.ClientImportOptions{Box: box.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	files, _, err := c.Files(ctx, query.Query{})
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	s, err := NewClient(c, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	endpoint := Prefix + "/files/" + files[0].ID.String() + "/extractions/media"
	first := call(s, "POST", endpoint, "local-media-extract", "{}")
	if first.Code != 200 {
		t.Fatal(first.Code, first.Body)
	}
	receipt := decodeBody[library.ClientRequestResult](t, s, "/requests/local-media-extract")
	if receipt.LocalID == "" || receipt.Noop {
		t.Fatal(receipt)
	}
	retry := call(s, "POST", endpoint, "local-media-extract", "{}")
	if retry.Code != 200 || retry.Body.String() != first.Body.String() {
		t.Fatal(retry.Code, retry.Body)
	}
	info, err := c.MediaInfo(ctx, files[0].ID)
	if err != nil || info.Kind != "document" {
		t.Fatal(info, err)
	}
}
