package integrity

import (
	"strings"
	"testing"
)

// The report is what a worried user reads. It has to be ordered — severity
// first, then kind — so that the thing most likely to be data loss is not
// buried under two hundred lines of "this is only wasting space".
func TestReportPutsTheWorstFirst(t *testing.T) {
	var r Report
	r.Add(Finding{Kind: BlobOrphan, Detail: "sha256:aa"})
	r.Add(Finding{Kind: BlobMissing, Detail: "sha256:bb"})
	r.Add(Finding{Kind: LinkStray, Detail: "boxes/x/junk.txt"})

	var sb strings.Builder
	if err := r.Write(&sb); err != nil {
		t.Fatal(err)
	}
	out := sb.String()
	missing := strings.Index(out, "sha256:bb")
	stray := strings.Index(out, "boxes/x/junk.txt")
	orphan := strings.Index(out, "sha256:aa")
	if missing < 0 || stray < 0 || orphan < 0 {
		t.Fatalf("something is not in the report:\n%s", out)
	}
	if !(missing < stray && stray < orphan) {
		t.Errorf("wrong order — error, then warning, then info:\n%s", out)
	}
}

// Worst is what the exit code is built from. Info-level findings must not make
// verify fail: a library with orphan blobs is a normal library (spec 3.9), and
// a check that cries wolf on every run gets taken out of cron.
func TestWorstIgnoresNothingButReportsHonestly(t *testing.T) {
	var r Report
	if got := r.Worst(); got != SeverityNone {
		t.Errorf("an empty report is %v, want SeverityNone", got)
	}
	r.Add(Finding{Kind: BlobOrphan})
	if got := r.Worst(); got != SeverityInfo {
		t.Errorf("orphan blobs are %v, want SeverityInfo", got)
	}
	r.Add(Finding{Kind: LinkStray})
	if got := r.Worst(); got != SeverityWarn {
		t.Errorf("a stray link is %v, want SeverityWarn", got)
	}
	r.Add(Finding{Kind: BlobMissing})
	if got := r.Worst(); got != SeverityError {
		t.Errorf("a missing blob is %v, want SeverityError", got)
	}
}

// A library with fifty thousand missing blobs must not print fifty thousand
// lines. The count is the finding; the examples are the evidence.
func TestReportCapsTheExamples(t *testing.T) {
	var r Report
	for i := 0; i < 500; i++ {
		r.Add(Finding{Kind: BlobMissing, Detail: "blob"})
	}
	var sb strings.Builder
	if err := r.Write(&sb); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(sb.String(), "blob"); n > examplesPerKind+1 {
		t.Errorf("printed %d examples, want at most %d", n, examplesPerKind)
	}
	if !strings.Contains(sb.String(), "500") {
		t.Errorf("the total is missing from the report:\n%s", sb.String())
	}
}

// Merge is how three layers become one report. The counts must survive it —
// including the ones past the example cap, or a library with a thousand
// missing blobs would report ten.
func TestMergeKeepsTheWholeTally(t *testing.T) {
	var blobs Report
	for i := 0; i < 1000; i++ {
		blobs.Add(Finding{Kind: BlobMissing, Detail: "x"})
	}
	blobs.Note("blobs", 1000)

	var all Report
	all.Add(Finding{Kind: LinkStray, Detail: "junk"})
	all.Merge(blobs)

	if got := all.Count(BlobMissing); got != 1000 {
		t.Errorf("merged count = %d, want 1000", got)
	}
	if got := all.Total(); got != 1001 {
		t.Errorf("total = %d, want 1001", got)
	}
	if got := all.Worst(); got != SeverityError {
		t.Errorf("worst = %v, want SeverityError", got)
	}
	if got := all.Checked["blobs"]; got != 1000 {
		t.Errorf("Checked[blobs] = %d, want 1000", got)
	}
}

// A clean library must say so in as many words. "No output means fine" is a
// terrible answer to "is my archive OK?".
func TestACleanReportSaysSo(t *testing.T) {
	var r Report
	var sb strings.Builder
	if err := r.Write(&sb); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "no problems") {
		t.Errorf("a clean report reads %q", sb.String())
	}
}

// Every kind must carry a sentence and a severity. A finding that prints as
// "kind 7" is a finding nobody can act on, and the compiler will not notice
// that a new constant was added to the list but not to the table.
func TestEveryKindHasASentence(t *testing.T) {
	for k := Kind(0); k < numKinds; k++ {
		if k.Headline() == "" {
			t.Errorf("kind %d has no headline", k)
		}
		if k.Severity() == SeverityNone {
			t.Errorf("kind %d (%s) has no severity", k, k.Headline())
		}
	}
}
