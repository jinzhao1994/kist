package store

import (
	"database/sql/driver"
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
	"modernc.org/sqlite"
)

func init() {
	// Description search uses the same Unicode folding as names, without a
	// redundant materialized column or SQLite's ASCII-only lower().
	sqlite.MustRegisterDeterministicScalarFunction("kist_fold", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("kist_fold requires text")
		}
		return domain.Fold(s), nil
	})
}

// SearchBoxes returns one bounded page and an opaque continuation cursor.
func SearchBoxes(db Queryer, q query.BoxQuery) ([]domain.Box, string, error) {
	if err := q.Validate(); err != nil {
		return nil, "", err
	}
	fragment, args, err := boxWhere(q)
	if err != nil {
		return nil, "", err
	}
	sql := `SELECT ` + boxColumns + ` FROM boxes WHERE 1=1` + fragment
	column := "updated_at"
	switch q.Sort.KeyName() {
	case "name":
		column = "name_fold"
	case "activity":
		column = "coalesce(start_at,end_at)"
	}
	dir, cmp := " DESC", " < ?"
	if q.Sort.Asc {
		dir, cmp = " ASC", " > ?"
	}
	if q.Cursor != "" {
		key, id, err := query.DecodeBoxCursor(q.Sort, q.Cursor)
		if err != nil {
			return nil, "", err
		}
		if q.Sort.KeyName() == "activity" && key == "" {
			sql += ` AND ` + column + ` IS NULL AND id` + cmp
			args = append(args, id)
		} else {
			sql += ` AND (`
			if q.Sort.KeyName() == "activity" {
				sql += column + ` IS NULL OR `
			}
			sql += column + cmp + ` OR (` + column + ` = ? AND id` + cmp + `))`
			args = append(args, key, key, id)
		}
	}
	sql += ` ORDER BY ` + column + dir + ` NULLS LAST, id` + dir + ` LIMIT ?`
	args = append(args, q.PageSize()+1)
	rows, err := db.Query(sql, args...)
	if err != nil {
		return nil, "", fmt.Errorf("search boxes: %w", err)
	}
	defer rows.Close()
	var out []domain.Box
	for rows.Next() {
		b, _, err := scanBox(rows.Scan)
		if err != nil {
			return nil, "", err
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(out) <= q.PageSize() {
		return out, "", nil
	}
	out = out[:q.PageSize()]
	return out, query.EncodeBoxCursor(q.Sort, out[len(out)-1]), nil
}

func boxWhere(q query.BoxQuery) (string, []any, error) {
	sql := ""
	args := []any{}
	if q.Text != "" {
		sql += ` AND (instr(name_fold, ?) > 0 OR instr(kist_fold(description), ?) > 0)`
		text := domain.Fold(q.Text)
		args = append(args, text, text)
	}
	for _, term := range q.Keywords {
		sql += ` AND (instr(name_fold, ?) > 0 OR instr(kist_fold(description), ?) > 0)`
		args = append(args, domain.Fold(term), domain.Fold(term))
	}
	if q.NameContains != "" {
		sql += ` AND instr(name_fold, ?) > 0`
		args = append(args, domain.Fold(q.NameContains))
	}
	if q.NameEquals != "" {
		sql += ` AND name_fold = ?`
		args = append(args, domain.Fold(q.NameEquals))
	}
	if q.Time != nil && (!q.Time.From.IsZero() || !q.Time.To.IsZero()) {
		sql += ` AND coalesce(start_at,end_at) IS NOT NULL`
		if !q.Time.To.IsZero() {
			sql += ` AND coalesce(start_at,end_at) <= ?`
			args = append(args, q.Time.To.End())
		}
		if !q.Time.From.IsZero() {
			sql += ` AND coalesce(end_at,start_at) >= substr(?,1,length(coalesce(end_at,start_at)))`
			args = append(args, q.Time.From.Start())
		}
	}
	if !query.MatchesEverything(q.Tags) {
		total := 0
		for _, leaf := range query.Leaves(q.Tags) {
			total += len(leaf)
			if total > maxParameters-16 {
				return "", nil, fmt.Errorf("too many tag parameters")
			}
		}
		pred, values := entityTagPredicate(q.Tags, "box_tags", "box_id", "boxes")
		sql += ` AND ` + pred
		args = append(args, values...)
	}
	for _, clause := range q.All {
		fragment, values, err := boxWhere(clause)
		if err != nil {
			return "", nil, err
		}
		sql += fragment
		args = append(args, values...)
	}
	if len(args) > maxParameters-16 {
		return "", nil, fmt.Errorf("too many query parameters")
	}
	return sql, args, nil
}
