package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

type videoInput struct {
	ID      string `path:"id"`
	Version string `query:"v" required:"true" maxLength:"256"`
}
type videoSegmentInput struct {
	ID      string `path:"id"`
	Version string `query:"v" required:"true" maxLength:"256"`
	Segment int    `path:"segment" minimum:"0" maximum:"3599"`
}

func videoFailure(err error) error {
	if errors.Is(err, library.ErrVideoChanged) {
		return huma.Error409Conflict("Video changed; reload the player")
	}
	var f *derive.Failure
	if errors.As(err, &f) {
		return huma.Error503ServiceUnavailable("Video playback unavailable: " + f.Code)
	}
	return failure(err)
}

func (s *Server) videoRoutes() {
	base := Prefix + "/files/{id}/playback"
	huma.Register(s.contract, huma.Operation{OperationID: "get-video-playlist", Method: http.MethodGet, Path: base + "/index.m3u8", Summary: "Read the VOD timeline; segments are generated only when requested", Responses: map[string]*huma.Response{"200": {Description: "HLS VOD playlist", Content: map[string]*huma.MediaType{"application/vnd.apple.mpegurl": {Schema: &huma.Schema{Type: "string"}}}}}}, func(ctx context.Context, in *videoInput) (*huma.StreamResponse, error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		source, err := s.video.Source(ctx, domain.FileID{ID: id})
		if err != nil {
			return nil, videoFailure(err)
		}
		// The browser knows the content key before fetching a playlist. Binding
		// it here also prevents a stale gallery from starting replacement bytes.
		if in.Version != source.BlobKey {
			return nil, videoFailure(library.ErrVideoChanged)
		}
		playlist := source.Playlist(func(index int) string {
			return fmt.Sprintf("segments/%d?v=%s", index, url.QueryEscape(source.Version()))
		})
		return &huma.StreamResponse{Body: func(hctx huma.Context) {
			_, w := humago.Unwrap(hctx)
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Set("Cache-Control", "private, no-cache")
			fmt.Fprint(w, playlist)
		}}, nil
	})
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		huma.Register(s.contract, huma.Operation{OperationID: method + "-video-segment", Method: method, Path: base + "/segments/{segment}", Summary: "Read or generate one bounded HLS segment", Responses: map[string]*huma.Response{"200": {Description: "Independently decodable H.264/AAC segment", Content: map[string]*huma.MediaType{"video/mp2t": {Schema: &huma.Schema{Type: "string", Format: "binary"}}}}, "206": {Description: "Requested byte range"}, "409": {Description: "Video content changed"}}}, func(ctx context.Context, in *videoSegmentInput) (*huma.StreamResponse, error) {
			id, err := parseID(in.ID)
			if err != nil {
				return nil, err
			}
			return &huma.StreamResponse{Body: func(hctx huma.Context) {
				req, w := humago.Unwrap(hctx)
				f, err := s.video.OpenSegment(ctx, domain.FileID{ID: id}, in.Version, in.Segment)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					writeFailure(w, videoFailure(err))
					return
				}
				defer f.Close()
				w.Header().Set("Content-Type", "video/mp2t")
				// Re-encoding after eviction or a tool upgrade can produce different
				// bytes. Cache validation follows the output digest, not the recipe.
				w.Header().Set("Cache-Control", "private, no-cache")
				w.Header().Set("ETag", `"`+f.ETag+`"`)
				w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
				http.ServeContent(w, req, fmt.Sprintf("%d.ts", in.Segment), time.Time{}, f)
			}}, nil
		})
	}
	huma.Post(s.contract, Prefix+"/settings/video-cache/clear", func(ctx context.Context, _ *struct{}) (*output[struct{}], error) {
		return &output[struct{}]{}, failure(s.video.Clear(ctx))
	})
}
