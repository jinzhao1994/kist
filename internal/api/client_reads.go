package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/buildinfo"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/wire"
)

// Client reads use request-scoped snapshots and never instantiate a NAS Library.
func (s *Server) clientReadRoutes() {
	s.clientBoxReadRoutes()
	s.clientBoxSearchRoute()
	s.clientFilesetReadRoutes()
	huma.Get(s.contract, Prefix+"/files", func(ctx context.Context, in *FileQueryInput) (*output[fileSearchResult], error) {
		out := fileSearchResult{Files: []fileSearchItem{}}
		err := s.client.View(ctx, func(v *library.ClientView) error {
			q, err := buildFileQuery(in, func(refs []string) (query.TagExpr, error) { return v.TagCondition(ctx, refs) }, func(refs []string) (query.TagExpr, error) { return v.ExcludeTags(ctx, refs) })
			if err != nil {
				return err
			}
			files, cursor, err := v.FilesWithTags(ctx, q)
			if err != nil {
				return err
			}
			out.Cursor = cursor
			for _, file := range files {
				item, err := renderFile(ctx, v, file.File)
				if err != nil {
					return err
				}
				result := fileSearchItem{File: item}
				for _, tag := range file.Tags {
					result.DisplayTags = append(result.DisplayTags, displayTag{ID: tag.ID.String(), Name: tag.Name})
				}
				out.Files = append(out.Files, result)
			}
			return nil
		})
		return response(out, err)
	})
	huma.Get(s.contract, Prefix+"/info", func(ctx context.Context, _ *struct{}) (*output[libraryInfo], error) {
		out := libraryInfo{Version: buildinfo.Get().Version, MaxUploadBytes: s.maxUploadBytes, MaxUploadFiles: s.maxUploadFiles}
		err := s.client.View(ctx, func(v *library.ClientView) error { out.Timezone = v.Zone().String(); return nil })
		return response(out, err)
	})
	huma.Get(s.contract, Prefix+"/client/catalog", func(ctx context.Context, _ *struct{}) (*output[library.ClientCatalogStatus], error) {
		status, err := s.client.CatalogStatus(ctx)
		return response(status, err)
	})
	huma.Get(s.contract, Prefix+"/boxes", func(ctx context.Context, in *struct {
		Tag string `query:"tag"`
	}) (*output[wire.BoxList], error) {
		opt := library.ListBoxesOptions{}
		if in.Tag != "" {
			id, err := parseID(in.Tag)
			if err != nil {
				return nil, err
			}
			tag := domain.TagID{ID: id}
			opt.Tag = &tag
		}
		boxes, err := s.client.Boxes(ctx, opt)
		return response(wire.BoxList{Boxes: wire.NewBoxes(boxes)}, err)
	})
	huma.Get(s.contract, Prefix+"/boxes/{id}", func(ctx context.Context, in *entityInput) (*output[wire.Box], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		box, found, err := s.client.Box(ctx, domain.BoxID{ID: id})
		if err == nil && !found {
			err = library.ErrNoSuchBox
		}
		return response(wire.NewBox(box), err)
	})
	huma.Get(s.contract, Prefix+"/files/{id}", func(ctx context.Context, in *entityInput) (*output[fileDetail], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		var out fileDetail
		err = s.client.View(ctx, func(v *library.ClientView) error {
			file, found, err := v.File(ctx, domain.FileID{ID: id})
			if err != nil {
				return err
			}
			if !found {
				return library.ErrNoSuchFile
			}
			out, err = renderFileDetail(ctx, v, file)
			return err
		})
		return response(out, err)
	})
	huma.Get(s.contract, Prefix+"/tags", func(ctx context.Context, _ *struct{}) (*output[wire.TagTree], error) {
		tree, counts, err := s.client.TagTreeCounts(ctx)
		if err != nil {
			return nil, failure(err)
		}
		return response(wire.TagTree{Tags: renderTags(tree, counts)}, nil)
	})
	huma.Get(s.contract, Prefix+"/saved-searches", func(ctx context.Context, _ *struct{}) (*output[savedSearchList], error) {
		items, err := s.client.SavedSearches(ctx)
		out := savedSearchList{Searches: []savedSearchView{}}
		for _, item := range items {
			out.Searches = append(out.Searches, savedSearchView{ID: item.ID.String(), Name: item.Name, Definition: item.Definition})
		}
		return response(out, err)
	})
}
