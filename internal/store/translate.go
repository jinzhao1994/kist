package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

// maxParameters is SQLite's MAX_VARIABLE_NUMBER, measured (spec 4.3).
//
// It is checked rather than trusted to be large enough. A tag tree of a few
// thousand nodes is well inside it, but "well inside" is not a guarantee, and
// without the check the failure is a driver error quoting a number nobody can
// connect back to a tag.
const maxParameters = 32766

// translate turns a Query into a WHERE fragment and its arguments.
//
// It is a pure function of its arguments — same query and zone in, same SQL
// out — so it can be tested exhaustively without a database. The zone is an
// argument for that reason as much as for spec 4.7's: a translator that read
// the process's local zone would not be pure. That division matters: this file
// is tested for the shape of what it emits, and the conformance suite is
// tested for what the results mean. Neither can stand in for the other.
//
// Every condition it emits starts with " AND ", so callers append it to a
// "WHERE 1 = 1".
func translate(q query.Query, zone *time.Location) (string, []any, error) {
	if err := q.Validate(); err != nil {
		return "", nil, err
	}

	var (
		sb   strings.Builder
		args []any
	)

	if len(q.Boxes) > 0 {
		if len(q.Boxes) > maxParameters {
			return "", nil, fmt.Errorf("%d boxes is beyond the %d-parameter limit", len(q.Boxes), maxParameters)
		}
		// A recursive descent from each box's root, because a file no longer
		// carries a box_id to compare against (spec 1.4). A WITH is allowed
		// inside a subquery, so this stays one appendable fragment rather
		// than forcing every caller to hoist a CTE.
		sb.WriteString(`
			AND folder_id IN (
				WITH RECURSIVE box_folders(id) AS (
					SELECT root_folder_id FROM boxes WHERE id IN (` + placeholders(len(q.Boxes)) + `)
					UNION
					SELECT f.id FROM folders f JOIN box_folders ON f.parent_id = box_folders.id
				)
				SELECT id FROM box_folders
			)`)
		for _, b := range q.Boxes {
			args = append(args, b)
		}
	}
	if len(q.Kinds) > 0 {
		if len(q.Kinds) > maxParameters {
			return "", nil, fmt.Errorf("too many media kinds")
		}
		sb.WriteString(` AND EXISTS (SELECT 1 FROM file_metadata m WHERE m.file_id=files.id AND m.key='media:kind' AND m.value IN (` + placeholders(len(q.Kinds)) + `))`)
		for _, kind := range q.Kinds {
			args = append(args, kind)
		}
	}
	if q.Folder != nil {
		// One directory, not its subtree. `kist ls` is ls (spec 1.4).
		sb.WriteString(` AND folder_id = ?`)
		args = append(args, *q.Folder)
	}
	if q.Text != "" {
		// ESCAPE is not optional: without it a search for "100%" matches
		// everything and "a_b" matches "axb". The pattern is folded with the
		// same function that built the column, so case-insensitivity is ours
		// rather than the engine's (spec 4.7).
		sb.WriteString(` AND name_fold LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(domain.Fold(q.Text))+"%")
	}
	if q.Time != nil {
		frag, targs, err := timePredicate("captured_at", *q.Time, zone)
		if err != nil {
			return "", nil, err
		}
		sb.WriteString(frag)
		args = append(args, targs...)
	}
	if !query.MatchesEverything(q.Tags) {
		for _, leaf := range query.Leaves(q.Tags) {
			if len(leaf) > maxParameters {
				return "", nil, fmt.Errorf("%d tags is beyond the %d-parameter limit", len(leaf), maxParameters)
			}
		}
		pred, targs := tagPredicate(q.Tags)
		sb.WriteString(` AND ` + pred)
		args = append(args, targs...)
	}
	return sb.String(), args, nil
}

// tagPredicate translates a tag expression.
//
// The empty cases are the ones worth stating out loud: an empty AND is the
// identity and becomes "1", an empty OR and an empty leaf are false and become
// "0". That is the algebra, and it is what makes "the user selected no tags"
// fail closed (spec 4.2.1). Everything composite is bracketed, because
// unbracketed "a AND b OR c" does not mean what it was built to mean.
func tagPredicate(e query.TagExpr) (string, []any) {
	switch v := e.(type) {
	case nil:
		return "1", nil

	case query.TagAny:
		if len(v) == 0 {
			return "0", nil
		}
		args := make([]any, len(v))
		for i, id := range v {
			args[i] = id
		}
		return `EXISTS (SELECT 1 FROM file_tags WHERE file_id = files.id AND tag_id IN (` +
			placeholders(len(v)) + `))`, args

	case query.TagAll:
		return joinPredicates(v, " AND ", "1")

	case query.TagAnyOf:
		return joinPredicates(v, " OR ", "0")

	case query.TagNot:
		inner, args := tagPredicate(v.Expr)
		return "NOT (" + inner + ")", args
	}
	// TagExpr is a closed interface, so this is unreachable — but a silent
	// "match everything" for an unknown case would be the worst answer
	// available, so an unknown case matches nothing instead.
	return "0", nil
}

func joinPredicates(children []query.TagExpr, op, empty string) (string, []any) {
	if len(children) == 0 {
		return empty, nil
	}
	parts := make([]string, len(children))
	var args []any
	for i, c := range children {
		p, a := tagPredicate(c)
		parts[i] = p
		args = append(args, a...)
	}
	if len(parts) == 1 {
		return parts[0], args
	}
	return "(" + strings.Join(parts, op) + ")", args
}

// timePredicate matches capture instants inside a range.
//
// The range the caller gave is a variable-precision LocalDate — "2019",
// "2019-08" — and it is turned into two instants here, in zone. What it is
// does not matter to the shape of the predicate; that it is a *parameter*
// does. Reading time.Local would make the same query return different rows on
// two machines, and spec 4.7 is about exactly that class of divergence.
//
// This used to be a string overlap against a variable-precision column
// (value <= hi AND value >= substr(lo, 1, length(value))), because a file's
// capture time used to be a LocalDate too. Spec 1.5.2 made it an instant, so
// the overlap degenerates into containment — and the predicate gets simpler
// rather than more complex. Box dates keep the overlap: those are intervals a
// person declared, and a box known only as "2019" really does overlap March.
func timePredicate(column string, r query.DateRange, zone *time.Location) (string, []any, error) {
	if r.From.IsZero() && r.To.IsZero() {
		return "", nil, nil
	}
	var (
		sb   strings.Builder
		args []any
	)
	sb.WriteString(` AND ` + column + ` IS NOT NULL`)
	if !r.To.IsZero() {
		hi, err := instantAt(r.To.End(), zone)
		if err != nil {
			return "", nil, err
		}
		// End names the start of the final minute. Include that entire
		// minute, down to its last nanosecond, with an exclusive upper bound.
		sb.WriteString(` AND ` + column + ` < ?`)
		args = append(args, hi+int64(time.Minute))
	}
	if !r.From.IsZero() {
		lo, err := instantAt(r.From.Start(), zone)
		if err != nil {
			return "", nil, err
		}
		sb.WriteString(` AND ` + column + ` >= ?`)
		args = append(args, lo)
	}
	return sb.String(), args, nil
}

// instantAt reads one end of a LocalDate interval as unix nanoseconds in zone.
//
// Start and End pad to minute precision, which is the widest form LocalDate
// has, so the layout here is fixed rather than discovered (spec 1.3).
func instantAt(padded string, zone *time.Location) (int64, error) {
	t, err := time.ParseInLocation("2006-01-02T15:04", padded, zone)
	if err != nil {
		return 0, fmt.Errorf("read date %q: %w", padded, err)
	}
	return t.UnixNano(), nil
}
