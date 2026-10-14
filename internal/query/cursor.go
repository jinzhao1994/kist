package query

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jinzhao1994/kist/internal/domain"
)

// ErrCursorSortMismatch means a cursor was made under a different ordering.
var ErrCursorSortMismatch = errors.New("this cursor belongs to a different sort order")

// ID names one ordering, and travels inside every cursor made under it.
func (s SortSpec) ID() string {
	if s.Asc {
		return s.Key.String() + ":asc"
	}
	return s.Key.String() + ":desc"
}

// Value is the sort key of one file under this ordering — the value a cursor
// carries and a predicate compares against.
//
// Cursor fields are strings. Updated timestamps use decimal nanoseconds;
// SQLite compares them using the integer affinity of the updated_at column.
func (s SortSpec) Value(f domain.File) string {
	switch s.Key {
	case SortUpdated:
		return strconv.FormatInt(f.UpdatedAt.UnixNano(), 10)
	case SortImported:
		return f.ID.String()
	case SortName:
		// The folded name, not the raw one: the column being ordered is
		// name_fold, and a cursor holding a raw name would compare against a
		// folded column and step over rows (spec 4.8).
		return f.NameFold
	default:
		return f.CapturedAt.String()
	}
}

// EncodeCursor renders the position of the last row of a page.
//
// The ordering travels with it. Continuing one order with another order's
// cursor is refused rather than approximated, because the approximation is a
// page that looks perfectly normal and is silently wrong (spec 4.2.1) — and
// nobody ever notices that.
func EncodeCursor(s SortSpec, f domain.File) string {
	raw := s.ID() + "\x00" + s.Value(f) + "\x00" + f.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor reads a cursor back, refusing one made under another ordering.
func DecodeCursor(s SortSpec, cursor string) (string, domain.FileID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", domain.FileID{}, fmt.Errorf("bad cursor: %w", err)
	}
	parts := strings.SplitN(string(raw), "\x00", 3)
	if len(parts) != 3 {
		return "", domain.FileID{}, fmt.Errorf("bad cursor: expected three fields, got %d", len(parts))
	}
	if parts[0] != s.ID() {
		return "", domain.FileID{}, fmt.Errorf("%w: it was made under %s, this query is %s",
			ErrCursorSortMismatch, parts[0], s.ID())
	}
	id, err := domain.ParseID(parts[2])
	if err != nil {
		return "", domain.FileID{}, fmt.Errorf("bad cursor: %w", err)
	}
	return parts[1], domain.FileID{ID: id}, nil
}
