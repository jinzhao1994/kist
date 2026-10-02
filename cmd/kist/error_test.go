package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

// mustErrorJSON parses what writeError produced.
func mustErrorJSON(t *testing.T, out string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("the error is not JSON: %v\n%s", err, out)
	}
	return v
}

// writeError renders through the same path main does, so a test can drive it
// without a subprocess.
func renderJSON(t *testing.T, err error) map[string]any {
	t.Helper()
	jsonFlag = true
	defer func() { jsonFlag = false }()
	var sb strings.Builder
	writeError(&sb, err)
	return mustErrorJSON(t, sb.String())
}

// The whole point of spec 2.9: the code is the contract, and the data a client
// needs comes as fields rather than as English it has to parse.
func TestErrorJSONCarriesItsFields(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		code  string
		class string
		field string
		want  any
	}{
		{
			"an undo names the op in the way",
			&library.BlockedUndoError{OpID: 47, BlockingOp: 51},
			"undo_blocked", "conflict", "blocking_op", float64(51),
		},
		{
			"a repeated undo names the one that already ran",
			&library.AlreadyRevertedError{OpID: 47, RevertedBy: 48},
			"already_reverted", "conflict", "reverted_by", float64(48),
		},
		{
			"a taken name says what holds it",
			&library.NameTakenError{Name: "地点", ExistingID: "abc"},
			"tag_name_taken", "conflict", "existing_id", "abc",
		},
		{
			"a busy library says who is writing",
			&lockfile.BusyError{Holder: "pid 42 kist import"},
			"library_busy", "unavailable", "holder", "pid 42 kist import",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := renderJSON(t, c.err)
			if v["code"] != c.code {
				t.Errorf("code = %v, want %q", v["code"], c.code)
			}
			if v["class"] != c.class {
				t.Errorf("class = %v, want %q", v["class"], c.class)
			}
			if v[c.field] != c.want {
				t.Errorf("%s = %#v, want %#v", c.field, v[c.field], c.want)
			}
		})
	}
}

// An ambiguous reference has to offer the choice. Reporting only "that matches
// several things" leaves the user to go find them by hand.
func TestAmbiguousErrorListsCandidates(t *testing.T) {
	v := renderJSON(t, &library.AmbiguousRefError{
		Ref:  "云南",
		Kind: "box",
		Candidates: []library.Candidate{
			{ID: "id-1", Name: "云南"},
			{ID: "id-2", Name: "云南"},
		},
	})
	got, _ := v["candidates"].([]any)
	if len(got) != 2 {
		t.Fatalf("candidates = %#v, want two", v["candidates"])
	}
	first, _ := got[0].(map[string]any)
	if first["id"] != "id-1" || first["name"] != "云南" {
		t.Errorf("first candidate = %#v", got[0])
	}
}

// Only a temporary failure gets its own exit code, and it has to be the one
// cron understands.
func TestExitCodes(t *testing.T) {
	if got := exitCodeFor(&lockfile.BusyError{}); got != 75 {
		t.Errorf("a busy library exits %d, want 75 (EX_TEMPFAIL)", got)
	}
	if got := exitCodeFor(library.ErrNoSuchTag); got != 1 {
		t.Errorf("a missing tag exits %d, want 1", got)
	}
	if got := exitCodeFor(errors.New("boom")); got != 1 {
		t.Errorf("an unclassified error exits %d, want 1", got)
	}
	// verify's severities are a different axis and must survive untouched: a
	// command that ran fine and found damage is not a command that failed.
	if got := exitCodeFor(&exitError{code: 2, msg: "damaged"}); got != 2 {
		t.Errorf("verify's severity exit became %d, want 2", got)
	}
}

// Without --json an error is one line of prose, as it always was.
func TestErrorWithoutJSONIsProse(t *testing.T) {
	var sb strings.Builder
	writeError(&sb, library.ErrNoSuchTag)
	if !strings.HasPrefix(sb.String(), "kist: ") {
		t.Errorf("error line = %q", sb.String())
	}
	var v any
	if json.Unmarshal([]byte(sb.String()), &v) == nil {
		t.Errorf("the error was JSON without being asked: %q", sb.String())
	}
}

// `kist find --json > out.json` must not leave a file whose contents are an
// error object waiting to be parsed as data. stdout is a result or it is
// nothing (spec 2.9).
func TestAFailingCommandWritesNothingToStdout(t *testing.T) {
	lib := newLibrary(t)
	out, _, err := execSplit(t, lib, "--json", "tag", "rename", "没有这个", "x")
	if err == nil {
		t.Fatal("renaming a tag that does not exist succeeded")
	}
	if out != "" {
		t.Errorf("stdout was not empty on failure:\n%s", out)
	}
}

func TestWriteOutcomeIsNotRetryable(t *testing.T) {
	for _, id := range []int64{0, 47} {
		err := &library.WriteOutcomeError{OpID: id, Cause: &lockfile.BusyError{}}
		if exitCodeFor(err) == 75 {
			t.Fatal("committed or uncertain writes must not suggest retry")
		}
		v := renderJSON(t, err)
		if id == 0 {
			if v["code"] != "write_outcome_unknown" || v["op_id"] != nil {
				t.Fatalf("unknown: %v", v)
			}
		} else if v["code"] != "write_committed" || v["op_id"] != float64(id) {
			t.Fatalf("committed: %v", v)
		}
	}
}
