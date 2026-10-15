package api

import (
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"testing"
)

func TestEntryAPITrashAndRename(t *testing.T) {
	s, l := testServer(t)
	ctx := t.Context()
	b, err := l.CreateBox(ctx, "entries", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.CreateFolder(ctx, b.ID, b.RootFolderID, "child"); err != nil {
		t.Fatal(err)
	}
	fs, _ := l.Folders(ctx, b.ID)
	var id domain.FolderID
	for _, f := range fs {
		if f.ParentID != nil {
			id = f.ID
		}
	}
	path := "/boxes/" + b.ID.String() + "/entries/" + id.String()
	r := call(s, "PUT", Prefix+path+"/name?folder=true", "rename-entry", `{"before":"child","name":"renamed"}`)
	if r.Code != 200 {
		t.Fatalf("rename: %d %s", r.Code, r.Body)
	}
	p := decodeBody[library.EntryDeletionPreview](t, s, path+"/deletion-preview?folder=true")
	if p.Name != "renamed" || p.FileCount != 0 || p.FolderCount != 0 {
		t.Fatal(p)
	}
	body := fmt.Sprintf(`{"revision":%d}`, p.Revision)
	r = call(s, "DELETE", Prefix+path+"?folder=true", "trash-entry", body)
	if r.Code != 200 {
		t.Fatalf("trash: %d %s", r.Code, r.Body)
	}
	retry := call(s, "DELETE", Prefix+path+"?folder=true", "trash-entry", body)
	if retry.Code != 200 || retry.Body.String() != r.Body.String() {
		t.Fatalf("retry: %d %s", retry.Code, retry.Body)
	}
}
