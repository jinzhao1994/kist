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

type filesetDetail struct {
	Fileset domain.Fileset `json:"fileset"`
	Folder  wire.Folder    `json:"folder"`
	BoxID   string         `json:"box_id"`
}

func (s *Server) filesetRoutes() {
	huma.Get(s.contract, Prefix+"/filesets/{id}", func(ctx context.Context, in *struct {
		ID string `path:"id"`
	}) (*output[filesetDetail], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		set, folder, box, err := s.library.FilesetDetail(ctx, domain.FolderID{ID: id})
		if err != nil {
			return nil, failure(err)
		}
		return response(filesetDetail{Fileset: set, Folder: wire.NewFolder(folder), BoxID: box.String()}, nil)
	})
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
		q, err := buildEntryQuery(in, s.library.TagCondition, s.library.ExcludeTags)
		if err != nil {
			return nil, err
		}

		entries, next, err := s.library.Browse(ctx, q, in.IncludeMembers)
		if err != nil {
			return nil, failure(err)
		}
		return renderEntries(ctx, s.library, entries, next, s.library.ResolveTag)
	})
}

func buildEntryQuery(in *entryQueryInput, include, exclude func([]string) (query.TagExpr, error)) (query.Query, error) {
	cursor := in.Cursor
	input := in.FileQueryInput
	input.Cursor = ""
	q, err := buildFileQuery(&input, include, exclude)
	if err != nil {
		return query.Query{}, err
	}
	if cursor != "" {
		prefix := "entries-v1:false:"
		if in.IncludeMembers {
			prefix = "entries-v1:true:"
		}
		if !strings.HasPrefix(cursor, prefix) {
			return query.Query{}, huma.Error400BadRequest("cursor belongs to another result mode")
		}
		if _, _, err := query.DecodeCursor(q.Sort, strings.TrimPrefix(cursor, prefix)); err != nil {
			return query.Query{}, huma.Error400BadRequest("invalid cursor")
		}
	}
	q.Cursor = cursor
	return q, nil
}

type entryReader interface {
	fileReader
	File(context.Context, domain.FileID) (domain.File, bool, error)
}

func renderEntries(ctx context.Context, reader entryReader, entries []library.BrowseEntry, next string, resolve func(string) (domain.Tag, error)) (*output[fileSearchResult], error) {
	out := fileSearchResult{Files: []fileSearchItem{}, Cursor: next}
	for _, e := range entries {
		var item wire.File
		if !e.Primary.IsZero() {
			f, ok, err := reader.File(ctx, e.Primary)
			if err != nil {
				return nil, failure(err)
			}
			if !ok {
				continue
			}
			item, err = renderFile(ctx, reader, f)
			if err != nil {
				return nil, failure(err)
			}
		} else {
			box, path, err := reader.FileLocation(ctx, e.File)
			if err != nil {
				return nil, failure(err)
			}
			item = wire.NewFile(e.File, box.String(), path, reader.Zone())
		}
		result := fileSearchItem{File: item}
		if !e.Fileset.IsZero() {
			result.FilesetID = e.Fileset.String()
			result.PrimaryID = e.Primary.String()
			result.NoPrimary = e.Primary.IsZero()
			result.PrimaryName = item.Name
			result.Name = e.File.Name
			result.FolderID = e.File.FolderID.String()
			result.UpdatedAt = e.File.UpdatedAt
			result.Size = &e.Size
			_, logicalPath, err := reader.FileLocation(ctx, e.File)
			if err != nil {
				return nil, failure(err)
			}
			result.Path = logicalPath
		}
		if !e.Primary.IsZero() {
			tags, err := reader.FileTags(ctx, e.Primary)
			if err != nil {
				return nil, failure(err)
			}
			for _, id := range tags {
				tag, err := resolve(id.String())
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
}
