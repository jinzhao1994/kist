package api

import (
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

func TestClientBoxReadRoutesReflectPendingCatalog(t *testing.T) {
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
	box, err := c.CreateBox(ctx, "Trip", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	tag, err := c.CreateTag(ctx, "Travel", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.TagBoxes(ctx, tag.ID, []domain.BoxID{box.ID}); err != nil {
		t.Fatal(err)
	}
	s, err := NewClient(c, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	prefix := "/boxes/" + box.ID.String()
	folders := decodeBody[folderList](t, s, prefix+"/folders")
	if len(folders.Folders) != 1 {
		t.Fatalf("folders=%+v", folders)
	}
	tags := decodeBody[tagList](t, s, prefix+"/tags")
	if len(tags.Tags) != 1 || tags.Tags[0] != tag.ID.String() {
		t.Fatalf("tags=%+v", tags)
	}
	cover := decodeBody[boxCover](t, s, prefix+"/cover")
	if cover.File != nil {
		t.Fatalf("cover=%+v", cover)
	}
	for _, suffix := range []string{"folders", "tags", "cover"} {
		result := call(s, "GET", Prefix+"/boxes/"+domain.NewBoxID().String()+"/"+suffix, "", "")
		if result.Code != 404 {
			t.Fatalf("%s: %d %s", suffix, result.Code, result.Body)
		}
	}
}
