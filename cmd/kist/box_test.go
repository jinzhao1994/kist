package main

import (
	"context"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/library"
)

// newLibrary makes an initialised library and returns the flag that points at
// it, so every test drives the real CLI surface.
func newLibrary(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if _, err := library.Create(context.Background(), root); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return "--library=" + root
}

func TestBoxNewPrintsTheID(t *testing.T) {
	lib := newLibrary(t)

	out, err := exec(t, lib, "box", "new", "云南 2019")
	if err != nil {
		t.Fatalf("box new: %v\n%s", err, out)
	}
	if !strings.Contains(out, "云南 2019") {
		t.Errorf("output does not name the box:\n%s", out)
	}
	// The id has to be printed: nothing else can reference the box yet.
	if !strings.Contains(out, "-") || len(strings.TrimSpace(out)) < 36 {
		t.Errorf("output does not look like it contains an id:\n%s", out)
	}

	list, err := exec(t, lib, "box", "list")
	if err != nil {
		t.Fatalf("box list: %v", err)
	}
	if !strings.Contains(list, "云南 2019") {
		t.Errorf("box list does not show the box:\n%s", list)
	}
}

func TestBoxNewWithDates(t *testing.T) {
	lib := newLibrary(t)
	out, err := exec(t, lib, "box", "new", "婚礼", "--start", "2019-08-12", "--end", "2019-08-12")
	if err != nil {
		t.Fatalf("box new: %v\n%s", err, out)
	}
	list, _ := exec(t, lib, "box", "list")
	if !strings.Contains(list, "2019-08-12") {
		t.Errorf("box list does not show the date:\n%s", list)
	}
}

func TestBoxNewRejectsABadDate(t *testing.T) {
	lib := newLibrary(t)
	if _, err := exec(t, lib, "box", "new", "x", "--start", "2019-8"); err == nil {
		t.Error("a non-zero-padded date was accepted")
	}
}

func TestBoxEdit(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "box", "new", "before")

	if _, err := exec(t, lib, "box", "edit", "before", "--name", "after"); err != nil {
		t.Fatalf("box edit: %v", err)
	}
	list, _ := exec(t, lib, "box", "list")
	if strings.Contains(list, "before") || !strings.Contains(list, "after") {
		t.Errorf("the rename did not take:\n%s", list)
	}
}

func TestBoxEditKeepsUntouchedFields(t *testing.T) {
	// Editing one field must not blank the others — the classic CLI trap.
	lib := newLibrary(t)
	exec(t, lib, "box", "new", "trip", "--start", "2019-08", "--description", "a note")

	if _, err := exec(t, lib, "box", "edit", "trip", "--name", "renamed"); err != nil {
		t.Fatalf("box edit: %v", err)
	}
	list, _ := exec(t, lib, "box", "list")
	if !strings.Contains(list, "2019-08") {
		t.Errorf("the start date was lost:\n%s", list)
	}
}

func TestBoxTagAndUntag(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "box", "new", "trip")
	exec(t, lib, "tag", "new", "云南")

	out, err := exec(t, lib, "box", "tag", "trip", "云南")
	if err != nil {
		t.Fatalf("box tag: %v\n%s", err, out)
	}
	tree, _ := exec(t, lib, "tag", "tree")
	if !strings.Contains(tree, "云南 (1)") {
		t.Errorf("tag tree does not show the count:\n%s", tree)
	}

	if _, err := exec(t, lib, "box", "untag", "trip", "云南"); err != nil {
		t.Fatalf("box untag: %v", err)
	}
	tree, _ = exec(t, lib, "tag", "tree")
	if !strings.Contains(tree, "云南 (0)") {
		t.Errorf("the count did not come back down:\n%s", tree)
	}
}

func TestBoxTagReportsAnAmbiguousName(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "box", "new", "重名")
	exec(t, lib, "box", "new", "重名")
	exec(t, lib, "tag", "new", "云南")

	_, err := exec(t, lib, "box", "tag", "重名", "云南")
	if err == nil {
		t.Fatal("an ambiguous box name was accepted")
	}
	if !strings.Contains(err.Error(), "matches") {
		t.Errorf("error = %q, want it to explain the ambiguity", err)
	}
}

func TestCommandsRefuseAMissingLibrary(t *testing.T) {
	out, err := exec(t, "--library="+t.TempDir(), "box", "list")
	if err == nil {
		t.Fatalf("box list on a non-library succeeded:\n%s", out)
	}
	if !strings.Contains(err.Error(), "kist init") {
		t.Errorf("error = %q, want it to point at `kist init`", err)
	}
}
