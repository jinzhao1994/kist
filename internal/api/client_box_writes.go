package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

func (s *Server) clientBoxWriteRoutes() {
	huma.Get(s.contract, Prefix+"/boxes/{id}/deletion-preview", func(ctx context.Context, in *entityInput) (*output[library.ClientDeletionPreview], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		preview, err := s.client.PreviewBoxDeletion(ctx, domain.BoxID{ID: id})
		return response(preview, err)
	})
	huma.Post(s.contract, Prefix+"/boxes/{id}/folders", func(ctx context.Context, in *struct {
		ID   string `path:"id"`
		Key  string `header:"Idempotency-Key" required:"true"`
		Body struct {
			Name   string `json:"name" minLength:"1"`
			Parent string `json:"parent,omitempty"`
		}
	}) (*output[library.ClientRequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		var parent domain.FolderID
		if in.Body.Parent != "" {
			parsed, err := parseID(in.Body.Parent)
			if err != nil {
				return nil, err
			}
			parent = domain.FolderID{ID: parsed}
		}
		return s.clientMutate(ctx, in.Key, "POST", Prefix+"/boxes/"+id.String()+"/folders", in.Body, func(ctx context.Context) error {
			_, err := s.client.CreateFolder(ctx, domain.BoxID{ID: id}, parent, in.Body.Name)
			return err
		})
	})

	huma.Delete(s.contract, Prefix+"/boxes/{id}", func(ctx context.Context, in *struct {
		ID   string `path:"id"`
		Key  string `header:"Idempotency-Key" required:"true"`
		Body struct {
			Revision string `json:"revision" minLength:"1"`
		}
	}) (*output[library.ClientRequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		return s.clientMutate(ctx, in.Key, "DELETE", Prefix+"/boxes/"+id.String(), in.Body, func(ctx context.Context) error {
			_, err := s.client.DeleteBox(ctx, domain.BoxID{ID: id}, in.Body.Revision)
			return err
		})
	})
	huma.Put(s.contract, Prefix+"/boxes/{id}/cover", func(ctx context.Context, in *struct {
		ID   string `path:"id"`
		Key  string `header:"Idempotency-Key" required:"true"`
		Body struct {
			FileID string `json:"file_id"`
		}
	}) (*output[library.ClientRequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		var file domain.FileID
		if in.Body.FileID != "" {
			parsed, err := parseID(in.Body.FileID)
			if err != nil {
				return nil, err
			}
			file = domain.FileID{ID: parsed}
		}
		return s.clientMutate(ctx, in.Key, "PUT", Prefix+"/boxes/"+id.String()+"/cover", in.Body, func(ctx context.Context) error {
			return s.client.SetBoxCover(ctx, domain.BoxID{ID: id}, file)
		})
	})

}
