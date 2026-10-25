package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

func (s *Server) clientPendingHistoryRoute() {
	huma.Get(s.contract, Prefix+"/client/pending-history", func(ctx context.Context, in *struct {
		Box     string `query:"box"`
		File    string `query:"file"`
		Fileset string `query:"fileset"`
		Trash   bool   `query:"trash"`
		Before  string `query:"before" maxLength:"256"`
		Limit   int    `query:"limit" minimum:"0" maximum:"1000"`
	}) (*output[clientLocalHistoryList], error) {
		opt := library.ListOpsOptions{Limit: in.Limit}
		if in.Trash {
			opt = library.TrashOptions(opt)
		}
		if in.Box != "" {
			id, err := parseID(in.Box)
			if err != nil {
				return nil, err
			}
			v := domain.BoxID{ID: id}
			opt.Box = &v
		}
		if in.File != "" {
			id, err := parseID(in.File)
			if err != nil {
				return nil, err
			}
			v := domain.FileID{ID: id}
			opt.File = &v
		}
		if in.Fileset != "" {
			id, err := parseID(in.Fileset)
			if err != nil {
				return nil, err
			}
			v := domain.FolderID{ID: id}
			opt.Fileset = &v
		}
		var result clientLocalHistoryList
		err := s.withHistoryBudget(ctx, func(ctx context.Context) error {
			var err error
			result.Entries, result.Before, err = s.client.PendingHistory(ctx, in.Before, opt)
			return err
		})
		return response(result, err)
	})
}
