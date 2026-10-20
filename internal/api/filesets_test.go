package api

import (
	"encoding/json"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestFilesetHTTPConfigurationAndSearch(t *testing.T) {
	s, l := testServer(t)
	ctx := t.Context()
	box, err := l.CreateBox(ctx, "box", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err = os.Mkdir(filepath.Join(root, "episode"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"movie.mkv": "movie", "sub.srt": "subtitle"} {
		if err = os.WriteFile(filepath.Join(root, "episode", name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = l.Import(ctx, root, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	folders, _ := l.Folders(ctx, box.ID)
	var folder domain.FolderID
	for _, f := range folders {
		if f.Name == "episode" {
			folder = f.ID
		}
	}
	files := decodeBody[fileSearchResult](t, s, "/files")
	var video, sub domain.FileID
	for _, f := range files.Files {
		id, _ := domain.ParseID(f.ID)
		if f.Name == "movie.mkv" {
			video = domain.FileID{ID: id}
		} else {
			sub = domain.FileID{ID: id}
		}
	}
	set := domain.Fileset{FolderID: folder, Kind: "video_subtitles", Layout: "flat", Members: []domain.FilesetMember{{FileID: video, Role: "video"}, {FileID: sub, Role: "subtitle"}}}
	body, _ := json.Marshal(map[string]any{"box": box.ID.String(), "after": set})
	mustWrite(t, s, "PUT", "/filesets/"+folder.String(), "fileset-create-001", string(body))
	detail := decodeBody[filesetDetail](t, s, "/filesets/"+folder.String())
	if detail.BoxID != box.ID.String() || detail.Folder.ID != folder.String() || detail.Folder.Name != "episode" || detail.Folder.TotalSize != 13 || !domain.EqualFileset(&detail.Fileset, &set) {
		t.Fatalf("unexpected combination detail: %+v", detail)
	}
	history := decodeBody[historyList](t, s, "/ops?fileset="+folder.String())
	if len(history.Ops) != 1 || history.Ops[0].Action != "set_fileset" {
		t.Fatal(history)
	}
	entries := decodeBody[fileSearchResult](t, s, "/entries?name_equals=episode")
	if len(entries.Files) != 1 || entries.Files[0].FilesetID != folder.String() || entries.Files[0].ID != video.String() {
		t.Fatal(entries)
	}
	hidden := decodeBody[fileSearchResult](t, s, "/entries?keyword=sub.srt")
	if len(hidden.Files) != 0 {
		t.Fatal(hidden)
	}
	visible := decodeBody[fileSearchResult](t, s, "/entries?include_members=true&keyword="+url.QueryEscape("sub.srt"))
	if len(visible.Files) != 1 || visible.Files[0].ID != sub.String() {
		t.Fatal(visible)
	}
	if got := decodeBody[fileSearchResult](t, s, "/files"); len(got.Files) != 2 {
		t.Fatal("legacy files changed")
	}
	body, _ = json.Marshal(map[string]any{"box": box.ID.String(), "before": set})
	mustWrite(t, s, "PUT", "/filesets/"+folder.String(), "fileset-remove-001", string(body))
	if got := decodeBody[filesetList](t, s, "/filesets"); len(got.Filesets) != 0 {
		t.Fatal(got)
	}
	if got := call(s, "GET", Prefix+"/filesets/"+folder.String(), "", ""); got.Code != 404 {
		t.Fatalf("removed configuration still has a detail: %d %s", got.Code, got.Body)
	}
}
