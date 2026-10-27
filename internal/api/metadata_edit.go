package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

type metadataEditInput struct {
	ID   string `path:"id"`
	Key  string `header:"Idempotency-Key" required:"true"`
	Body library.MetadataEdit
}

func (s *Server) metadataEditRoutes() {
	const path = Prefix + "/files/{id}/metadata"
	if s.client != nil {
		huma.Put(s.contract, path, func(ctx context.Context, in *metadataEditInput) (*output[library.ClientRequestResult], error) {
			id, err := parseID(in.ID)
			if err != nil {
				return nil, err
			}
			return s.clientMutate(ctx, in.Key, "PUT", Prefix+"/files/"+id.String()+"/metadata", in.Body, func(ctx context.Context) error {
				return s.client.EditMetadata(ctx, domain.FileID{ID: id}, in.Body)
			})
		})
		return
	}
	huma.Put(s.contract, path, func(ctx context.Context, in *metadataEditInput) (*output[library.RequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		return s.mutate(ctx, in.Key, "PUT", Prefix+"/files/"+id.String()+"/metadata", in.Body, func(ctx context.Context) error {
			return s.library.EditMetadata(ctx, domain.FileID{ID: id}, in.Body)
		})
	})
}
