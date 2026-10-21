package library

import (
	"errors"
	"reflect"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

func TestClientCatalogSearchIncludesUncachedFilesAndLocalAnnotations(t *testing.T) {
	nas, box := openWithBox(t)
	file := importOneFile(t, nas, box, "passport.png", "uncached original")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	tag, err := c.CreateTag(ctx, "Travel", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.TagFiles(ctx, tag.ID, []domain.FileID{file.ID}); err != nil {
		t.Fatal(err)
	}
	files, _, err := c.Files(ctx, query.Query{Tags: query.TagAll{query.TagAny{tag.ID}}})
	if err != nil || len(files) != 1 || files[0].ID != file.ID {
		t.Fatalf("search=%+v %v", files, err)
	}
	if _, err := c.OpenOriginal(ctx, file.BlobKey); !errors.Is(err, ErrNotCached) {
		t.Fatalf("search required content: %v", err)
	}
	got, found, err := c.File(ctx, file.ID)
	if err != nil || !found || got.Name != file.Name {
		t.Fatalf("file=%+v %t %v", got, found, err)
	}
	tags, err := c.FileTags(ctx, file.ID)
	if err != nil || len(tags) != 1 || tags[0] != tag.ID {
		t.Fatalf("tags=%+v %v", tags, err)
	}
	media, err := c.MediaInfo(ctx, file.ID)
	nasMedia, nasErr := nas.MediaInfo(ctx, file.ID)
	if err != nil || nasErr != nil || !reflect.DeepEqual(media, nasMedia) {
		t.Fatalf("media=%+v NAS=%+v errors=%v/%v", media, nasMedia, err, nasErr)
	}
	captured, err := c.CapturedInfo(ctx, got)
	nasCaptured, nasErr := nas.CapturedInfo(ctx, got)
	if err != nil || nasErr != nil || !reflect.DeepEqual(captured, nasCaptured) {
		t.Fatalf("capture=%+v NAS=%+v errors=%v/%v", captured, nasCaptured, err, nasErr)
	}
	rows, err := c.FileMetadata(ctx, file.ID)
	if err != nil || len(rows) == 0 {
		t.Fatalf("metadata=%+v %v", rows, err)
	}
	located, name, err := c.FileLocation(ctx, file)
	if err != nil || located != box || name != file.Name {
		t.Fatalf("location=%s %s %v", located, name, err)
	}
	folders, err := c.Folders(ctx, box)
	if err != nil || len(folders) != 1 {
		t.Fatalf("folders=%+v %v", folders, err)
	}
	tree, counts, err := c.TagTreeCounts(ctx)
	if err != nil || len(tree.Roots()) != 1 || len(counts) != 1 {
		t.Fatalf("tree=%+v counts=%+v %v", tree, counts, err)
	}
}
