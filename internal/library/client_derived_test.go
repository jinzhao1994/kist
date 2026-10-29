package library

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/derive"
)

type derivedTestSource struct {
	*testSyncPeer
	media       *Media
	offline     bool
	calls       int
	failOnce    bool
	offsets     []int64
	unsupported bool
}

func (s *derivedTestSource) Capabilities(ctx context.Context) (SyncCapabilities, error) {
	s.calls++
	if s.offline {
		return SyncCapabilities{}, io.ErrClosedPipe
	}
	caps, err := s.testSyncPeer.Capabilities(ctx)
	caps.DerivedPreviews = !s.unsupported
	return caps, err
}
func (s *derivedTestSource) DescribeDerived(ctx context.Context, key, recipe string, retry bool) (SyncDerived, error) {
	descriptor, err := s.media.DescribeSyncDerived(ctx, key, recipe, retry)
	if err != nil {
		return descriptor, err
	}
	if descriptor.Job.State == "queued" {
		if err := s.media.work(ctx); err != nil {
			return descriptor, err
		}
		return s.media.DescribeSyncDerived(ctx, key, recipe, false)
	}
	return descriptor, err
}
func (s *derivedTestSource) OpenDerived(ctx context.Context, d SyncDerived, offset int64) (io.ReadCloser, error) {
	s.offsets = append(s.offsets, offset)
	_, file, err := s.media.OpenSyncDerived(ctx, d.Job.BlobKey, d.Job.Recipe, d.Job.Generation, d.ContentKey)
	if err != nil {
		return nil, err
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		file.Close()
		return nil, err
	}
	if s.failOnce {
		s.failOnce = false
		return struct {
			io.Reader
			io.Closer
		}{io.LimitReader(file, 20), file}, nil
	}
	return file, nil
}

func TestClientDerivedUsesSharedBudgetPinsAndOfflineReads(t *testing.T) {
	nas, id, _ := mediaFixture(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	original, _, err := nas.File(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	source := &derivedTestSource{testSyncPeer: &testSyncPeer{nas: nas}, media: nas.Media()}
	if err := c.SetOfflineOriginals(ctx, "preview", []string{original.BlobKey}); err != nil {
		t.Fatal(err)
	}
	descriptor, err := c.CacheDerived(ctx, original.BlobKey, derive.Thumbnail, false, source)
	if err != nil || descriptor.Job.State != "ready" {
		t.Fatal(descriptor, err)
	}
	if _, err := c.OpenOriginal(ctx, original.BlobKey); !errors.Is(err, ErrNotCached) {
		t.Fatal("downloaded original unnecessarily", err)
	}
	status, err := c.CacheStatus(ctx)
	if err != nil || status.CachedBytes != descriptor.Job.Result.Bytes || status.PinnedBytes != descriptor.Job.Result.Bytes {
		t.Fatal(status, descriptor, err)
	}
	source.offline = true
	calls := source.calls
	if _, err := c.CacheDerived(ctx, original.BlobKey, derive.Thumbnail, false, source); err != nil || source.calls != calls {
		t.Fatal("cached preview contacted NAS", err)
	}
	if _, err := c.SetCacheLimits(ctx, time.Hour, 1); err != nil {
		t.Fatal(err)
	}
	_, reader, err := c.OpenDerived(ctx, original.BlobKey, derive.Thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetOfflineOriginals(ctx, "preview", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PruneCache(ctx); err != nil {
		t.Fatal(err)
	}
	if data, err := io.ReadAll(reader); err != nil || int64(len(data)) != descriptor.Job.Result.Bytes {
		t.Fatal(len(data), err)
	}
	reader.Close()
	if _, err := c.PruneCache(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.OpenDerived(ctx, original.BlobKey, derive.Thumbnail); !errors.Is(err, ErrNotCached) {
		t.Fatal("evicted preview misreported", err)
	}
}

func TestClientDerivedCacheRebuildAndSupersededOutput(t *testing.T) {
	nas, id, _ := mediaFixture(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	original, _, _ := nas.File(ctx, id)
	source := &derivedTestSource{testSyncPeer: &testSyncPeer{nas: nas}, media: nas.Media()}
	first, err := c.CacheDerived(ctx, original.BlobKey, derive.Thumbnail, false, source)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a new renderer generation producing different valid cache bytes.
	_, output, _ := source.media.paths(first.Job)
	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
	source.media.render = func(_ context.Context, _, output, _ string) (derive.Result, error) {
		data := []byte("different derived generation")
		err := os.WriteFile(output, data, 0600)
		return derive.Result{Bytes: int64(len(data)), Width: 24, Height: 12}, err
	}
	second, err := c.CacheDerived(ctx, original.BlobKey, derive.Thumbnail, true, source)
	if err != nil || first.ContentKey == second.ContentKey {
		t.Fatal(second, err)
	}
	for _, key := range []string{first.ContentKey, second.ContentKey} {
		if err := c.deleteCacheObject(key); err != nil {
			t.Fatal(err)
		}
	}
	rebuilt, err := c.RebuildCache(ctx)
	if err != nil || rebuilt.Verified != 2 {
		t.Fatal(rebuilt, err)
	}
	_, reader, err := c.OpenDerived(ctx, original.BlobKey, derive.Thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || string(data) != "different derived generation" {
		t.Fatal(string(data), err)
	}
}

func TestClientDerivedResumesVerifiedDescriptorAndRejectsOldProtocol(t *testing.T) {
	nas, id, _ := mediaFixture(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	original, _, _ := nas.File(ctx, id)
	source := &derivedTestSource{testSyncPeer: &testSyncPeer{nas: nas}, media: nas.Media(), unsupported: true}
	if _, err := c.CacheDerived(ctx, original.BlobKey, derive.Thumbnail, false, source); !errors.Is(err, ErrSyncVersion) {
		t.Fatal("old protocol", err)
	}
	source.unsupported = false
	source.failOnce = true
	first, err := c.CacheDerived(ctx, original.BlobKey, derive.Thumbnail, false, source)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal(err)
	}
	status, err := c.CacheStatus(ctx)
	if err != nil || status.PartialBytes != 20 || status.ReservedBytes != first.Job.Result.Bytes-20 {
		t.Fatal(status, err)
	}
	if _, _, err := c.OpenDerived(ctx, original.BlobKey, derive.Thumbnail); !errors.Is(err, ErrNotCached) {
		t.Fatal("partial preview visible", err)
	}
	if _, err := c.CacheDerived(ctx, original.BlobKey, derive.Thumbnail, false, source); err != nil {
		t.Fatal(err)
	}
	if len(source.offsets) != 2 || source.offsets[0] != 0 || source.offsets[1] != 20 {
		t.Fatal(source.offsets)
	}
}

func TestClientDerivedRebuildWithoutMapping(t *testing.T) {
	for _, mode := range []string{"mapping", "mapping-and-index", "damaged-bytes", "damaged-bytes-oversized", "damaged-mapping"} {
		t.Run(mode, func(t *testing.T) {
			nas, id, _ := mediaFixture(t)
			c := newSyncClient(t, nas)
			ctx := t.Context()
			damaged := mode == "damaged-bytes" || mode == "damaged-bytes-oversized"
			damagedData := []byte("partial preview")
			original, _, _ := nas.File(ctx, id)
			source := &derivedTestSource{testSyncPeer: &testSyncPeer{nas: nas}, media: nas.Media()}
			output, err := c.CacheDerived(ctx, original.BlobKey, derive.Thumbnail, false, source)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "damaged-bytes-oversized" {
				damagedData = make([]byte, output.Job.Result.Bytes+100)
			}
			mapping, err := c.derivedPath(original.BlobKey, derive.Thumbnail)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "damaged-mapping" {
				if err = damageDerivedRecord(c, mapping); err != nil {
					t.Fatal(err)
				}
			} else if err = c.deleteDerivedRecord(mapping); err != nil {
				t.Fatal(err)
			}
			if mode != "mapping" {
				if err = c.deleteCacheObject(output.ContentKey); err != nil {
					t.Fatal(err)
				}
			}
			if damaged {
				if err = os.WriteFile(c.cachePath(output.ContentKey, ".data"), damagedData, 0600); err != nil {
					t.Fatal(err)
				}
			}
			result, err := c.RebuildCache(ctx)
			if err != nil {
				t.Fatal("lost mapping must not block accounting recovery", err)
			}
			if damaged {
				if result.Corrupt != 1 {
					t.Fatal(result)
				}
			} else if result.Verified != 1 {
				t.Fatal(result)
			}
			status, err := c.CacheStatus(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if damaged {
				if status.PartialBytes != int64(len(damagedData)) {
					t.Fatal(status)
				}
			} else if status.CachedBytes != output.Job.Result.Bytes {
				t.Fatal(status)
			}
			if _, _, err = c.OpenDerived(ctx, original.BlobKey, derive.Thumbnail); !errors.Is(err, ErrNotCached) {
				t.Fatal("invented a missing recipe mapping", err)
			}
			if mode == "damaged-bytes-oversized" {
				if err = c.SetOfflineOriginals(ctx, "repair", []string{output.ContentKey}); err != nil {
					t.Fatal(err)
				}
				status, err = c.SetCacheLimits(ctx, time.Hour, output.Job.Result.Bytes)
				if err != nil || !status.OverLimit {
					t.Fatal(status, err)
				}
			}
			{
				downloads := len(source.offsets)
				restored, err := c.CacheDerived(ctx, original.BlobKey, derive.Thumbnail, false, source)
				if err != nil || restored.ContentKey != output.ContentKey || (!damaged && len(source.offsets) != downloads) {
					t.Fatal(restored, err)
				}
				_, reader, err := c.OpenDerived(ctx, original.BlobKey, derive.Thumbnail)
				if err != nil {
					t.Fatal(err)
				}
				result, err = c.RebuildCache(ctx)
				reader.Close()
				if err != nil || result.Skipped != 1 {
					t.Fatal(result, err)
				}
			}
		})
	}
}

func TestClientDerivedRebuildPreservesFutureVersions(t *testing.T) {
	for _, kind := range []string{"mapping", "index"} {
		t.Run(kind, func(t *testing.T) {
			nas, id, _ := mediaFixture(t)
			c := newSyncClient(t, nas)
			ctx := t.Context()
			original, _, _ := nas.File(ctx, id)
			source := &derivedTestSource{testSyncPeer: &testSyncPeer{nas: nas}, media: nas.Media()}
			output, err := c.CacheDerived(ctx, original.BlobKey, derive.Thumbnail, false, source)
			if err != nil {
				t.Fatal(err)
			}
			mapping, err := c.derivedPath(original.BlobKey, derive.Thumbnail)
			if err != nil {
				t.Fatal(err)
			}
			path := mapping
			if kind == "mapping" {
				err = c.writeDerivedRecord(cachedDerived{Version: 2, Descriptor: output})
			} else {
				if err = c.deleteDerivedRecord(mapping); err != nil {
					t.Fatal(err)
				}
				path = c.cachePath(output.ContentKey, ".json")
				var record cachedOriginal
				if err = func() error { var e error; record, e = c.readCacheObject(output.ContentKey); return e }(); err != nil {
					t.Fatal(err)
				}
				record.Version = 2
				err = c.writeCacheObject(record)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := testCacheRecordBytes(c, path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = c.RebuildCache(ctx); !errors.Is(err, ErrSyncVersion) {
				t.Fatal(err)
			}
			after, err := testCacheRecordBytes(c, path)
			if err != nil || string(after) != string(before) {
				t.Fatal("future metadata overwritten", err)
			}
			info, err := os.Stat(c.cachePath(output.ContentKey, ".data"))
			if err != nil || info.Size() != output.Job.Result.Bytes {
				t.Fatal("future content removed", err)
			}
		})
	}
}
