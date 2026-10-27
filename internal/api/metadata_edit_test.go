package api

import (
	"encoding/json"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func TestMetadataEditHTTP(t *testing.T) {
	s, l := testServer(t)
	box, err := l.CreateBox(t.Context(), "synthetic", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	body, media := multipartUpload(t, []uploadFile{{Path: "note.txt"}}, []uploadFixture{{"note.txt", "test"}})
	if w := upload(t, s, box.ID.String(), "meta-upload", body, media); w.Code != 200 {
		t.Fatal(w.Body)
	}
	files := decodeBody[wire.FileList](t, s, "/files")
	f := files.Files[0]
	detail := decodeBody[fileDetail](t, s, "/files/"+f.ID)
	var row wire.Metadata
	for _, r := range detail.Metadata {
		if r.Key == "file:size" {
			row = r
		}
	}
	edit := library.MetadataEdit{BlobKey: f.BlobKey, Key: row.Key, BeforeValue: row.Value, BeforeSource: row.Source, BeforeType: row.Type, Value: "12"}
	raw, _ := json.Marshal(edit)
	path := Prefix + "/files/" + f.ID + "/metadata"
	first := call(s, "PUT", path, "metadata-edit", string(raw))
	if first.Code != 200 {
		t.Fatal(first.Code, first.Body)
	}
	replay := call(s, "PUT", path, "metadata-edit", string(raw))
	if replay.Code != 200 || replay.Body.String() != first.Body.String() {
		t.Fatal(replay.Body)
	}
	stale := call(s, "PUT", path, "metadata-stale", string(raw))
	if stale.Code != 409 {
		t.Fatal(stale.Code, stale.Body)
	}
	edit.BeforeValue = "12"
	edit.BeforeSource = "user"
	edit.Value = "invalid"
	raw, _ = json.Marshal(edit)
	bad := call(s, "PUT", path, "metadata-invalid", string(raw))
	if bad.Code != 400 {
		t.Fatal(bad.Code, bad.Body)
	}
	hintPath := Prefix + "/files/" + f.ID + "/metadata/hints/ocr"
	hint := library.AIHintEdit{BlobKey: f.BlobKey, Value: "Example Person"}
	raw, _ = json.Marshal(hint)
	saved := call(s, "PUT", hintPath, "hint-save", string(raw))
	if saved.Code != 200 {
		t.Fatal(saved.Code, saved.Body)
	}
	again := call(s, "PUT", hintPath, "hint-save", string(raw))
	if again.Body.String() != saved.Body.String() {
		t.Fatal("hint retry changed receipt")
	}
	stale = call(s, "PUT", hintPath, "hint-stale", string(raw))
	if stale.Code != 409 {
		t.Fatal(stale.Code, stale.Body)
	}
	detail = decodeBody[fileDetail](t, s, "/files/"+f.ID)
	found := false
	for _, row := range detail.Metadata {
		if row.Key == "ocr:hint" && row.Value == hint.Value && row.Source == "user" {
			found = true
		}
	}
	if !found {
		t.Fatal("hint missing from detail")
	}

}
