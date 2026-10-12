package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/wire"
)

type boxQueryInput struct {
	Text   string   `query:"text"`
	All    []string `query:"tag_all"`
	Any    []string `query:"tag_any"`
	Not    []string `query:"tag_not"`
	From   string   `query:"from"`
	To     string   `query:"to"`
	Sort   string   `query:"sort" enum:"updated,name,activity" default:"updated"`
	Asc    bool     `query:"asc"`
	Cursor string   `query:"cursor"`
	Limit  int      `query:"limit" minimum:"0" maximum:"1000"`
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
		q := query.BoxQuery{Text: in.Text, Tags: shared.Tags, Time: shared.Time, Sort: query.BoxSort{Key: in.Sort, Asc: in.Asc}, Cursor: in.Cursor, Limit: in.Limit}
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
