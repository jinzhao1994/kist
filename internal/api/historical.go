package api

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/wire"
)

type historicalEntityInput struct {
	Op int64  `path:"op" minimum:"1"`
	ID string `path:"id"`
}
type historicalBoxInput struct {
	Op     int64  `path:"op" minimum:"1"`
	ID     string `path:"id"`
	Cursor string `query:"cursor"`
	Limit  int    `query:"limit" minimum:"0" maximum:"1000"`
}
type historicalBox struct {
	Op      int64         `json:"op"`
	Box     wire.Box      `json:"box"`
	Files   []wire.File   `json:"files"`
	Cursor  string        `json:"cursor,omitempty"`
	Folders []wire.Folder `json:"folders"`
	BoxTags []string      `json:"box_tags"`
	Tags    []wire.Tag    `json:"tags"`
}
type historicalFile struct {
	Op   int64      `json:"op"`
	File fileDetail `json:"file"`
	Tags []wire.Tag `json:"tags"`
}

type historyReader interface {
	fileReader
	Box(context.Context, domain.BoxID) (domain.Box, bool, error)
	BoxTags(context.Context, domain.BoxID) ([]domain.TagID, error)
	File(context.Context, domain.FileID) (domain.File, bool, error)
	Files(context.Context, query.Query) ([]domain.File, string, error)
	Folders(context.Context, domain.BoxID) ([]domain.Folder, error)
	TagTreeCounts(context.Context) (*library.TagTree, map[domain.TagID]library.TagCounts, error)
	OpenContent(context.Context, domain.FileID) (domain.File, io.ReadSeekCloser, error)
}

type nasHistoryReader struct{ *library.HistoricalView }

func (v nasHistoryReader) OpenContent(ctx context.Context, id domain.FileID) (domain.File, io.ReadSeekCloser, error) {
	return v.HistoricalView.OpenContent(ctx, id)
}

// Bound concurrent replay work, including queue time in the deadline. The
// semaphore only limits resources; it contains no cached archive state.
func (s *Server) withHistoryBudget(ctx context.Context, fn func(context.Context) error) (err error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	defer func() {
		if errors.Is(err, context.DeadlineExceeded) {
			err = huma.NewError(503, "Historical query exceeded its time limit")
		}
	}()
	select {
	case s.historySlots <- struct{}{}:
		defer func() { <-s.historySlots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	return fn(ctx)
}

func (s *Server) withHistory(ctx context.Context, target int64, fn func(context.Context, historyReader) error) error {
	return s.withHistoryBudget(ctx, func(ctx context.Context) error {
		if s.client != nil {
			return s.client.ViewAt(ctx, target, func(view *library.ClientHistoricalView) error { return fn(ctx, view) })
		}
		return s.library.ViewAt(ctx, target, func(view *library.HistoricalView) error { return fn(ctx, nasHistoryReader{view}) })
	})
}

func historicalTags(ctx context.Context, view historyReader) ([]wire.Tag, error) {
	tree, counts, err := view.TagTreeCounts(ctx)
	if err != nil {
		return nil, err
	}
	return renderTags(tree, counts), nil
}

func (s *Server) historicalRoutes() {
	huma.Get(s.contract, Prefix+"/history/{op}/boxes/{id}", func(ctx context.Context, in *historicalBoxInput) (*output[historicalBox], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		boxID := domain.BoxID{ID: id}
		q := query.Query{Boxes: []domain.BoxID{boxID}, Cursor: in.Cursor, Limit: in.Limit, Sort: query.SortSpec{Key: query.SortName, Asc: true}}
		if err := q.Validate(); err != nil {
			return nil, invalid(err.Error())
		}
		if in.Cursor != "" {
			if _, _, err := query.DecodeCursor(q.Sort, in.Cursor); err != nil {
				return nil, invalid(err.Error())
			}
		}
		out := historicalBox{Op: in.Op, Files: []wire.File{}, Folders: []wire.Folder{}}
		err = s.withHistory(ctx, in.Op, func(ctx context.Context, view historyReader) error {
			box, found, err := view.Box(ctx, boxID)
			if err != nil {
				return err
			}
			if !found {
				return library.ErrNoSuchBox
			}
			out.Box = wire.NewBox(box)
			files, cursor, err := view.Files(ctx, q)
			if err != nil {
				return err
			}
			out.Cursor = cursor
			for _, file := range files {
				if err := ctx.Err(); err != nil {
					return err
				}
				item, err := renderFile(ctx, view, file)
				if err != nil {
					return err
				}
				// Current extractor availability is not historical state.
				item.ExtractionPending = []string{}
				out.Files = append(out.Files, item)
			}
			folders, err := view.Folders(ctx, boxID)
			if err != nil {
				return err
			}
			for _, folder := range folders {
				out.Folders = append(out.Folders, wire.NewFolder(folder))
			}
			tags, err := view.BoxTags(ctx, boxID)
			if err != nil {
				return err
			}
			out.BoxTags = tagStrings(tags)
			out.Tags, err = historicalTags(ctx, view)
			return err
		})
		return response(out, err)
	})
	huma.Get(s.contract, Prefix+"/history/{op}/files/{id}", func(ctx context.Context, in *historicalEntityInput) (*output[historicalFile], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		out := historicalFile{Op: in.Op}
		err = s.withHistory(ctx, in.Op, func(ctx context.Context, view historyReader) error {
			file, found, err := view.File(ctx, domain.FileID{ID: id})
			if err != nil {
				return err
			}
			if !found {
				return library.ErrNoSuchFile
			}
			out.File, err = renderFileDetail(ctx, view, file)
			if err != nil {
				return err
			}
			out.File.File.ExtractionPending = []string{}
			out.Tags, err = historicalTags(ctx, view)
			return err
		})
		return response(out, err)
	})
}
