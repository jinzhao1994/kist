package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/library"
)

type clientLocalHistoryList struct {
	Entries []library.ClientLocalHistoryEntry `json:"entries"`
	Before  string                            `json:"before,omitempty"`
}

func (s *Server) clientLocalHistoryRoutes() {
	s.clientPendingHistoryRoute()
	huma.Get(s.contract, Prefix+"/client/history", func(ctx context.Context, in *struct {
		Before string `query:"before" maxLength:"256"`
		Limit  int    `query:"limit" minimum:"0" maximum:"1000"`
	}) (*output[clientLocalHistoryList], error) {
		var result clientLocalHistoryList
		err := s.withHistoryBudget(ctx, func(ctx context.Context) error {
			var err error
			result.Entries, result.Before, err = s.client.LocalHistory(ctx, in.Before, in.Limit)
			return err
		})
		return response(result, err)
	})
	huma.Get(s.contract, Prefix+"/client/history/{id}", func(ctx context.Context, in *struct {
		ID string `path:"id" minLength:"1" maxLength:"256"`
	}) (*output[library.ClientLocalHistoryDetail], error) {
		var result library.ClientLocalHistoryDetail
		err := s.withHistoryBudget(ctx, func(ctx context.Context) error {
			var err error
			result, err = s.client.LocalHistoryOp(ctx, in.ID)
			return err
		})
		return response(result, err)
	})
	huma.Post(s.contract, Prefix+"/client/history/{id}/undo", func(ctx context.Context, in *struct {
		ID   string `path:"id" minLength:"1" maxLength:"256"`
		Key  string `header:"Idempotency-Key" required:"true"`
		Body struct{}
	}) (*output[library.ClientRequestResult], error) {
		return s.clientMutate(ctx, in.Key, "POST", Prefix+"/client/history/"+in.ID+"/undo", in.Body, func(ctx context.Context) error {
			_, err := s.client.Undo(ctx, library.SyncReference{LocalID: in.ID})
			return err
		})
	})
}
