package library

import (
	"context"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

// FilesWithTags reads file results and direct tag labels from one overlay snapshot.
func (c *Client) FilesWithTags(ctx context.Context, q query.Query) ([]FileSearchItem, string, error) {
	var items []FileSearchItem
	var cursor string
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		zone, err := c.readZone()
		if err != nil {
			return err
		}
		files, next, err := store.ListFiles(tx.Queryer(), q, zone)
		if err != nil {
			return err
		}
		tags, err := store.TagsOfFiles(tx.Queryer(), files)
		if err != nil {
			return err
		}
		cursor = next
		for _, file := range files {
			items = append(items, FileSearchItem{File: file, Tags: tags[file.ID]})
		}
		return nil
	})
	return items, cursor, err
}

// SearchBoxes includes pending edits and does not require cached originals.
func (c *Client) SearchBoxes(ctx context.Context, q query.BoxQuery) ([]BoxSearchItem, string, error) {
	var items []BoxSearchItem
	var cursor string
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		boxes, next, err := store.SearchBoxes(tx.Queryer(), q)
		if err != nil {
			return err
		}
		tags, err := store.TagsOfBoxes(tx.Queryer(), boxes)
		if err != nil {
			return err
		}
		cursor = next
		for _, box := range boxes {
			items = append(items, BoxSearchItem{Box: box, Tags: tags[box.ID]})
		}
		return nil
	})
	return items, cursor, err
}

// BoxCover resolves the soft reference in one snapshot, independently of cache availability.
func (c *Client) BoxCover(ctx context.Context, id domain.BoxID) (domain.File, bool, error) {
	var file domain.File
	var found bool
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		box, exists, err := tx.GetBox(id)
		if err != nil {
			return err
		}
		if !exists {
			return ErrNoSuchBox
		}
		if box.CoverFileID.IsZero() {
			return nil
		}
		file, found, err = tx.GetFile(box.CoverFileID)
		if err != nil || !found {
			return err
		}
		owner, _, err := store.FolderChain(tx.Queryer(), file.FolderID)
		found = err == nil && owner == id
		return err
	})
	return file, found, err
}
