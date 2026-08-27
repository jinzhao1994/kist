package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupBoxWithFiles makes a library holding one box and a small tree, and
// returns the --library flag.
func setupBoxWithFiles(t *testing.T) string {
	t.Helper()
	lib := newLibrary(t)
	if _, err := exec(t, lib, "box", "new", "云南", "--start", "2019-08"); err != nil {
		t.Fatal(err)
	}
	src := writeTree(t, map[string]string{
		"b.txt":          "x",
		"a.txt":          "y",
		"day1/photo.jpg": "z",
	})
	if _, err := exec(t, lib, "import", "--box", "云南", src); err != nil {
		t.Fatal(err)
	}
	return lib
}

func TestLsWithNoArgumentsListsBoxes(t *testing.T) {
	lib := setupBoxWithFiles(t)
	out, err := exec(t, lib, "ls")
	if err != nil {
		t.Fatalf("ls: %v\n%s", err, out)
	}
	if !strings.Contains(out, "云南") || !strings.Contains(out, "2019-08") {
		t.Errorf("ls does not show the box and its date:\n%s", out)
	}
}

// Directories come first, then files; both in byte order (spec 4.8), which is
// the order the database gives — so the CLI and a future API cannot disagree.
func TestLsListsDirectoriesFirst(t *testing.T) {
	lib := setupBoxWithFiles(t)
	out, err := exec(t, lib, "ls", "云南")
	if err != nil {
		t.Fatalf("ls: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("%d lines, want day1/ plus two files:\n%s", len(lines), out)
	}
	if !strings.Contains(lines[0], "day1/") {
		t.Errorf("the directory is not first:\n%s", out)
	}
	if a, b := strings.Index(out, "a.txt"), strings.Index(out, "b.txt"); a > b {
		t.Errorf("files are not in byte order:\n%s", out)
	}
}

func TestLsWalksAPath(t *testing.T) {
	lib := setupBoxWithFiles(t)
	out, err := exec(t, lib, "ls", "云南", "day1")
	if err != nil {
		t.Fatalf("ls: %v\n%s", err, out)
	}
	if !strings.Contains(out, "photo.jpg") {
		t.Errorf("ls of the subfolder does not show its file:\n%s", out)
	}
	if strings.Contains(out, "a.txt") {
		t.Errorf("ls of the subfolder leaked the parent's files:\n%s", out)
	}
}

// A path that does not exist has to say so and say which part failed, or the
// user is left guessing whether they got the box wrong or the path.
func TestLsNamesAMissingPath(t *testing.T) {
	lib := setupBoxWithFiles(t)
	out, err := exec(t, lib, "ls", "云南", "day1/nope")
	if err == nil {
		t.Fatalf("a missing path was accepted:\n%s", out)
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("the error does not name the missing part: %v", err)
	}
}

// spec 3.4.1: the dirty marker has to be visible, or boxes/ goes stale
// silently until someone happens to run verify.
func TestLsWarnsWhenTheProjectionIsStale(t *testing.T) {
	lib := setupBoxWithFiles(t)
	root := strings.TrimPrefix(lib, "--library=")
	marker := filepath.Join(root, "boxes", ".kist-projection-dirty")
	if err := os.WriteFile(marker, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := exec(t, lib, "ls")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "relink") {
		t.Errorf("no warning about the stale projection:\n%s", out)
	}
}
