package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/library"
)

type clientEntryDeleteInput struct {
	EntryInput
	Key  string `header:"Idempotency-Key" required:"true"`
	Body struct {
		Revision string `json:"revision" minLength:"1"`
	}
}

func (s *Server) clientEntryRoutes() {
	const path = Prefix + "/boxes/{box}/entries/{id}"
	huma.Get(s.contract, path+"/deletion-preview", func(ctx context.Context, in *EntryInput) (*output[library.ClientDeletionPreview], error) {
		box, id, err := in.ids()
		if err != nil {
			return nil, err
		}
		p, err := s.client.PreviewEntryDeletion(ctx, box, id, in.Folder)
		return response(p, err)
	})
	huma.Put(s.contract, path+"/name", func(ctx context.Context, in *entryRenameInput) (*output[library.ClientRequestResult], error) {
		box, id, err := in.ids()
		if err != nil {
			return nil, err
		}
		requestPath := Prefix + "/boxes/" + box.String() + "/entries/" + id.String() + "/name"
		if in.Folder {
			requestPath += "?folder=true"
		}
		return s.clientMutate(ctx, in.Key, "PUT", requestPath, in.Body, func(ctx context.Context) error {
			_, err := s.client.RenameEntry(ctx, box, id, in.Folder, in.Body.Before, in.Body.Name)
			return err
		})
	})
	huma.Delete(s.contract, path, func(ctx context.Context, in *clientEntryDeleteInput) (*output[library.ClientRequestResult], error) {
		box, id, err := in.ids()
		if err != nil {
			return nil, err
		}
		requestPath := Prefix + "/boxes/" + box.String() + "/entries/" + id.String()
		if in.Folder {
			requestPath += "?folder=true"
		}
		return s.clientMutate(ctx, in.Key, "DELETE", requestPath, in.Body, func(ctx context.Context) error {
			_, err := s.client.DeleteEntry(ctx, box, id, in.Folder, in.Body.Revision)
			return err
		})
	})
}
