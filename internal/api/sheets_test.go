package api

import (
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/sheetpreview"
	"github.com/jinzhao1994/kist/internal/wire"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestSheetPreviewRoute(t *testing.T) {
	s, l := testServer(t)
	box, err := l.CreateBox(t.Context(), "sheets", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "sample.csv")
	if err = os.WriteFile(path, []byte("name,value\n中文,12\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = l.Import(t.Context(), path, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files := decodeBody[wire.FileList](t, s, "/files")
	previewPath := "/files/" + files.Files[0].ID + "/sheet-preview"
	p := decodeBody[sheetpreview.SheetPreview](t, s, previewPath+"?detail=true")
	if !p.Supported || p.Cells[1][0].Text != "中文" {
		t.Fatal(p)
	}
	for _, query := range []string{"row=-1", "column=16384", "sheet=256", "encoding=invalid"} {
		if got := call(s, "GET", Prefix+previewPath+"?"+query, "", ""); got.Code != 422 {
			t.Fatalf("%s: %d", query, got.Code)
		}
	}
	version := "sheet-v1:" + files.Files[0].BlobKey + ":sample.csv"
	versioned := Prefix + previewPath + "?v=" + url.QueryEscape(version)
	cached := call(s, "GET", versioned, "", "")
	if cached.Code != 200 || cached.Header().Get("Cache-Control") != "private, max-age=31536000, immutable" {
		t.Fatalf("versioned: %d %v", cached.Code, cached.Header())
	}
	unversioned := call(s, "GET", Prefix+previewPath, "", "")
	if unversioned.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(unversioned.Header())
	}
	id, _ := domain.ParseID(files.Files[0].ID)
	if _, err := l.RenameEntry(t.Context(), box.ID, id, false, "sample.csv", "sample.tsv"); err != nil {
		t.Fatal(err)
	}
	stale := call(s, "GET", versioned, "", "")
	if stale.Code != 409 || stale.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("stale version: %d %v", stale.Code, stale.Header())
	}

}
