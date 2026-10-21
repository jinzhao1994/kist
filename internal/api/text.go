package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/textpreview"
)

type textPreviewInput struct {
	ID       string `path:"id"`
	Detail   bool   `query:"detail" doc:"Read up to 1 MiB instead of an 8 KiB, 20-line thumbnail"`
	Encoding string `query:"encoding" enum:"auto,gb18030" default:"auto"`
}

func (s *Server) textRoutes() {
	huma.Get(s.contract, Prefix+"/files/{id}/text-preview", func(ctx context.Context, in *textPreviewInput) (*output[textpreview.Preview], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		_, handle, err := s.openCurrentContent(ctx, domain.FileID{ID: id})
		if err != nil {
			return nil, failure(err)
		}
		defer handle.Close()
		preview, err := textpreview.Read(handle, in.Detail, in.Encoding)
		return response(preview, err)
	})
}
