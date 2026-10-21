package library

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

func TestClientSearchReadsPendingLabelsWithoutContent(t *testing.T) {
	nas, box := openWithBox(t)
	file := importOneFile(t, nas, box, "cover.png", "not downloaded")
	ctx := t.Context()
	if err := nas.SetBoxCover(ctx, box, file.ID); err != nil {
		t.Fatal(err)
	}
	c := newSyncClient(t, nas)
	tag, err := c.CreateTag(ctx, "Trip", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.TagFiles(ctx, tag.ID, []domain.FileID{file.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.TagBoxes(ctx, tag.ID, []domain.BoxID{box}); err != nil {
		t.Fatal(err)
	}
	files, _, err := c.FilesWithTags(ctx, query.Query{Tags: query.TagAny{tag.ID}})
	if err != nil || len(files) != 1 || len(files[0].Tags) != 1 || files[0].Tags[0].Name != "Trip" {
		t.Fatalf("files=%+v error=%v", files, err)
	}
	boxes, _, err := c.SearchBoxes(ctx, query.BoxQuery{Tags: query.TagAny{tag.ID}})
	if err != nil || len(boxes) != 1 || len(boxes[0].Tags) != 1 || boxes[0].Tags[0].Name != "Trip" {
		t.Fatalf("boxes=%+v error=%v", boxes, err)
	}
	if err := c.RenameTag(ctx, tag.ID, "Journey"); err != nil {
		t.Fatal(err)
	}
	files, _, err = c.FilesWithTags(ctx, query.Query{Tags: query.TagAny{tag.ID}})
	if err != nil || len(files) != 1 || len(files[0].Tags) != 1 || files[0].Tags[0].Name != "Journey" {
		t.Fatalf("renamed=%+v error=%v", files, err)
	}
	cover, found, err := c.BoxCover(ctx, box)
	if err != nil || !found || cover.ID != file.ID {
		t.Fatalf("cover=%+v found=%v error=%v", cover, found, err)
	}
	if _, err := c.OpenOriginal(ctx, cover.BlobKey); !errors.Is(err, ErrNotCached) {
		t.Fatalf("content=%v", err)
	}
	if _, _, err := c.BoxCover(ctx, domain.NewBoxID()); !errors.Is(err, ErrNoSuchBox) {
		t.Fatalf("unknown box=%v", err)
	}
}
