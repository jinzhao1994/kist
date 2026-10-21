package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
)

func (s *Server) clientBoxSearchRoute() {
	huma.Get(s.contract, Prefix+"/boxes/search", func(ctx context.Context, in *boxQueryInput) (*output[boxSearchResult], error) {
		out := boxSearchResult{Boxes: []boxSearchItem{}}
		err := s.client.View(ctx, func(v *library.ClientView) error {
			q, err := buildBoxQuery(in, func(refs []string) (query.TagExpr, error) { return v.TagCondition(ctx, refs) }, func(refs []string) (query.TagExpr, error) { return v.ExcludeTags(ctx, refs) })
			if err != nil {
				return err
			}
			boxes, cursor, err := v.SearchBoxes(ctx, q)
			if err != nil {
				return err
			}
			out = renderBoxSearch(boxes, cursor)
			return nil
		})
		return response(out, err)
	})
}
