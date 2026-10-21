package library

import (
	"context"
	"path"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

// Catalog reads use known object facts whether or not any content is cached.
func (c *Client) File(ctx context.Context, id domain.FileID) (domain.File, bool, error) {
	var file domain.File
	var found bool
	err := c.readView(ctx, func(tx *store.ViewTx) error { var err error; file, found, err = tx.GetFile(id); return err })
	return file, found, err
}

func (c *Client) Folders(ctx context.Context, box domain.BoxID) ([]domain.Folder, error) {
	var result []domain.Folder
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		var err error
		result, err = store.FoldersOfBox(tx.Queryer(), box)
		return err
	})
	return result, err
}

func (c *Client) Boxes(ctx context.Context, opt ListBoxesOptions) ([]domain.Box, error) {
	var result []domain.Box
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		if opt.Tag != nil {
			_, found, err := store.TagByID(tx.Queryer(), *opt.Tag)
			if err != nil {
				return err
			}
			if !found {
				return ErrNoSuchTag
			}
		}
		var err error
		result, err = store.ListBoxes(tx.Queryer(), opt)
		return err
	})
	return result, err
}

func (c *Client) FileTags(ctx context.Context, id domain.FileID) ([]domain.TagID, error) {
	var result []domain.TagID
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		var err error
		result, err = store.FileTags(tx.Queryer(), id)
		return err
	})
	return result, err
}

func (c *Client) BoxTags(ctx context.Context, id domain.BoxID) ([]domain.TagID, error) {
	var result []domain.TagID
	err := c.readView(ctx, func(tx *store.ViewTx) error { var err error; result, err = store.BoxTags(tx.Queryer(), id); return err })
	return result, err
}

func (c *Client) FileMetadata(ctx context.Context, id domain.FileID) ([]extract.Row, error) {
	var result []extract.Row
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		var err error
		result, err = store.FileMetadata(tx.Queryer(), id)
		return err
	})
	return result, err
}

func (c *Client) FileLocation(ctx context.Context, file domain.File) (domain.BoxID, string, error) {
	var box domain.BoxID
	var dirs []string
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		var err error
		box, dirs, err = store.FolderChain(tx.Queryer(), file.FolderID)
		return err
	})
	if err != nil {
		return domain.BoxID{}, "", err
	}
	return box, path.Join(append(dirs, file.Name)...), nil
}

// readZone is called while the client lock protects the settings snapshot.
func (c *Client) readZone() (*time.Location, error) {
	settings, err := c.settings()
	if err != nil {
		return nil, err
	}
	return time.LoadLocation(settings.Timezone)
}

func (c *Client) Files(ctx context.Context, q query.Query) ([]domain.File, string, error) {
	var result []domain.File
	var cursor string
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		zone, err := c.readZone()
		if err != nil {
			return err
		}
		result, cursor, err = store.ListFiles(tx.Queryer(), q, zone)
		return err
	})
	return result, cursor, err
}

func (c *Client) Filesets(ctx context.Context) ([]domain.Fileset, error) {
	var result []domain.Fileset
	err := c.readView(ctx, func(tx *store.ViewTx) error { var err error; result, err = store.Filesets(tx.Queryer()); return err })
	return result, err
}

func (c *Client) Browse(ctx context.Context, q query.Query, include bool) ([]BrowseEntry, string, error) {
	var result []BrowseEntry
	var cursor string
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		zone, err := c.readZone()
		if err != nil {
			return err
		}
		result, cursor, err = store.ListEntries(tx.Queryer(), q, include, zone)
		return err
	})
	return result, cursor, err
}

func (c *Client) TagTreeCounts(ctx context.Context) (*TagTree, map[domain.TagID]TagCounts, error) {
	var tree *TagTree
	var counts map[domain.TagID]TagCounts
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		var err error
		tree, err = LoadTagTree(tx.Queryer())
		if err != nil {
			return err
		}
		counts, err = store.TagSubtreeCounts(tx.Queryer())
		return err
	})
	return tree, counts, err
}

func (c *Client) FilesetDetail(ctx context.Context, id domain.FolderID) (domain.Fileset, domain.Folder, domain.BoxID, error) {
	var set domain.Fileset
	var folder domain.Folder
	var box domain.BoxID
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		var err error
		set, folder, box, err = readFilesetDetail(tx, id)
		return err
	})
	return set, folder, box, err
}

func (c *Client) MediaInfo(ctx context.Context, id domain.FileID) (MediaInfo, error) {
	var result MediaInfo
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		var err error
		result, err = readMediaInfo(tx.Queryer(), id)
		return err
	})
	return result, err
}

func (c *Client) CapturedInfo(ctx context.Context, file domain.File) (CapturedInfo, error) {
	var result CapturedInfo
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		zone, err := c.readZone()
		if err != nil {
			return err
		}
		result, err = readCapturedInfo(tx.Queryer(), zone, file)
		return err
	})
	return result, err
}
