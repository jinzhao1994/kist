package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
)

type catalogRevision struct {
	Revision string `json:"revision"`
}

func (s *Server) catalogRevisionRoute() {
	huma.Get(s.contract, Prefix+"/catalog-revision", func(ctx context.Context, _ *struct{}) (*output[catalogRevision], error) {
		var revision string
		var err error
		if s.client != nil {
			revision, err = s.client.CatalogRevision(ctx)
		} else {
			revision, err = s.library.CatalogRevision(ctx)
		}
		return response(catalogRevision{Revision: revision}, err)
	})
}
