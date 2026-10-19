package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func TestBoxAndFolderResponsesExposeLogicalSize(t *testing.T) {
	s, l := testServer(t)
	b, err := l.CreateBox(t.Context(), "sizes", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	if err = os.Mkdir(filepath.Join(src, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(src, "sub", "f.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = l.Import(t.Context(), src, library.ImportOptions{Box: b.ID}); err != nil {
		t.Fatal(err)
	}
	box := decodeBody[wire.Box](t, s, "/boxes/"+b.ID.String())
	if box.TotalSize != 5 {
		t.Fatalf("box: %+v", box)
	}
	folders := decodeBody[wire.FolderList](t, s, "/boxes/"+b.ID.String()+"/folders")
	if len(folders.Folders) != 2 {
		t.Fatal(folders)
	}
	for _, f := range folders.Folders {
		if f.TotalSize != 5 {
			t.Fatal(f)
		}
	}
}
