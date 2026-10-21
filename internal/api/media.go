package api

import (
	"context"
	"io"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

type mediaBackend interface {
	Status(context.Context, domain.FileID, string) (library.MediaJob, error)
	Request(context.Context, domain.FileID, string, bool) (library.MediaJob, error)
	Cancel(context.Context, domain.FileID, string) (library.MediaJob, error)
	Open(context.Context, domain.FileID, string) (library.MediaJob, io.ReadSeekCloser, error)
	Run(context.Context)
}

type nasMedia struct{ *library.Media }

func (m nasMedia) Open(ctx context.Context, id domain.FileID, recipe string) (library.MediaJob, io.ReadSeekCloser, error) {
	return m.Media.Open(ctx, id, recipe)
}

type mediaInput struct {
	ID     string `path:"id"`
	Recipe string `path:"recipe"`
}
type mediaRequest struct {
	ID     string `path:"id"`
	Recipe string `path:"recipe"`
	Body   struct {
		Retry bool `json:"retry,omitempty"`
	}
}
type mediaOutput struct{ Body library.MediaJob }

func mediaID(in mediaInput) (domain.FileID, error) {
	id, err := parseID(in.ID)
	if err != nil {
		return domain.FileID{}, err
	}
	if _, ok := derive.Lookup(in.Recipe); !ok {
		return domain.FileID{}, invalid("unknown media recipe")
	}
	return domain.FileID{ID: id}, nil
}

// RunMedia processes disposable derived jobs until shutdown. Callers join it
// before closing the library; HTTP requests merely enqueue work.
func (s *Server) RunMedia(ctx context.Context) { s.previews.Run(ctx) }

func (s *Server) mediaRoutes() {
	path := Prefix + "/files/{id}/derived/{recipe}"
	huma.Get(s.contract, path, func(ctx context.Context, in *mediaInput) (*mediaOutput, error) {
		id, err := mediaID(*in)
		if err != nil {
			return nil, err
		}
		j, err := s.previews.Status(ctx, id, in.Recipe)
		return &mediaOutput{Body: j}, failure(err)
	})
	huma.Post(s.contract, path, func(ctx context.Context, in *mediaRequest) (*mediaOutput, error) {
		id, err := mediaID(mediaInput{ID: in.ID, Recipe: in.Recipe})
		if err != nil {
			return nil, err
		}
		j, err := s.previews.Request(ctx, id, in.Recipe, in.Body.Retry)
		return &mediaOutput{Body: j}, failure(err)
	})
	huma.Delete(s.contract, path, func(ctx context.Context, in *mediaInput) (*mediaOutput, error) {
		id, err := mediaID(*in)
		if err != nil {
			return nil, err
		}
		j, err := s.previews.Cancel(ctx, id, in.Recipe)
		return &mediaOutput{Body: j}, failure(err)
	})
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		huma.Register(s.contract, huma.Operation{OperationID: method + "-derived-content", Method: method, Path: path + "/content", Summary: "Read completed derived media; supports HTTP ranges", Responses: map[string]*huma.Response{
			"200": {Description: "Derived image, PDF, video or geographic data", Content: map[string]*huma.MediaType{"application/pdf": {Schema: &huma.Schema{Type: "string", Format: "binary"}}, "application/geo+json": {Schema: &huma.Schema{Type: "object"}}, "image/jpeg": {Schema: &huma.Schema{Type: "string", Format: "binary"}}, "video/mp4": {Schema: &huma.Schema{Type: "string", Format: "binary"}}}},
			"206": {Description: "Requested byte range"}, "416": {Description: "Unsatisfiable byte range"},
		}}, func(ctx context.Context, in *mediaInput) (*huma.StreamResponse, error) {
			id, err := mediaID(*in)
			if err != nil {
				return nil, err
			}
			return &huma.StreamResponse{Body: func(hctx huma.Context) {
				req, w := humago.Unwrap(hctx)
				j, f, err := s.previews.Open(ctx, id, in.Recipe)
				if err != nil {
					writeFailure(w, err)
					return
				}
				defer f.Close()
				recipe, _ := derive.Lookup(in.Recipe)
				w.Header().Set("Content-Type", recipe.MediaType)
				w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
				w.Header().Set("Cache-Control", "private, no-cache")
				w.Header().Set("ETag", `"`+j.Generation+`"`)
				http.ServeContent(w, req, recipe.Filename(), j.UpdatedAt, f)
			}}, nil
		})
	}
}
