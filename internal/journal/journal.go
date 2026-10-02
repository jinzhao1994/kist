// Package journal is the file layer of ops/: the authoritative op log.
//
// Spec 2.1 names two authorities, ops/ and blobs/, and both are plain
// directories. This package owns one of them and blob owns the other, which is
// why neither may import anything else in the repository (spec 2.6): verify
// --deep checks the database against the log, and a log layer that could read
// the database would no longer be able to contradict it.
//
// It moves action and origin as strings and payload as raw bytes. It does not
// know what import_files means, and it must not learn: the conversion between
// a Line and an op lives in library, the same way the conversion between
// store.OpRow and oplog's types does.
//
// Layout is one file per UTC day, under a year/month path:
//
//	ops/2026/09/2026-09-07_ops.jsonl
//
// Names are zero-padded, so sorting them as strings sorts them as days, and
// Append refuses to move backwards (spec 3.1.1). Together those give the
// property everything else rests on: reading the buckets in name order reads
// the ops in id order.
package journal

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Version is the line format this build writes. It is stored on every line
// because these lines are never rewritten: a reader five years from now has
// only the line itself to go on (spec 3.1.1).
const Version = 2

// tmpDir holds the tail of a torn append after RecoverTail moves it aside.
// Nothing reads it; it exists so that a crash never destroys bytes.
const tmpDir = "tmp"

var (
	// ErrCorrupt reports a log that cannot be trusted: a line whose checksum
	// does not match, a gap in the ids, or a bucket out of order. It is
	// deliberately not returned for a torn final line, which is a different
	// thing — see RecoverTail.
	ErrCorrupt = errors.New("op log is corrupt")

	// ErrUnsupportedVersion reports a line written by a newer kist. The
	// library is not damaged and the caller did nothing wrong; this build
	// simply cannot read it.
	ErrUnsupportedVersion = errors.New("op log was written by a newer version of kist")
)

// Dir is an ops/ directory.
type Dir struct {
	root string
}

// NewDir prepares root as a journal directory, creating it if needed.
func NewDir(root string) (*Dir, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve op log root: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(abs, tmpDir), 0o755); err != nil {
		return nil, fmt.Errorf("create op log layout: %w", err)
	}
	return &Dir{root: abs}, nil
}

// OpenDir opens an existing journal without creating anything, for the checks
// that must not write (spec 3.8).
func OpenDir(root string) (*Dir, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve op log root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("open op log %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("open op log %s: not a directory", root)
	}
	return &Dir{root: abs}, nil
}

// Root is the directory this journal lives in.
func (d *Dir) Root() string { return d.root }

// bucketFor is the path a line with this timestamp belongs in, relative to the
// root and in slash form.
//
// It reads the timestamp as text rather than parsing it into a time.Time. The
// stored form is fixed-width UTC (spec 3.1.1), so the year, month and day are
// at known offsets, and taking them directly means a bucket name can never
// disagree with the ts written beside it.
func bucketFor(ts string) (string, error) {
	if len(ts) < len("2006-01-02") {
		return "", fmt.Errorf("%w: timestamp %q is too short to bucket", ErrCorrupt, ts)
	}
	day := ts[:10]
	return day[:4] + "/" + day[5:7] + "/" + day + "_ops.jsonl", nil
}

// buckets lists the log's files in name order, which is day order.
func (d *Dir) buckets() ([]string, error) {
	var out []string
	err := filepath.WalkDir(d.root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			if path != d.root && e.Name() == tmpDir && filepath.Dir(path) == d.root {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(e.Name(), "_ops.jsonl") {
			return nil
		}
		rel, err := filepath.Rel(d.root, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list op log: %w", err)
	}
	sort.Strings(out)
	return out, nil
}

// path turns a bucket name into an absolute path.
func (d *Dir) path(bucket string) string {
	return filepath.Join(d.root, filepath.FromSlash(bucket))
}
