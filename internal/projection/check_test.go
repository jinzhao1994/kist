package projection

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
)

// skipIfLinkModeUnsupported probes one link mode on the filesystem the test is
// running on. Hard links and symlinks are not available everywhere (spec 3.5),
// and a test that goes red on a filesystem is a test people learn to ignore.
func skipIfLinkModeUnsupported(t *testing.T, mode blob.LinkMode) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := blob.Link(mode, src, filepath.Join(dir, "dst")); err != nil {
		t.Skipf("this filesystem does not support %s: %v", mode, err)
	}
}

// DiffDir answers "what would SyncDir do" without doing it. That is verify's
// projection check (spec 3.8): the same code that repairs boxes/ is the code
// that reports on it, so the two can never disagree about what "correct" means.
func TestDiffDirReportsWithoutTouchingAnything(t *testing.T) {
	s, root := newSyncer(t)
	want := treeFixture()
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}

	// Break it two ways: remove something real, add something invented.
	if err := os.Remove(filepath.Join(root, "box/a.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "box/invented.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := s.DiffDir("box", want)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Create) != 1 || d.Create[0].Path != "box/a.txt" {
		t.Errorf("Create = %+v, want just box/a.txt", d.Create)
	}
	if len(d.Remove) != 1 || d.Remove[0] != "box/invented.txt" {
		t.Errorf("Remove = %+v, want just box/invented.txt", d.Remove)
	}

	// And nothing was touched: the invented file is still there, the missing
	// one is still missing. verify does not repair (spec 3.8).
	if _, err := os.Stat(filepath.Join(root, "box/invented.txt")); err != nil {
		t.Error("DiffDir removed the stray")
	}
	if _, err := os.Stat(filepath.Join(root, "box/a.txt")); err == nil {
		t.Error("DiffDir created the missing link")
	}
}

// A healthy projection diffs to nothing. Same property as relink idempotence,
// asked as a question instead of as an action.
func TestDiffDirIsEmptyForAHealthyProjection(t *testing.T) {
	s, _ := newSyncer(t)
	want := treeFixture()
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}
	d, err := s.DiffDir("box", want)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Empty() {
		t.Errorf("a freshly synced directory diffs to %+v", d)
	}
}

// Removals come back deepest-first, because that is the order they have to be
// applied in: a directory refuses to go while anything is inside it.
func TestDiffDirOrdersRemovalsDeepestFirst(t *testing.T) {
	s, root := newSyncer(t)
	if _, err := s.SyncDir("box", treeFixture()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "box/stray/deeper"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "box/stray/deeper/x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := s.DiffDir("box", treeFixture())
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(d.Remove); i++ {
		if len(d.Remove[i]) > len(d.Remove[i-1]) {
			t.Errorf("Remove is not deepest-first: %v", d.Remove)
			break
		}
	}
}

// Under hardlink, "points at the right blob" is free: the link and the blob
// are the same inode. Both paths exist here, so os.SameFile is usable — unlike
// in TestRenameMovesTheDirectory, where the old path is gone by the time it
// would be asked.
func TestCheckLinksCatchesAWrongTargetUnderHardlink(t *testing.T) {
	skipIfLinkModeUnsupported(t, blob.LinkHardlink)
	s, root := newSyncer(t)
	s.LinkMode = blob.LinkHardlink
	want := treeFixture()
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}

	problems, err := s.CheckLinks(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("a healthy projection has problems: %+v", problems)
	}

	// Replace one link with an unrelated file of the same size.
	p := filepath.Join(root, "box/a.txt")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("sha256:XX"), 0o644); err != nil {
		t.Fatal(err)
	}

	problems, err = s.CheckLinks(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || problems[0].Path != "box/a.txt" {
		t.Errorf("problems = %+v, want one about box/a.txt", problems)
	}
}

// Under symlink the target is readable, so a link pointing somewhere else is
// caught for the price of one readlink.
func TestCheckLinksCatchesAWrongTargetUnderSymlink(t *testing.T) {
	skipIfLinkModeUnsupported(t, blob.LinkSymlink)
	s, root := newSyncer(t)
	s.LinkMode = blob.LinkSymlink
	want := treeFixture()
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}
	if problems, err := s.CheckLinks(want); err != nil || len(problems) != 0 {
		t.Fatalf("a healthy projection has problems %+v (err %v)", problems, err)
	}

	p := filepath.Join(root, "box/a.txt")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "box/day1/b.txt"), p); err != nil {
		t.Fatal(err)
	}

	problems, err := s.CheckLinks(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 {
		t.Errorf("problems = %+v, want one", problems)
	}
}

// Under copy there is no identity to check, and verify does not read content
// (spec 3.8). Size is all there is, and that has to be enough — a check that
// hashed here would have turned verify into scrub.
func TestCheckLinksUnderCopyChecksOnlySize(t *testing.T) {
	s, root := newSyncer(t)
	s.LinkMode = blob.LinkCopy
	want := treeFixture()
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}

	p := filepath.Join(root, "box/a.txt")
	original, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// Same length, different bytes: invisible, on purpose.
	same := make([]byte, len(original))
	for i := range same {
		same[i] = 'z'
	}
	if err := os.WriteFile(p, same, 0o644); err != nil {
		t.Fatal(err)
	}
	problems, err := s.CheckLinks(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("copy mode read the content: %+v", problems)
	}

	// A different length is visible, because stat says so.
	if err := os.WriteFile(p, []byte("short"), 0o644); err != nil {
		t.Fatal(err)
	}
	problems, err = s.CheckLinks(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 {
		t.Errorf("a wrong-sized file was not noticed: %+v", problems)
	}
}

// A file that is not there at all is DiffDir's finding. CheckLinks must not
// report it too, or every missing file is two lines in the report.
func TestCheckLinksSaysNothingAboutMissingFiles(t *testing.T) {
	s, root := newSyncer(t)
	want := treeFixture()
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "box/a.txt")); err != nil {
		t.Fatal(err)
	}
	problems, err := s.CheckLinks(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("CheckLinks reported a missing file: %+v", problems)
	}
}

// Removing a stray bumps its parent's mtime, so the stamping pass has to cover
// every directory in the plan and not only the ones just created.
//
// Nothing else in this package catches that. Measured: changing apply's final
// loop to walk only the newly created entries leaves all twenty-four of the
// pre-existing tests green, TestSyncStampsDirectoryTimes included — because on
// a first sync every directory is a new one. It takes a second sync that
// changes something to tell the two apart.
func TestSyncRestampsDirectoriesItOnlyRemovedFrom(t *testing.T) {
	s, root := newSyncer(t)
	want := treeFixture()
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "box/stray.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}

	info := statOf(t, filepath.Join(root, "box"))
	stamp := want[0].ModTime
	if !info.ModTime().Truncate(time.Second).Equal(stamp) {
		t.Errorf("box mtime = %s after the stray was removed, want %s", info.ModTime(), stamp)
	}
}
