package api

import (
	"context"
	"errors"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/library"
)

func (s *Server) locationRoutes() {
	s.queueRoutes("/", map[string]*library.OCRQueue{"geo": s.library.LocationQueue()})
	huma.Get(s.contract, Prefix+"/geo/settings", func(ctx context.Context, _ *struct{}) (*output[library.LocationSettings], error) {
		v, err := s.library.LocationSettings(ctx)
		return response(v, err)
	})
	huma.Put(s.contract, Prefix+"/geo/settings", func(ctx context.Context, in *struct{ Body library.LocationSettings }) (*output[library.LocationSettings], error) {
		v, err := s.library.SaveLocationSettings(ctx, in.Body)
		if errors.Is(err, config.ErrChanged) {
			return nil, huma.Error409Conflict("Location settings changed; reload before saving")
		}
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return response(v, nil)
	})
}
