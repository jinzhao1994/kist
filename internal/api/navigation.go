package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

type navigationInput struct {
	Path   string `query:"path" doc:"Escaped readable pathname, decoded once per component"`
	Box    string `query:"box"`
	Folder string `query:"folder"`
	File   string `query:"file" doc:"File or fileset identity"`
	View   string `query:"view" enum:"folder"`
}

func (s *Server) navigationRoutes() {
	huma.Get(s.contract, Prefix+"/navigation", func(ctx context.Context, in *navigationInput) (*output[library.NavigationLocation], error) {
		target := library.NavigationTarget{Path: in.Path, FolderView: in.View == "folder"}
		for _, field := range []struct {
			raw string
			out *domain.ID
		}{{in.Box, &target.Box.ID}, {in.Folder, &target.Folder.ID}, {in.File, &target.File}} {
			raw, out := field.raw, field.out
			if raw == "" {
				continue
			}
			id, err := parseID(raw)
			if err != nil {
				return nil, err
			}
			*out = id
		}
		if s.client != nil {
			location, err := s.client.Navigation(ctx, target)
			return response(location, err)
		}
		location, err := s.library.Navigation(ctx, target)
		return response(location, err)
	})
}
