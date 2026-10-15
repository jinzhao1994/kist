package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

type EntryInput struct {
	Box    string `path:"box"`
	ID     string `path:"id"`
	Folder bool   `query:"folder"`
}
type entryRenameInput struct {
	EntryInput
	Key  string `header:"Idempotency-Key" required:"true"`
	Body struct {
		Before string `json:"before"`
		Name   string `json:"name" minLength:"1" maxLength:"255"`
	}
}
type entryDeleteInput struct {
	EntryInput
	Key  string `header:"Idempotency-Key" required:"true"`
	Body struct {
		Revision int64 `json:"revision" minimum:"1"`
	}
}

func (in EntryInput) ids() (domain.BoxID, domain.ID, error) {
	box, err := parseID(in.Box)
	if err != nil {
		return domain.BoxID{}, domain.ID{}, err
	}
	id, err := parseID(in.ID)
	return domain.BoxID{ID: box}, id, err
}
func (s *Server) entryRoutes() {
	const path = Prefix + "/boxes/{box}/entries/{id}"
	huma.Get(s.contract, path+"/deletion-preview", func(ctx context.Context, in *EntryInput) (*output[library.EntryDeletionPreview], error) {
		box, id, err := in.ids()
		if err != nil {
			return nil, err
		}
		p, err := s.library.PreviewEntryDeletion(ctx, box, id, in.Folder)
		return response(p, err)
	})
	huma.Put(s.contract, path+"/name", func(ctx context.Context, in *entryRenameInput) (*output[library.RequestResult], error) {
		box, id, err := in.ids()
		if err != nil {
			return nil, err
		}
		requestPath := Prefix + "/boxes/" + box.String() + "/entries/" + id.String() + "/name"
		if in.Folder {
			requestPath += "?folder=true"
		}
		return s.mutate(ctx, in.Key, "PUT", requestPath, in.Body, func(ctx context.Context) error {
			_, err := s.library.RenameEntry(ctx, box, id, in.Folder, in.Body.Before, in.Body.Name)
			return err
		})
	})
	huma.Delete(s.contract, path, func(ctx context.Context, in *entryDeleteInput) (*output[library.RequestResult], error) {
		box, id, err := in.ids()
		if err != nil {
			return nil, err
		}
		requestPath := Prefix + "/boxes/" + box.String() + "/entries/" + id.String()
		if in.Folder {
			requestPath += "?folder=true"
		}
		return s.mutate(ctx, in.Key, "DELETE", requestPath, in.Body, func(ctx context.Context) error {
			_, err := s.library.DeleteEntry(ctx, box, id, in.Folder, in.Body.Revision)
			return err
		})
	})
}
