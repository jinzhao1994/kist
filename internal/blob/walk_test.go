package blob

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// corrupt overwrites a stored blob with different content, the way bit rot or
// a bad restore would. Blobs are stored read-only, so the mode comes off first.
func corrupt(t *testing.T, d *Dir, k Key, content string) {
	t.Helper()
	p, _ := d.LocalPath(k)
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o444); err != nil {
		t.Fatal(err)
	}
}

// Walk is what finds orphans: content on disk the database has no row for.
func TestWalkFindsEveryStoredBlob(t *testing.T) {
	d := newTestDir(t)
	want := map[Key]int64{
		put(t, d, "one").Key:   3,
		put(t, d, "two!!").Key: 5,
	}

	got := map[Key]int64{}
	if err := d.Walk(func(k Key, size int64) error {
		got[k] = size
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("walked %d blobs, want %d: %v", len(got), len(want), got)
	}
	for k, size := range want {
		if got[k] != size {
			t.Errorf("%s walked with size %d, want %d", k, got[k], size)
		}
	}
}

// tmp/ holds half-written files that are not content-addressed. Reporting one
// as unknown content would be pure noise, every time an import is running.
//
// Note what this does and does not prove: it pins the behaviour, but the
// mechanism is keyFromRelPath's strictness, not the SkipDir in Walk. Deleting
// the SkipDir leaves this test green, because tmp/<name> is two path segments
// and a key needs four. The SkipDir earns its place by not stating thousands
// of staging files during an import, which is a cost, not a correctness
// property, and is not testable here.
func TestWalkIgnoresTmp(t *testing.T) {
	d := newTestDir(t)
	put(t, d, "real")
	if err := os.WriteFile(filepath.Join(d.Root(), "tmp", "put-123"), []byte("half"), 0o644); err != nil {
		t.Fatal(err)
	}

	n := 0
	if err := d.Walk(func(Key, int64) error { n++; return nil }); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("walked %d entries, want 1 — tmp/ must be skipped", n)
	}
}

// A file under blobs/ whose name is not a key at all — somebody dropped
// something in there — must not stop the walk or be reported as a blob.
func TestWalkSkipsThingsThatAreNotKeys(t *testing.T) {
	d := newTestDir(t)
	real := put(t, d, "real").Key
	junk := filepath.Join(d.Root(), "sha256", "zz", "zz", "not-a-hash")
	if err := os.MkdirAll(filepath.Dir(junk), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(junk, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// And a stray at the top level, where a backup tool would leave one.
	if err := os.WriteFile(filepath.Join(d.Root(), ".DS_Store"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var seen []Key
	if err := d.Walk(func(k Key, _ int64) error { seen = append(seen, k); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != real {
		t.Errorf("walked %v, want just %s", seen, real)
	}
}

// A blob stored under a collision suffix is still a blob, and Walk has to come
// back with the suffixed key — not the bare one, which is a different blob
// holding different content (spec 3.3).
func TestWalkKeepsTheCollisionSuffix(t *testing.T) {
	d := newTestDir(t)
	k := put(t, d, "content").Key
	suffixed := NewKey(k.Algo(), k.Hex(), 1)
	p, _ := d.LocalPath(suffixed)
	if err := os.WriteFile(p, []byte("different"), 0o444); err != nil {
		t.Fatal(err)
	}

	seen := map[Key]bool{}
	if err := d.Walk(func(k Key, _ int64) error { seen[k] = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if !seen[k] || !seen[suffixed] {
		t.Errorf("walked %v, want both %s and %s", seen, k, suffixed)
	}
}

// Rehash is scrub's primitive: read every byte, hash it, compare.
func TestRehashDetectsAFlippedBit(t *testing.T) {
	d := newTestDir(t)
	k := put(t, d, "the original content").Key

	hex, size, err := d.Rehash(context.Background(), k)
	if err != nil {
		t.Fatal(err)
	}
	if hex != k.Hex() {
		t.Fatalf("a fresh blob rehashes to %s, want %s", hex, k.Hex())
	}
	if size != int64(len("the original content")) {
		t.Errorf("size = %d", size)
	}

	corrupt(t, d, k, "the corrupted content")

	hex, _, err = d.Rehash(context.Background(), k)
	if err != nil {
		t.Fatal(err)
	}
	if hex == k.Hex() {
		t.Error("a corrupted blob still rehashes to its own name")
	}
}

// scrub runs for hours, so Ctrl-C has to be answered inside a single large
// file, not only between files.
func TestRehashStopsWhenCancelled(t *testing.T) {
	d := newTestDir(t)
	big := make([]byte, 1<<20)
	k := put(t, d, string(big)).Key

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := d.Rehash(ctx, k); err == nil {
		t.Error("Rehash ignored a cancelled context")
	}
}

// The tmp cleanup is not GC. It removes crash residue and nothing else
// (spec 3.9), and it must never touch a temp file that a running import is
// still writing — hence a cutoff rather than "everything in tmp/".
func TestCleanTmpRemovesOnlyOldResidue(t *testing.T) {
	d := newTestDir(t)
	tmp := filepath.Join(d.Root(), "tmp")

	fresh := filepath.Join(tmp, "put-fresh")
	stale := filepath.Join(tmp, "put-stale")
	for _, p := range []string{fresh, stale} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	n, err := d.CleanTmp(time.Now().Add(-TmpGrace))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("removed %d files, want 1", n)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("the fresh temp file was removed: %v", err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("the stale temp file survived")
	}
}

// A blob directory with no tmp/ at all — a read-only remote, say — is not an
// error to clean.
func TestCleanTmpOnADirectoryWithoutTmp(t *testing.T) {
	root := t.TempDir()
	d, err := OpenDir(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := d.CleanTmp(time.Now()); err != nil || n != 0 {
		t.Errorf("CleanTmp = %d, %v; want 0, nil", n, err)
	}
}

// OpenDir must not create anything. A repair source is normally mounted
// read-only, and NewDir's MkdirAll of tmp/ fails there before the caller gets
// to do anything (spec 3.8).
func TestOpenDirCreatesNothing(t *testing.T) {
	root := t.TempDir()
	d, err := OpenDir(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(d.Root())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("OpenDir created %v", entries)
	}
}

func TestOpenDirRefusesWhatIsNotADirectory(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenDir(p, nil); err == nil {
		t.Error("OpenDir accepted a regular file")
	}
	if _, err := OpenDir(filepath.Join(t.TempDir(), "nope"), nil); err == nil {
		t.Error("OpenDir accepted a path that does not exist")
	}
}
