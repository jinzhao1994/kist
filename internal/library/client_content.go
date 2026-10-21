package library

import (
	"context"
	"io"

	"github.com/jinzhao1994/kist/internal/domain"
)

// OpenContent resolves the current catalog entry and retains the original's read
// lease until the caller closes it. It never contacts the NAS or its projection.
func (c *Client) OpenContent(ctx context.Context, id domain.FileID) (domain.File, io.ReadSeekCloser, error) {
	file, found, err := c.File(ctx, id)
	if err != nil {
		return file, nil, err
	}
	if !found {
		return file, nil, ErrNoSuchFile
	}
	handle, err := c.OpenOriginal(ctx, file.BlobKey)
	return file, handle, err
}
