package api

import (
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/sheetpreview"
	"github.com/jinzhao1994/kist/internal/wire"
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
	url := "/files/" + files.Files[0].ID + "/sheet-preview"
	p := decodeBody[sheetpreview.SheetPreview](t, s, url+"?detail=true")
	if !p.Supported || p.Cells[1][0].Text != "中文" {
		t.Fatal(p)
	}
	for _, query := range []string{"row=-1", "column=16384", "sheet=256", "encoding=invalid"} {
		if got := call(s, "GET", Prefix+url+"?"+query, "", ""); got.Code != 422 {
			t.Fatalf("%s: %d", query, got.Code)
		}
	}
}
