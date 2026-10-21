package library

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

func TestClientOfflineOriginalsIncludesNestedMediaAndIndependentPrimary(t *testing.T) {
	nas, box := openWithBox(t)
	ctx := t.Context()
	_, err := nas.Import(ctx, writeTree(t, map[string]string{
		"document/cover.jpg": "cover", "document/unused.txt": "excluded",
		"document/pages/live/photo.jpg": "photo", "document/pages/live/motion.mov": "motion",
		"movie/video.mp4": "video", "movie/en.srt": "subtitle", "movie/zh.srt": "subtitle",
	}), ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	folders, err := nas.Folders(ctx, box)
	if err != nil {
		t.Fatal(err)
	}
	folder := map[string]domain.FolderID{}
	for _, f := range folders {
		folder[f.Name] = f.ID
	}
	all, _, err := nas.Files(ctx, query.Query{})
	if err != nil {
		t.Fatal(err)
	}
	file := map[string]domain.FileID{}
	for _, f := range all {
		file[f.Name] = f.ID
	}
	sets := []domain.Fileset{
		{FolderID: folder["live"], Kind: "live_photo", Layout: "flat", Members: []domain.FilesetMember{{FileID: file["photo.jpg"], Role: "image"}, {FileID: file["motion.mov"], Role: "video"}}},
		{FolderID: folder["pages"], Kind: "document_pages", Layout: "flat", Members: []domain.FilesetMember{{FolderID: folder["live"], Role: "page"}}},
		{FolderID: folder["document"], Kind: "document_pages", Layout: "flat", Primary: file["cover.jpg"], Members: []domain.FilesetMember{{FolderID: folder["pages"], Role: "page"}}},
		{FolderID: folder["movie"], Kind: "video_subtitles", Layout: "flat", Members: []domain.FilesetMember{{FileID: file["video.mp4"], Role: "video"}, {FileID: file["en.srt"], Role: "subtitle"}, {FileID: file["zh.srt"], Role: "subtitle"}}},
	}
	for _, s := range sets {
		if _, err := nas.SetFileset(ctx, box, s.FolderID, nil, &s); err != nil {
			t.Fatal(err)
		}
	}
	c := newSyncClient(t, nas)
	selection := ClientOfflineSelection{Files: []domain.FileID{file["photo.jpg"]}, Filesets: []domain.FolderID{folder["document"], folder["movie"], folder["live"]}}
	originals, err := c.OfflineOriginals(ctx, selection)
	if err != nil {
		t.Fatal(err)
	}
	if len(originals) != 5 {
		t.Fatalf("expected five unique contents: %+v", originals)
	}
	seen := map[domain.FileID]int{}
	for _, o := range originals {
		for _, f := range o.Files {
			seen[f]++
		}
	}
	for name, id := range file {
		want := 1
		if name == "unused.txt" {
			want = 0
		}
		if seen[id] != want {
			t.Fatalf("%s count=%d want=%d", name, seen[id], want)
		}
	}
	// A catalog update changes dependencies even when the logical selection is
	// unchanged. A previously resolved key list is never an enduring readiness fact.
	before := sets[0]
	after := before
	after.Members = after.Members[:1]
	if _, err := nas.SetFileset(ctx, box, after.FolderID, &before, &after); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	originals, err = c.OfflineOriginals(ctx, selection)
	if err != nil || len(originals) != 4 {
		t.Fatalf("changed dependencies=%+v %v", originals, err)
	}
	if _, err := c.OfflineOriginals(ctx, ClientOfflineSelection{Files: []domain.FileID{{}}}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("zero file=%v", err)
	}
}

func TestOfflineSelectionRejectsCycleAndMissingFileset(t *testing.T) {
	nas, box := openWithBox(t)
	folders, err := nas.Folders(t.Context(), box)
	if err != nil {
		t.Fatal(err)
	}
	id := folders[0].ID
	selection := ClientOfflineSelection{Filesets: []domain.FolderID{id}}
	_, err = offlineSelectionFiles(selection, []domain.Fileset{{FolderID: id, Kind: "document_pages", Layout: "flat", Members: []domain.FilesetMember{{FolderID: id, Role: "page"}}}})
	if !errors.Is(err, ErrInvariantViolated) {
		t.Fatalf("cycle=%v", err)
	}
	if _, err := offlineSelectionFiles(selection, nil); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("missing=%v", err)
	}
}
