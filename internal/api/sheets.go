package api

import (
	"context"
	"github.com/jinzhao1994/kist/internal/library"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/sheetpreview"
)

type sheetPreviewInput struct {
	ID       string `path:"id"`
	Version  string `query:"v" doc:"Content version: sheet-v1:blob-key:filename"`
	Detail   bool   `query:"detail"`
	Sheet    int    `query:"sheet" minimum:"0" maximum:"255" default:"0"`
	Row      int    `query:"row" minimum:"0" maximum:"999999" default:"0"`
	Column   int    `query:"column" minimum:"0" maximum:"16383" default:"0"`
	Encoding string `query:"encoding" enum:"auto,gb18030" default:"auto"`
}

type sheetPreviewOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         sheetpreview.SheetPreview
}

// Bump sheet-v1 in both the client and this check when preview semantics change.
func sheetPreviewVersion(file domain.File) string {
	return "sheet-v1:" + string(file.BlobKey) + ":" + file.Name
}

func (s *Server) sheetRoutes() {
	huma.Get(s.contract, Prefix+"/files/{id}/sheet-preview", func(ctx context.Context, in *sheetPreviewInput) (*sheetPreviewOutput, error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		file, handle, err := s.openCurrentContent(ctx, domain.FileID{ID: id})
		if err != nil {
			return nil, failure(err)
		}
		defer handle.Close()
		if in.Version != "" && in.Version != sheetPreviewVersion(file) {
			return nil, huma.Error409Conflict("Preview version changed; reload the file")
		}
		source, ok := handle.(sheetpreview.Source)
		if !ok {
			return nil, failure(library.ErrInvariantViolated)
		}
		preview, err := sheetpreview.Read(ctx, source, file.Name, sheetpreview.Options{Sheet: in.Sheet, Row: in.Row, Column: in.Column, Detail: in.Detail, Encoding: in.Encoding})
		if err != nil {
			return nil, failure(err)
		}
		cache := "no-store"
		if in.Version != "" && preview.Supported {
			cache = "private, max-age=31536000, immutable"
		}
		return &sheetPreviewOutput{CacheControl: cache, Body: preview}, nil
	})
}
