package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// The first Ctrl-C must let the finished files commit; only the second gives
// up on them (spec 3.7.1 ①).
func TestInterruptFirstCancelsSecondExits(t *testing.T) {
	sig := make(chan os.Signal, 2)
	var notices []string
	exited := make(chan int, 1)

	ctx, cancel := interruptContext(t.Context(), sig,
		func(s string) { notices = append(notices, s) },
		func(code int) { exited <- code })
	defer cancel()

	sig <- os.Interrupt
	<-ctx.Done() // the first signal stops the batch, it does not kill us

	select {
	case code := <-exited:
		t.Fatalf("the first Ctrl-C exited with %d; it must let the finished files commit", code)
	default:
	}

	sig <- os.Interrupt
	if code := <-exited; code != interruptExitCode {
		t.Errorf("second Ctrl-C exited with %d, want %d", code, interruptExitCode)
	}
	if len(notices) == 0 {
		t.Error("the user was told nothing about what the first Ctrl-C did")
	}
	if len(notices) > 0 && !strings.Contains(notices[0], "kept") {
		t.Errorf("the notice does not say the finished files are kept: %q", notices[0])
	}
}

// A context that is never signalled is never cancelled: the ordinary path must
// not be affected by the handler existing.
func TestInterruptContextLeavesAQuietRunAlone(t *testing.T) {
	sig := make(chan os.Signal, 1)
	ctx, cancel := interruptContext(t.Context(), sig, func(string) {}, func(int) {})
	defer cancel()
	if err := ctx.Err(); err != nil {
		t.Errorf("ctx.Err() = %v with no signal sent", err)
	}
}

func TestImportPutsFilesIn(t *testing.T) {
	lib := newLibrary(t)
	if _, err := exec(t, lib, "box", "new", "云南", "--start", "2019-08"); err != nil {
		t.Fatal(err)
	}
	src := writeTree(t, map[string]string{"a.txt": "x", "day1/b.txt": "y"})

	out, err := exec(t, lib, "import", "--box", "云南", src)
	if err != nil {
		t.Fatalf("import: %v\n%s", err, out)
	}
	if !strings.Contains(out, "imported 2") {
		t.Errorf("summary does not report two files:\n%s", out)
	}

	// And they landed in the projection, which is the whole point of boxes/.
	root := strings.TrimPrefix(lib, "--library=")
	found := 0
	filepath.WalkDir(filepath.Join(root, "boxes"), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			found++
		}
		return nil
	})
	if found != 2 {
		t.Errorf("%d files under boxes/, want 2", found)
	}
}

// Skipped files are not failures and must not read like them: after an
// interrupted import, a re-run is almost entirely skips and that is success.
func TestImportSummaryDistinguishesSkippedFromFailed(t *testing.T) {
	lib := newLibrary(t)
	if _, err := exec(t, lib, "box", "new", "云南"); err != nil {
		t.Fatal(err)
	}
	src := writeTree(t, map[string]string{"a.txt": "x"})

	if _, err := exec(t, lib, "import", "--box", "云南", src); err != nil {
		t.Fatal(err)
	}
	out, err := exec(t, lib, "import", "--box", "云南", src)
	if err != nil {
		t.Fatalf("a run that only skipped must succeed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "skipped 1") {
		t.Errorf("summary does not report the skip:\n%s", out)
	}
	if strings.Contains(out, "failed:") {
		t.Errorf("a skip was reported under 'failed':\n%s", out)
	}
}

// A summary that says only "done" makes a partial failure invisible, which is
// the one thing per-file results exist to prevent.
func TestImportSummaryNamesFailuresAndExitsNonZero(t *testing.T) {
	lib := newLibrary(t)
	if _, err := exec(t, lib, "box", "new", "云南"); err != nil {
		t.Fatal(err)
	}
	// A name with a trailing space cannot be projected identically on every
	// platform, so it is a per-file failure (spec 3.4).
	src := writeTree(t, map[string]string{"fine.txt": "x", "bad ": "y"})
	if !hasExactName(t, src, "bad ") {
		t.Skip("this filesystem will not hold a name ending in a space — which is the rule being tested")
	}

	out, err := exec(t, lib, "import", "--box", "云南", src)
	if err == nil {
		t.Errorf("a batch with a failure exited zero:\n%s", out)
	}
	if !strings.Contains(out, "failed:") {
		t.Errorf("the failure is not listed:\n%s", out)
	}
	if !strings.Contains(out, "imported 1") {
		t.Errorf("the good file was not imported:\n%s", out)
	}
}

func TestImportRequiresABox(t *testing.T) {
	lib := newLibrary(t)
	src := writeTree(t, map[string]string{"a.txt": "x"})
	if _, err := exec(t, lib, "import", src); err == nil {
		t.Error("import without --box was accepted")
	}
}

func TestImportNamesAnUnknownBox(t *testing.T) {
	lib := newLibrary(t)
	src := writeTree(t, map[string]string{"a.txt": "x"})
	out, err := exec(t, lib, "import", "--box", "没有这个", src)
	if err == nil {
		t.Fatal("importing into a box that does not exist was accepted")
	}
	if !strings.Contains(err.Error()+out, "没有这个") {
		t.Errorf("the error does not name the box: %v\n%s", err, out)
	}
}

func TestImportIntoASubfolder(t *testing.T) {
	lib := newLibrary(t)
	if _, err := exec(t, lib, "box", "new", "云南"); err != nil {
		t.Fatal(err)
	}
	first := writeTree(t, map[string]string{"day1/a.txt": "x"})
	if _, err := exec(t, lib, "import", "--box", "云南", first); err != nil {
		t.Fatal(err)
	}

	second := writeTree(t, map[string]string{"b.txt": "y"})
	out, err := exec(t, lib, "import", "--box", "云南", "--into", "day1", second)
	if err != nil {
		t.Fatalf("import --into: %v\n%s", err, out)
	}

	root := strings.TrimPrefix(lib, "--library=")
	var got []string
	filepath.WalkDir(filepath.Join(root, "boxes"), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			got = append(got, filepath.Base(p))
		}
		return nil
	})
	if len(got) != 2 {
		t.Fatalf("%d files, want 2: %v", len(got), got)
	}
	// b.txt has to be inside day1, not next to it.
	rel := ""
	filepath.WalkDir(filepath.Join(root, "boxes"), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Base(p) == "b.txt" {
			rel = p
		}
		return nil
	})
	if !strings.Contains(filepath.ToSlash(rel), "/day1/") {
		t.Errorf("b.txt is at %q, want it inside day1", rel)
	}
}

// hasExactName reports whether the filesystem kept a name exactly as written.
//
// Windows silently drops trailing dots and spaces, so a fixture that needs one
// cannot exist there — which is precisely why kist refuses such names on every
// platform (spec 3.4). Checking the fixture rather than checking GOOS says that
// out loud, and covers any other filesystem that normalizes names.
func hasExactName(t *testing.T, dir, name string) bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == name {
			return true
		}
	}
	return false
}
