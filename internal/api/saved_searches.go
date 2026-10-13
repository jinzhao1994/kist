package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

type savedSearchBody struct {
	Name       string                  `json:"name" minLength:"1" maxLength:"255"`
	Definition domain.SearchDefinition `json:"definition"`
}
type savedSearchView struct {
	ID         string                  `json:"id"`
	Name       string                  `json:"name"`
	Definition domain.SearchDefinition `json:"definition"`
}
type savedSearchList struct {
	Searches []savedSearchView `json:"searches"`
}
type createSavedSearchInput struct {
	Key  string `header:"Idempotency-Key" required:"true"`
	Body savedSearchBody
}
type updateSavedSearchBody struct {
	Name       string                  `json:"name" minLength:"1" maxLength:"255"`
	Definition domain.SearchDefinition `json:"definition"`
	Before     savedSearchBody         `json:"before"`
}
type updateSavedSearchInput struct {
	ID   string `path:"id"`
	Key  string `header:"Idempotency-Key" required:"true"`
	Body updateSavedSearchBody
}

func (s *Server) savedSearchRoutes() {
	huma.Get(s.contract, Prefix+"/saved-searches", func(ctx context.Context, _ *struct{}) (*output[savedSearchList], error) {
		items, err := s.library.SavedSearches(ctx)
		if err != nil {
			return nil, failure(err)
		}
		out := savedSearchList{Searches: []savedSearchView{}}
		for _, item := range items {
			out.Searches = append(out.Searches, savedSearchView{ID: item.ID.String(), Name: item.Name, Definition: item.Definition})
		}
		return response(out, nil)
	})
	huma.Post(s.contract, Prefix+"/saved-searches", func(ctx context.Context, in *createSavedSearchInput) (*output[library.RequestResult], error) {
		return s.mutate(ctx, in.Key, "POST", Prefix+"/saved-searches", in.Body, func(ctx context.Context) error {
			_, err := s.library.CreateSavedSearch(ctx, in.Body.Name, in.Body.Definition)
			return err
		})
	})
	huma.Put(s.contract, Prefix+"/saved-searches/{id}", func(ctx context.Context, in *updateSavedSearchInput) (*output[library.RequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		return s.mutate(ctx, in.Key, "PUT", Prefix+"/saved-searches/"+id.String(), in.Body, func(ctx context.Context) error {
			return s.library.UpdateSavedSearch(ctx, domain.SavedSearch{ID: domain.SavedSearchID{ID: id}, Name: in.Body.Before.Name, Definition: in.Body.Before.Definition}, in.Body.Name, in.Body.Definition)
		})
	})
	huma.Delete(s.contract, Prefix+"/saved-searches/{id}", func(ctx context.Context, in *deleteTagInput) (*output[library.RequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		return s.mutate(ctx, in.Key, "DELETE", Prefix+"/saved-searches/"+id.String(), nil, func(ctx context.Context) error { return s.library.DeleteSavedSearch(ctx, domain.SavedSearchID{ID: id}) })
	})
}
