package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/query"
)

// The CLI's job is turning flags into a Query. What is worth testing here is
// the translation — the query itself has a whole conformance suite.
func TestParseSort(t *testing.T) {
	cases := map[string]query.SortKey{
		"captured": query.SortCaptured,
		"imported": query.SortImported,
		"updated":  query.SortUpdated,
		"name":     query.SortName,
	}
	for name, want := range cases {
		got, err := parseSort(name, false)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got.Key != want || got.Asc {
			t.Errorf("%s -> %+v", name, got)
		}
	}
	if got, _ := parseSort("name", true); !got.Asc {
		t.Error("--asc was not applied")
	}
}

// An unknown --sort must name the valid values. The contract's own message is
// "unknown sort key 99", which is true and useless to somebody who typed
// --sort date.
func TestParseSortRejectsUnknownWithAUsefulMessage(t *testing.T) {
	_, err := parseSort("date", false)
	if err == nil {
		t.Fatal("an unknown sort was accepted")
	}
	for _, want := range []string{"captured", "imported", "name", "updated"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the message does not offer %q: %v", want, err)
		}
	}
}

func TestParseRange(t *testing.T) {
	if r, err := parseRange("", ""); err != nil || r != nil {
		t.Errorf("no flags gave %+v, %v; want nil", r, err)
	}
	r, err := parseRange("2019-03", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.From.String() != "2019-03" || !r.To.IsZero() {
		t.Errorf("--since alone gave %+v", r)
	}
	if _, err := parseRange("not-a-date", ""); err == nil {
		t.Error("a bad --since was accepted")
	}
	if _, err := parseRange("", "20190812"); err == nil {
		t.Error("a bad --until was accepted")
	}
}

// Either half of the tag condition may be absent, and nil must not be confused
// with "a condition matching nothing".
func TestCombineTagConditions(t *testing.T) {
	if got := combineTagConditions(nil, nil); got != nil {
		t.Errorf("no conditions gave %#v, want nil", got)
	}
	req := query.TagAll{query.TagAny{}}
	if got := combineTagConditions(req, nil); got == nil {
		t.Error("the required half was dropped")
	}
	excl := query.TagNot{Expr: query.TagAny{}}
	if got := combineTagConditions(nil, excl); got == nil {
		t.Error("the excluded half was dropped")
	}
	both := combineTagConditions(req, excl)
	all, ok := both.(query.TagAll)
	if !ok || len(all) != 2 {
		t.Errorf("both halves gave %#v", both)
	}
}

// An unknown tag is a mistake worth naming: returning nothing looks exactly
// like "you have no such files" (the rule M1c established).
func TestFindRejectsAnUnknownTag(t *testing.T) {
	root := libraryWithAFile(t)
	out, err := runKist(t, "", "--library", root, "find", "--tag", "没有这个")
	if err == nil {
		t.Fatalf("an unknown tag was accepted:\n%s", out)
	}
}

// End to end: the flags reach the query and the query reaches the database.
func TestFindSortsByName(t *testing.T) {
	root := libraryWithAFile(t)
	out, err := runKist(t, "", "--library", root, "find", "--sort", "name", "--asc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "a.txt") || !strings.Contains(out, "b.txt") {
		t.Fatalf("both files should be listed:\n%s", out)
	}
	if strings.Index(out, "a.txt") > strings.Index(out, "b.txt") {
		t.Errorf("--sort name --asc did not sort:\n%s", out)
	}
}

// A time-filtered search finds nothing until M3, and that is the truth rather
// than a defect (spec 4.2.1) — but the command must still work.
// Time filtering used to be asserted as empty: no file had a capture time
// until M3's extractor ran, so the honest behaviour was to return nothing.
//
// That premise is gone. Import now records the source file's modification
// time and resolves a capture time from it (spec 1.7), so an ordinary text
// file has one from the moment it lands — which is the point of the milestone,
// and the reason this test is now about the range working rather than about it
// being vacuous.
func TestFindWithATimeRange(t *testing.T) {
	root := libraryWithAFile(t)

	// The fixture was written just now, so this year finds it.
	year := strconv.Itoa(time.Now().Year())
	out, err := runKist(t, "", "--library", root, "find", "--since", year)
	if err != nil {
		t.Fatalf("a time-filtered search failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, ".txt") {
		t.Errorf("the file was not found in the year it was written:\n%s", out)
	}

	out, err = runKist(t, "", "--library", root, "find", "--since", "1990", "--until", "1991")
	if err != nil {
		t.Fatalf("a time-filtered search failed: %v\n%s", err, out)
	}
	if strings.Contains(out, ".txt") {
		t.Errorf("a range that contains nothing matched anyway:\n%s", out)
	}
}
