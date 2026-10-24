package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/library"
)

func (s *Server) syncDerivedRoutes() {
	huma.Register(s.contract, syncOperation("sync-derived-describe", "POST", "derived/describe"), func(ctx context.Context, in *struct {
		Body struct {
			Key    string `json:"key" minLength:"1" maxLength:"128"`
			Recipe string `json:"recipe" minLength:"1" maxLength:"128"`
			Retry  bool   `json:"retry,omitempty"`
		}
	}) (*output[library.SyncDerived], error) {
		return response(s.describeSyncDerived(ctx, in.Body.Key, in.Body.Recipe, in.Body.Retry))
	})
	for _, method := range []string{"GET", "HEAD"} {
		op := syncOperation(method+"-sync-derived-content", method, "derived/content")
		op.Responses = map[string]*huma.Response{"200": {Description: "Immutable derived bytes"}, "206": {Description: "Requested byte range"}, "409": {Description: "The requested generation changed"}, "416": {Description: "Unsatisfiable byte range"}}
		huma.Register(s.contract, op, func(ctx context.Context, in *struct {
			Key        string `query:"key" required:"true" maxLength:"128"`
			Recipe     string `query:"recipe" required:"true" maxLength:"128"`
			Generation string `query:"generation" required:"true" maxLength:"128"`
			ContentKey string `query:"content_key" required:"true" maxLength:"128"`
		}) (*huma.StreamResponse, error) {
			return &huma.StreamResponse{Body: func(hctx huma.Context) {
				req, w := humago.Unwrap(hctx)
				job, file, err := s.openSyncDerived(ctx, in.Key, in.Recipe, in.Generation, in.ContentKey)
				if err != nil {
					writeFailure(w, err)
					return
				}
				defer file.Close()
				recipe, _ := derive.Lookup(job.Recipe)
				w.Header().Set("Content-Type", recipe.MediaType)
				w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
				w.Header().Set("Cache-Control", "private, no-cache")
				w.Header().Set("ETag", `"`+in.ContentKey+`"`)
				http.ServeContent(w, req, recipe.Filename(), job.UpdatedAt, file)
			}}, nil
		})
	}
}
