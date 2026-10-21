package library

import (
	"context"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/store"
	"path/filepath"
)

// UploadTarget binds a prepared upload to the current destination, independently
// of whether the laptop has cached existing files in that directory.
func (c *Client) UploadTarget(ctx context.Context, box domain.BoxID, folder domain.FolderID) (string, error) {
	return uploadTarget(func(fn func(*store.ViewTx) error) error { return c.readView(ctx, fn) }, box, folder)
}

// PreviewUpload uses the same collision policies as the NAS against local edits.
func (c *Client) PreviewUpload(ctx context.Context, box domain.BoxID, into domain.FolderID, paths []string) (UploadConflicts, error) {
	return previewUpload(func(fn func(*store.ViewTx) error) error { return c.readView(ctx, fn) }, box, into, paths)
}

// UploadStagingPath is the discardable browser upload area, separate from protected originals.
func (c *Client) UploadStagingPath() string { return filepath.Join(c.root, "uploads") }
