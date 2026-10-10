package api

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

type historyInput struct {
	Box    string `query:"box"`
	File   string `query:"file"`
	Action string `query:"action"`
	Group  int64  `query:"group" minimum:"0"`
	Before int64  `query:"before" minimum:"0"`
	Limit  int    `query:"limit" minimum:"0" maximum:"1000"`
}
type historyOp struct {
	wire.Op
	Undoable bool `json:"undoable"`
}
type historyList struct {
	Ops    []historyOp `json:"ops"`
	Before int64       `json:"before,omitempty"`
}
type historyDetail struct {
	historyOp
	Payload map[string]any `json:"payload"`
}
type opInput struct {
	ID int64 `path:"id" minimum:"1"`
}
type undoInput struct {
	ID   int64  `path:"id" minimum:"1"`
	Key  string `header:"Idempotency-Key" required:"true"`
	Body struct{}
}

func renderHistory(entry library.HistoryEntry) historyOp {
	return historyOp{Op: wire.Op{ID: entry.ID, TS: entry.TS, Action: string(entry.Action), Origin: string(entry.Origin), RevertsID: entry.RevertsID, RootOpID: entry.RootOpID, UndoneBy: entry.UndoneBy}, Undoable: entry.Undoable}
}
func (s *Server) historyRoutes() {
	huma.Get(s.contract, Prefix+"/ops", func(ctx context.Context, in *historyInput) (*output[historyList], error) {
		opt := library.ListOpsOptions{Action: in.Action, Before: in.Before, Limit: in.Limit}
		if in.Group != 0 {
			opt.Group = &in.Group
		}
		if in.Box != "" {
			id, err := parseID(in.Box)
			if err != nil {
				return nil, err
			}
			box := domain.BoxID{ID: id}
			opt.Box = &box
		}
		if in.File != "" {
			id, err := parseID(in.File)
			if err != nil {
				return nil, err
			}
			file := domain.FileID{ID: id}
			opt.File = &file
		}
		entries, before, err := s.library.History(ctx, opt)
		out := historyList{Ops: make([]historyOp, 0, len(entries)), Before: before}
		for _, entry := range entries {
			out.Ops = append(out.Ops, renderHistory(entry))
		}
		return response(out, err)
	})
	huma.Get(s.contract, Prefix+"/ops/{id}", func(ctx context.Context, in *opInput) (*output[historyDetail], error) {
		entry, err := s.library.HistoryOp(ctx, in.ID)
		if err != nil {
			return nil, failure(err)
		}
		out := historyDetail{historyOp: renderHistory(entry)}
		// Keep JSON numbers exact, including nanosecond metadata in old payloads.
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(entry.Payload, &payload); err != nil {
			return nil, failure(err)
		}
		out.Payload = make(map[string]any, len(payload))
		for key, value := range payload {
			out.Payload[key] = value
		}
		return response(out, nil)
	})
	huma.Post(s.contract, Prefix+"/ops/{id}/undo", func(ctx context.Context, in *undoInput) (*output[library.RequestResult], error) {
		return s.mutate(ctx, in.Key, "POST", Prefix+"/ops/"+strconv.FormatInt(in.ID, 10)+"/undo", in.Body, func(ctx context.Context) error {
			_, err := s.library.Undo(ctx, in.ID)
			return err
		})
	})
}
