package library

import (
	"errors"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

// spec 1.5.1: updated_at means "something happened under here", and it
// propagates up the folder chain to the box. Importing into 云南/day1/raw must
// bump raw, day1, the root and the box — otherwise browsing by recent activity
// shows a box that changed but no folder inside it that did.
func TestImportTouchesTheWholeFolderChain(t *testing.T) {
	l, box := openWithBox(t)

	// Create the tree with one import, then note where every folder stands.
	src := writeTree(t, map[string]string{"day1/raw/a.jpg": "x"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	before := map[domain.FolderID]time.Time{}
	folders, err := l.Folders(t.Context(), box)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 3 {
		t.Fatalf("%d folders, want root + day1 + raw", len(folders))
	}
	for _, f := range folders {
		before[f.ID] = f.UpdatedAt
	}

	// A second import into the same leaf must move every folder above it.
	src2 := writeTree(t, map[string]string{"day1/raw/b.jpg": "y"})
	if _, err := l.Import(t.Context(), src2, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}

	folders, err = l.Folders(t.Context(), box)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range folders {
		if !f.UpdatedAt.After(before[f.ID]) {
			t.Errorf("folder %q was not touched: still %s", f.Name, f.UpdatedAt)
		}
	}
}

// Maintenance ops must not propagate (spec 1.5.1 ②) — the same rule that
// already covers boxes has to cover folders, or a relink would look like the
// user did something.
func TestMaintenanceOpsDoNotTouchFolders(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"day1/a.jpg": "x"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	folders, _ := l.Folders(t.Context(), box)
	before := map[domain.FolderID]time.Time{}
	for _, f := range folders {
		before[f.ID] = f.UpdatedAt
	}

	// Replay the same library. Rebuild uses the recorded source, so a
	// user-sourced op keeps its own ts rather than picking up the moment of
	// the rebuild.
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	folders, _ = l.Folders(t.Context(), box)
	for _, f := range folders {
		if !f.UpdatedAt.Equal(before[f.ID]) {
			t.Errorf("folder %q moved from %s to %s across a rebuild", f.Name, before[f.ID], f.UpdatedAt)
		}
	}
}

func importOneFile(t *testing.T, l *Library, box domain.BoxID, name, content string) domain.File {
	t.Helper()
	src := writeTree(t, map[string]string{name: content})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	files := listAll(t, l, box)
	return files[0]
}

// Only the files that actually change go into the op. A file that already
// carries the tag must not be listed, or the inverse would strip a tag this op
// never added.
func TestTagFilesSkipsFilesThatAlreadyHaveIt(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.jpg": "x", "b.jpg": "y"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	files := listAll(t, l, box)
	tag, err := l.CreateTag(t.Context(), "云南", nil)
	if err != nil {
		t.Fatal(err)
	}

	n, err := l.TagFiles(t.Context(), tag.ID, []domain.FileID{files[0].ID})
	if err != nil || n != 1 {
		t.Fatalf("first: n=%d err=%v", n, err)
	}
	n, err = l.TagFiles(t.Context(), tag.ID, []domain.FileID{files[0].ID, files[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("n = %d, want 1 — the already-tagged file must not be in the op", n)
	}
}

// Tagging nothing is not an error and not an op: an op with no targets would
// be an undo that does nothing, cluttering the history for no reason.
func TestTagFilesWritesNoOpWhenNothingChanges(t *testing.T) {
	l, box := openWithBox(t)
	f := importOneFile(t, l, box, "a.jpg", "x")
	tag, _ := l.CreateTag(t.Context(), "云南", nil)
	if _, err := l.TagFiles(t.Context(), tag.ID, []domain.FileID{f.ID}); err != nil {
		t.Fatal(err)
	}

	before, _ := l.Ops(t.Context(), ListOpsOptions{})
	n, err := l.TagFiles(t.Context(), tag.ID, []domain.FileID{f.ID})
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	after, _ := l.Ops(t.Context(), ListOpsOptions{})
	if len(after) != len(before) {
		t.Errorf("a no-op tag wrote an op: %d then %d", len(before), len(after))
	}
}

// spec 1.6: the in-memory tree caches per-tag counts, and library keeps them
// current as ops commit. A stale count is invisible until someone notices the
// number in the tag picker is wrong.
func TestTagFilesUpdatesTheTreeCounts(t *testing.T) {
	l, box := openWithBox(t)
	f := importOneFile(t, l, box, "a.jpg", "x")
	tag, _ := l.CreateTag(t.Context(), "云南", nil)

	if _, err := l.TagFiles(t.Context(), tag.ID, []domain.FileID{f.ID}); err != nil {
		t.Fatal(err)
	}
	if got := tagCount(t, l, tag.ID); got != 1 {
		t.Errorf("count = %d, want 1", got)
	}
	if _, err := l.UntagFiles(t.Context(), tag.ID, []domain.FileID{f.ID}); err != nil {
		t.Fatal(err)
	}
	if got := tagCount(t, l, tag.ID); got != 0 {
		t.Errorf("count after untagging = %d, want 0", got)
	}

	// And the cached count must agree with the database it mirrors.
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := tagCount(t, l, tag.ID); got != 0 {
		t.Errorf("count after a rebuild = %d, want 0", got)
	}
}

func TestTagFilesRefusesAnUnknownFile(t *testing.T) {
	l, _ := openWithBox(t)
	tag, _ := l.CreateTag(t.Context(), "云南", nil)
	_, err := l.TagFiles(t.Context(), tag.ID, []domain.FileID{domain.NewFileID()})
	if !errors.Is(err, ErrNoSuchFile) {
		t.Errorf("err = %v, want ErrNoSuchFile", err)
	}
}

func TestFileTagsRoundTrip(t *testing.T) {
	l, box := openWithBox(t)
	f := importOneFile(t, l, box, "a.jpg", "x")
	tag, _ := l.CreateTag(t.Context(), "云南", nil)
	if _, err := l.TagFiles(t.Context(), tag.ID, []domain.FileID{f.ID}); err != nil {
		t.Fatal(err)
	}

	got, err := l.FileTags(t.Context(), f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != tag.ID {
		t.Errorf("FileTags = %v, want [%v]", got, tag.ID)
	}
}

func TestResolveFolderWalksAPath(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"Day1/raw/a.jpg": "x"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}

	root, err := l.ResolveFolder(t.Context(), box, "")
	if err != nil || root.ParentID != nil {
		t.Fatalf("the empty path is not the root: %+v, %v", root, err)
	}

	// Folded matching, the same rule the uniqueness constraint uses.
	leaf, err := l.ResolveFolder(t.Context(), box, "day1/RAW")
	if err != nil {
		t.Fatalf("ResolveFolder: %v", err)
	}
	if leaf.Name != "raw" {
		t.Errorf("resolved to %q, want raw", leaf.Name)
	}

	if _, err := l.ResolveFolder(t.Context(), box, "day1/nope"); !errors.Is(err, ErrNoSuchFolder) {
		t.Errorf("err = %v, want ErrNoSuchFolder", err)
	}
}

// spec 4.3 step 1: a tag condition includes the tag's descendants, resolved
// against the in-memory tree.
func TestTagConditionExpandsDescendants(t *testing.T) {
	l, box := openWithBox(t)
	people, _ := l.CreateTag(t.Context(), "人物", nil)
	dad, _ := l.CreateTag(t.Context(), "爸", &people.ID)

	f := importOneFile(t, l, box, "a.jpg", "x")
	if _, err := l.TagFiles(t.Context(), dad.ID, []domain.FileID{f.ID}); err != nil {
		t.Fatal(err)
	}

	conds, err := l.TagCondition([]string{"人物"})
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := l.Files(t.Context(), query.Query{Tags: conds})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != f.ID {
		t.Errorf("searching for the parent tag found %v, want the file tagged with its child", fileNames(got))
	}
}
