package store

import (
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

func localDate(t *testing.T, s string) domain.LocalDate {
	t.Helper()
	d, err := domain.ParseLocalDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// An empty query filters nothing, and must not emit a stray AND.
func TestTranslateEmptyQuery(t *testing.T) {
	where, args, err := translate(query.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(where, "AND") || len(args) != 0 {
		t.Errorf("where = %q, args = %v; want no conditions", where, args)
	}
}

// LIKE without ESCAPE is the bug where searching for "100%" matches
// everything. It stays invisible until somebody types a percent sign.
func TestTextSearchEscapesWildcards(t *testing.T) {
	where, args, err := translate(query.Query{Text: "100%_x"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(where, `ESCAPE '\'`) {
		t.Errorf("no ESCAPE clause: %q", where)
	}
	if !strings.Contains(where, "name_fold") {
		t.Errorf("text search does not use the folded column (spec 4.7): %q", where)
	}
	pattern, _ := args[0].(string)
	if !strings.Contains(pattern, `\%`) || !strings.Contains(pattern, `\_`) {
		t.Errorf("pattern %q does not neutralise the wildcards", pattern)
	}
}

// The pattern is folded with the same function that built the column, or
// case-insensitivity becomes the engine's decision instead of ours (spec 4.7).
func TestTextSearchFoldsThePattern(t *testing.T) {
	_, args, err := translate(query.Query{Text: "IMG"})
	if err != nil {
		t.Fatal(err)
	}
	if got := args[0].(string); got != "%"+domain.Fold("IMG")+"%" {
		t.Errorf("pattern = %q, want the folded form", got)
	}
}

// The empty-set algebra, at the SQL level. Getting one of these backwards
// silently widens or narrows every query built with no tags.
func TestTagPredicateEmptySets(t *testing.T) {
	cases := []struct {
		name string
		expr query.TagExpr
		want string
	}{
		{"an empty AND is true", query.TagAll{}, "1"},
		{"an empty OR is false", query.TagAnyOf{}, "0"},
		{"an empty leaf is false", query.TagAny{}, "0"},
		{"NOT of an empty leaf is true", query.TagNot{Expr: query.TagAny{}}, "NOT (0)"},
		{"nil is no filter", nil, "1"},
	}
	for _, c := range cases {
		got, args := tagPredicate(c.expr)
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
		if len(args) != 0 {
			t.Errorf("%s: emitted args %v", c.name, args)
		}
	}
}

// Nesting has to come out bracketed, or precedence changes the meaning:
// "a AND (b OR c)" written without brackets is "(a AND b) OR c".
func TestTagPredicateParenthesisesNesting(t *testing.T) {
	a, b, c := domain.NewTagID(), domain.NewTagID(), domain.NewTagID()
	got, args := tagPredicate(query.TagAll{
		query.TagAny{a},
		query.TagAnyOf{query.TagAny{b}, query.TagAny{c}},
	})
	if !strings.Contains(got, " OR ") || !strings.Contains(got, " AND ") {
		t.Fatalf("predicate lost the structure: %q", got)
	}
	if strings.Count(got, "(") != strings.Count(got, ")") {
		t.Errorf("unbalanced brackets: %q", got)
	}
	if len(args) != 3 {
		t.Errorf("%d args, want 3", len(args))
	}
	// The OR has to be grouped as one operand of the AND. Without the
	// brackets the string still contains both words and still balances, so
	// the assertion has to be about grouping specifically.
	inner, _ := tagPredicate(query.TagAnyOf{query.TagAny{b}, query.TagAny{c}})
	if !strings.Contains(got, inner) || !strings.HasPrefix(inner, "(") {
		t.Errorf("the nested OR was not grouped: %q", got)
	}
}

// A single child needs no brackets, and adding them anyway would make every
// predicate in a report harder to read for no gain.
func TestTagPredicateDoesNotBracketASingleChild(t *testing.T) {
	got, _ := tagPredicate(query.TagAll{query.TagAny{domain.NewTagID()}})
	if strings.HasPrefix(got, "(") {
		t.Errorf("a one-child AND was bracketed: %q", got)
	}
}

// The overlap predicate for variable-precision dates (spec 4.2.1). Comparing
// the value against only the first length(value) characters of the lower bound
// is what makes "2019" overlap "2019-03..2019-06".
func TestTimeRangeUsesOverlapNotContainment(t *testing.T) {
	where, args, err := translate(query.Query{Time: &query.DateRange{
		From: localDate(t, "2019-03"), To: localDate(t, "2019-06"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(where, "substr(") || !strings.Contains(where, "length(captured_at)") {
		t.Errorf("not the overlap form: %q", where)
	}
	if len(args) != 2 {
		t.Fatalf("%d args, want 2: %v", len(args), args)
	}
	// The bounds are the interval ends, padded to the widest precision — a
	// bare "2019-06-30" would sort before a stored "2019-06-30T14:00".
	if args[0] != "2019-06-30T23:59" {
		t.Errorf("upper bound = %v, want the end of June", args[0])
	}
	if args[1] != "2019-03-01T00:00" {
		t.Errorf("lower bound = %v, want the start of March", args[1])
	}
}

func TestTimeRangeOpenEnds(t *testing.T) {
	_, args, err := translate(query.Query{Time: &query.DateRange{From: localDate(t, "2019")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 1 {
		t.Errorf("an open-ended range emitted %d args", len(args))
	}
	_, args, err = translate(query.Query{Time: &query.DateRange{To: localDate(t, "2019")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 1 {
		t.Errorf("an open-started range emitted %d args", len(args))
	}
	// A range with neither end is no filter at all.
	where, args, err := translate(query.Query{Time: &query.DateRange{}})
	if err != nil {
		t.Fatal(err)
	}
	if where != "" || len(args) != 0 {
		t.Errorf("an empty range emitted %q / %v", where, args)
	}
}

// A file with no capture time is in no time range (spec 4.2.1). The predicate
// has to say so: NULL comparisons are neither true nor false, and leaving it
// implicit makes the answer depend on the engine.
func TestTimeRangeExcludesFilesWithNoCaptureTime(t *testing.T) {
	where, _, err := translate(query.Query{Time: &query.DateRange{From: localDate(t, "2019")}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(where, "captured_at IS NOT NULL") {
		t.Errorf("nulls are not excluded from the range: %q", where)
	}
}

// A tag set larger than the engine's parameter limit must be a clear error
// rather than a driver failure quoting a number nobody can read (spec 4.3).
func TestTooManyTagIdsIsAClearError(t *testing.T) {
	huge := make(query.TagAny, maxParameters+1)
	for i := range huge {
		huge[i] = domain.NewTagID()
	}
	if _, _, err := translate(query.Query{Tags: huge}); err == nil {
		t.Error("a tag set beyond the parameter limit was accepted")
	}
	// And inside a NOT, which is where an exclusion of a huge subtree lands.
	if _, _, err := translate(query.Query{Tags: query.TagNot{Expr: huge}}); err == nil {
		t.Error("a huge tag set inside a NOT was accepted")
	}
}

// An invalid query is refused by the translator too, so every backend rejects
// the same things (spec 4.2.1).
func TestTranslateValidatesFirst(t *testing.T) {
	if _, _, err := translate(query.Query{Limit: -1}); err == nil {
		t.Error("a negative limit reached the SQL builder")
	}
}
