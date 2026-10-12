package library

import (
	"context"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

// SearchBoxes queries boxes without changing the legacy whole-library listing.
func (l *Library) SearchBoxes(ctx context.Context, q query.BoxQuery) ([]domain.Box, string, error) {
	var boxes []domain.Box
	var cursor string
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		var err error
		boxes, cursor, err = store.SearchBoxes(tx.Queryer(), q)
		return err
	})
	return boxes, cursor, err
}
