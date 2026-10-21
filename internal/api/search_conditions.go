package api

import (
	"encoding/json"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

func (s *Server) searchConditions(raw []string, entity string) ([]query.Query, error) {
	return buildSearchConditions(raw, entity, s.library.TagCondition, s.library.ExcludeTags)
}

func buildSearchConditions(raw []string, entity string, include, exclude func([]string) (query.TagExpr, error)) ([]query.Query, error) {
	if len(raw) > 128 {
		return nil, invalid("Too many conditions")
	}
	clauses := make([]query.Query, 0, len(raw))
	for _, text := range raw {
		var c domain.SearchCondition
		if len(text) > 65536 || json.Unmarshal([]byte(text), &c) != nil {
			return nil, invalid("Invalid search condition")
		}
		if err := c.Validate(entity); err != nil {
			return nil, invalid(err.Error())
		}
		in := FileQueryInput{}
		switch c.Field {
		case "keyword":
			in.Name = c.Values[0]
		case "name_contains":
			in.NameContains = c.Values[0]
		case "name_equals":
			in.NameEquals = c.Values[0]
		case "box":
			in.Boxes = c.Values
		case "kind":
			in.Kinds = c.Values
		case "tag_all":
			in.All = c.Values
		case "tag_any":
			in.Any = c.Values
		case "tag_not":
			in.Not = c.Values
		case "from":
			in.From = c.Values[0]
		case "to":
			in.To = c.Values[0]
		}
		clause, err := buildFileQuery(&in, include, exclude)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	return clauses, nil
}
