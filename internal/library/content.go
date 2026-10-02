package library

import (
	"context"
	"fmt"
	"os"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
)

// OpenContent resolves an archived file to its authoritative blob. The caller
// owns the returned handle. It never follows the human-readable projection.
func (l *Library) OpenContent(ctx context.Context, id domain.FileID) (domain.File, *os.File, error) {
	file, found, err := l.File(ctx, id)
	if err != nil {
		return file, nil, err
	}
	if !found {
		return file, nil, ErrNoSuchFile
	}
	key, err := blob.ParseKey(file.BlobKey)
	if err != nil {
		return file, nil, fmt.Errorf("invalid stored blob key: %w", ErrInvariantViolated)
	}
	path, _ := l.blobs.LocalPath(key)
	handle, err := os.Open(path)
	return file, handle, err
}
