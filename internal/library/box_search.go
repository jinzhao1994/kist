package library

import (
	"context"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

// BoxSearchItem includes direct tags from the same read snapshot as the box.
type BoxSearchItem struct {
	domain.Box
	Tags []domain.Tag
}

// SearchBoxes queries boxes without changing the legacy whole-library listing.
func (l *Library) SearchBoxes(ctx context.Context, q query.BoxQuery) ([]BoxSearchItem, string, error) {
	var boxes []domain.Box
	var items []BoxSearchItem
	var cursor string
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		var err error
		boxes, cursor, err = store.SearchBoxes(tx.Queryer(), q)
		if err != nil {
			return err
		}
		tags, err := store.TagsOfBoxes(tx.Queryer(), boxes)
		if err != nil {
			return err
		}
		for _, box := range boxes {
			items = append(items, BoxSearchItem{Box: box, Tags: tags[box.ID]})
		}
		return nil
	})
	return items, cursor, err
}
