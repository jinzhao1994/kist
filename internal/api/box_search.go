package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/wire"
)

type boxQueryInput struct {
	Conditions   []string `query:"condition,explode" maxItems:"128"`
	Keywords     []string `query:"keyword,explode" maxItems:"128"`
	NameContains string   `query:"name_contains" maxLength:"4096"`
	NameEquals   string   `query:"name_equals" maxLength:"4096"`
	Text         string   `query:"text"`
	All          []string `query:"tag_all"`
	Any          []string `query:"tag_any"`
	Not          []string `query:"tag_not"`
	From         string   `query:"from"`
	To           string   `query:"to"`
	Sort         string   `query:"sort" enum:"updated,name,activity" default:"updated"`
	Asc          bool     `query:"asc"`
	Cursor       string   `query:"cursor"`
	Limit        int      `query:"limit" minimum:"0" maximum:"1000"`
}
type boxSearchResult struct {
	Boxes  []wire.Box `json:"boxes"`
	Cursor string     `json:"cursor,omitempty"`
}

func (s *Server) boxSearchRoute() {
	huma.Get(s.contract, Prefix+"/boxes/search", func(ctx context.Context, in *boxQueryInput) (*output[boxSearchResult], error) {
		// Share date validation and descendant-aware tag expression construction.
		shared, err := s.fileQuery(&fileQueryInput{All: in.All, Any: in.Any, Not: in.Not, From: in.From, To: in.To, Limit: in.Limit})
		if err != nil {
			return nil, err
		}
		q := query.BoxQuery{Keywords: in.Keywords, NameContains: in.NameContains, NameEquals: in.NameEquals, Text: in.Text, Tags: shared.Tags, Time: shared.Time, Sort: query.BoxSort{Key: in.Sort, Asc: in.Asc}, Cursor: in.Cursor, Limit: in.Limit}
		clauses, err := s.searchConditions(in.Conditions, "boxes")
		if err != nil {
			return nil, err
		}
		for _, clause := range clauses {
			q.All = append(q.All, query.BoxQuery{Text: clause.Text, NameContains: clause.NameContains, NameEquals: clause.NameEquals, Tags: clause.Tags, Time: clause.Time})
		}
		if err := q.Validate(); err != nil {
			return nil, invalid(err.Error())
		}
		if q.Cursor != "" {
			if _, _, err := query.DecodeBoxCursor(q.Sort, q.Cursor); err != nil {
				return nil, invalid(err.Error())
			}
		}
		boxes, cursor, err := s.library.SearchBoxes(ctx, q)
		return response(boxSearchResult{Boxes: wire.NewBoxes(boxes), Cursor: cursor}, err)
	})
}
