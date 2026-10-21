package api

import (
	"context"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

type contentInput struct {
	ID       string `path:"id"`
	Download bool   `query:"download"`
	At       int64  `query:"at" minimum:"0"`
}

// Browser media and PDFs may render inline. PDFs use the browser-isolated
// reader. File extensions and uploaded Content-Type headers are untrusted.
func inlineMedia(media string) bool {
	switch media {
	case "application/pdf", "image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp", "image/x-icon",
		"audio/mpeg", "audio/wave", "audio/aiff", "audio/midi", "audio/ogg", "video/ogg", "video/mp4", "video/webm":
		return true
	}
	return false
}
func (s *Server) contentRoutes() {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		huma.Register(s.contract, huma.Operation{OperationID: method + "-file-content", Method: method, Path: Prefix + "/files/{id}/content", Summary: "Read original file bytes; supports HTTP ranges", Responses: map[string]*huma.Response{
			"200": {Description: "Original file bytes", Content: map[string]*huma.MediaType{"application/octet-stream": {Schema: &huma.Schema{Type: "string", Format: "binary"}}}},
			"206": {Description: "Requested byte range", Content: map[string]*huma.MediaType{"application/octet-stream": {Schema: &huma.Schema{Type: "string", Format: "binary"}}}},
			"416": {Description: "Unsatisfiable byte range"},
		}}, func(ctx context.Context, in *contentInput) (*huma.StreamResponse, error) {
			id, err := parseID(in.ID)
			if err != nil {
				return nil, err
			}
			return &huma.StreamResponse{Body: func(hctx huma.Context) {
				req, w := humago.Unwrap(hctx)
				var file domain.File
				var handle io.ReadSeekCloser
				var err error
				if s.client != nil {
					if in.At != 0 {
						writeFailure(w, huma.Error400BadRequest("Historical client content is not available"))
						return
					}
					file, handle, err = s.client.OpenContent(ctx, domain.FileID{ID: id})
				} else if in.At == 0 {
					file, handle, err = s.library.OpenContent(ctx, domain.FileID{ID: id})
				} else {
					err = s.withHistory(ctx, in.At, func(ctx context.Context, view *library.HistoricalView) error {
						var err error
						file, handle, err = view.OpenContent(ctx, domain.FileID{ID: id})
						return err
					})
				}
				if handle != nil {
					defer handle.Close()
				}
				if err != nil {
					writeFailure(w, err)
					return
				}
				var head [512]byte
				n, err := handle.Read(head[:])
				if err != nil && err != io.EOF {
					writeFailure(w, err)
					return
				}
				media := http.DetectContentType(head[:n])
				if _, err = handle.Seek(0, io.SeekStart); err != nil {
					writeFailure(w, err)
					return
				}
				disposition := "attachment"
				if !in.Download && inlineMedia(media) {
					disposition = "inline"
				}
				w.Header().Set("Content-Type", media)
				w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": file.Name}))
				policy := "sandbox; default-src 'none'; frame-ancestors 'self'"
				if disposition == "inline" && strings.HasPrefix(media, "image/") {
					// Chrome's native image document uses inline styles for centering
					// and zoom. Only sniffed, allowlisted passive images reach here.
					policy += "; style-src 'unsafe-inline'"
				}
				w.Header().Set("Content-Security-Policy", policy)
				http.ServeContent(w, req, file.Name, file.CreatedAt, handle)
			}}, nil
		})
	}
}
