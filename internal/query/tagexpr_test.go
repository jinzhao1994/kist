package query

import (
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

// The empty-set semantics, written down because the standard algebraic answer
// is exactly the thing people get backwards when writing a translator: an
// empty AND is the identity and matches everything, an empty OR or an empty
// leaf matches nothing.
//
// Getting one of these wrong does not error. It silently widens or narrows
// every query built by a caller that happened to pass no tags — which is the
// common case, because "the user ticked nothing" is the common case.
func TestEmptyExpressionsHaveDefinedMeanings(t *testing.T) {
	cases := []struct {
		name string
		expr TagExpr
		want bool // matches everything?
	}{
		{"an empty AND is the identity", TagAll{}, true},
		{"an empty OR matches nothing", TagAnyOf{}, false},
		{"an empty leaf matches nothing", TagAny{}, false},
	}
	for _, c := range cases {
		if got := MatchesEverything(c.expr); got != c.want {
			t.Errorf("%s: MatchesEverything = %v, want %v", c.name, got, c.want)
		}
	}
}

// nil is not an expression; it is the absence of one, and it must not be
// confused with "an expression that matches nothing".
func TestNilExpressionIsNoFilter(t *testing.T) {
	if !MatchesEverything(nil) {
		t.Error("a nil TagExpr filtered something out")
	}
}

// An AND of identities is still an identity, which is what lets a caller build
// a condition list incrementally without special-casing the empty step.
func TestNestedIdentitiesAreStillIdentities(t *testing.T) {
	if !MatchesEverything(TagAll{TagAll{}, TagAll{TagAll{}}}) {
		t.Error("an AND of empty ANDs was treated as a filter")
	}
}

// Nesting has to survive being written down. This is the property the tree
// exists for, and v0.1 has no UI that produces one — the conformance suite is
// its only exerciser, which is the point (spec 4.3).
func TestNestingComposes(t *testing.T) {
	a, b, c, d := domain.NewTagID(), domain.NewTagID(), domain.NewTagID(), domain.NewTagID()
	e := TagAll{
		TagAny{a},
		TagAnyOf{TagAny{b}, TagAny{c}},
		TagNot{Expr: TagAny{d}},
	}
	if MatchesEverything(e) {
		t.Error("a tree with real conditions was treated as no filter")
	}
	if got := Leaves(e); len(got) != 4 {
		t.Errorf("walked %d leaves, want 4", len(got))
	}
}

// Leaves has to reach inside a NOT too, or the parameter-limit check misses
// exactly the condition somebody built by excluding a huge tag subtree.
func TestLeavesReachesInsideNot(t *testing.T) {
	if got := Leaves(TagNot{Expr: TagAny{domain.NewTagID(), domain.NewTagID()}}); len(got) != 1 || len(got[0]) != 2 {
		t.Errorf("Leaves of a NOT = %v", got)
	}
}
