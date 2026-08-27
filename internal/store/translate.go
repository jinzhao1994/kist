package store

import (
	"fmt"
	"strings"

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
// It is a pure function of the query — same query in, same SQL out — so it can
// be tested exhaustively without a database. That division matters: this file
// is tested for the shape of what it emits, and the conformance suite is
// tested for what the results mean. Neither can stand in for the other.
//
// Every condition it emits starts with " AND ", so callers append it to a
// "WHERE 1 = 1".
func translate(q query.Query) (string, []any, error) {
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
		sb.WriteString(` AND box_id IN (` + placeholders(len(q.Boxes)) + `)`)
		for _, b := range q.Boxes {
			args = append(args, b)
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
		frag, targs := timePredicate("captured_at", *q.Time)
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

// timePredicate matches variable-precision values that overlap a range.
//
// Values are stored as ISO-8601 prefixes (spec 1.3), so a value denotes an
// interval: "2019" is the whole of 2019. Overlap rather than containment is
// the decision in spec 4.2.1 — an archive exists to help you find things, so a
// box known only to be "2019" should turn up in a search for spring 2019.
//
// Both halves fall out of prefix ordering:
//
//   - the value's interval starts at or before the upper bound  ⟺  value <= hi
//   - the value's interval ends at or after the lower bound     ⟺  value >= the
//     first length(value) characters of lo — compare only as far as the value's
//     own precision goes, which is exactly what "we only know this much" means
//
// NULL is excluded explicitly. A file with no capture time is in no range
// (spec 4.2.1), and leaving that to NULL comparison semantics would make the
// answer depend on the engine.
func timePredicate(column string, r query.DateRange) (string, []any) {
	if r.From.IsZero() && r.To.IsZero() {
		return "", nil
	}
	var (
		sb   strings.Builder
		args []any
	)
	sb.WriteString(` AND ` + column + ` IS NOT NULL`)
	if !r.To.IsZero() {
		sb.WriteString(` AND ` + column + ` <= ?`)
		args = append(args, r.To.End())
	}
	if !r.From.IsZero() {
		sb.WriteString(` AND ` + column + ` >= substr(?, 1, length(` + column + `))`)
		args = append(args, r.From.Start())
	}
	return sb.String(), args
}
