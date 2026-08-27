package library

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/store"
)

func snapshotOf(t *testing.T, l *Library) string {
	t.Helper()
	s, err := store.Snapshot(l.db)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func report(t *testing.T, r integrity.Report) string {
	t.Helper()
	var sb strings.Builder
	if err := r.Write(&sb); err != nil {
		t.Fatal(err)
	}
	return sb.String()
}

// someBlobPath returns the path of one stored blob, with its read-only mode
// cleared so the test can damage it.
func someBlobPath(t *testing.T, l *Library) string {
	t.Helper()
	var found string
	err := filepath.WalkDir(filepath.Join(l.Layout().Blobs(), "sha256"),
		func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || found != "" {
				return err
			}
			found = p
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if found == "" {
		t.Fatal("the library has no blobs")
	}
	if err := os.Chmod(found, 0o644); err != nil {
		t.Fatal(err)
	}
	return found
}

// A library that import just built must verify clean. If this is ever red,
// either the import path or the checker is wrong, and both are worth stopping
// for.
func TestAFreshLibraryVerifiesClean(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x", "day1/b.txt": "y"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}

	r, err := l.Verify(t.Context(), VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Total() != 0 {
		t.Errorf("a fresh library has findings:\n%s", report(t, r))
	}
	if r.Checked["blobs"] != 2 {
		t.Errorf("checked %d blobs, want 2", r.Checked["blobs"])
	}
	if r.Checked["boxes"] != 1 {
		t.Errorf("checked %d boxes, want 1", r.Checked["boxes"])
	}
}

// The finding that matters most: your content is gone. This is the sentence
// spec 3.8 opens with — knowing in 2026 rather than in 2031.
func TestVerifyFindsMissingContent(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"passport.jpg": "scan"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(someBlobPath(t, l)); err != nil {
		t.Fatal(err)
	}

	r, err := l.Verify(t.Context(), VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Count(integrity.BlobMissing) != 1 {
		t.Errorf("BlobMissing = %d, want 1:\n%s", r.Count(integrity.BlobMissing), report(t, r))
	}
	if r.Worst() != integrity.SeverityError {
		t.Errorf("worst = %v, want SeverityError", r.Worst())
	}
}

// boxes/ drifting is a warning, not an error: it is a projection, and relink
// rebuilds it from the database (spec 3.2). Reporting it as an error would
// teach people that a red verify is normal.
func TestVerifyReportsProjectionDriftAsAWarning(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(l.Layout().Boxes()); err != nil {
		t.Fatal(err)
	}

	r, err := l.Verify(t.Context(), VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Count(integrity.LinkMissing) == 0 {
		t.Errorf("the whole projection is gone and verify did not notice:\n%s", report(t, r))
	}
	if r.Worst() != integrity.SeverityWarn {
		t.Errorf("worst = %v, want SeverityWarn", r.Worst())
	}
}

// A directory under boxes/ belonging to no box is what a deleted box or an
// interrupted rename leaves. relink removes it; verify has to see it first.
func TestVerifyReportsADirectoryBelongingToNoBox(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(l.Layout().Boxes(), "not-a-box"), 0o755); err != nil {
		t.Fatal(err)
	}

	r, err := l.Verify(t.Context(), VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Count(integrity.LinkStray) != 1 {
		t.Errorf("LinkStray = %d, want 1:\n%s", r.Count(integrity.LinkStray), report(t, r))
	}
}

// verify must not repair. It is the first tool a worried user reaches for, and
// a tool that changes things while you are working out what is wrong is worse
// than no tool.
func TestVerifyChangesNothing(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(l.Layout().Boxes(), soleBoxDir(t, l), "invented.txt")
	if err := os.WriteFile(stray, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotOf(t, l)

	if _, err := l.Verify(t.Context(), VerifyOptions{}); err != nil {
		t.Fatal(err)
	}
	if after := snapshotOf(t, l); after != before {
		t.Error("verify wrote to the database")
	}
	if _, err := os.Stat(stray); err != nil {
		t.Error("verify removed the stray it was supposed to report")
	}
}

// verify clears crash residue, because that is the one thing under blobs/ that
// is safe to remove and would otherwise sit in the report looking like a
// finding (spec 3.9).
func TestVerifyClearsOldCrashResidue(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}

	tmp := filepath.Join(l.Layout().Blobs(), "tmp")
	stale := filepath.Join(tmp, "put-crashed")
	if err := os.WriteFile(stale, []byte("half a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := l.now().Add(-48 * 60 * 60 * 1e9)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	if _, err := l.Verify(t.Context(), VerifyOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("the crash residue survived a verify")
	}
}
