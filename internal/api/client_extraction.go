package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/library"
)

func (s *Server) clientExtractionRoutes() {
	huma.Post(s.contract, Prefix+"/files/{id}/extractions/{extractor}", func(ctx context.Context, in *extractionInput) (*output[library.ClientRequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		if _, err := extract.Lookup(in.Extractor); err != nil {
			return nil, invalid("Unknown extractor")
		}
		prepared, prepareErr := s.client.PrepareExtraction(ctx, domain.FileID{ID: id}, in.Extractor)
		return s.clientMutate(ctx, in.Key, "POST", Prefix+"/files/"+id.String()+"/extractions/"+in.Extractor, in.Body, func(ctx context.Context) error {
			if prepareErr != nil {
				return prepareErr
			}
			return prepared.Commit(ctx)
		})
	})
}
