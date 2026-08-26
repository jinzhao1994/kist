package blob

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// Dir stores blobs in a local directory: <root>/sha256/ab/cd/<hex>, with
// <root>/tmp as the staging area.
type Dir struct {
	root string
	log  *slog.Logger
}

var _ Store = (*Dir)(nil)

// NewDir prepares root as a blob directory, creating it if needed.
func NewDir(root string, log *slog.Logger) (*Dir, error) {
	if log == nil {
		log = slog.Default()
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve blob root: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(abs, "tmp"), 0o755); err != nil {
		return nil, fmt.Errorf("create blob layout: %w", err)
	}
	return &Dir{root: abs, log: log}, nil
}

// Root is the directory this store writes into.
func (d *Dir) Root() string { return d.root }

func (d *Dir) LocalPath(k Key) (string, bool) {
	return filepath.Join(d.root, k.RelPath()), true
}

// Put streams r into the staging area while hashing it, then decides where it
// belongs. The order — blob durable before the database ever hears about it —
// is what makes a crash leave an orphan rather than a dangling reference
// (spec 3.7).
func (d *Dir) Put(ctx context.Context, r io.Reader) (PutResult, error) {
	tmp, err := os.CreateTemp(filepath.Join(d.root, "tmp"), "put-*")
	if err != nil {
		return PutResult{}, fmt.Errorf("create temp blob: %w", err)
	}
	tmpPath := tmp.Name()
	keep := false
	defer func() {
		tmp.Close()
		if !keep {
			os.Remove(tmpPath)
		}
	}()

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, h), r)
	if err != nil {
		return PutResult{}, fmt.Errorf("stage blob: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return PutResult{}, fmt.Errorf("fsync staged blob: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return PutResult{}, fmt.Errorf("close staged blob: %w", err)
	}

	res, err := d.place(ctx, tmpPath, hex.EncodeToString(h.Sum(nil)), size)
	if err != nil {
		return PutResult{}, err
	}
	keep = res.Outcome == OutcomeStored || res.Outcome == OutcomeRepaired || res.Outcome == OutcomeCollided
	return res, nil
}

// putWithHash exists for tests that need to drive the collision branch, which
// cannot be reached by supplying honest content.
func (d *Dir) putWithHash(ctx context.Context, r io.Reader, digest string, size int64) (PutResult, error) {
	tmp, err := os.CreateTemp(filepath.Join(d.root, "tmp"), "put-*")
	if err != nil {
		return PutResult{}, err
	}
	tmpPath := tmp.Name()
	keep := false
	defer func() {
		tmp.Close()
		if !keep {
			os.Remove(tmpPath)
		}
	}()
	if _, err := io.Copy(tmp, r); err != nil {
		return PutResult{}, err
	}
	if err := tmp.Close(); err != nil {
		return PutResult{}, err
	}
	res, err := d.place(ctx, tmpPath, digest, size)
	if err != nil {
		return PutResult{}, err
	}
	keep = res.Outcome != OutcomeDeduped
	return res, nil
}

// place walks the collision suffixes until it finds a home for the staged file.
func (d *Dir) place(ctx context.Context, tmpPath, digest string, size int64) (PutResult, error) {
	for suffix := 0; ; suffix++ {
		if err := ctx.Err(); err != nil {
			return PutResult{}, err
		}

		key := NewKey(AlgoSHA256, digest, suffix)
		dst := filepath.Join(d.root, key.RelPath())

		_, err := os.Stat(dst)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if err := d.commit(tmpPath, dst); err != nil {
				return PutResult{}, err
			}
			outcome := OutcomeStored
			if suffix != 0 {
				outcome = OutcomeCollided
			}
			return PutResult{Info: Info{Key: key, Size: size}, Outcome: outcome}, nil
		case err != nil:
			return PutResult{}, fmt.Errorf("stat existing blob: %w", err)
		}

		identical, existingHex, err := compareAndHash(dst, tmpPath)
		if err != nil {
			return PutResult{}, err
		}

		switch {
		case existingHex != digest:
			// The blob on disk no longer hashes to its own name: bit rot, a
			// half-restored backup, an antivirus quarantine. The staged file
			// was just hashed and is trustworthy, so heal with it.
			d.log.Error("blob on disk did not match its own hash; replacing it with freshly verified content",
				"key", key, "expected", digest, "found", existingHex, "path", dst)
			if err := d.replace(tmpPath, dst); err != nil {
				return PutResult{}, err
			}
			return PutResult{Info: Info{Key: key, Size: size}, Outcome: OutcomeRepaired}, nil

		case identical:
			return PutResult{Info: Info{Key: key, Size: size}, Outcome: OutcomeDeduped}, nil

		default:
			// Same digest, different bytes. Keep both — git kept only one and
			// called it a data-loss bug for years (spec 3.3).
			d.log.Error("SHA-256 collision: different content hashes to an existing blob; storing under the next suffix",
				"digest", digest, "existing", key)
			continue
		}
	}
}

// commit makes the staged file permanent at dst.
func (d *Dir) commit(tmpPath, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create shard directory: %w", err)
	}
	// chmod happens here, not right after fsync, so that every path which
	// discards the temp file is dealing with a writable one — deleting a
	// read-only file needs extra work on Windows (spec 3.3).
	if err := os.Chmod(tmpPath, 0o444); err != nil {
		return fmt.Errorf("seal blob: %w", err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("commit blob: %w", err)
	}
	return syncDir(filepath.Dir(dst))
}

// replace overwrites a corrupt blob. The existing file is read-only, which
// blocks rename on Windows, so its mode is cleared first.
func (d *Dir) replace(tmpPath, dst string) error {
	if err := os.Chmod(dst, 0o644); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("unseal corrupt blob: %w", err)
	}
	return d.commit(tmpPath, dst)
}

func (d *Dir) Get(_ context.Context, k Key) (io.ReadSeekCloser, error) {
	p, _ := d.LocalPath(k)
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("open blob %s: %w", k, err)
	}
	return f, nil
}

func (d *Dir) Stat(_ context.Context, k Key) (Info, error) {
	p, _ := d.LocalPath(k)
	fi, err := os.Stat(p)
	if err != nil {
		return Info{}, fmt.Errorf("stat blob %s: %w", k, err)
	}
	return Info{Key: k, Size: fi.Size()}, nil
}

func (d *Dir) Delete(_ context.Context, k Key) error {
	p, _ := d.LocalPath(k)
	// Blobs are 0444. On Windows that alone blocks deletion.
	if err := os.Chmod(p, 0o644); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("unseal blob %s: %w", k, err)
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete blob %s: %w", k, err)
	}
	return nil
}
