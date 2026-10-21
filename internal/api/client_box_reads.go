package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func (s *Server) clientBoxReadRoutes() {
	huma.Get(s.contract, Prefix+"/boxes/{id}/folders", func(ctx context.Context, in *entityInput) (*output[folderList], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		out := folderList{Folders: []wire.Folder{}}
		err = s.client.View(ctx, func(v *library.ClientView) error {
			box := domain.BoxID{ID: id}
			_, found, err := v.Box(ctx, box)
			if err != nil {
				return err
			}
			if !found {
				return library.ErrNoSuchBox
			}
			folders, err := v.Folders(ctx, box)
			if err != nil {
				return err
			}
			for _, folder := range folders {
				out.Folders = append(out.Folders, wire.NewFolder(folder))
			}
			return nil
		})
		return response(out, err)
	})
	huma.Get(s.contract, Prefix+"/boxes/{id}/tags", func(ctx context.Context, in *entityInput) (*output[tagList], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		out := tagList{Tags: []string{}}
		err = s.client.View(ctx, func(v *library.ClientView) error {
			box := domain.BoxID{ID: id}
			_, found, err := v.Box(ctx, box)
			if err != nil {
				return err
			}
			if !found {
				return library.ErrNoSuchBox
			}
			tags, err := v.BoxTags(ctx, box)
			out.Tags = tagStrings(tags)
			return err
		})
		return response(out, err)
	})
	huma.Get(s.contract, Prefix+"/boxes/{id}/cover", func(ctx context.Context, in *entityInput) (*output[boxCover], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		out := boxCover{}
		err = s.client.View(ctx, func(v *library.ClientView) error {
			file, found, err := v.BoxCover(ctx, domain.BoxID{ID: id})
			if err != nil || !found {
				return err
			}
			item, err := renderFile(ctx, v, file)
			if err != nil {
				return err
			}
			out.File = &item
			return nil
		})
		return response(out, err)
	})
}
