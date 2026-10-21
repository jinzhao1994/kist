package api

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/sheetpreview"
	"github.com/jinzhao1994/kist/internal/textpreview"
)

type previewOriginalSource struct {
	nas  *library.Library
	data string
}

func (s previewOriginalSource) Capabilities(ctx context.Context) (library.SyncCapabilities, error) {
	head, err := s.nas.SyncHead(ctx)
	return library.SyncCapabilities{Version: 1, LibraryID: head.LibraryID, Timezone: s.nas.Zone().String(), JournalVersion: journal.Version, Actions: library.SyncActions(), UndoReferences: true}, err
}
func (s previewOriginalSource) OpenOriginal(_ context.Context, _ string, _ int64, offset int64) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(s.data[offset:])), nil
}

func TestClientCachedTextAndSheetPreviews(t *testing.T) {
	_, nas := testServer(t)
	ctx := t.Context()
	box, err := nas.CreateBox(ctx, "sheets", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	data := "name,value\n中文,12\n"
	source := filepath.Join(t.TempDir(), "sample.csv")
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
	s, err := NewClient(c, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	base := Prefix + "/files/" + files[0].ID.String()
	for _, kind := range []string{"text-preview", "sheet-preview"} {
		missing := call(s, "GET", base+"/"+kind, "", "")
		if missing.Code != 503 || !strings.Contains(missing.Body.String(), `"code":"not_cached"`) {
			t.Fatal(missing.Code, missing.Body)
		}
	}
	if err := c.CacheOriginal(ctx, files[0].BlobKey, previewOriginalSource{nas, data}); err != nil {
		t.Fatal(err)
	}
	text := decodeBody[textpreview.Preview](t, s, strings.TrimPrefix(base, Prefix)+"/text-preview?detail=true")
	if !text.Supported || text.Text != data {
		t.Fatal(text)
	}
	sheet := decodeBody[sheetpreview.SheetPreview](t, s, strings.TrimPrefix(base, Prefix)+"/sheet-preview?detail=true")
	if !sheet.Supported || sheet.Cells[1][0].Text != "中文" || sheet.Cells[1][1].Text != "12" {
		t.Fatal(sheet)
	}
	bad := call(s, "GET", base+"/sheet-preview?row=-1", "", "")
	if bad.Code != 422 {
		t.Fatal(bad.Code, bad.Body)
	}
}
