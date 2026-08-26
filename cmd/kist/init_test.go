package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesALibraryNonInteractively(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")

	out, err := exec(t, "init", root, "--yes")
	if err != nil {
		t.Fatalf("kist init: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "kist.toml")); err != nil {
		t.Errorf("no library at %s: %v", root, err)
	}
	// The summary has to name the link mode: "this storage cannot deduplicate,
	// boxes/ will cost an extra N GB" is something the user must see (spec 3.5).
	if !strings.Contains(out, "Link mode") {
		t.Errorf("init output does not report the link mode:\n%s", out)
	}
}

func TestInitInteractiveAcceptsAPathAndConfirmation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")

	out, err := execStdin(t, root+"\ny\n", "init")
	if err != nil {
		t.Fatalf("kist init: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "kist.toml")); err != nil {
		t.Errorf("no library at %s: %v", root, err)
	}
}

func TestInitAbortsWhenTheUserDeclines(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")

	out, err := execStdin(t, root+"\nn\n", "init")
	if err != nil {
		t.Fatalf("declining should not be an error: %v\n%s", err, out)
	}
	if _, err := os.Stat(root); err == nil {
		t.Error("the library was created even though the user said no")
	}
}

func TestInitRefusesAnExistingLibrary(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")
	if _, err := exec(t, "init", root, "--yes"); err != nil {
		t.Fatalf("first init: %v", err)
	}

	_, err := exec(t, "init", root, "--yes")
	if err == nil {
		t.Fatal("second init = nil error, want a refusal")
	}
	if !strings.Contains(err.Error(), "already") {
		t.Errorf("error = %q, want it to say the library already exists", err)
	}
}

func TestInitShowsWhereItWillGoBeforeAsking(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")

	// Declining still shows the location and the space — displaying them is
	// the entire reason for the confirmation step (spec 3.1). The link mode
	// cannot appear here: probing it needs the directories to exist.
	out, err := execStdin(t, root+"\nn\n", "init")
	if err != nil {
		t.Fatalf("kist init: %v", err)
	}
	for _, want := range []string{"Location", "Free space"} {
		if !strings.Contains(out, want) {
			t.Errorf("init did not show %q before asking:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Link mode") {
		t.Errorf("init claimed a link mode without having created anything:\n%s", out)
	}
}
