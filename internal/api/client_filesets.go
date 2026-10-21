package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/wire"
)

func (s *Server) clientFilesetReadRoutes() {
	huma.Get(s.contract, Prefix+"/filesets", func(ctx context.Context, _ *struct{}) (*output[filesetList], error) {
		sets, err := s.client.Filesets(ctx)
		if sets == nil {
			sets = []domain.Fileset{}
		}
		return response(filesetList{sets}, err)
	})
	huma.Get(s.contract, Prefix+"/filesets/{id}", func(ctx context.Context, in *entityInput) (*output[filesetDetail], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		set, folder, box, err := s.client.FilesetDetail(ctx, domain.FolderID{ID: id})
		return response(filesetDetail{Fileset: set, Folder: wire.NewFolder(folder), BoxID: box.String()}, err)
	})
	huma.Get(s.contract, Prefix+"/entries", func(ctx context.Context, in *entryQueryInput) (*output[fileSearchResult], error) {
		var out *output[fileSearchResult]
		err := s.client.View(ctx, func(v *library.ClientView) error {
			q, err := buildEntryQuery(in, func(refs []string) (query.TagExpr, error) { return v.TagCondition(ctx, refs) }, func(refs []string) (query.TagExpr, error) { return v.ExcludeTags(ctx, refs) })
			if err != nil {
				return err
			}
			entries, next, err := v.Browse(ctx, q, in.IncludeMembers)
			if err != nil {
				return err
			}
			out, err = renderEntries(ctx, v, entries, next, func(ref string) (domain.Tag, error) { return v.ResolveTag(ctx, ref) })
			return err
		})
		if err != nil {
			return nil, failure(err)
		}
		return out, nil
	})
}
