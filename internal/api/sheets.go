package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/sheetpreview"
)

type sheetPreviewInput struct {
	ID       string `path:"id"`
	Detail   bool   `query:"detail"`
	Sheet    int    `query:"sheet" minimum:"0" maximum:"255" default:"0"`
	Row      int    `query:"row" minimum:"0" maximum:"999999" default:"0"`
	Column   int    `query:"column" minimum:"0" maximum:"16383" default:"0"`
	Encoding string `query:"encoding" enum:"auto,gb18030" default:"auto"`
}

func (s *Server) sheetRoutes() {
	huma.Get(s.contract, Prefix+"/files/{id}/sheet-preview", func(ctx context.Context, in *sheetPreviewInput) (*output[sheetpreview.SheetPreview], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		file, handle, err := s.library.OpenContent(ctx, domain.FileID{ID: id})
		if err != nil {
			return nil, failure(err)
		}
		defer handle.Close()
		preview, err := sheetpreview.Read(ctx, handle, file.Name, sheetpreview.Options{Sheet: in.Sheet, Row: in.Row, Column: in.Column, Detail: in.Detail, Encoding: in.Encoding})
		return response(preview, err)
	})
}
