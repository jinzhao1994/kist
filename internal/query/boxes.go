package query

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/jinzhao1994/kist/internal/domain"
)

// BoxQuery searches boxes independently of the files they contain.
// Text matches name or description; Tags only matches direct box assignments.
// Time overlaps the declared activity interval. A missing endpoint uses the
// known endpoint's precision; entirely undated boxes do not match a range.
type BoxQuery struct {
	// Keywords are independent text terms combined with AND.
	Keywords     []string
	NameContains string
	NameEquals   string
	Text         string
	Tags         TagExpr
	Time         *DateRange
	Sort         BoxSort
	Cursor       string
	Limit        int
}

// BoxSort orders by updated time (the default), folded name, or activity start.
// Unknown activity dates sort last in both directions. IDs break ties.
type BoxSort struct {
	Key string
	Asc bool
}

func (s BoxSort) KeyName() string {
	if s.Key == "" {
		return "updated"
	}
	return s.Key
}
func (s BoxSort) ID() string {
	direction := ":desc"
	if s.Asc {
		direction = ":asc"
	}
	return "box:" + s.KeyName() + direction
}
func (q BoxQuery) Validate() error {
	switch q.Sort.KeyName() {
	case "updated", "name", "activity":
	default:
		return fmt.Errorf("unknown box sort %q", q.Sort.Key)
	}
	return (Query{Limit: q.Limit, Time: q.Time, Keywords: q.Keywords, NameContains: q.NameContains, NameEquals: q.NameEquals}).Validate()
}
func (q BoxQuery) PageSize() int { return (Query{Limit: q.Limit}).PageSize() }

func EncodeBoxCursor(s BoxSort, b domain.Box) string {
	key := strconv.FormatInt(b.UpdatedAt.UnixNano(), 10)
	switch s.KeyName() {
	case "name":
		key = b.NameFold
	case "activity":
		key = b.StartAt.String()
		if key == "" {
			key = b.EndAt.String()
		}
	}
	return base64.RawURLEncoding.EncodeToString([]byte(s.ID() + "\x00" + key + "\x00" + b.ID.String()))
}

func DecodeBoxCursor(s BoxSort, cursor string) (string, domain.BoxID, error) {
	var zero domain.BoxID
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", zero, fmt.Errorf("bad box cursor: %w", err)
	}
	parts := strings.Split(string(raw), "\x00")
	if len(parts) != 3 {
		return "", zero, fmt.Errorf("bad box cursor: expected three fields")
	}
	if parts[0] != s.ID() {
		return "", zero, ErrCursorSortMismatch
	}
	id, err := domain.ParseID(parts[2])
	if err != nil || id.IsZero() {
		return "", zero, fmt.Errorf("bad box cursor ID")
	}
	switch s.KeyName() {
	case "updated":
		_, err = strconv.ParseInt(parts[1], 10, 64)
	case "activity":
		if parts[1] != "" {
			_, err = domain.ParseLocalDate(parts[1])
		}
	}
	if err != nil {
		return "", zero, fmt.Errorf("bad box cursor value: %w", err)
	}
	return parts[1], domain.BoxID{ID: id}, nil
}
