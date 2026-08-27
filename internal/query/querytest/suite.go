// Package querytest is the conformance suite for the query contract.
//
// Any implementation of spec 4.2's Query must pass it. v0.1 has exactly one —
// SQLite — and the suite is written anyway, because it is the acceptance
// criterion a second implementation will be held to, and writing it afterwards
// means writing it against whatever the first implementation happened to do
// (spec 4.2).
//
// It asserts results, never plans. There is no EXPLAIN here, no assertion
// about indexes, no knowledge of any engine: the only questions are "given
// this library and this query, which rows come back, and in what order". A
// suite that knew how the answer was computed would be a regression test for
// one backend rather than a contract.
package querytest

import (
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

// Backend is one implementation under test.
type Backend interface {
	// Seed fills an empty library with exactly this content. It is called once
	// per case, on a fresh backend.
	Seed(t *testing.T, f Fixture)
	// Files answers a query.
	Files(q query.Query) ([]domain.File, string, error)
}

// Fixture is the library a case runs against. It is plain data so the suite
// can say what it needs without knowing how any backend stores it.
type Fixture struct {
	Boxes    []domain.Box
	Folders  []domain.Folder
	Files    []domain.File
	Tags     []domain.Tag
	FileTags map[domain.FileID][]domain.TagID
}

// Run executes the whole suite against a backend.
//
// open is called once per case with a fresh, empty library. A case that had to
// undo the previous one's fixture is a case nobody wants to add to.
func Run(t *testing.T, open func(*testing.T) Backend) {
	t.Helper()
	for _, c := range cases() {
		t.Run(c.name, func(t *testing.T) {
			b := open(t)
			f := c.build()
			b.Seed(t, f.Fixture())
			c.check(t, b, f)
		})
	}
}
