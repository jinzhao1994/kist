package store

import (
	"strings"
	"testing"
	"time"

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
	where, args, err := translate(query.Query{}, time.UTC)
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
	where, args, err := translate(query.Query{Text: "100%_x"}, time.UTC)
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
	_, args, err := translate(query.Query{Text: "IMG"}, time.UTC)
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

// A file's capture time is an instant (spec 1.5.2), so the range the caller
// gave — a variable-precision LocalDate — becomes two instants here.
//
// This replaced a string-overlap predicate that compared the stored value
// against only the first length(value) characters of the lower bound. That
// form existed because captured_at used to be variable-precision too; against
// an instant the overlap degenerates into containment, and the predicate gets
// simpler rather than more complex. Box dates keep the overlap.
func TestTimeRangeTurnsDatesIntoInstants(t *testing.T) {
	where, args, err := translate(query.Query{Time: &query.DateRange{
		From: localDate(t, "2019-03"), To: localDate(t, "2019-06"),
	}}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(where, "substr(") {
		t.Errorf("still the string-overlap form: %q", where)
	}
	if len(args) != 2 {
		t.Fatalf("%d args, want 2: %v", len(args), args)
	}
	// The bounds are the ends of the interval the caller named, not the ends
	// of the day they happen to start on.
	if want := time.Date(2019, 6, 30, 23, 59, 0, 0, time.UTC).Unix(); args[0] != want {
		t.Errorf("upper bound = %v, want %d (the end of June)", args[0], want)
	}
	if want := time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC).Unix(); args[1] != want {
		t.Errorf("lower bound = %v, want %d (the start of March)", args[1], want)
	}
}

// The zone is a parameter, and it has to actually be used. A translator that
// read time.Local would return different rows for the same query on two
// machines — which is the exact class of divergence spec 4.7 exists to
// prevent, and the kind nobody notices until two people compare screenshots.
func TestTimeRangeIsInterpretedInTheGivenZone(t *testing.T) {
	q := query.Query{Time: &query.DateRange{From: localDate(t, "2019-03")}}

	_, west, err := translate(q, time.FixedZone("west", -8*3600))
	if err != nil {
		t.Fatal(err)
	}
	_, east, err := translate(q, time.FixedZone("east", 8*3600))
	if err != nil {
		t.Fatal(err)
	}
	if west[0] == east[0] {
		t.Errorf("the zone had no effect: both bounds are %v", west[0])
	}
	if diff := west[0].(int64) - east[0].(int64); diff != 16*3600 {
		t.Errorf("the two zones differ by %ds, want %d", diff, 16*3600)
	}
}

func TestTimeRangeOpenEnds(t *testing.T) {
	_, args, err := translate(query.Query{Time: &query.DateRange{From: localDate(t, "2019")}}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 1 {
		t.Errorf("an open-ended range emitted %d args", len(args))
	}
	_, args, err = translate(query.Query{Time: &query.DateRange{To: localDate(t, "2019")}}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 1 {
		t.Errorf("an open-started range emitted %d args", len(args))
	}
	// A range with neither end is no filter at all.
	where, args, err := translate(query.Query{Time: &query.DateRange{}}, time.UTC)
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
	where, _, err := translate(query.Query{Time: &query.DateRange{From: localDate(t, "2019")}}, time.UTC)
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
	if _, _, err := translate(query.Query{Tags: huge}, time.UTC); err == nil {
		t.Error("a tag set beyond the parameter limit was accepted")
	}
	// And inside a NOT, which is where an exclusion of a huge subtree lands.
	if _, _, err := translate(query.Query{Tags: query.TagNot{Expr: huge}}, time.UTC); err == nil {
		t.Error("a huge tag set inside a NOT was accepted")
	}
}

// An invalid query is refused by the translator too, so every backend rejects
// the same things (spec 4.2.1).
func TestTranslateValidatesFirst(t *testing.T) {
	if _, _, err := translate(query.Query{Limit: -1}, time.UTC); err == nil {
		t.Error("a negative limit reached the SQL builder")
	}
}
