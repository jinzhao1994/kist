package api

import (
	"context"
	"io"
	"mime"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jinzhao1994/kist/internal/domain"
)

type contentInput struct {
	ID       string `path:"id"`
	Download bool   `query:"download"`
}

// Only inert browser media may render inline. File extensions and uploaded
// Content-Type headers are untrusted; everything else is an attachment.
func inlineMedia(media string) bool {
	switch media {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp", "image/x-icon",
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
				file, handle, err := s.library.OpenContent(ctx, domain.FileID{ID: id})
				if err != nil {
					writeFailure(w, err)
					return
				}
				defer handle.Close()
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
				w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
				http.ServeContent(w, req, file.Name, file.CreatedAt, handle)
			}}, nil
		})
	}
}
