package api

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func TestTagResultsCountDistinctEntities(t *testing.T) {
	s, l := testServer(t)
	ctx := t.Context()
	box, err := l.CreateBox(ctx, "tagged", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	empty, err := l.CreateBox(ctx, "empty", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	parent, err := l.CreateTag(ctx, "parent", nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := l.CreateTag(ctx, "child", &parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := l.CreateTag(ctx, "sibling", &parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	for _, name := range []string{"tagged.txt", "untagged.txt"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := l.Import(ctx, src, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files := decodeBody[wire.FileList](t, s, "/files?sort=name&asc=true")
	for i, tag := range []domain.Tag{parent, child, sibling} {
		mustWrite(t, s, "PUT", "/tags/"+tag.ID.String()+"/boxes", fmt.Sprintf("tag-box-%d", i), `{"ids":["`+box.ID.String()+`"]}`)
		mustWrite(t, s, "PUT", "/tags/"+tag.ID.String()+"/files", fmt.Sprintf("tag-file-%d", i), `{"ids":["`+files.Files[0].ID+`"]}`)
	}
	if _, err := l.TagBoxes(ctx, child.ID, []domain.BoxID{empty.ID}); err != nil {
		t.Fatal(err)
	}
	assert := func() {
		t.Helper()
		tree := decodeBody[wire.TagTree](t, s, "/tags")
		p := tree.Tags[0]
		if p.Count == nil || *p.Count != 3 || p.BoxCount == nil || *p.BoxCount != 2 || p.FileCount == nil || *p.FileCount != 1 {
			t.Fatalf("counts: %+v", p)
		}
		boxes := decodeBody[wire.BoxList](t, s, "/boxes?tag="+parent.ID.String())
		if len(boxes.Boxes) != 2 {
			t.Fatalf("boxes: %+v", boxes)
		}
		f := decodeBody[wire.FileList](t, s, "/files?tag_all="+parent.ID.String())
		if len(f.Files) != 1 || f.Files[0].ID != files.Files[0].ID {
			t.Fatalf("files: %+v", f)
		}
	}
	assert()
	op, err := l.OpCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.ViewAt(ctx, op, func(v *library.HistoricalView) error {
		_, counts, err := v.TagTreeCounts(ctx)
		if err == nil && counts[parent.ID] != (library.TagCounts{Boxes: 2, Files: 1}) {
			t.Fatal(counts)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := l.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	assert()
	for _, path := range []string{"/boxes?tag=bad", "/boxes?tag=00000000-0000-0000-0000-000000000001"} {
		w := call(s, "GET", Prefix+path, "", "")
		if w.Code < 400 || w.Code >= 500 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
}
