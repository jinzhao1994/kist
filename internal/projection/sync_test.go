package projection

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
)

// newSyncer builds a syncer over a temp boxes/ directory, backed by a fake
// blob store: a directory of files named after their key.
func newSyncer(t *testing.T) (*Syncer, string) {
	t.Helper()
	root := t.TempDir()
	blobs := t.TempDir()
	return &Syncer{
		Root:     root,
		LinkMode: blob.LinkCopy, // works on every filesystem, so the test is about Sync
		Locate: func(key string) (string, bool) {
			p := filepath.Join(blobs, sanitizeKey(key))
			if _, err := os.Stat(p); err != nil {
				if err := os.WriteFile(p, []byte(key), 0o444); err != nil {
					t.Fatal(err)
				}
			}
			return p, true
		},
	}, root
}

func sanitizeKey(key string) string {
	out := []rune(key)
	for i, r := range out {
		if r == ':' || r == '/' {
			out[i] = '_'
		}
	}
	return string(out)
}

func statOf(t *testing.T, p string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func treeFixture() []Entry {
	return []Entry{
		{Path: "box", IsDir: true, ModTime: time.Date(2019, 8, 12, 14, 30, 0, 0, time.UTC)},
		{Path: "box/a.txt", BlobKey: "sha256:aa"},
		{Path: "box/day1", IsDir: true, ModTime: time.Date(2019, 8, 13, 9, 0, 0, 0, time.UTC)},
		{Path: "box/day1/b.txt", BlobKey: "sha256:bb"},
	}
}

// Sync is a diff, not a rebuild: a box with 20,000 photos must not have every
// link torn down and remade because one file was added.
func TestSyncOnlyTouchesWhatChanged(t *testing.T) {
	s, root := newSyncer(t)
	want := []Entry{
		{Path: "box", IsDir: true},
		{Path: "box/a.txt", BlobKey: "sha256:aa"},
	}
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}
	before := statOf(t, filepath.Join(root, "box/a.txt"))

	want = append(want, Entry{Path: "box/b.txt", BlobKey: "sha256:bb"})
	stats, err := s.SyncDir("box", want)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Created != 1 || stats.Removed != 0 {
		t.Errorf("stats = %+v, want exactly one creation", stats)
	}
	after := statOf(t, filepath.Join(root, "box/a.txt"))
	if !os.SameFile(before, after) {
		t.Error("the untouched file was relinked")
	}
}

// Running it twice must change nothing at all. This is the property that lets
// relink be the safe fallback for every projection problem (spec 3.4.1).
func TestSyncIsIdempotent(t *testing.T) {
	s, _ := newSyncer(t)
	want := treeFixture()
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}
	stats, err := s.SyncDir("box", want)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Created != 0 || stats.Removed != 0 || stats.Failed != 0 {
		t.Errorf("the second run did work: %+v", stats)
	}
}

// Things on disk but not in the plan are what an interrupted rename or a hand
// edit leaves behind. relink's job is to make the directory match the
// database, which means removing them (spec 3.2: no reconciliation, just
// rebuild).
func TestSyncRemovesStrays(t *testing.T) {
	s, root := newSyncer(t)
	want := treeFixture()
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "box/hand-made.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "box/stray/deeper"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "box/stray/deeper/x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	stats, err := s.SyncDir("box", want)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Removed == 0 {
		t.Errorf("nothing was removed: %+v", stats)
	}
	for _, p := range []string{"box/hand-made.txt", "box/stray"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err == nil {
			t.Errorf("%s is still there", p)
		}
	}
	// And the real contents survived.
	if _, err := os.Stat(filepath.Join(root, "box/day1/b.txt")); err != nil {
		t.Errorf("a real file was removed with the strays: %v", err)
	}
}

// Directory mtimes are applied last, because creating a child bumps its
// parent's mtime and would undo the stamp.
func TestSyncStampsDirectoryTimes(t *testing.T) {
	s, root := newSyncer(t)
	when := time.Date(2019, 8, 12, 14, 30, 0, 0, time.UTC)
	_, err := s.SyncDir("box", []Entry{
		{Path: "box", IsDir: true, ModTime: when},
		{Path: "box/day1", IsDir: true, ModTime: when},
		{Path: "box/day1/a.txt", BlobKey: "sha256:aa"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"box", "box/day1"} {
		info := statOf(t, filepath.Join(root, filepath.FromSlash(p)))
		if !info.ModTime().Truncate(time.Second).Equal(when) {
			t.Errorf("%s mtime = %s, want %s", p, info.ModTime(), when)
		}
	}
}

// A blob that is not on local disk is a finding, not a crash, and everything
// else in the box must still be projected (spec 3.10).
func TestSyncReportsMissingBlobsAndKeepsGoing(t *testing.T) {
	s, root := newSyncer(t)
	locate := s.Locate
	s.Locate = func(key string) (string, bool) {
		if key == "sha256:gone" {
			return "", false
		}
		return locate(key)
	}

	stats, err := s.SyncDir("box", []Entry{
		{Path: "box", IsDir: true},
		{Path: "box/here.txt", BlobKey: "sha256:aa"},
		{Path: "box/gone.txt", BlobKey: "sha256:gone"},
	})
	if err != nil {
		t.Fatalf("a missing blob must not fail the sync: %v", err)
	}
	if stats.Missing != 1 {
		t.Errorf("Missing = %d, want 1: %+v", stats.Missing, stats)
	}
	if _, err := os.Stat(filepath.Join(root, "box/here.txt")); err != nil {
		t.Errorf("the file whose blob was present was not projected: %v", err)
	}
}

// Rebuilding from nothing is the relink case: whatever state boxes/ was in,
// this puts it back.
func TestSyncBuildsFromNothing(t *testing.T) {
	s, root := newSyncer(t)
	if _, err := s.SyncDir("box", treeFixture()); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"box", "box/a.txt", "box/day1", "box/day1/b.txt"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil {
			t.Errorf("%s was not created: %v", p, err)
		}
	}
}

func TestPruneRootRemovesUnknownBoxDirs(t *testing.T) {
	s, root := newSyncer(t)
	if _, err := s.SyncDir("box", treeFixture()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "orphan"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The dirty marker lives here too and must survive.
	if err := os.WriteFile(filepath.Join(root, ".kist-projection-dirty"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	removed, err := s.PruneRoot(map[string]bool{"box": true})
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Errorf("removed %d, want 1", removed)
	}
	if _, err := os.Stat(filepath.Join(root, "box")); err != nil {
		t.Error("the real box directory was pruned")
	}
	if _, err := os.Stat(filepath.Join(root, ".kist-projection-dirty")); err != nil {
		t.Error("the dirty marker was pruned")
	}
}

// Renaming moves the directory: an O(1) metadata operation that leaves every
// link inside untouched (spec 3.4.1).
//
// The assertion is "nothing was linked again", not os.SameFile. SameFile is
// unusable here: on Windows it reopens both paths to read the file id, and the
// old path is gone by the time it is asked — which made this test fail on the
// windows runner while passing everywhere else. Counting the work is a better
// question anyway, because the work is what the rule is about: a box with
// twenty thousand photos must not have them relinked to change its name.
func TestRenameMovesTheDirectory(t *testing.T) {
	s, root := newSyncer(t)
	if _, err := s.SyncDir("box", treeFixture()); err != nil {
		t.Fatal(err)
	}

	if err := s.Rename("box", "renamed"); err != nil {
		t.Fatal(err)
	}

	stats, err := s.SyncDir("renamed", renamedFixture())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Created != 0 || stats.Removed != 0 {
		t.Errorf("syncing after the rename did work: %+v", stats)
	}

	if _, err := os.Stat(filepath.Join(root, "box")); err == nil {
		t.Error("the old directory is still there")
	}
	// And the content came along, readable at its new path.
	body, err := os.ReadFile(filepath.Join(root, "renamed/day1/b.txt"))
	if err != nil {
		t.Fatalf("the file did not come along: %v", err)
	}
	if string(body) != "sha256:bb" {
		t.Errorf("the moved file reads %q", body)
	}
}

// renamedFixture is treeFixture under the new directory name.
func renamedFixture() []Entry {
	out := treeFixture()
	for i := range out {
		out[i].Path = "renamed" + strings.TrimPrefix(out[i].Path, "box")
	}
	return out
}

// Renaming onto a taken name is refused rather than done badly; the caller
// falls back to a full relink.
func TestRenameRefusesAnOccupiedTarget(t *testing.T) {
	s, root := newSyncer(t)
	if _, err := s.SyncDir("box", treeFixture()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "taken"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.Rename("box", "taken"); err == nil {
		t.Error("renaming onto an existing directory was allowed")
	}
}

func TestRenameOfSomethingNotYetProjectedIsFine(t *testing.T) {
	s, _ := newSyncer(t)
	if err := s.Rename("never-made", "somewhere"); err != nil {
		t.Errorf("renaming an unprojected box = %v, want nil", err)
	}
}

func TestSyncRepairsExistingWrongEntries(t *testing.T) {
	for _, kind := range []string{"content", "file is directory", "directory is file", "directory is symlink", "wrong hardlink", "wrong symlink"} {
		t.Run(kind, func(t *testing.T) {
			s, root := newSyncer(t)
			want := treeFixture()
			if kind == "wrong hardlink" {
				s.LinkMode = blob.LinkHardlink
			}
			if kind == "wrong symlink" {
				s.LinkMode = blob.LinkSymlink
			}
			if _, err := s.SyncDir("box", want); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(root, "box", "a.txt")
			if strings.HasPrefix(kind, "directory") {
				target = filepath.Join(root, "box", "day1")
			}
			if err := os.RemoveAll(target); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "file is directory":
				if err := os.Mkdir(target, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(target, "stray"), []byte("stray"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "directory is symlink", "wrong symlink":
				outside := t.TempDir()
				if err := os.WriteFile(filepath.Join(outside, "sentinel"), []byte("safe"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, target); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				defer func() {
					body, err := os.ReadFile(filepath.Join(outside, "sentinel"))
					if err != nil || string(body) != "safe" {
						t.Error("repair modified the symlink target")
					}
				}()
			case "wrong hardlink":
				other := filepath.Join(t.TempDir(), "other")
				if err := os.WriteFile(other, []byte("sha256:zz"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(other, target); err != nil {
					t.Fatal(err)
				}
			default:
				if err := os.WriteFile(target, []byte("wrong"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			stats, err := s.SyncDir("box", want)
			if err != nil {
				t.Fatal(err)
			}
			if stats.Created == 0 || stats.Removed == 0 || stats.Failed != 0 {
				t.Fatalf("repair stats: %+v", stats)
			}
			for _, e := range want {
				if e.IsDir {
					continue
				}
				body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(e.Path)))
				if err != nil || string(body) != e.BlobKey {
					t.Fatalf("%s: %q, %v", e.Path, body, err)
				}
			}
			stats, err = s.SyncDir("box", want)
			if err != nil || stats.Created != 0 || stats.Removed != 0 {
				t.Fatalf("second sync: %+v, %v", stats, err)
			}
		})
	}
}
