package api

import (
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
	"os"
	"path/filepath"
	"testing"
)

func TestBoxCoverAPI(t *testing.T) {
	s, l := testServer(t)
	ctx := t.Context()
	b, err := l.CreateBox(ctx, "cover", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "cover.txt"), []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Import(ctx, src, library.ImportOptions{Box: b.ID}); err != nil {
		t.Fatal(err)
	}
	files := decodeBody[wire.FileList](t, s, "/files")
	f := files.Files[0]
	path := "/boxes/" + b.ID.String() + "/cover"
	body := `{"file_id":"` + f.ID + `"}`
	mustWrite(t, s, "PUT", path, "set-cover-api-1", body)
	mustWrite(t, s, "PUT", path, "set-cover-api-1", body)
	got := decodeBody[boxCover](t, s, path)
	if got.File == nil || got.File.ID != f.ID || got.File.BlobKey != f.BlobKey {
		t.Fatal(got)
	}
	boxes := decodeBody[wire.BoxList](t, s, "/boxes")
	if boxes.Boxes[0].CoverFileID != f.ID {
		t.Fatal(boxes)
	}
	// Content replacement resolves the new blob through the stable file ID.
	if err := os.WriteFile(filepath.Join(src, "cover.txt"), []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Import(ctx, src, library.ImportOptions{Box: b.ID, ConflictMode: "overwrite"}); err != nil {
		t.Fatal(err)
	}
	got = decodeBody[boxCover](t, s, path)
	if got.File == nil || got.File.ID != f.ID || got.File.BlobKey == f.BlobKey {
		t.Fatal(got)
	}
	mustWrite(t, s, "PUT", path, "remove-cover-api-1", `{"file_id":""}`)
	if got := decodeBody[boxCover](t, s, path); got.File != nil {
		t.Fatal(got)
	}
	// Removing a cover never removes its source.
	if got := decodeBody[wire.FileList](t, s, "/files"); len(got.Files) != 1 {
		t.Fatal(got)
	}
}
