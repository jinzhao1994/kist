package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// mustJSON parses a command's stdout, failing with the output when it is not
// JSON — which is what "somebody printed with fmt.Fprintf instead of going
// through emit" looks like.
func mustJSON(t *testing.T, out string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("output is not a JSON object: %v\n%s", err, out)
	}
	return v
}

// Every result is an object, never a bare array. An array cannot grow a
// sibling field later without breaking every consumer, and the first sibling
// that was needed — find's cursor — was needed immediately.
func TestJSONResultsAreObjects(t *testing.T) {
	lib := newLibrary(t)
	if _, err := exec(t, lib, "box", "new", "云南 2019"); err != nil {
		t.Fatal(err)
	}
	if _, err := exec(t, lib, "tag", "new", "地点"); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"box", "list"},
		{"ls"},
		{"find"},
		{"tag", "tree"},
		{"log"},
	} {
		out, err := exec(t, append([]string{lib, "--json"}, args...)...)
		if err != nil {
			t.Fatalf("kist %s --json: %v\n%s", strings.Join(args, " "), err, out)
		}
		mustJSON(t, out)
	}
}

// The folded columns are how spec 4.7 gets case-insensitive matching without
// asking the database for a collation. They are a storage decision, and a
// client that saw them would be tempted to match on them.
func TestJSONNeverExposesFoldedColumns(t *testing.T) {
	lib := newLibrary(t)
	if _, err := exec(t, lib, "box", "new", "云南 2019", "--description", "Dali"); err != nil {
		t.Fatal(err)
	}
	if _, err := exec(t, lib, "tag", "new", "地点"); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"box", "list"}, {"tag", "tree"}} {
		out, err := exec(t, append([]string{lib, "--json"}, args...)...)
		if err != nil {
			t.Fatal(err)
		}
		for _, banned := range []string{"name_fold", "NameFold", "description_fold"} {
			if strings.Contains(out, banned) {
				t.Errorf("kist %s --json leaks %s:\n%s", strings.Join(args, " "), banned, out)
			}
		}
	}
}

// Without --json the output is for a person. A command that always emitted
// JSON would pass every test above and be unusable.
func TestWithoutTheFlagTheOutputIsNotJSON(t *testing.T) {
	lib := newLibrary(t)
	if _, err := exec(t, lib, "box", "new", "云南 2019"); err != nil {
		t.Fatal(err)
	}
	out, err := exec(t, lib, "box", "list")
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if json.Unmarshal([]byte(out), &v) == nil {
		t.Errorf("box list printed JSON without being asked:\n%s", out)
	}
	if !strings.Contains(out, "云南 2019") {
		t.Errorf("box list did not name the box:\n%s", out)
	}
}
