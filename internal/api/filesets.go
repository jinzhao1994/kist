package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/wire"
	"strings"
)

type filesetList struct {
	Filesets []domain.Fileset `json:"filesets"`
}
type filesetWriteInput struct {
	ID   string `path:"id"`
	Key  string `header:"Idempotency-Key" required:"true"`
	Body struct {
		Box    string          `json:"box"`
		Before *domain.Fileset `json:"before,omitempty"`
		After  *domain.Fileset `json:"after,omitempty"`
	}
}
type entryQueryInput struct {
	FileQueryInput
	IncludeMembers bool `query:"include_members"`
}

func (s *Server) filesetRoutes() {
	huma.Get(s.contract, Prefix+"/filesets", func(ctx context.Context, _ *struct{}) (*output[filesetList], error) {
		sets, err := s.library.Filesets(ctx)
		if sets == nil {
			sets = []domain.Fileset{}
		}
		return response(filesetList{sets}, err)
	})
	huma.Put(s.contract, Prefix+"/filesets/{id}", func(ctx context.Context, in *filesetWriteInput) (*output[library.RequestResult], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		box, err := parseID(in.Body.Box)
		if err != nil {
			return nil, err
		}
		return s.mutate(ctx, in.Key, "PUT", Prefix+"/filesets/"+id.String(), in.Body, func(ctx context.Context) error {
			_, err := s.library.SetFileset(ctx, domain.BoxID{ID: box}, domain.FolderID{ID: id}, in.Body.Before, in.Body.After)
			return err
		})
	})
	huma.Get(s.contract, Prefix+"/entries", func(ctx context.Context, in *entryQueryInput) (*output[fileSearchResult], error) {
		cursor := in.Cursor
		in.Cursor = ""
		q, err := s.fileQuery(&in.FileQueryInput)
		if err != nil {
			return nil, err
		}
		if cursor != "" {
			prefix := "entries-v1:false:"
			if in.IncludeMembers {
				prefix = "entries-v1:true:"
			}
			if !strings.HasPrefix(cursor, prefix) {
				return nil, huma.Error400BadRequest("cursor belongs to another result mode")
			}
			if _, _, err := query.DecodeCursor(q.Sort, strings.TrimPrefix(cursor, prefix)); err != nil {
				return nil, huma.Error400BadRequest("invalid cursor")
			}
		}
		q.Cursor = cursor
		entries, next, err := s.library.Browse(ctx, q, in.IncludeMembers)
		if err != nil {
			return nil, failure(err)
		}
		out := fileSearchResult{Files: []fileSearchItem{}, Cursor: next}
		for _, e := range entries {
			var item wire.File
			if !e.Primary.IsZero() {
				f, ok, err := s.library.File(ctx, e.Primary)
				if err != nil {
					return nil, failure(err)
				}
				if !ok {
					continue
				}
				item, err = renderFile(ctx, s.library, f)
				if err != nil {
					return nil, failure(err)
				}
			} else {
				box, path, err := s.library.FileLocation(ctx, e.File)
				if err != nil {
					return nil, failure(err)
				}
				item = wire.NewFile(e.File, box.String(), path, s.library.Zone())
			}
			result := fileSearchItem{File: item}
			if !e.Fileset.IsZero() {
				result.FilesetID = e.Fileset.String()
				result.PrimaryID = e.Primary.String()
				result.NoPrimary = e.Primary.IsZero()
				result.Name = e.File.Name
				result.FolderID = e.File.FolderID.String()
				result.UpdatedAt = e.File.UpdatedAt
				result.Size = &e.Size
				_, logicalPath, err := s.library.FileLocation(ctx, e.File)
				if err != nil {
					return nil, failure(err)
				}
				result.Path = logicalPath
			}
			if !e.Primary.IsZero() {
				tags, err := s.library.FileTags(ctx, e.Primary)
				if err != nil {
					return nil, failure(err)
				}
				for _, id := range tags {
					tag, err := s.library.ResolveTag(id.String())
					if err != nil {
						return nil, failure(err)
					}
					{
						result.DisplayTags = append(result.DisplayTags, displayTag{ID: tag.ID.String(), Name: tag.Name})
					}
				}
			}
			out.Files = append(out.Files, result)
		}
		return response(out, nil)
	})
}
