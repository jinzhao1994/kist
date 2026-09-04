package projection

import (
	"slices"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

func paths(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Path
	}
	return out
}

func mustFolder(t *testing.T, id domain.FolderID, parent domain.FolderID, name string) domain.Folder {
	t.Helper()
	f, err := domain.NewFolder(id, parent, name)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func mustFile(t *testing.T, folder domain.FolderID, name, key string) domain.File {
	t.Helper()
	f, err := domain.NewFile(domain.NewFileID(), folder, name, key)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestPlanMirrorsTheFolderTree(t *testing.T) {
	box := mustBox(t, "云南", "2019-08", "")
	root, day1 := domain.NewFolderID(), domain.NewFolderID()
	folders := []domain.Folder{
		domain.NewRootFolder(root),
		mustFolder(t, day1, root, "day1"),
	}
	files := []domain.File{
		mustFile(t, root, "封面.jpg", "sha256:aa"),
		mustFile(t, day1, "IMG_1.jpg", "sha256:bb"),
	}

	got, err := Plan("2019-08_云南", box, folders, files)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"2019-08_云南",
		"2019-08_云南/day1",
		"2019-08_云南/day1/IMG_1.jpg",
		"2019-08_云南/封面.jpg",
	}
	if g := paths(got); !slices.Equal(g, want) {
		t.Errorf("paths = %v, want %v", g, want)
	}
}

// Parents must come before children, because Sync creates in order and mkdir
// of a nested path with no parent fails.
func TestPlanOrdersParentsFirst(t *testing.T) {
	box := mustBox(t, "b", "2019", "")
	root, a, b := domain.NewFolderID(), domain.NewFolderID(), domain.NewFolderID()
	folders := []domain.Folder{
		domain.NewRootFolder(root),
		mustFolder(t, a, root, "a"),
		mustFolder(t, b, a, "b"),
	}
	got, err := Plan("box", box, folders, []domain.File{mustFile(t, b, "deep.txt", "sha256:aa")})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range got {
		if parent := parentOf(e.Path); parent != "" && !seen[parent] {
			t.Errorf("%q comes before its parent %q", e.Path, parent)
		}
		seen[e.Path] = true
	}
}

func parentOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return ""
}

// A name collision that only exists after sanitization has to be resolved
// here, in the plan — not discovered by the filesystem halfway through.
func TestPlanResolvesCollisionsWithinADirectory(t *testing.T) {
	box := mustBox(t, "b", "2019", "")
	root := domain.NewFolderID()
	// "a:b.txt" and "a：b.txt" are two different names in the database and one
	// name on disk.
	files := []domain.File{
		mustFile(t, root, "a:b.txt", "sha256:aa"),
		mustFile(t, root, "a：b.txt", "sha256:bb"),
	}
	got, err := Plan("box", box, []domain.Folder{domain.NewRootFolder(root)}, files)
	if err != nil {
		t.Fatal(err)
	}
	if dup := duplicatePath(got); dup != "" {
		t.Errorf("two entries share the path %q: %v", dup, paths(got))
	}
}

// A folder and a file with the same name are one name on disk too.
func TestPlanResolvesFolderAgainstFile(t *testing.T) {
	box := mustBox(t, "b", "2019", "")
	root, sub := domain.NewFolderID(), domain.NewFolderID()
	folders := []domain.Folder{
		domain.NewRootFolder(root),
		mustFolder(t, sub, root, "notes"),
	}
	files := []domain.File{mustFile(t, root, "notes", "sha256:aa")}

	got, err := Plan("box", box, folders, files)
	if err != nil {
		t.Fatal(err)
	}
	if dup := duplicatePath(got); dup != "" {
		t.Errorf("a folder and a file both claimed %q: %v", dup, paths(got))
	}
}

// A file in a folder that is not in the folder set is a corrupt view, not
// something to project halfway. Say so rather than dropping it silently.
func TestPlanRejectsAnOrphanFile(t *testing.T) {
	box := mustBox(t, "b", "2019", "")
	root := domain.NewFolderID()
	files := []domain.File{mustFile(t, domain.NewFolderID(), "lost.txt", "sha256:aa")}
	if _, err := Plan("box", box, []domain.Folder{domain.NewRootFolder(root)}, files); err == nil {
		t.Error("a file with no folder was projected without complaint")
	}
}

func TestPlanRejectsAMissingRoot(t *testing.T) {
	box := mustBox(t, "b", "2019", "")
	if _, err := Plan("box", box, nil, nil); err == nil {
		t.Error("a box with no root folder was accepted")
	}
}

// Only directories carry a modification time.
func TestPlanStampsDirectoriesOnly(t *testing.T) {
	box := mustBox(t, "b", "2019", "")
	box.UpdatedAt = time.Date(2019, 8, 12, 14, 30, 0, 0, time.UTC)
	root := domain.NewFolderID()
	folders := []domain.Folder{domain.NewRootFolder(root)}
	folders[0].UpdatedAt = box.UpdatedAt
	files := []domain.File{mustFile(t, root, "a.txt", "sha256:aa")}
	files[0].UpdatedAt = box.UpdatedAt

	got, err := Plan("box", box, folders, files)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range got {
		if e.IsDir && e.ModTime.IsZero() {
			t.Errorf("directory %q has no mtime", e.Path)
		}
		if !e.IsDir && !e.ModTime.IsZero() {
			t.Errorf("file %q carries an mtime; a hard link shares its inode with the blob", e.Path)
		}
	}
}

// duplicatePath reports the first path claimed twice, folded — because two
// paths differing only in case are one file on macOS and on Windows.
func duplicatePath(entries []Entry) string {
	seen := map[string]bool{}
	for _, e := range entries {
		key := domain.Fold(e.Path)
		if seen[key] {
			return e.Path
		}
		seen[key] = true
	}
	return ""
}
