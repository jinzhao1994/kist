package library

import (
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
	"reflect"
	"testing"
)

func TestFilesetLifecycleAndAtomicOrder(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	_, err := l.Import(ctx, writeTree(t, map[string]string{"document/a.jpg": "a", "document/b.jpg": "bb", "document/page/c.jpg": "ccc", "document/page/c.mov": "video"}), ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	folders, _ := l.Folders(ctx, box)
	byName := map[string]domain.FolderID{}
	for _, f := range folders {
		byName[f.Name] = f.ID
	}
	files, _, err := l.Files(ctx, query.Query{})
	if err != nil {
		t.Fatal(err)
	}
	byFile := map[string]domain.FileID{}
	for _, f := range files {
		byFile[f.Name] = f.ID
	}
	child := domain.Fileset{FolderID: byName["page"], Kind: "live_photo", Layout: "flat", Members: []domain.FilesetMember{{FileID: byFile["c.jpg"], Role: "image"}, {FileID: byFile["c.mov"], Role: "video"}}}
	if _, err = l.SetFileset(ctx, box, child.FolderID, nil, &child); err != nil {
		t.Fatal(err)
	}
	parent := domain.Fileset{FolderID: byName["document"], Kind: "document_pages", Layout: "flat", Members: []domain.FilesetMember{{FileID: byFile["a.jpg"], Role: "page"}, {FolderID: child.FolderID, Role: "page"}, {FileID: byFile["b.jpg"], Role: "page"}}}
	create, err := l.SetFileset(ctx, box, parent.FolderID, nil, &parent)
	if err != nil {
		t.Fatal(err)
	}
	after := parent
	after.Members = append([]domain.FilesetMember{}, parent.Members...)
	after.Members[0], after.Members[1] = after.Members[1], after.Members[0]
	reorder, err := l.SetFileset(ctx, box, parent.FolderID, &parent, &after)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.SetFileset(ctx, box, parent.FolderID, &parent, &parent); err == nil {
		t.Fatal("stale update succeeded")
	}
	sets, _ := l.Filesets(ctx)
	m := map[domain.FolderID]domain.Fileset{}
	for _, s := range sets {
		m[s.FolderID] = s
	}
	if got := domain.FilesetPrimary(parent.FolderID, m); got != byFile["c.jpg"] {
		t.Fatal(got)
	}
	if _, err = l.Undo(ctx, create); err == nil {
		t.Fatal("undo discarded subsequent order")
	}
	if _, err = l.Undo(ctx, reorder); err != nil {
		t.Fatal(err)
	}
	// A member deletion and its role removal are one undoable operation.
	preview, err := l.PreviewEntryDeletion(ctx, box, byFile["b.jpg"].ID, false)
	if err != nil {
		t.Fatal(err)
	}
	deletion, err := l.DeleteEntry(ctx, box, byFile["b.jpg"].ID, false, preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	state, _ := store.FilesetByID(l.db, parent.FolderID)
	if len(state.Members) != 2 {
		t.Fatal(state)
	}
	if _, err = l.Undo(ctx, deletion); err != nil {
		t.Fatal(err)
	}
	// Whole-subtree trash restores nested configurations and references.
	preview, err = l.PreviewEntryDeletion(ctx, box, parent.FolderID.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	deletion, err = l.DeleteEntry(ctx, box, parent.FolderID.ID, true, preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	sets, _ = l.Filesets(ctx)
	if len(sets) != 0 {
		t.Fatal(sets)
	}
	if _, err = l.Undo(ctx, deletion); err != nil {
		t.Fatal(err)
	}
	before, err := store.Snapshot(l.db)
	if err != nil {
		t.Fatal(err)
	}
	if err = l.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	replayed, err := store.Snapshot(l.db)
	if err != nil || before != replayed {
		t.Fatalf("rebuild drift: %v", err)
	}
	report, err := l.Verify(ctx, VerifyOptions{Deep: true})
	if err != nil || report.Count(integrity.ViewDrift) != 0 {
		t.Fatalf("verify: %+v %v", report, err)
	}
	state, _ = store.FilesetByID(l.db, parent.FolderID)
	if !reflect.DeepEqual(state.Members, parent.Members) {
		t.Fatal(state)
	}
	// Clearing a nested set still referenced by a page must fail before journaling.
	op := lastOpID(t, l)
	if _, err = l.SetFileset(ctx, box, child.FolderID, &child, nil); err == nil {
		t.Fatal("removed referenced set")
	}
	if lastOpID(t, l) != op {
		t.Fatal("invalid operation journaled")
	}
}

func TestFilesetBrowseScopeAndPaging(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	_, err := l.Import(ctx, writeTree(t, map[string]string{"episode/movie.mkv": "video", "episode/en.srt": "sub", "plain.txt": "p"}), ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	folders, _ := l.Folders(ctx, box)
	var setID, root domain.FolderID
	for _, f := range folders {
		if f.Name == "episode" {
			setID = f.ID
		}
		if f.ParentID == nil {
			root = f.ID
		}
	}
	files, _, _ := l.Files(ctx, query.Query{})
	var video, sub domain.FileID
	for _, f := range files {
		if f.Name == "movie.mkv" {
			video = f.ID
		}
		if f.Name == "en.srt" {
			sub = f.ID
		}
	}
	s := domain.Fileset{FolderID: setID, Kind: "video_subtitles", Layout: "flat", Members: []domain.FilesetMember{{FileID: video, Role: "video"}, {FileID: sub, Role: "subtitle", Language: "en"}}}
	if _, err = l.SetFileset(ctx, box, setID, nil, &s); err != nil {
		t.Fatal(err)
	}
	for _, include := range []bool{false, true} {
		for _, key := range []query.SortKey{query.SortName, query.SortCaptured, query.SortImported, query.SortUpdated, query.SortSize, query.SortType} {
			for _, asc := range []bool{false, true} {
				q := query.Query{Limit: 1, Sort: query.SortSpec{Key: key, Asc: asc}}
				seen := map[domain.FileID]bool{}
				for {
					entries, next, err := l.Browse(ctx, q, include)
					if err != nil {
						t.Fatalf("%v %v: %v", key, asc, err)
					}
					for _, e := range entries {
						if seen[e.File.ID] {
							t.Fatal("duplicate page")
						}
						seen[e.File.ID] = true
					}
					if next == "" {
						break
					}
					q.Cursor = next
				}
				want := 2
				if include {
					want = 3
				}
				if len(seen) != want {
					t.Fatalf("include=%v sort=%v: %+v", include, key, seen)
				}
			}
		}
	}
	for _, tc := range []struct {
		q       query.Query
		include bool
		count   int
	}{{query.Query{Keywords: []string{"en.srt"}}, false, 0}, {query.Query{Keywords: []string{"en.srt"}}, true, 1}, {query.Query{NameEquals: "episode"}, false, 1}, {query.Query{Keywords: []string{"movie"}}, false, 1}, {query.Query{Folder: &root}, false, 2}, {query.Query{Folder: &setID}, false, 2}} {
		entries, _, err := l.Browse(ctx, tc.q, tc.include)
		if err != nil || len(entries) != tc.count {
			t.Fatalf("%+v: %+v %v", tc, entries, err)
		}
	}
}

func TestFilesetPrimaryAnnotationsAndEmptySearch(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	if _, err := l.Import(ctx, writeTree(t, map[string]string{"pages/a.jpg": "a", "pages/b.jpg": "b"}), ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	folders, _ := l.Folders(ctx, box)
	var folder domain.FolderID
	for _, f := range folders {
		if f.Name == "pages" {
			folder = f.ID
		}
	}
	files, _, _ := l.Files(ctx, query.Query{Sort: query.SortSpec{Key: query.SortName, Asc: true}})
	tag, err := l.CreateTag(ctx, "member", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.TagFiles(ctx, tag.ID, []domain.FileID{files[1].ID}); err != nil {
		t.Fatal(err)
	}
	condition, err := l.TagCondition([]string{tag.ID.String()})
	if err != nil {
		t.Fatal(err)
	}
	set := domain.Fileset{FolderID: folder, Kind: "document_pages", Layout: "directory", Members: []domain.FilesetMember{{FileID: files[0].ID, Role: "page"}, {FileID: files[1].ID, Role: "page"}}}
	if _, err = l.SetFileset(ctx, box, folder, nil, &set); err != nil {
		t.Fatal(err)
	}
	for _, include := range []bool{false, true} {
		got, _, err := l.Browse(ctx, query.Query{Tags: condition}, include)
		want := 0
		if include {
			want = 1
		}
		if err != nil || len(got) != want {
			t.Fatalf("include=%v: %+v %v", include, got, err)
		}
	}
	next := set
	next.Primary = files[1].ID
	op, err := l.SetFileset(ctx, box, folder, &set, &next)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := l.Browse(ctx, query.Query{Tags: condition}, false)
	if err != nil || len(got) != 1 || got[0].Fileset != folder {
		t.Fatalf("primary tag: %+v %v", got, err)
	}
	preview, err := l.PreviewEntryDeletion(ctx, box, files[1].ID.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.DeleteEntry(ctx, box, files[1].ID.ID, false, preview.Revision); err == nil {
		t.Fatal("deleted explicit primary")
	}
	if _, err = l.Undo(ctx, op); err != nil {
		t.Fatal(err)
	}
	empty := set
	empty.Members = nil
	if _, err = l.SetFileset(ctx, box, folder, &set, &empty); err != nil {
		t.Fatal(err)
	}
	got, _, err = l.Browse(ctx, query.Query{NameEquals: "pages"}, false)
	if err != nil || len(got) != 1 || !got[0].Primary.IsZero() {
		t.Fatalf("empty: %+v %v", got, err)
	}
}
