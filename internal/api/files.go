package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/wire"
)

type fileQueryInput struct {
	Kinds  []string `query:"kind"`
	Boxes  []string `query:"box"`
	Folder string   `query:"folder"`
	All    []string `query:"tag_all"`
	Any    []string `query:"tag_any"`
	Not    []string `query:"tag_not"`
	Name   string   `query:"name"`
	From   string   `query:"from"`
	To     string   `query:"to"`
	Sort   string   `query:"sort" enum:"captured,imported,name" default:"captured"`
	Asc    bool     `query:"asc"`
	Cursor string   `query:"cursor"`
	Limit  int      `query:"limit" minimum:"0" maximum:"1000"`
}

type fileDetail struct {
	wire.FileDetail
	Tags []string `json:"tags"`
}
type folderList struct {
	Folders []wire.Folder `json:"folders"`
}
type tagList struct {
	Tags []string `json:"tags"`
}

func (s *Server) fileQuery(in *fileQueryInput) (query.Query, error) {
	q := query.Query{Kinds: in.Kinds, Text: in.Name, Cursor: in.Cursor, Limit: in.Limit, Sort: query.SortSpec{Asc: in.Asc}}
	switch in.Sort {
	case "name":
		q.Sort.Key = query.SortName
	case "imported":
		q.Sort.Key = query.SortImported
	}
	for _, raw := range in.Boxes {
		id, err := parseID(raw)
		if err != nil {
			return q, err
		}
		q.Boxes = append(q.Boxes, domain.BoxID{ID: id})
	}
	if in.Folder != "" {
		id, err := parseID(in.Folder)
		if err != nil {
			return q, err
		}
		folder := domain.FolderID{ID: id}
		q.Folder = &folder
	}
	if in.From != "" || in.To != "" {
		q.Time = &query.DateRange{}
		var err error
		if in.From != "" {
			q.Time.From, err = domain.ParseLocalDate(in.From)
			if err != nil {
				return q, invalid("Invalid from date")
			}
		}
		if in.To != "" {
			q.Time.To, err = domain.ParseLocalDate(in.To)
			if err != nil {
				return q, invalid("Invalid to date")
			}
		}
	}
	if err := q.Validate(); err != nil {
		return q, invalid(err.Error())
	}
	if q.Cursor != "" {
		if _, _, err := query.DecodeCursor(q.Sort, q.Cursor); err != nil {
			return q, invalid(err.Error())
		}
	}
	all, err := s.library.TagCondition(in.All)
	if err != nil {
		return q, failure(err)
	}
	not, err := s.library.ExcludeTags(in.Not)
	if err != nil {
		return q, failure(err)
	}
	var conditions query.TagAll
	if all != nil {
		conditions = append(conditions, all)
	}
	if not != nil {
		conditions = append(conditions, not)
	}
	var any query.TagAnyOf
	for _, ref := range in.Any {
		expr, err := s.library.TagCondition([]string{ref})
		if err != nil {
			return q, failure(err)
		}
		any = append(any, expr)
	}
	if len(any) > 0 {
		conditions = append(conditions, any)
	}
	if len(conditions) > 0 {
		q.Tags = conditions
	}
	return q, nil
}
func (s *Server) wireFile(ctx context.Context, f domain.File) (wire.File, error) {
	box, path, err := s.library.FileLocation(ctx, f)
	if err != nil {
		return wire.File{}, err
	}
	out := wire.NewFile(f, box.String(), path, s.library.Zone())
	info, err := s.library.MediaInfo(ctx, f.ID)
	if err != nil {
		return out, err
	}
	out.MediaType, out.Kind, out.ExtractionPending = info.MIME, info.Kind, info.Pending
	return out, nil
}
func tagStrings(tags []domain.TagID) []string {
	out := make([]string, len(tags))
	for i, id := range tags {
		out[i] = id.String()
	}
	return out
}
func (s *Server) fileRoutes() {
	huma.Get(s.contract, Prefix+"/files", func(ctx context.Context, in *fileQueryInput) (*output[wire.FileList], error) {
		q, err := s.fileQuery(in)
		if err != nil {
			return nil, err
		}
		files, cursor, err := s.library.Files(ctx, q)
		if err != nil {
			return nil, failure(err)
		}
		out := wire.FileList{Files: make([]wire.File, 0, len(files)), Cursor: cursor}
		for _, f := range files {
			item, err := s.wireFile(ctx, f)
			if err != nil {
				return nil, failure(err)
			}
			out.Files = append(out.Files, item)
		}
		return response(out, nil)
	})
	huma.Get(s.contract, Prefix+"/files/{id}", func(ctx context.Context, in *entityInput) (*output[fileDetail], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		f, found, err := s.library.File(ctx, domain.FileID{ID: id})
		if err != nil {
			return nil, failure(err)
		}
		if !found {
			return nil, failure(library.ErrNoSuchFile)
		}
		item, err := s.wireFile(ctx, f)
		if err != nil {
			return nil, failure(err)
		}
		info, err := s.library.CapturedInfo(ctx, f)
		if err != nil {
			return nil, failure(err)
		}
		rows, err := s.library.FileMetadata(ctx, f.ID)
		if err != nil {
			return nil, failure(err)
		}
		tags, err := s.library.FileTags(ctx, f.ID)
		if err != nil {
			return nil, failure(err)
		}
		out := fileDetail{FileDetail: wire.FileDetail{File: item, CapturedFrom: info.From, CapturedZone: info.Zone, Extracted: info.Extracted, Metadata: make([]wire.Metadata, 0, len(rows))}, Tags: tagStrings(tags)}
		if out.Extracted == nil {
			out.Extracted = []string{}
		}
		for _, row := range rows {
			if row.Tombstone {
				continue
			}
			out.Metadata = append(out.Metadata, wire.Metadata{Key: row.Key, Value: row.Value, Type: string(row.Type), Source: row.Source})
		}
		return response(out, nil)
	})
	huma.Get(s.contract, Prefix+"/boxes/{id}/folders", func(ctx context.Context, in *entityInput) (*output[folderList], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		box := domain.BoxID{ID: id}
		_, found, err := s.library.Box(ctx, box)
		if err != nil {
			return nil, failure(err)
		}
		if !found {
			return nil, failure(library.ErrNoSuchBox)
		}
		folders, err := s.library.Folders(ctx, box)
		out := folderList{Folders: make([]wire.Folder, 0, len(folders))}
		for _, f := range folders {
			out.Folders = append(out.Folders, wire.NewFolder(f))
		}
		return response(out, err)
	})
	huma.Get(s.contract, Prefix+"/boxes/{id}/tags", func(ctx context.Context, in *entityInput) (*output[tagList], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		box := domain.BoxID{ID: id}
		_, found, err := s.library.Box(ctx, box)
		if err != nil {
			return nil, failure(err)
		}
		if !found {
			return nil, failure(library.ErrNoSuchBox)
		}
		tags, err := s.library.BoxTags(ctx, box)
		return response(tagList{Tags: tagStrings(tags)}, err)
	})
}
