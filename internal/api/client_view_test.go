package api

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
)

var _ fileReader = (*library.ClientView)(nil)

func TestClientViewUsesExistingFileDetailRendererWithoutOriginal(t *testing.T) {
	_, nas := testServer(t)
	ctx := t.Context()
	box, err := nas.CreateBox(ctx, "Travel", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "notes.txt"), []byte("offline detail"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.Import(ctx, source, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files, _, err := nas.Files(ctx, query.Query{})
	if err != nil || len(files) != 1 {
		t.Fatalf("files=%+v error=%v", files, err)
	}
	head, err := nas.SyncHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	client, err := library.CreateClient(ctx, t.TempDir(), library.ClientSettings{LibraryID: head.LibraryID, Timezone: nas.Zone().String()})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var after int64
	for {
		page, err := nas.SyncCatalog(ctx, head, after, 2, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.ReceiveCatalogPage(ctx, page); err != nil {
			t.Fatal(err)
		}
		if page.Done {
			break
		}
		after = page.Next
	}
	expected, err := renderFileDetail(ctx, nas, files[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := client.View(ctx, func(view *library.ClientView) error {
		file, found, err := view.File(ctx, files[0].ID)
		if err != nil {
			return err
		}
		if !found {
			t.Fatal("uncached file disappeared")
		}
		actual, err := renderFileDetail(ctx, view, file)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("client=%+v NAS=%+v", actual, expected)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	server, err := NewClient(client, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	tag, err := client.CreateTag(ctx, "Offline", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.TagFiles(ctx, tag.ID, []domain.FileID{files[0].ID}); err != nil {
		t.Fatal(err)
	}
	result := decodeBody[fileSearchResult](t, server, "/files?tag_all="+tag.ID.String())
	if len(result.Files) != 1 || len(result.Files[0].DisplayTags) != 1 || result.Files[0].DisplayTags[0].Name != "Offline" {
		t.Fatalf("search=%+v", result)
	}
	result = decodeBody[fileSearchResult](t, server, "/files?"+encodedCondition("tag_not", tag.ID.String()))
	if len(result.Files) != 0 {
		t.Fatalf("excluded search=%+v", result)
	}
	detail := decodeBody[fileDetail](t, server, "/files/"+files[0].ID.String())
	if len(detail.Tags) != 1 || detail.Tags[0] != tag.ID.String() {
		t.Fatalf("detail=%+v", detail)
	}
	if _, err := client.OpenOriginal(ctx, files[0].BlobKey); !errors.Is(err, library.ErrNotCached) {
		t.Fatalf("original=%v", err)
	}
}
