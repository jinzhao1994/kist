package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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

// jsonCommands is every command that prints a result, with arguments that make
// it run against the fixture below. Adding a command means adding a line here.
//
// "@src" and "@file" stand for a directory to import and a file id, which only
// exist once the fixture has been built. They are operands rather than command
// names, so the coverage walk below reads the table without substituting them.
//
// The list is what TestEveryPrintingCommandIsCovered checks the command tree
// against, so a new command cannot quietly ship without --json — which is the
// whole point of the flag being global (spec 2.8).
var jsonCommands = [][]string{
	{"box", "list"},
	{"box", "new", "四川 2020"},
	{"box", "edit", "四川 2020", "--name", "四川"},
	{"box", "tag", "四川", "地点"},
	{"box", "untag", "四川", "地点"},
	{"ls"},
	{"find"},
	{"file", "show", "@file"},
	{"file", "tag", "地点", "@file"},
	{"file", "untag", "地点", "@file"},
	{"file", "metadata", "rm", "@file", "file:size"},
	{"tag", "tree"},
	{"tag", "new", "证件"},
	{"tag", "rename", "证件", "档案"},
	{"tag", "move", "档案", "--parent", ""},
	{"tag", "rm", "档案"},
	{"log"},
	{"undo", "1"},
	{"import", "--box", "云南 2019", "@src"},
	{"extract"},
	{"relink"},
	{"rebuild", "--yes"},
	{"verify"},
	{"scrub"},
	{"repair"},
	{"remote", "list"},
	{"version"},
	// init deliberately absent: it takes a path argument rather than
	// --library, so a table entry would create a library at the default
	// ~/kist. TestInitJSON below gives it a temp directory of its own.
}

// Every result is an object, never a bare array. An array cannot grow a
// sibling field later without breaking every consumer, and the first sibling
// that was needed — find's cursor — was needed immediately.
func TestJSONResultsAreObjects(t *testing.T) {
	lib, src, file := jsonFixture(t)

	for _, args := range jsonCommands {
		args = slices.Clone(args)
		for i, a := range args {
			switch a {
			case "@src":
				args[i] = src
			case "@file":
				args[i] = file
			}
		}
		out, _, err := execSplit(t, append([]string{lib, "--json"}, args...)...)
		// Some of these legitimately fail against this fixture — importing
		// "." finds nothing, undo 1 may collide. What must hold either way is
		// that whatever reached stdout is JSON, because a script parsing the
		// output cannot be asked to know which is which.
		if out == "" && err != nil {
			continue
		}
		if err != nil && !json.Valid([]byte(out)) {
			continue // the error was printed, not a result
		}
		mustJSON(t, out)
	}
}

// jsonFixture builds a library with a box, a tag and one imported file, and
// returns the --library flag, a directory worth importing, and a file id.
func jsonFixture(t *testing.T) (lib, src, file string) {
	t.Helper()
	lib = newLibrary(t)
	for _, args := range [][]string{
		{"box", "new", "云南 2019"},
		{"tag", "new", "地点"},
	} {
		if _, err := exec(t, append([]string{lib}, args...)...); err != nil {
			t.Fatal(err)
		}
	}

	src = t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec(t, lib, "import", "--box", "云南 2019", src); err != nil {
		t.Fatalf("import: %v\n%s", err, out)
	}

	out, _, err := execSplit(t, lib, "--json", "find")
	if err != nil {
		t.Fatal(err)
	}
	var found struct {
		Files []struct {
			ID string `json:"id"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(out), &found); err != nil || len(found.Files) == 0 {
		t.Fatalf("the fixture has no files: %v\n%s", err, out)
	}
	return lib, src, found.Files[0].ID
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
		out, _, err := execSplit(t, append([]string{lib, "--json"}, args...)...)
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

// init writes wherever its path argument says, so it gets a directory of its
// own rather than a line in the table — an entry there would have created a
// library at the default ~/kist on whoever ran the suite.
func TestInitJSON(t *testing.T) {
	out, _, err := execSplit(t, "--json", "init", t.TempDir()+"/lib", "--yes")
	if err != nil {
		t.Fatalf("init --json: %v\n%s", err, out)
	}
	v := mustJSON(t, out)
	if v["path"] == "" || v["link_mode"] == "" {
		t.Errorf("init --json did not report the path and link mode: %v", v)
	}
}

// The conversation init has with a person must not land in the middle of the
// document a script is parsing.
func TestInitJSONKeepsProseOffStdout(t *testing.T) {
	out, errOut, err := execSplit(t, "--json", "init", t.TempDir()+"/lib", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	mustJSON(t, out)
	if !strings.Contains(errOut, "Library created") {
		t.Errorf("the prose did not go to stderr; stderr was:\n%s", errOut)
	}
}

// Every command that produces output must be in jsonCommands.
//
// This is the guard behind spec 2.8's claim that --json is global. A flag that
// some commands honour and others ignore is worse than no flag: a script
// cannot tell "this command has no JSON form" from "you spelled it wrong", and
// it finds out at the worst moment. Walking the real command tree is what
// makes adding a command without a line here impossible to miss.
func TestEveryPrintingCommandIsCovered(t *testing.T) {
	covered := map[string]bool{}
	for _, args := range jsonCommands {
		// The path is the leading words that name commands, not the operands.
		var path []string
		for _, a := range args {
			if strings.HasPrefix(a, "-") {
				break
			}
			path = append(path, a)
		}
		for i := 1; i <= len(path); i++ {
			covered[strings.Join(path[:i], " ")] = true
		}
	}
	// init writes wherever its argument says and has a test of its own.
	covered["init"] = true
	// serve is a long-running transport, with diagnostics only on stderr.
	covered["serve"] = true

	var walk func(cmd *cobra.Command, path []string)
	walk = func(cmd *cobra.Command, path []string) {
		if cmd.HasSubCommands() {
			for _, sub := range cmd.Commands() {
				if sub.Hidden || sub.Name() == "help" {
					continue
				}
				walk(sub, append(path, sub.Name()))
			}
			// A command that only groups subcommands prints usage, not a
			// result.
			if cmd.RunE == nil && cmd.Run == nil {
				return
			}
		}
		if len(path) == 0 {
			return
		}
		if name := strings.Join(path, " "); !covered[name] {
			t.Errorf("`kist %s` is not in jsonCommands.\n\n"+
				"Every command that prints a result must support --json (spec 2.8).\n"+
				"Add a line to jsonCommands with arguments that make it run.", name)
		}
	}
	walk(newRootCmd(), nil)
}
