package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

// VideoPlayback generates only requested HLS segments. File leases protect
// readers and encoders from eviction across processes, without archive locks.
type VideoPlayback struct {
	library *Library
	probe   func(context.Context, string) (float64, error)
	render  func(context.Context, string, string, float64, int) error
}

func (l *Library) VideoPlayback() *VideoPlayback {
	r := derive.Renderer{}
	return &VideoPlayback{library: l, probe: r.VideoDuration, render: r.RenderHLSSegment}
}

type VideoSource struct {
	BlobKey  string
	Duration float64
}

var ErrVideoChanged = errors.New("video content changed; reload the player")

func (s VideoSource) Version() string { return derive.HLSRecipe + ":" + s.BlobKey }
func (s VideoSource) Segments() int   { return int(math.Ceil(s.Duration / derive.HLSSegmentSeconds)) }
func (s VideoSource) SegmentDuration(index int) float64 {
	return math.Min(derive.HLSSegmentSeconds, s.Duration-float64(index*derive.HLSSegmentSeconds))
}

// VideoSegment retains its OS lease until the HTTP response has finished.
type VideoSegment struct {
	*os.File
	release func()
	ETag    string
}

func (s *VideoSegment) Close() error { err := s.File.Close(); s.release(); return err }

func (v *VideoPlayback) root() string { return filepath.Join(v.library.layout.Root, "transcodes") }
func (v *VideoPlayback) cacheLock() *lockfile.Locker {
	return lockfile.New(filepath.Join(v.library.layout.Root, "video-cache.lock"))
}
func (v *VideoPlayback) encoderLock() *lockfile.Locker {
	return lockfile.New(filepath.Join(v.library.layout.Root, "video-transcode.lock"))
}
func (v *VideoPlayback) paths(key, recipe string, index int) (string, string, error) {
	b, err := blob.ParseKey(key)
	if err != nil || recipe != derive.HLSRecipe || index < 0 || index >= 3600 {
		return "", "", ErrInvalidRequest
	}
	dir := filepath.Join(v.root(), filepath.FromSlash(b.RelPath()), recipe)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", "", err
	}
	base := filepath.Join(dir, strconv.Itoa(index))
	return base + ".ts", base + ".lock", nil
}

// Encoder admission is separate from the media and extraction workers. Waiting
// is bounded by the request deadline and remains cancellable across processes.
func (v *VideoPlayback) encoder(ctx context.Context) (func(), error) {
	l := v.encoderLock()
	for {
		release, ok, err := l.TryAcquire()
		if err != nil {
			return nil, err
		}
		if ok {
			return release, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (v *VideoPlayback) Source(ctx context.Context, id domain.FileID) (VideoSource, error) {
	f, found, err := v.library.File(ctx, id)
	if err != nil {
		return VideoSource{}, err
	}
	if !found {
		return VideoSource{}, ErrNoSuchFile
	}
	return v.source(ctx, f.BlobKey)
}
func (v *VideoPlayback) source(ctx context.Context, key string) (VideoSource, error) {
	s := VideoSource{BlobKey: key}
	b, err := blob.ParseKey(key)
	if err != nil {
		return s, err
	}
	db, err := v.library.runtimeStore()
	if err != nil {
		return s, err
	}
	read := func() error {
		return db.DB.QueryRowContext(ctx, `SELECT duration FROM video_sources WHERE blob_key=? AND recipe=?`, key, derive.HLSRecipe).Scan(&s.Duration)
	}
	if err = read(); err == nil {
		return s, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return s, err
	}
	unlock, err := v.encoder(ctx)
	if err != nil {
		return s, err
	}
	defer unlock()
	if err = read(); err == nil {
		return s, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return s, err
	}
	input, _ := v.library.blobs.LocalPath(b)
	s.Duration, err = v.probe(ctx, input)
	if err != nil {
		return s, err
	}
	if math.IsNaN(s.Duration) || math.IsInf(s.Duration, 0) || s.Duration <= 0 || s.Duration > 21600 {
		return s, ErrInvalidRequest
	}
	_, err = db.DB.ExecContext(ctx, `INSERT INTO video_sources VALUES(?,?,?)`, key, derive.HLSRecipe, s.Duration)
	return s, err
}

func (v *VideoPlayback) cached(ctx context.Context, s VideoSource, index int) (*VideoSegment, error) {
	path, leasePath, err := v.paths(s.BlobKey, derive.HLSRecipe, index)
	if err != nil {
		return nil, err
	}
	unlock, err := v.cacheLock().Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	release, ok, err := lockfile.NewShared(leasePath).TryAcquire()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, os.ErrNotExist
	}
	fail := func(err error) (*VideoSegment, error) { release(); return nil, err }
	db, err := v.library.runtimeStore()
	if err != nil {
		return fail(err)
	}
	var size int64
	var digest string
	err = db.DB.QueryRowContext(ctx, `SELECT bytes,digest FROM video_segments WHERE blob_key=? AND recipe=? AND segment=?`, s.BlobKey, derive.HLSRecipe, index).Scan(&size, &digest)
	if errors.Is(err, sql.ErrNoRows) {
		return fail(os.ErrNotExist)
	}
	if err != nil {
		return fail(err)
	}
	f, err := os.Open(path)
	if err != nil {
		return fail(err)
	}
	st, err := f.Stat()
	if err == nil && (st.Size() != size || size <= 0 || size > derive.MaxHLSSegmentBytes) {
		err = os.ErrNotExist
	}
	if err != nil {
		f.Close()
		return fail(err)
	}
	_, err = db.DB.ExecContext(ctx, `UPDATE video_segments SET accessed_at=? WHERE blob_key=? AND recipe=? AND segment=?`, time.Now().UTC().Format(time.RFC3339Nano), s.BlobKey, derive.HLSRecipe, index)
	if err != nil {
		f.Close()
		return fail(err)
	}
	return &VideoSegment{f, release, digest}, nil
}

// OpenSegment resolves the current file on every request. A version mismatch
// never serves replacement content under an immutable old URL.
func (v *VideoPlayback) OpenSegment(ctx context.Context, id domain.FileID, version string, index int) (*VideoSegment, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	s, err := v.Source(ctx, id)
	if err != nil {
		return nil, err
	}
	if version != s.Version() {
		return nil, ErrVideoChanged
	}
	if index < 0 || index >= s.Segments() {
		return nil, ErrInvalidRequest
	}
	if f, err := v.cached(ctx, s, index); !errors.Is(err, os.ErrNotExist) {
		return f, err
	}
	unlock, err := v.encoder(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if f, err := v.cached(ctx, s, index); !errors.Is(err, os.ErrNotExist) {
		return f, err
	}
	path, leasePath, err := v.paths(s.BlobKey, derive.HLSRecipe, index)
	if err != nil {
		return nil, err
	}
	release, err := lockfile.New(leasePath).Acquire(ctx)
	if err != nil {
		return nil, err
	}
	leased := true
	defer func() {
		if leased {
			release()
		}
	}()
	// A failed publication may have left an index reservation or invalid output.
	// This segment is exclusively leased and was checked twice as unavailable.
	cacheUnlock, err := v.cacheLock().Acquire(ctx)
	if err != nil {
		return nil, err
	}
	db, err := v.library.runtimeStore()
	if err == nil {
		_, err = db.DB.ExecContext(ctx, `DELETE FROM video_segments WHERE blob_key=? AND recipe=? AND segment=?`, s.BlobKey, derive.HLSRecipe, index)
	}
	if err == nil {
		err = os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
	}
	cacheUnlock()
	if err != nil {
		return nil, err
	}
	// Only the encoder owner can have a live temporary output. A successor may
	// remove leftovers from a killed process without guessing from a timestamp.
	tempRoot, err := v.cleanEncoderTemps()
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(tempRoot, "segment-*.ts")
	if err != nil {
		return nil, err
	}
	temp := tmp.Name()
	tmp.Close()
	defer os.Remove(temp)
	b, _ := blob.ParseKey(s.BlobKey)
	input, _ := v.library.blobs.LocalPath(b)
	if err = v.render(ctx, input, temp, s.Duration, index); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(temp, os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err == nil {
		err = f.Sync()
	}
	hash := sha256.New()
	if err == nil {
		_, err = io.Copy(hash, f)
	}
	digest := fmt.Sprintf("%x", hash.Sum(nil))
	f.Close()
	if err != nil {
		return nil, err
	}
	if st.Size() <= 0 || st.Size() > derive.MaxHLSSegmentBytes {
		return nil, ErrCacheQuota
	}
	cacheUnlock, err = v.cacheLock().Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer cacheUnlock()
	if err = v.pruneLocked(ctx, st.Size(), false); err != nil {
		return nil, err
	}
	db, err = v.library.runtimeStore()
	if err != nil {
		return nil, err
	}
	// Reserve before rename: after a crash the index may reference missing
	// bytes, which are treated as a miss. No unindexed completed file is leaked.
	_, err = db.DB.ExecContext(ctx, `INSERT INTO video_segments VALUES(?,?,?,?,?,?) ON CONFLICT(blob_key,recipe,segment) DO UPDATE SET bytes=excluded.bytes,digest=excluded.digest,accessed_at=excluded.accessed_at`, s.BlobKey, derive.HLSRecipe, index, st.Size(), digest, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	// A stale file can exist after a failed prior publication. It is disposable
	// and exclusively leased, including on Windows where rename cannot replace it.
	if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err = os.Rename(temp, path); err != nil {
		return nil, err
	}
	f, err = os.Open(path)
	if err != nil {
		return nil, err
	}
	// Downgrade while the cache lock and encoder admission still exclude all
	// removers/writers. Concurrent readers may now share the completed segment.
	leased = false
	release()
	readerRelease, ok, err := lockfile.NewShared(leasePath).TryAcquire()
	if err != nil || !ok {
		f.Close()
		if err == nil {
			err = ErrMediaNotReady
		}
		return nil, err
	}
	return &VideoSegment{f, readerRelease, digest}, nil
}

// pruneLocked is called under the short cache lock. Leased objects are skipped;
// quota admission fails instead of evicting an active response or overfilling.
func (v *VideoPlayback) pruneLocked(ctx context.Context, incoming int64, clear bool) error {
	cfg, err := config.Load(v.library.layout.ConfigPath())
	if err != nil {
		return err
	}
	age, err := config.ParseCacheMaxAge(cfg.Video.CacheMaxAge)
	if err != nil {
		return err
	}
	db, err := v.library.runtimeStore()
	if err != nil {
		return err
	}
	rows, err := db.DB.QueryContext(ctx, `SELECT blob_key,recipe,segment,bytes,accessed_at FROM video_segments ORDER BY accessed_at,blob_key,recipe,segment`)
	if err != nil {
		return err
	}
	type record struct {
		key, recipe, accessed string
		index                 int
		size                  int64
	}
	var records []record
	var total int64
	for rows.Next() {
		var r record
		if err = rows.Scan(&r.key, &r.recipe, &r.index, &r.size, &r.accessed); err != nil {
			break
		}
		records = append(records, r)
		total += r.size
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	for _, r := range records {
		accessed, err := time.Parse(time.RFC3339Nano, r.accessed)
		if err != nil {
			return err
		}
		if !clear && time.Since(accessed) < age && total+incoming <= cfg.Video.CacheMaxBytes {
			continue
		}
		path, leasePath, err := v.paths(r.key, r.recipe, r.index)
		if err != nil {
			return err
		}
		release, ok, err := lockfile.New(leasePath).TryAcquire()
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		err = os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
		if err == nil {
			_, err = db.DB.ExecContext(ctx, `DELETE FROM video_segments WHERE blob_key=? AND recipe=? AND segment=?`, r.key, r.recipe, r.index)
		}
		release()
		if err != nil {
			return err
		}
		total -= r.size
	}
	if incoming > 0 && total+incoming > cfg.Video.CacheMaxBytes {
		return ErrCacheQuota
	}
	return nil
}

// Clear removes only unleased HLS segments. Originals, ordinary derived files
// and runtime history are never part of this operation.
func (v *VideoPlayback) Clear(ctx context.Context) error {
	if err := v.cleanIdleTemps(); err != nil {
		return err
	}
	unlock, err := v.cacheLock().Acquire(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return v.pruneLocked(ctx, 0, true)
}

func (v *VideoPlayback) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if err := v.cleanIdleTemps(); err != nil && ctx.Err() == nil {
			slog.Error("video temporary cache maintenance", "error", err)
		}
		unlock, err := v.cacheLock().Acquire(ctx)
		if err == nil {
			err = v.pruneLocked(ctx, 0, false)
			unlock()
		}
		if err != nil && ctx.Err() == nil {
			slog.Error("video cache maintenance", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
		}
	}
}

// Only an encoder owner may remove these files. No PID or age heuristic is
// used, so clearing a cache never removes another process's active output.
func (v *VideoPlayback) cleanEncoderTemps() (string, error) {
	root := filepath.Join(v.root(), "temp")
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "segment-") {
			if err = os.Remove(filepath.Join(root, entry.Name())); err != nil {
				return "", err
			}
		}
	}
	return root, nil
}
func (v *VideoPlayback) cleanIdleTemps() error {
	release, ok, err := v.encoderLock().TryAcquire()
	if err != nil || !ok {
		return err
	}
	defer release()
	_, err = v.cleanEncoderTemps()
	return err
}

// Playlist declares a finite timeline independently of cache availability.
// Segment URLs are generated by the API from a trusted file ID and version.
func (s VideoSource) Playlist(segmentURL func(int) string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-INDEPENDENT-SEGMENTS\n", derive.HLSSegmentSeconds)
	for i := 0; i < s.Segments(); i++ {
		fmt.Fprintf(&b, "#EXTINF:%.6f,\n%s\n", s.SegmentDuration(i), segmentURL(i))
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}
