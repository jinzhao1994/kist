package library

import (
	"context"
	"sync"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

type clientViewContextKey struct{}
type clientViewScope struct {
	mu     sync.Mutex
	active bool
	client *Client
	tx     *store.ViewTx
}

// ClientView exposes only catalog reads from a single local editing snapshot.
// It is valid only within the View callback. It never opens original content.
type ClientView struct {
	client *Client
	scope  *clientViewScope
	zone   *time.Location
}

// View refreshes the overlay once and keeps all callback reads on one snapshot.
// The callback must not mutate this client, including through another handle.
func (c *Client) View(ctx context.Context, fn func(*ClientView) error) error {
	if fn == nil {
		return ErrInvalidRequest
	}
	return c.readView(ctx, func(tx *store.ViewTx) error {
		zone, err := c.readZone()
		if err != nil {
			return err
		}
		scope := &clientViewScope{active: true, client: c, tx: tx}
		defer func() { scope.mu.Lock(); scope.active = false; scope.mu.Unlock() }()
		if err := fn(&ClientView{client: c, scope: scope, zone: zone}); err != nil {
			return err
		}
		return ctx.Err()
	})
}

func (v *ClientView) context(ctx context.Context) context.Context {
	return context.WithValue(ctx, clientViewContextKey{}, v.scope)
}

// Zone is the timezone captured when this view was opened.
func (v *ClientView) Zone() *time.Location { return v.zone }

func (v *ClientView) Box(ctx context.Context, id domain.BoxID) (domain.Box, bool, error) {
	return v.client.Box(v.context(ctx), id)
}

func (v *ClientView) Boxes(ctx context.Context, opt ListBoxesOptions) ([]domain.Box, error) {
	return v.client.Boxes(v.context(ctx), opt)
}

func (v *ClientView) BoxTags(ctx context.Context, id domain.BoxID) ([]domain.TagID, error) {
	return v.client.BoxTags(v.context(ctx), id)
}

func (v *ClientView) BoxCover(ctx context.Context, id domain.BoxID) (domain.File, bool, error) {
	return v.client.BoxCover(v.context(ctx), id)
}

func (v *ClientView) File(ctx context.Context, id domain.FileID) (domain.File, bool, error) {
	return v.client.File(v.context(ctx), id)
}

func (v *ClientView) Files(ctx context.Context, q query.Query) ([]domain.File, string, error) {
	return v.client.Files(v.context(ctx), q)
}

func (v *ClientView) FilesWithTags(ctx context.Context, q query.Query) ([]FileSearchItem, string, error) {
	return v.client.FilesWithTags(v.context(ctx), q)
}

func (v *ClientView) SearchBoxes(ctx context.Context, q query.BoxQuery) ([]BoxSearchItem, string, error) {
	return v.client.SearchBoxes(v.context(ctx), q)
}

func (v *ClientView) Folders(ctx context.Context, id domain.BoxID) ([]domain.Folder, error) {
	return v.client.Folders(v.context(ctx), id)
}

func (v *ClientView) FileTags(ctx context.Context, id domain.FileID) ([]domain.TagID, error) {
	return v.client.FileTags(v.context(ctx), id)
}

func (v *ClientView) FileMetadata(ctx context.Context, id domain.FileID) ([]extract.Row, error) {
	return v.client.FileMetadata(v.context(ctx), id)
}

func (v *ClientView) FileLocation(ctx context.Context, file domain.File) (domain.BoxID, string, error) {
	return v.client.FileLocation(v.context(ctx), file)
}

func (v *ClientView) MediaInfo(ctx context.Context, id domain.FileID) (MediaInfo, error) {
	return v.client.MediaInfo(v.context(ctx), id)
}

func (v *ClientView) CapturedInfo(ctx context.Context, file domain.File) (CapturedInfo, error) {
	return v.client.CapturedInfo(v.context(ctx), file)
}

func (v *ClientView) Filesets(ctx context.Context) ([]domain.Fileset, error) {
	return v.client.Filesets(v.context(ctx))
}

func (v *ClientView) FilesetDetail(ctx context.Context, id domain.FolderID) (domain.Fileset, domain.Folder, domain.BoxID, error) {
	return v.client.FilesetDetail(v.context(ctx), id)
}

func (v *ClientView) Browse(ctx context.Context, q query.Query, include bool) ([]BrowseEntry, string, error) {
	return v.client.Browse(v.context(ctx), q, include)
}

func (v *ClientView) SavedSearches(ctx context.Context) ([]domain.SavedSearch, error) {
	return v.client.SavedSearches(v.context(ctx))
}

func (v *ClientView) TagTreeCounts(ctx context.Context) (*TagTree, map[domain.TagID]TagCounts, error) {
	return v.client.TagTreeCounts(v.context(ctx))
}

func (v *ClientView) ResolveTag(ctx context.Context, ref string) (domain.Tag, error) {
	return v.client.ResolveTag(v.context(ctx), ref)
}

func (v *ClientView) TagCondition(ctx context.Context, refs []string) (query.TagExpr, error) {
	return v.client.TagCondition(v.context(ctx), refs)
}

func (v *ClientView) ExcludeTags(ctx context.Context, refs []string) (query.TagExpr, error) {
	return v.client.ExcludeTags(v.context(ctx), refs)
}
