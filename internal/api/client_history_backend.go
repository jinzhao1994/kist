package api

import (
	"context"

	"github.com/jinzhao1994/kist/internal/library"
)

// Client history replays a confirmed prefix and shares the historical query budget.
type clientHistoryBackend struct{ server *Server }

func (b clientHistoryBackend) History(ctx context.Context, opt library.ListOpsOptions) ([]library.HistoryEntry, int64, error) {
	var entries []library.HistoryEntry
	var before int64
	err := b.server.withHistoryBudget(ctx, func(ctx context.Context) error {
		var err error
		entries, before, err = b.server.client.History(ctx, opt)
		return err
	})
	return entries, before, err
}
func (b clientHistoryBackend) HistoryOp(ctx context.Context, id int64) (library.HistoryEntry, error) {
	var entry library.HistoryEntry
	err := b.server.withHistoryBudget(ctx, func(ctx context.Context) error {
		var err error
		entry, err = b.server.client.HistoryOp(ctx, id)
		return err
	})
	return entry, err
}
func (b clientHistoryBackend) HistorySummary(ctx context.Context, id int64) (library.HistorySummary, error) {
	var summary library.HistorySummary
	err := b.server.withHistoryBudget(ctx, func(ctx context.Context) error {
		var err error
		summary, err = b.server.client.HistorySummary(ctx, id)
		return err
	})
	return summary, err
}
