package blob

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// TmpGrace is how long a file in tmp/ is left alone before it counts as crash
// residue.
//
// It is the same 24 hours as gc's grace period, for the same reason: it has to
// be comfortably longer than the window between "the blob is written" and "the
// transaction commits" (spec 3.9). A temp file only lives for the copy of one
// file, so even a day-long import keeps the whole margin.
const TmpGrace = 24 * time.Hour

// Walk calls fn for every blob stored under this directory, with its size on
// disk.
//
// Anything whose path is not one a key would produce is ignored. That is the
// only rule, and it is what keeps a half-written staging file or somebody's
// .DS_Store from being reported as unknown content: this directory belongs to
// kist, but "belongs to" is not "is protected from".
//
// Descending into tmp/ is skipped on top of that, and the honest reason is not
// correctness — measured, removing the skip changes nothing, because tmp/<name>
// is two path segments and a key needs four. It is there so that a walk of a
// library with a large import running does not stat thousands of staging files
// that are being created and deleted underneath it.
func (d *Dir) Walk(fn func(k Key, size int64) error) error {
	root := d.root
	tmp := filepath.Join(root, "tmp")
	return filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if e.IsDir() {
			if p == tmp {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		k, ok := keyFromRelPath(rel)
		if !ok {
			d.log.Debug("ignoring a file under blobs/ that is not a blob", "path", rel)
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		return fn(k, info.Size())
	})
}

// keyFromRelPath is the inverse of Key.RelPath, and deliberately strict.
//
// It rebuilds the key from the file name and then checks that the shard
// directories are the ones that key would have produced. Doing it this way
// leaves exactly one piece of knowledge about the layout — Key.RelPath — so
// there is no way to end up writing two shard levels and reading three, which
// is the sort of divergence that takes a day to find.
func keyFromRelPath(rel string) (Key, bool) {
	segs := strings.Split(filepath.ToSlash(rel), "/")
	if len(segs) != 4 {
		return "", false
	}
	algo, s1, s2, name := segs[0], segs[1], segs[2], segs[3]

	digest, suffix := name, 0
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		n, err := strconv.Atoi(name[i+1:])
		if err != nil || n <= 0 {
			return "", false
		}
		digest, suffix = name[:i], n
	}
	if len(digest) < 4 || s1 != digest[0:2] || s2 != digest[2:4] {
		return "", false
	}
	k, err := ParseKey(string(NewKey(algo, digest, suffix)))
	if err != nil {
		return "", false
	}
	return k, true
}

// Rehash reads a blob end to end and reports what it actually hashes to.
//
// This is scrub's whole job (spec 3.8), and the only thing in verify's
// neighbourhood that reads content rather than stating it.
func (d *Dir) Rehash(ctx context.Context, k Key) (string, int64, error) {
	p, ok := d.LocalPath(k)
	if !ok {
		return "", 0, fmt.Errorf("blob %s has no local path", k)
	}
	return d.rehashPath(ctx, p)
}

// rehashPath is Rehash by path, for the caller that is about to overwrite a
// file and therefore cannot assume its name says anything true about it.
func (d *Dir) rehashPath(ctx context.Context, path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("read blob: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(h, readerContext{ctx: ctx, r: f})
	if err != nil {
		return "", 0, fmt.Errorf("read blob: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

// CleanTmp removes staging files last modified before cutoff, and reports how
// many went.
//
// This is not GC. v0.1 collects nothing (spec 3.9); what this removes is crash
// residue. The cutoff is what keeps it from eating a running import's staging
// file, which is a bug that only appears when two processes are involved and
// so would never show up in a test. Callers pass time.Now().Add(-TmpGrace).
func (d *Dir) CleanTmp(cutoff time.Time) (int, error) {
	dir := filepath.Join(d.root, "tmp")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("read %s: %w", dir, err)
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			// One stubborn file must not stop the rest. This is a cleanup job:
			// nothing downstream depends on it having succeeded.
			d.log.Warn("could not remove crash residue from blobs/tmp", "name", e.Name(), "err", err)
			continue
		}
		removed++
	}
	return removed, nil
}

// readerContext makes a long read cancellable, so a scrub answers Ctrl-C in
// the middle of a 4 GB video rather than at the end of it.
type readerContext struct {
	ctx context.Context
	r   io.Reader
}

func (rc readerContext) Read(p []byte) (int, error) {
	if err := rc.ctx.Err(); err != nil {
		return 0, err
	}
	return rc.r.Read(p)
}
