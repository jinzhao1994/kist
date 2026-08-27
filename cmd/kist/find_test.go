package main

import (
	"strings"
	"testing"
)

// spec 4.4: the text search is a folded substring over file names.
func TestFindMatchesNameCaseInsensitively(t *testing.T) {
	lib := newLibrary(t)
	if _, err := exec(t, lib, "box", "new", "云南"); err != nil {
		t.Fatal(err)
	}
	src := writeTree(t, map[string]string{"IMG_0001.JPG": "x", "notes.txt": "y"})
	if _, err := exec(t, lib, "import", "--box", "云南", src); err != nil {
		t.Fatal(err)
	}

	out, err := exec(t, lib, "find", "--name", "img_00")
	if err != nil {
		t.Fatalf("find: %v\n%s", err, out)
	}
	if !strings.Contains(out, "IMG_0001.JPG") {
		t.Errorf("the JPG was not found:\n%s", out)
	}
	if strings.Contains(out, "notes.txt") {
		t.Errorf("an unrelated file matched:\n%s", out)
	}
}

// Output shows the path inside the box, because a bare file name is not enough
// to go find the thing under boxes/.
func TestFindPrintsTheBoxRelativePath(t *testing.T) {
	lib := newLibrary(t)
	if _, err := exec(t, lib, "box", "new", "云南"); err != nil {
		t.Fatal(err)
	}
	src := writeTree(t, map[string]string{"day1/raw/photo.jpg": "x"})
	if _, err := exec(t, lib, "import", "--box", "云南", src); err != nil {
		t.Fatal(err)
	}

	out, err := exec(t, lib, "find", "--name", "photo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "day1/raw/photo.jpg") {
		t.Errorf("the result is not a path:\n%s", out)
	}
}

// spec 4.3: several tags intersect, and a tag includes its descendants.
func TestFindIntersectsTagsAndIncludesDescendants(t *testing.T) {
	lib := newLibrary(t)
	if _, err := exec(t, lib, "box", "new", "云南"); err != nil {
		t.Fatal(err)
	}
	src := writeTree(t, map[string]string{"a.jpg": "x", "b.jpg": "y"})
	if _, err := exec(t, lib, "import", "--box", "云南", src); err != nil {
		t.Fatal(err)
	}
	if _, err := exec(t, lib, "tag", "new", "人物"); err != nil {
		t.Fatal(err)
	}
	if _, err := exec(t, lib, "tag", "new", "爸", "--parent", "人物"); err != nil {
		t.Fatal(err)
	}
	if _, err := exec(t, lib, "tag", "new", "隐私"); err != nil {
		t.Fatal(err)
	}

	ids := fileIDs(t, lib)
	if _, err := exec(t, lib, "file", "tag", "爸", ids["a.jpg"]); err != nil {
		t.Fatal(err)
	}
	if _, err := exec(t, lib, "file", "tag", "爸", ids["b.jpg"]); err != nil {
		t.Fatal(err)
	}
	if _, err := exec(t, lib, "file", "tag", "隐私", ids["b.jpg"]); err != nil {
		t.Fatal(err)
	}

	// Searching the parent tag finds files carrying its child.
	out, err := exec(t, lib, "find", "--tag", "人物")
	if err != nil {
		t.Fatalf("find --tag: %v\n%s", err, out)
	}
	if !strings.Contains(out, "a.jpg") || !strings.Contains(out, "b.jpg") {
		t.Errorf("the parent tag did not expand to its descendants:\n%s", out)
	}

	out, err = exec(t, lib, "find", "--tag", "人物", "--not-tag", "隐私")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "a.jpg") || strings.Contains(out, "b.jpg") {
		t.Errorf("--not-tag did not exclude:\n%s", out)
	}
}

// fileIDs maps each file's name onto its id, via find.
func fileIDs(t *testing.T, lib string) map[string]string {
	t.Helper()
	out, err := exec(t, lib, "find")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		parts := strings.Split(fields[1], "/")
		got[parts[len(parts)-1]] = fields[0]
	}
	return got
}

// An unknown tag name is a mistake worth naming: silently returning nothing
// looks exactly like "you have no such files".
func TestFindRejectsAnUnknownTag(t *testing.T) {
	lib := newLibrary(t)
	out, err := exec(t, lib, "find", "--tag", "没有这个标签")
	if err == nil {
		t.Fatalf("an unknown tag returned an empty result instead of an error:\n%s", out)
	}
	if !strings.Contains(err.Error(), "没有这个标签") {
		t.Errorf("the error does not name the tag: %v", err)
	}
}

func TestFileTagAndUntag(t *testing.T) {
	lib := newLibrary(t)
	if _, err := exec(t, lib, "box", "new", "云南"); err != nil {
		t.Fatal(err)
	}
	src := writeTree(t, map[string]string{"a.jpg": "x"})
	if _, err := exec(t, lib, "import", "--box", "云南", src); err != nil {
		t.Fatal(err)
	}
	if _, err := exec(t, lib, "tag", "new", "云南省"); err != nil {
		t.Fatal(err)
	}
	id := fileIDs(t, lib)["a.jpg"]

	out, err := exec(t, lib, "file", "tag", "云南省", id)
	if err != nil {
		t.Fatalf("file tag: %v\n%s", err, out)
	}
	if !strings.Contains(out, "tagged 1") {
		t.Errorf("output = %q", out)
	}
	if out, _ := exec(t, lib, "find", "--tag", "云南省"); !strings.Contains(out, "a.jpg") {
		t.Errorf("the tag did not take:\n%s", out)
	}

	// Re-tagging changes nothing and says so, rather than writing an op that
	// would undo something it never did.
	out, err = exec(t, lib, "file", "tag", "云南省", id)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "tagged 0") {
		t.Errorf("re-tagging reported %q", out)
	}

	if out, err := exec(t, lib, "file", "untag", "云南省", id); err != nil {
		t.Fatalf("file untag: %v\n%s", err, out)
	}
	if out, _ := exec(t, lib, "find", "--tag", "云南省"); strings.Contains(out, "a.jpg") {
		t.Errorf("the tag survived untagging:\n%s", out)
	}
}

func TestFileTagRejectsANonID(t *testing.T) {
	lib := newLibrary(t)
	if _, err := exec(t, lib, "tag", "new", "云南"); err != nil {
		t.Fatal(err)
	}
	if _, err := exec(t, lib, "file", "tag", "云南", "not-an-id"); err == nil {
		t.Error("a non-id was accepted as a file reference")
	}
}
