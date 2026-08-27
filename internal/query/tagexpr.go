package query

import "github.com/jinzhao1994/kist/internal/domain"

// TagExpr is a boolean condition over the tags a file carries.
//
// The leaves hold tag ids that are already expanded: spec 4.3's first step is
// a walk of the in-memory tag tree, done by library before a query is built,
// and the second step is the id set arriving here. Putting the expansion
// behind this interface would drag the tag tree into every implementation of
// the contract.
//
// v0.1 has no UI that produces anything but "several tags intersected plus an
// exclusion list" (spec 4.3). The tree exists anyway because this is the
// contract, and a contract that grows a case later is a contract its
// conformance suite was not written against. The suite being its only
// exerciser is the point rather than a gap.
type TagExpr interface {
	// isTagExpr keeps the set of cases closed, so an implementation can
	// translate by type switch and be sure it is exhaustive.
	isTagExpr()
}

// TagAny is the leaf: the file carries at least one of these tags.
//
// An empty TagAny matches nothing. That is the algebra — the disjunction of no
// alternatives is false — and it is the answer that makes "the user selected
// no tags" fail closed rather than quietly select everything.
type TagAny []domain.TagID

// TagAll matches when every child matches. Empty matches everything: it is the
// identity of AND, and it is what "no conditions" has to mean.
type TagAll []TagExpr

// TagAnyOf matches when any child matches. Empty matches nothing.
type TagAnyOf []TagExpr

// TagNot inverts its child.
type TagNot struct{ Expr TagExpr }

func (TagAny) isTagExpr()   {}
func (TagAll) isTagExpr()   {}
func (TagAnyOf) isTagExpr() {}
func (TagNot) isTagExpr()   {}

// MatchesEverything reports whether an expression filters nothing out, so a
// caller can skip building a predicate for it.
//
// It is deliberately conservative: it recognises nil and the identity forms
// and says false for anything else, rather than trying to prove tautologies.
func MatchesEverything(e TagExpr) bool {
	switch v := e.(type) {
	case nil:
		return true
	case TagAll:
		for _, child := range v {
			if !MatchesEverything(child) {
				return false
			}
		}
		return true
	}
	return false
}

// Leaves returns every TagAny in the tree, so a caller can check id counts
// against an engine's parameter limit (spec 4.3 records SQLite's 32766).
func Leaves(e TagExpr) []TagAny {
	var out []TagAny
	var walk func(TagExpr)
	walk = func(e TagExpr) {
		switch v := e.(type) {
		case TagAny:
			out = append(out, v)
		case TagAll:
			for _, c := range v {
				walk(c)
			}
		case TagAnyOf:
			for _, c := range v {
				walk(c)
			}
		case TagNot:
			walk(v.Expr)
		}
	}
	walk(e)
	return out
}
