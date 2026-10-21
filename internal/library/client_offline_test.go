package library

import (
	"errors"
	"testing"
	"time"

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

func TestClientOfflineSelectionPersistsAndProtectsOverlappingOwners(t *testing.T) {
	nas, box := openWithBox(t)
	f := importOneFile(t, nas, box, "kept.txt", "keep this content")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	selection := ClientOfflineSelection{Files: []domain.FileID{f.ID}}
	for _, owner := range []string{"trip", "favorites"} {
		if err := c.SetOfflineSelection(ctx, owner, selection); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.CacheOriginal(ctx, f.BlobKey, &cacheTestSource{nas: nas, content: "keep this content"}); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenClient(ctx, c.root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	selections, err := reopened.OfflineSelections(ctx)
	if err != nil || len(selections) != 2 || len(selections["trip"].Files) != 1 || selections["trip"].Files[0] != f.ID {
		t.Fatalf("selection lost: %+v %v", selections, err)
	}
	if err := reopened.SetOfflineSelection(ctx, "trip", ClientOfflineSelection{Files: []domain.FileID{{}}}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("invalid replacement=%v", err)
	}
	selections, err = reopened.OfflineSelections(ctx)
	if err != nil || len(selections) != 2 {
		t.Fatalf("failed replacement lost intent: %+v %v", selections, err)
	}
	if err := reopened.SetOfflineSelection(ctx, "trip", ClientOfflineSelection{}); err != nil {
		t.Fatal(err)
	}
	status, err := reopened.SetCacheLimits(ctx, time.Hour, 1)
	if err != nil || !status.OverLimit || status.PinnedBytes == 0 {
		t.Fatalf("overlapping pin lost: %+v %v", status, err)
	}
	if err := reopened.SetOfflineSelection(ctx, "favorites", ClientOfflineSelection{}); err != nil {
		t.Fatal(err)
	}
	status, err = reopened.PruneCache(ctx)
	if err != nil || status.CachedBytes != 0 {
		t.Fatalf("unselected content not reclaimable: %+v %v", status, err)
	}
}

func TestClientCachePruningRefreshesChangedFilesetRetention(t *testing.T) {
	nas, box := openWithBox(t)
	ctx := t.Context()
	if _, err := nas.Import(ctx, writeTree(t, map[string]string{"pages/old.txt": "old", "pages/new.txt": "newer"}), ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	folders, err := nas.Folders(ctx, box)
	if err != nil {
		t.Fatal(err)
	}
	var folder domain.FolderID
	for _, f := range folders {
		if f.Name == "pages" {
			folder = f.ID
		}
	}
	files, _, err := nas.Files(ctx, query.Query{})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]domain.File{}
	for _, f := range files {
		byName[f.Name] = f
	}
	before := domain.Fileset{FolderID: folder, Kind: "document_pages", Layout: "flat", Members: []domain.FilesetMember{{FileID: byName["old.txt"].ID, Role: "page"}}}
	if _, err := nas.SetFileset(ctx, box, folder, nil, &before); err != nil {
		t.Fatal(err)
	}
	c := newSyncClient(t, nas)
	for name, content := range map[string]string{"old.txt": "old", "new.txt": "newer"} {
		if err := c.CacheOriginal(ctx, byName[name].BlobKey, &cacheTestSource{nas: nas, content: content}); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.SetOfflineSelection(ctx, "pages", ClientOfflineSelection{Filesets: []domain.FolderID{folder}}); err != nil {
		t.Fatal(err)
	}
	after := before
	after.Members = []domain.FilesetMember{{FileID: byName["new.txt"].ID, Role: "page"}}
	if _, err := nas.SetFileset(ctx, box, folder, &before, &after); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	// Do not call prepare/status: eviction itself must refresh the retention.
	usage, err := c.SetCacheLimits(ctx, time.Hour, 5)
	if err != nil || usage.CachedBytes != 5 || usage.PinnedBytes != 5 || usage.OverLimit {
		t.Fatalf("stale retention=%+v %v", usage, err)
	}
	reader, err := c.OpenCachedOriginal(ctx, byName["new.txt"].BlobKey)
	if err != nil {
		t.Fatal(err)
	}
	reader.Close()
	if _, err := c.OpenCachedOriginal(ctx, byName["old.txt"].BlobKey); !errors.Is(err, ErrNotCached) {
		t.Fatalf("old content not released: %v", err)
	}
}
