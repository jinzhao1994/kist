package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

func (s *Server) clientSavedSearchWriteRoutes() {
	huma.Post(s.contract, Prefix+"/saved-searches", func(ctx context.Context, in *createSavedSearchInput) (*output[library.ClientRequestResult], error) {
		return s.clientMutate(ctx, in.Key, "POST", Prefix+"/saved-searches", in.Body, func(ctx context.Context) error {
			_, err := s.client.CreateSavedSearch(ctx, in.Body.Name, in.Body.Definition)
			return err
		})
	})
	huma.Put(s.contract, Prefix+"/saved-searches/{id}", func(ctx context.Context, in *updateSavedSearchInput) (*output[library.ClientRequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		return s.clientMutate(ctx, in.Key, "PUT", Prefix+"/saved-searches/"+id.String(), in.Body, func(ctx context.Context) error {
			return s.client.UpdateSavedSearch(ctx, domain.SavedSearch{ID: domain.SavedSearchID{ID: id}, Name: in.Body.Before.Name, Definition: in.Body.Before.Definition}, in.Body.Name, in.Body.Definition)
		})
	})
	huma.Delete(s.contract, Prefix+"/saved-searches/{id}", func(ctx context.Context, in *deleteTagInput) (*output[library.ClientRequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		return s.clientMutate(ctx, in.Key, "DELETE", Prefix+"/saved-searches/"+id.String(), nil, func(ctx context.Context) error { return s.client.DeleteSavedSearch(ctx, domain.SavedSearchID{ID: id}) })
	})
}
