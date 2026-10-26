package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/library"
)

type extractionInput struct {
	ID        string `path:"id"`
	Extractor string `path:"extractor"`
	Key       string `header:"Idempotency-Key" required:"true"`
	Body      struct {
		Force bool `json:"force,omitempty"`
	}
}

func (s *Server) extractionRoutes() {
	huma.Post(s.contract, Prefix+"/files/{id}/extractions/{extractor}", func(ctx context.Context, in *extractionInput) (*output[library.RequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		if _, err := extract.Lookup(in.Extractor); err != nil {
			return nil, invalid("Unknown extractor")
		}
		prepare := s.library.PrepareExtraction
		if in.Body.Force {
			prepare = s.library.PrepareReExtraction
		}
		prepared, prepareErr := prepare(ctx, domain.FileID{ID: id}, in.Extractor)
		return s.mutate(ctx, in.Key, "POST", Prefix+"/files/"+id.String()+"/extractions/"+in.Extractor, in.Body, func(ctx context.Context) error {
			if prepareErr != nil {
				return prepareErr
			}
			return prepared.Commit(ctx)
		})
	})
}
