package api

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/wire"
)

type FileQueryInput struct {
	Conditions   []string `query:"condition,explode" maxItems:"128"`
	Keywords     []string `query:"keyword,explode" maxItems:"128"`
	NameContains string   `query:"name_contains" maxLength:"4096"`
	NameEquals   string   `query:"name_equals" maxLength:"4096"`
	Kinds        []string `query:"kind"`
	Boxes        []string `query:"box"`
	Folder       string   `query:"folder"`
	Recursive    bool     `query:"recursive" doc:"Include descendants of folder"`
	All          []string `query:"tag_all"`
	Any          []string `query:"tag_any"`
	Not          []string `query:"tag_not"`
	Name         string   `query:"name"`
	From         string   `query:"from"`
	To           string   `query:"to"`
	Sort         string   `query:"sort" enum:"captured,imported,name,updated,type,size" default:"captured"`
	Asc          bool     `query:"asc"`
	Cursor       string   `query:"cursor"`
	Limit        int      `query:"limit" minimum:"0" maximum:"1000"`
}

type fileDetail struct {
	wire.FileDetail
	Tags []string `json:"tags"`
}
type fileSearchItem struct {
	FilesetID   string `json:"fileset_id,omitempty"`
	PrimaryID   string `json:"primary_id,omitempty"`
	PrimaryName string `json:"primary_name,omitempty"`
	NoPrimary   bool   `json:"no_primary,omitempty"`
	wire.File
	DisplayTags []displayTag `json:"display_tags,omitempty"`
}
type fileSearchResult struct {
	Files  []fileSearchItem `json:"files"`
	Cursor string           `json:"cursor,omitempty"`
}
type folderList struct {
	Folders []wire.Folder `json:"folders"`
}
type tagList struct {
	Tags []string `json:"tags"`
}

func (s *Server) fileQuery(in *FileQueryInput) (query.Query, error) {
	return buildFileQuery(in, s.library.TagCondition, s.library.ExcludeTags)
}

func buildFileQuery(in *FileQueryInput, include, exclude func([]string) (query.TagExpr, error)) (query.Query, error) {
	q := query.Query{Recursive: in.Recursive, Keywords: in.Keywords, NameContains: in.NameContains, NameEquals: in.NameEquals, Kinds: in.Kinds, Text: in.Name, Cursor: in.Cursor, Limit: in.Limit, Sort: query.SortSpec{Asc: in.Asc}}
	switch in.Sort {
	case "type":
		q.Sort.Key = query.SortType
	case "size":
		q.Sort.Key = query.SortSize
	case "updated":
		q.Sort.Key = query.SortUpdated
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
	all, err := include(in.All)
	if err != nil {
		return q, failure(err)
	}
	not, err := exclude(in.Not)
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
		expr, err := include([]string{ref})
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
	if len(in.Conditions) > 0 {
		var err error
		q.All, err = buildSearchConditions(in.Conditions, "files", include, exclude)
		if err != nil {
			return q, err
		}
	}
	return q, nil
}

type fileReader interface {
	FileLocation(context.Context, domain.File) (domain.BoxID, string, error)
	Zone() *time.Location
	MediaInfo(context.Context, domain.FileID) (library.MediaInfo, error)
	CapturedInfo(context.Context, domain.File) (library.CapturedInfo, error)
	FileMetadata(context.Context, domain.FileID) ([]extract.Row, error)
	FileTags(context.Context, domain.FileID) ([]domain.TagID, error)
}

func renderFile(ctx context.Context, reader fileReader, f domain.File) (wire.File, error) {
	box, path, err := reader.FileLocation(ctx, f)
	if err != nil {
		return wire.File{}, err
	}
	out := wire.NewFile(f, box.String(), path, reader.Zone())
	info, err := reader.MediaInfo(ctx, f.ID)
	if err != nil {
		return out, err
	}
	out.MediaType, out.Kind, out.ExtractionPending = info.MIME, info.Kind, info.Pending
	if info.RotationBlob == f.BlobKey {
		out.ImageRotation = info.Rotation
		out.ImageRotationManual = info.RotationManual
	}
	out.Size = info.Size
	out.TypeLabel = domain.DisplayFileType(f.Name, info.MIME)
	return out, nil
}

func renderFileDetail(ctx context.Context, reader fileReader, f domain.File) (fileDetail, error) {
	item, err := renderFile(ctx, reader, f)
	if err != nil {
		return fileDetail{}, err
	}
	info, err := reader.CapturedInfo(ctx, f)
	if err != nil {
		return fileDetail{}, err
	}
	rows, err := reader.FileMetadata(ctx, f.ID)
	if err != nil {
		return fileDetail{}, err
	}
	tags, err := reader.FileTags(ctx, f.ID)
	if err != nil {
		return fileDetail{}, err
	}
	out := fileDetail{FileDetail: wire.FileDetail{File: item, CapturedFrom: info.From, CapturedZone: info.Zone, Extracted: info.Extracted, Metadata: make([]wire.Metadata, 0, len(rows))}, Tags: tagStrings(tags)}
	if out.Extracted == nil {
		out.Extracted = []string{}
	}
	for _, row := range rows {
		if !row.Tombstone {
			out.Metadata = append(out.Metadata, wire.Metadata{Key: row.Key, Value: row.Value, Type: string(row.Type), Source: row.Source})
		}
	}
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
	huma.Get(s.contract, Prefix+"/files", func(ctx context.Context, in *FileQueryInput) (*output[fileSearchResult], error) {
		q, err := s.fileQuery(in)
		if err != nil {
			return nil, err
		}
		files, cursor, err := s.library.FilesWithTags(ctx, q)
		if err != nil {
			return nil, failure(err)
		}
		out := fileSearchResult{Files: make([]fileSearchItem, 0, len(files)), Cursor: cursor}
		for _, f := range files {
			item, err := renderFile(ctx, s.library, f.File)
			if err != nil {
				return nil, failure(err)
			}
			result := fileSearchItem{File: item}
			for _, tag := range f.Tags {
				result.DisplayTags = append(result.DisplayTags, displayTag{ID: tag.ID.String(), Name: tag.Name})
			}
			out.Files = append(out.Files, result)
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
		out, err := renderFileDetail(ctx, s.library, f)
		return response(out, err)
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
