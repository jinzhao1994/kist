package api

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func decodeBody[T any](t *testing.T, s *Server, path string) T {
	t.Helper()
	w := call(s, "GET", Prefix+path, "", "")
	if w.Code != 200 {
		t.Fatalf("GET %s: %d %s", path, w.Code, w.Body)
	}
	var out T
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func mustWrite(t *testing.T, s *Server, method, path, key, body string) {
	t.Helper()
	w := call(s, method, Prefix+path, key, body)
	if w.Code != 200 {
		t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body)
	}
}
func TestFileQueryAndTagMutations(t *testing.T) {
	s, l := testServer(t)
	box, err := l.CreateBox(t.Context(), "archive", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := l.Import(t.Context(), src, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, s, "POST", "/tags", "tag-parent-01", `{"name":"parent"}`)
	tree := decodeBody[wire.TagTree](t, s, "/tags")
	parent := tree.Tags[0].ID
	mustWrite(t, s, "POST", "/tags", "tag-child-001", `{"name":"child","parent_id":"`+parent+`"}`)
	tree = decodeBody[wire.TagTree](t, s, "/tags")
	child := tree.Tags[0].Children[0].ID
	mustWrite(t, s, "POST", "/tags", "tag-other-001", `{"name":"other"}`)
	tree = decodeBody[wire.TagTree](t, s, "/tags")
	var other string
	for _, tag := range tree.Tags {
		if tag.Name == "other" {
			other = tag.ID
		}
	}
	all := decodeBody[wire.FileList](t, s, "/files?sort=name&asc=true")
	if len(all.Files) != 3 {
		t.Fatal(all)
	}
	for i, f := range all.Files {
		if f.Size == nil || *f.Size != int64(len(f.Name)) {
			t.Fatalf("unexpected file size: %+v", f)
		}
		if f.Name != []string{"a.txt", "b.txt", "c.txt"}[i] || f.BoxID != box.ID.String() || f.Path != f.Name {
			t.Fatal(f)
		}
	}
	attach := "/tags/" + child + "/files"
	body := `{"ids":["` + all.Files[0].ID + `"]}`
	before, _ := l.OpCount(t.Context())
	mustWrite(t, s, "PUT", attach, "attach-file-01", body)
	mustWrite(t, s, "PUT", attach, "attach-file-01", body)
	after, _ := l.OpCount(t.Context())
	if after != before+1 {
		t.Fatalf("retry wrote %d operations", after-before)
	}
	mustWrite(t, s, "PUT", "/tags/"+other+"/files", "attach-file-02", `{"ids":["`+all.Files[1].ID+`"]}`)
	for _, tc := range []struct {
		filter string
		count  int
	}{
		{"tag_all=" + parent, 1}, {"tag_any=" + parent + "," + other, 2}, {"tag_not=" + parent, 2},
		{"tag_any=" + parent + "," + other + "&tag_not=" + other, 1},
		{"tag_all=" + parent + "," + other, 0}, {"name=B", 1}, {"from=1900&to=1901", 0},
		{"box=" + box.ID.String(), 3}, {"box=" + domain.NewBoxID().String(), 0},
	} {
		page := decodeBody[wire.FileList](t, s, "/files?"+tc.filter)
		if len(page.Files) != tc.count {
			t.Fatalf("%s = %d want %d", tc.filter, len(page.Files), tc.count)
		}
	}
	var seen []string
	cursor := ""
	for range 4 {
		page := decodeBody[wire.FileList](t, s, "/files?sort=name&asc=true&limit=1&cursor="+url.QueryEscape(cursor))
		for _, f := range page.Files {
			seen = append(seen, f.ID)
		}
		cursor = page.Cursor
		if cursor == "" {
			break
		}
	}
	if len(seen) != 3 || seen[0] != all.Files[0].ID || seen[1] != all.Files[1].ID || seen[2] != all.Files[2].ID {
		t.Fatal(seen)
	}
	detail := decodeBody[fileDetail](t, s, "/files/"+all.Files[0].ID)
	if len(detail.Tags) != 1 || detail.Tags[0] != child || detail.File.ID != all.Files[0].ID || detail.Metadata == nil || detail.Extracted == nil {
		t.Fatalf("detail=%+v", detail)
	}
	folders := decodeBody[folderList](t, s, "/boxes/"+box.ID.String()+"/folders")
	if len(folders.Folders) != 1 {
		t.Fatal(folders)
	}
	mustWrite(t, s, "PUT", "/tags/"+parent+"/boxes", "attach-box-001", `{"ids":["`+box.ID.String()+`"]}`)
	tags := decodeBody[tagList](t, s, "/boxes/"+box.ID.String()+"/tags")
	if len(tags.Tags) != 1 || tags.Tags[0] != parent {
		t.Fatal(tags)
	}
	mustWrite(t, s, "DELETE", attach, "detach-file-01", body)
	detail = decodeBody[fileDetail](t, s, "/files/"+all.Files[0].ID)
	if len(detail.Tags) != 0 {
		t.Fatal(detail.Tags)
	}
	mustWrite(t, s, "DELETE", "/tags/"+parent+"/boxes", "detach-box-001", `{"ids":["`+box.ID.String()+`"]}`)
	mustWrite(t, s, "PUT", "/tags/"+child+"/parent", "move-child-001", `{"parent_id":null}`)
	mustWrite(t, s, "PUT", "/tags/"+child+"/name", "rename-child-1", `{"name":"renamed"}`)
	mustWrite(t, s, "DELETE", "/tags/"+child, "delete-child-1", "")
	mustWrite(t, s, "DELETE", "/tags/"+child, "delete-child-1", "")
}
func TestInvalidFileQueries(t *testing.T) {
	s, _ := testServer(t)
	for _, path := range []string{"/files?box=bad", "/files?folder=bad", "/files?from=nope", "/files?from=2021&to=2020", "/files?cursor=bad", "/files?limit=1001", "/files?sort=size", "/files/nope"} {
		w := call(s, "GET", Prefix+path, "", "")
		if w.Code != 400 && w.Code != 422 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	for _, path := range []string{"/files/" + domain.NewFileID().String(), "/boxes/" + domain.NewBoxID().String() + "/folders"} {
		w := call(s, "GET", Prefix+path, "", "")
		if w.Code != 404 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
}

func TestAPIReadsOtherProcessWrites(t *testing.T) {
	s, l := testServer(t)
	other, err := library.Open(l.Layout().Root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	box, err := other.CreateBox(t.Context(), "created elsewhere", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	boxes := decodeBody[wire.BoxList](t, s, "/boxes")
	if len(boxes.Boxes) != 1 || boxes.Boxes[0].ID != box.ID.String() {
		t.Fatal(boxes)
	}
	parent, err := other.CreateTag(t.Context(), "parent", nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := other.CreateTag(t.Context(), "child", &parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	tree := decodeBody[wire.TagTree](t, s, "/tags")
	if len(tree.Tags) != 1 || len(tree.Tags[0].Children) != 1 {
		t.Fatal(tree)
	}
	if err := other.MoveTag(t.Context(), child.ID, nil); err != nil {
		t.Fatal(err)
	}
	tree = decodeBody[wire.TagTree](t, s, "/tags")
	if len(tree.Tags) != 2 {
		t.Fatal(tree)
	}
}

func TestTagCycleAndRejectedMutation(t *testing.T) {
	s, l := testServer(t)
	parent, err := l.CreateTag(t.Context(), "parent", nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := l.CreateTag(t.Context(), "child", &parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := l.OpCount(t.Context())
	w := call(s, "PUT", Prefix+"/tags/"+parent.ID.String()+"/parent", "cycle-test-001", `{"parent_id":"`+child.ID.String()+`"}`)
	if w.Code != 409 {
		t.Fatalf("cycle: %d %s", w.Code, w.Body)
	}
	after, _ := l.OpCount(t.Context())
	if after != before {
		t.Fatal("rejected cycle wrote an op")
	}
	// A rejected request does not consume its key.
	mustWrite(t, s, "PUT", "/tags/"+parent.ID.String()+"/parent", "cycle-test-001", `{"parent_id":null}`)
	w = call(s, "PUT", Prefix+"/tags/"+parent.ID.String()+"/files", "invalid-target-1", `{"ids":["`+domain.NewFileID().String()+`"]}`)
	if w.Code != 404 {
		t.Fatalf("missing file: %d %s", w.Code, w.Body)
	}
}

func TestFileUpdatedSortQuery(t *testing.T) {
	s, _ := testServer(t)
	response := call(s, "GET", Prefix+"/files?sort=updated&asc=true", "", "")
	if response.Code != 200 {
		t.Fatalf("updated sort rejected: %d %s", response.Code, response.Body)
	}
	q, err := s.fileQuery(&fileQueryInput{Sort: "updated", Asc: true})
	if err != nil || q.Sort.Key.String() != "updated" || !q.Sort.Asc {
		t.Fatalf("query: %+v %v", q, err)
	}
}
