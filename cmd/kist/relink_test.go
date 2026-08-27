package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func countProjected(t *testing.T, lib string) int {
	t.Helper()
	root := strings.TrimPrefix(lib, "--library=")
	n := 0
	filepath.WalkDir(filepath.Join(root, "boxes"), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.HasPrefix(d.Name(), ".") {
			n++
		}
		return nil
	})
	return n
}

func TestRelinkRebuildsAfterBoxesIsDeleted(t *testing.T) {
	lib := setupBoxWithFiles(t)
	before := countProjected(t, lib)
	if before != 3 {
		t.Fatalf("%d projected files to begin with, want 3", before)
	}

	root := strings.TrimPrefix(lib, "--library=")
	if err := os.RemoveAll(filepath.Join(root, "boxes")); err != nil {
		t.Fatal(err)
	}

	out, err := exec(t, lib, "relink")
	if err != nil {
		t.Fatalf("relink: %v\n%s", err, out)
	}
	if got := countProjected(t, lib); got != before {
		t.Errorf("%d projected files after relink, want %d\n%s", got, before, out)
	}
}

// Running it twice must report no work the second time, or it cannot be the
// thing you reach for whenever boxes/ looks wrong (spec 3.4.1).
func TestRelinkIsIdempotentFromTheCLI(t *testing.T) {
	lib := setupBoxWithFiles(t)
	if _, err := exec(t, lib, "relink"); err != nil {
		t.Fatal(err)
	}
	out, err := exec(t, lib, "relink")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "created 0") || !strings.Contains(out, "removed 0") {
		t.Errorf("the second relink did work:\n%s", out)
	}
}

func TestRelinkClearsTheStaleWarning(t *testing.T) {
	lib := setupBoxWithFiles(t)
	root := strings.TrimPrefix(lib, "--library=")
	marker := filepath.Join(root, "boxes", ".kist-projection-dirty")
	if err := os.WriteFile(marker, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := exec(t, lib, "relink")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "cleared") {
		t.Errorf("relink did not say the warning was cleared:\n%s", out)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("the marker file is still there")
	}
}
