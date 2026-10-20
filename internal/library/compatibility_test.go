package library

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/store"
)

// Unlike a round trip using today's writers, this consumes frozen historical
// bytes and an independently stored expected state. Never regenerate the
// fixture merely to make a changed reader pass.
func TestFrozenJournalsRemainReadable(t *testing.T) {
	for _, version := range []string{"journal-v1", "journal-v2"} {
		t.Run(version, func(t *testing.T) { checkFrozenJournal(t, version) })
	}
}

func checkFrozenJournal(t *testing.T, version string) {
	fixture := filepath.Join("testdata", version)
	root := t.TempDir()
	if _, err := Create(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ops", "blobs"} {
		if _, err := os.Stat(filepath.Join(fixture, name)); name == "blobs" && os.IsNotExist(err) {
			continue
		}
		if err := os.CopyFS(filepath.Join(root, name), os.DirFS(filepath.Join(fixture, name))); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(filepath.Join(fixture, "snapshot.txt"))
	if err != nil {
		t.Fatal(err)
	}
	// Historical fixtures predate covers. Extend only the independent expected
	// box rows with the migration's empty reference; keep frozen logs untouched.
	lines := strings.Split(string(want), "\n")
	inBoxes := false
	for i, line := range lines {
		if strings.HasPrefix(line, "== ") {
			inBoxes = line == "== boxes =="
			continue
		}
		if inBoxes && line != "" {
			lines[i] += "\t00000000000000000000000000000000"
		}
	}
	want = []byte(strings.Join(lines, "\n"))
	l, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	check := func(stage string) {
		t.Helper()
		got, err := store.Snapshot(l.db)
		if err != nil {
			t.Fatal(err)
		}
		if got != string(want) {
			t.Fatalf("%s changed historical semantics: %s", stage, firstDifference(string(want), got))
		}
	}
	check("catch-up")
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	check("rebuild")
	if _, err := l.Relink(t.Context()); err != nil {
		t.Fatal(err)
	}
	r, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Count(integrity.ViewDrift) != 0 || r.Count(integrity.LogCorrupt) != 0 {
		t.Fatalf("historical fixture failed deep verification: %s", report(t, r))
	}
	check("deep verification")
	err = filepath.WalkDir(filepath.Join(fixture, "ops"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(fixture, path)
		if err != nil {
			return err
		}
		original, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		after, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return err
		}
		if !bytes.Equal(original, after) {
			t.Errorf("historical log was rewritten: %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
