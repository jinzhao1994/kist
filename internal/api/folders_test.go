package api

import (
	"bytes"
	"github.com/jinzhao1994/kist/internal/domain"
	"testing"
)

func TestCreateFolderRequest(t *testing.T) {
	s, l := testServer(t)
	box, err := l.CreateBox(t.Context(), "archive", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	path := Prefix + "/boxes/" + box.ID.String() + "/folders"
	first := call(s, "POST", path, "new-folder-request", `{"name":"photos"}`)
	retry := call(s, "POST", path, "new-folder-request", `{"name":"photos"}`)
	if first.Code != 200 || retry.Code != 200 || !bytes.Equal(first.Body.Bytes(), retry.Body.Bytes()) {
		t.Fatalf("%d %s; %d %s", first.Code, first.Body, retry.Code, retry.Body)
	}
	folders, _ := l.Folders(t.Context(), box.ID)
	if len(folders) != 2 {
		t.Fatalf("folders: %v", folders)
	}
	duplicate := call(s, "POST", path, "duplicate-folder-request", `{"name":"photos"}`)
	if duplicate.Code != 400 {
		t.Fatalf("duplicate: %d %s", duplicate.Code, duplicate.Body)
	}
	foreign := call(s, "POST", path, "foreign-folder-request", `{"name":"bad","parent":"`+domain.NewFolderID().String()+`"}`)
	if foreign.Code != 409 {
		t.Fatalf("foreign: %d %s", foreign.Code, foreign.Body)
	}
}
