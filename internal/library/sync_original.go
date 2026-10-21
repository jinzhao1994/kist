package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/jinzhao1994/kist/internal/blob"
)

// SyncOriginalStatus describes verified NAS content, independently of whether
// an archive operation has referenced it yet. Interrupted uploads may leave
// durable unreferenced originals which a retry can safely reuse.
type SyncOriginalStatus struct {
	Present bool  `json:"present"`
	Size    int64 `json:"size"`
}

func (l *Library) SyncOriginal(ctx context.Context, name string) (SyncOriginalStatus, error) {
	key, err := blob.ParseKey(name)
	if err != nil {
		return SyncOriginalStatus{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	digest, size, err := l.blobs.Rehash(ctx, key)
	if errors.Is(err, os.ErrNotExist) {
		return SyncOriginalStatus{}, nil
	}
	if err != nil {
		return SyncOriginalStatus{}, err
	}
	if digest != key.Hex() {
		return SyncOriginalStatus{}, blob.ErrIntegrityMismatch
	}
	return SyncOriginalStatus{Present: true, Size: size}, nil
}

// StoreSyncOriginal verifies the immutable source claim before placement. It
// does not create an archive event; AcceptSync must independently verify all
// required originals before committing the referencing operation.
func (l *Library) StoreSyncOriginal(ctx context.Context, name string, size int64, source io.Reader) error {
	key, err := blob.ParseKey(name)
	if err != nil || size < 0 || size == 1<<63-1 {
		return ErrInvalidRequest
	}
	result, err := l.blobs.PutVerified(ctx, io.LimitReader(source, size+1), blob.ExpectedContent{SHA256: key.Hex(), Size: size})
	if err != nil {
		return err
	}
	if result.Key != key {
		// Collision suffixes belong to the receiving store. Never bind an
		// existing NAS address to different bytes to satisfy a remote intent.
		return fmt.Errorf("%w: received original uses a different collision address", ErrSyncConflict)
	}
	return nil
}

// OpenSyncOriginal exposes a seekable immutable object for authenticated range
// downloads. The receiver verifies the complete digest before publishing it.
func (l *Library) OpenSyncOriginal(ctx context.Context, name string) (io.ReadSeekCloser, error) {
	key, err := blob.ParseKey(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	return l.blobs.Get(ctx, key)
}
