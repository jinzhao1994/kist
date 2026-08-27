package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/library"
)

// runKist runs the CLI the way a user would, with stdin scripted.
func runKist(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// libraryWithAFile builds a real library with one box and two files in it.
//
// The files go in through library rather than through `kist import`, and that
// is deliberate: the import command installs a SIGINT handler for the duration
// of the run (spec 3.7.1 ①). On Windows that means installing and removing a
// console control handler, and doing that repeatedly inside one process is
// something only a test does — a real invocation of kist installs it once and
// then exits. Driving it from here killed the whole test binary on the windows
// runner with STATUS_CONTROL_C_EXIT while passing everywhere else.
//
// Nothing is lost: what these tests are about is rebuild, and the import path
// is covered end to end by internal/library.
func libraryWithAFile(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "lib")
	if _, err := runKist(t, "", "init", root, "--yes"); err != nil {
		t.Fatal(err)
	}
	if _, err := runKist(t, "", "--library", root, "box", "new", "云南"); err != nil {
		t.Fatal(err)
	}

	l, err := library.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	boxes, err := l.Boxes(t.Context(), library.ListBoxesOptions{})
	if err != nil {
		t.Fatal(err)
	}
	src := writeTree(t, map[string]string{"a.txt": "x", "day1/b.txt": "y"})
	if _, err := l.Import(t.Context(), src, library.ImportOptions{Box: boxes[0].ID}); err != nil {
		t.Fatal(err)
	}
	return root
}

// Answering no must change nothing. Rebuild is safe by construction, but
// "recoverable" is not "what you meant to do", and it is minutes on a real
// library.
func TestRebuildAsksFirst(t *testing.T) {
	root := libraryWithAFile(t)

	out, err := runKist(t, "n\n", "--library", root, "rebuild")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "nothing was changed") {
		t.Errorf("answering no did not say so:\n%s", out)
	}
	if strings.Contains(out, "replayed") {
		t.Errorf("it rebuilt anyway:\n%s", out)
	}
}

// And the library is still correct afterwards — including boxes/, which is
// built from the tables that were just thrown away and replayed.
func TestRebuildReplaysAndRelinks(t *testing.T) {
	root := libraryWithAFile(t)

	out, err := runKist(t, "", "--library", root, "rebuild", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "replayed") || !strings.Contains(out, "relinked") {
		t.Errorf("rebuild did not report both halves of what it did:\n%s", out)
	}

	out, err = runKist(t, "", "--library", root, "verify", "--deep")
	if err != nil {
		t.Fatalf("the library is not clean after a rebuild: %v\n%s", err, out)
	}
	if !strings.Contains(out, "no problems found") {
		t.Errorf("verify after rebuild:\n%s", out)
	}
}
