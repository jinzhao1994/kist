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

type aiHintInput struct {
	ID   string `path:"id"`
	Kind string `path:"kind" enum:"ocr,desc"`
	Key  string `header:"Idempotency-Key" required:"true"`
	Body library.AIHintEdit
}

func (s *Server) aiHintRoutes() {
	const path = Prefix + "/files/{id}/metadata/hints/{kind}"
	if s.client != nil {
		huma.Put(s.contract, path, func(ctx context.Context, in *aiHintInput) (*output[library.ClientRequestResult], error) {
			id, err := parseID(in.ID)
			if err != nil {
				return nil, err
			}
			return s.clientMutate(ctx, in.Key, "PUT", Prefix+"/files/"+id.String()+"/metadata/hints/"+in.Kind, in.Body, func(ctx context.Context) error {
				return s.client.SetAIHint(ctx, domain.FileID{ID: id}, in.Kind, in.Body)
			})
		})
		return
	}
	huma.Put(s.contract, path, func(ctx context.Context, in *aiHintInput) (*output[library.RequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		return s.mutate(ctx, in.Key, "PUT", Prefix+"/files/"+id.String()+"/metadata/hints/"+in.Kind, in.Body, func(ctx context.Context) error {
			return s.library.SetAIHint(ctx, domain.FileID{ID: id}, in.Kind, in.Body)
		})
	})
}
