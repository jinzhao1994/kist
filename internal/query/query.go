// Package query is the data access layer's contract.
//
// It holds no SQL and knows nothing about any engine (spec 4.1): a Query says
// what is wanted in the vocabulary of boxes, folders, tags, time and text, and
// each implementation translates it into whatever it is good at.
//
// It lives in its own package rather than inside store because cmd builds
// queries and cmd may not import store (invariant 2). That is the shape oplog
// already has: the contract is one package, the thing that satisfies it is
// another.
package query

import (
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
)

// MaxLimit bounds one page. A caller that wants everything pages for it; a
// caller that asks for a hundred thousand rows at once is told no, because the
// alternative is a memory spike an implementation cannot defend against.
const MaxLimit = 1000

// DefaultLimit is what an unset Limit means.
const DefaultLimit = 100

// SortKey names an ordering. Byte order throughout (spec 4.8).
type SortKey int

const (
	// SortCaptured orders by when the content was made. Files with no known
	// capture time sort last in both directions — unknown is unknown, and it
	// belongs at the end whichever way you are reading (spec 4.5.1).
	SortCaptured SortKey = iota
	// SortImported orders by when the file entered the library, which is the
	// id: ids are UUIDv7 and are minted when the op is generated, so id order
	// is creation order for free (spec 4.5.1, 3.12).
	SortImported
	// SortName orders by the folded name, so "Apple" and "apple" sort
	// together, in byte order (spec 4.8).
	SortName

	// SortUpdated orders by the archive record modification time.
	SortUpdated

	// SortType orders by the displayed extension or MIME fallback.
	SortType
	// SortSize orders by blob size in bytes.
	SortSize

	numSortKeys
)

func (k SortKey) String() string {
	switch k {
	case SortCaptured:
		return "captured"
	case SortImported:
		return "imported"
	case SortName:
		return "name"
	case SortUpdated:
		return "updated"
	case SortType:
		return "type"
	case SortSize:
		return "size"
	}
	return fmt.Sprintf("SortKey(%d)", int(k))
}

// SortSpec is an ordering and a direction.
//
// The field is Asc rather than Desc so that the zero value is "capture time,
// newest first" — the order spec 4.5 assumes and the one a photo grid wants.
// A zero value that had to be normalised before use would be a zero value
// somebody eventually forgets to normalise.
type SortSpec struct {
	Key SortKey
	Asc bool
}

// DateRange is a closed interval. Either end may be the zero LocalDate,
// meaning open.
//
// Both the ends and the values being matched are variable-precision
// (spec 1.3), so matching is overlap rather than containment: a box known only
// to be "2019" overlaps "spring 2019" and should be found. For an archive
// whose whole purpose is finding your things, a false positive beats a false
// negative (spec 4.2.1).
type DateRange struct {
	From domain.LocalDate
	To   domain.LocalDate
}

// Query is one question about the files in a library.
//
// The zero value is a valid query: everything, newest first.
type Query struct {
	// All holds independent, non-nested clauses combined with AND.
	All []Query
	// Keywords are independent text terms combined with AND.
	Keywords     []string
	NameContains string
	NameEquals   string
	// Kinds matches recorded content classification, never filename extensions.
	Kinds []string
	// Boxes restricts to these boxes. Empty means all of them.
	Boxes []domain.BoxID
	// Folder restricts to one directory, without recursing. It is what
	// `kist ls` asks for.
	Folder *domain.FolderID
	// Recursive includes descendants of Folder; it requires a folder scope.
	Recursive bool
	// Tags is the tag condition. nil means no tag filtering at all, which is
	// not the same as an expression that happens to match everything.
	Tags TagExpr
	// Time restricts by capture time. A file with no capture time is in no
	// range: nothing is known about when it was made, and a time filter is an
	// explicit narrowing. Before M3 no file has one, so a time-filtered query
	// is empty — the truth rather than a defect (spec 4.2.1).
	Time *DateRange
	// Text is matched as a folded substring of the name (spec 4.4). The
	// implementation folds it with the same function that built the column, so
	// case-insensitivity is ours rather than the engine's (spec 4.7).
	Text string
	Sort SortSpec
	// Cursor continues a previous page. It carries the ordering it was made
	// under, and an implementation must refuse one from a different ordering
	// (spec 4.2.1).
	Cursor string
	Limit  int
}

// Validate reports whether this query is answerable at all.
//
// Every implementation calls it first, so a bad query fails the same way
// everywhere. That is part of the contract too: a conformance suite that only
// pinned successes would let two implementations disagree about what counts as
// an error.
func (q Query) Validate() error {
	if q.Recursive && q.Folder == nil {
		return fmt.Errorf("recursive search requires a folder")
	}
	if len(q.All) > 128 {
		return fmt.Errorf("too many conditions")
	}
	for _, clause := range q.All {
		if len(clause.All) > 0 {
			return fmt.Errorf("nested conditions are not supported")
		}
		if err := clause.Validate(); err != nil {
			return err
		}
	}

	if len(q.Keywords) > 128 {
		return fmt.Errorf("too many keywords")
	}
	for _, text := range append(append([]string{}, q.Keywords...), q.NameContains, q.NameEquals) {
		if len(text) > 4096 {
			return fmt.Errorf("search text is too long")
		}
	}

	for _, kind := range q.Kinds {
		switch kind {
		case "image", "video", "audio", "document", "other":
		default:
			return fmt.Errorf("unknown media kind %q", kind)
		}
	}
	if q.Limit < 0 {
		return fmt.Errorf("limit %d is negative", q.Limit)
	}
	if q.Limit > MaxLimit {
		return fmt.Errorf("limit %d is above the maximum of %d", q.Limit, MaxLimit)
	}
	if q.Sort.Key < 0 || q.Sort.Key >= numSortKeys {
		return fmt.Errorf("unknown sort key %d", int(q.Sort.Key))
	}
	if q.Time != nil {
		from, to := q.Time.From, q.Time.To
		if !from.IsZero() && !to.IsZero() && from.Start() > to.End() {
			return fmt.Errorf("time range ends (%s) before it starts (%s)", to, from)
		}
	}
	return nil
}

// PageSize is the limit to actually use.
func (q Query) PageSize() int {
	if q.Limit == 0 {
		return DefaultLimit
	}
	return q.Limit
}
