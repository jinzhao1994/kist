package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/derive"
)

// SyncDerived describes one replaceable output, bound to an immutable original,
// recipe, generation, and output digest. Non-ready jobs have no content key.
type SyncDerived struct {
	Job        MediaJob `json:"job"`
	ContentKey string   `json:"content_key,omitempty"`
}

func (m *Media) resolveSyncOriginal(ctx context.Context, name, recipe string) (MediaJob, error) {
	job := MediaJob{BlobKey: name, Recipe: recipe, State: "missing"}
	key, err := blob.ParseKey(name)
	if err != nil {
		return job, ErrInvalidRequest
	}
	if _, ok := derive.Lookup(recipe); !ok {
		return job, ErrInvalidRequest
	}
	if _, err := m.library.blobs.Stat(ctx, key); err != nil {
		return job, err
	}
	return job, nil
}

// DescribeSyncDerived also queues absent outputs. It does not require a live
// file reference, so retained historical originals remain previewable.
func (m *Media) DescribeSyncDerived(ctx context.Context, key, recipe string, retry bool) (SyncDerived, error) {
	var result SyncDerived
	job, err := m.resolveSyncOriginal(ctx, key, recipe)
	if err != nil {
		return result, err
	}
	job, err = m.requestJob(ctx, job, retry)
	result.Job = job
	if err != nil || job.State != "ready" {
		return result, err
	}
	job, file, err := m.openJob(ctx, job)
	if err != nil {
		return result, err
	}
	defer file.Close()
	result.Job = job
	result.ContentKey, err = derivedDigest(ctx, file, job.Result.Bytes)
	return result, err
}

// OpenSyncDerived rejects stale descriptors rather than mixing generations
// during a resumed transfer. The returned handle stays on the opened inode.
func (m *Media) OpenSyncDerived(ctx context.Context, key, recipe, generation, contentKey string) (MediaJob, *MediaContent, error) {
	job, err := m.resolveSyncOriginal(ctx, key, recipe)
	if err != nil {
		return job, nil, err
	}
	if _, err := blob.ParseKey(contentKey); err != nil || generation == "" {
		return job, nil, ErrInvalidRequest
	}
	job, file, err := m.openJob(ctx, job)
	if err != nil {
		return job, nil, err
	}
	if job.Generation != generation {
		file.Close()
		return job, nil, ErrSyncConflict
	}
	actual, err := derivedDigest(ctx, file, job.Result.Bytes)
	if err == nil && actual != contentKey {
		err = ErrSyncConflict
	}
	if err != nil {
		file.Close()
		return job, nil, err
	}
	return job, file, nil
}

func derivedDigest(ctx context.Context, file io.ReadSeeker, size int64) (string, error) {
	if size <= 0 || size > derive.MaxOutputBytes {
		return "", ErrInvalidRequest
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(cacheContextReader{ctx: ctx, reader: file}, size+1))
	if err != nil {
		return "", err
	}
	if n != size {
		return "", blob.ErrIntegrityMismatch
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

// ValidateSyncDerived checks remote/cache descriptors before they reserve disk
// space or select output bytes. Recipe and source identity are caller claims.
func ValidateSyncDerived(value SyncDerived, original, recipe string) error {
	if _, err := blob.ParseKey(original); err != nil {
		return ErrInvalidRequest
	}
	if _, ok := derive.Lookup(recipe); !ok {
		return ErrInvalidRequest
	}
	job := value.Job
	if job.BlobKey != original || job.Recipe != recipe || job.Generation == "" || job.UpdatedAt.IsZero() {
		return ErrSyncConflict
	}
	switch job.State {
	case "ready":
		if _, err := blob.ParseKey(value.ContentKey); err != nil || job.Result.Bytes <= 0 || job.Result.Bytes > derive.MaxOutputBytes {
			return ErrSyncConflict
		}
	case "queued", "running", "blocked", "failed", "cancelled":
		if value.ContentKey != "" {
			return ErrSyncConflict
		}
	default:
		return ErrSyncConflict
	}
	return nil
}
