package projection

import (
	"github.com/jinzhao1994/kist/internal/domain"
	"path"
	"reflect"
	"strings"
	"testing"
)

func TestFilesetProjectionGroupsAndCollisions(t *testing.T) {
	root := domain.NewRootFolder(domain.NewFolderID())
	box := domain.Box{ID: domain.NewBoxID(), RootFolderID: root.ID}
	a, _ := domain.NewFolder(domain.NewFolderID(), root.ID, "a")
	ab, _ := domain.NewFolder(domain.NewFolderID(), root.ID, "a_b")
	x, _ := domain.NewFolder(domain.NewFolderID(), a.ID, "b_c")
	y, _ := domain.NewFolder(domain.NewFolderID(), ab.ID, "c")
	var files []domain.File
	var sets []domain.Fileset
	for _, f := range []domain.Folder{x, y} {
		video, _ := domain.NewFile(domain.NewFileID(), f.ID, "movie.mkv", "v")
		sub, _ := domain.NewFile(domain.NewFileID(), f.ID, "captions.srt", "s")
		files = append(files, video, sub)
		sets = append(sets, domain.Fileset{FolderID: f.ID, Kind: "video_subtitles", Layout: "flat", Members: []domain.FilesetMember{{FileID: video.ID, Role: "video"}, {FileID: sub.ID, Role: "subtitle", Language: "zh"}}})
	}
	// Force two logical paths to flatten to identical output prefixes.
	sets = append(sets, domain.Fileset{FolderID: a.ID, Kind: "document_pages", Layout: "flat"}, domain.Fileset{FolderID: ab.ID, Kind: "document_pages", Layout: "flat"})
	a.Name = "a"
	ab.Name = "a.b"
	x.Name = "b.c"
	y.Name = "c"
	folders := []domain.Folder{root, a, ab, x, y}
	out, err := Plan("box", box, folders, files, sets...)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 5 {
		t.Fatal(out)
	}
	used := map[string]bool{}
	for _, e := range out {
		if used[domain.Fold(e.Path)] {
			t.Fatal("duplicate", e.Path)
		}
		used[domain.Fold(e.Path)] = true
	}
	for _, e := range out {
		if path.Ext(e.Path) == ".mkv" {
			subtitle := strings.TrimSuffix(e.Path, ".mkv") + ".zh.srt"
			if !used[domain.Fold(subtitle)] {
				t.Fatalf("lost pairing: %s in %+v", subtitle, out)
			}
		}
	}
	files[0], files[3] = files[3], files[0]
	again, err := Plan("box", box, folders, files, sets...)
	if err != nil || !reflect.DeepEqual(out, again) {
		t.Fatalf("nondeterministic projection: %v", err)
	}
}
