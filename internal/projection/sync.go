package projection

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jinzhao1994/kist/internal/blob"
)

// Stats counts what one sync did.
type Stats struct {
	Created int
	Removed int
	// Missing counts entries whose blob is not on local disk. v0.1 is all
	// local, but Locate returning false is exactly what a cold-storage backend
	// will do (spec 3.10), and it is a finding rather than a crash.
	Missing int
	// Failed counts entries that could not be linked. One failure does not
	// stop the rest: relink is the repair tool, and a repair that stops at the
	// first problem repairs nothing.
	Failed int
}

func (s *Stats) add(o Stats) {
	s.Created += o.Created
	s.Removed += o.Removed
	s.Missing += o.Missing
	s.Failed += o.Failed
}

// Syncer writes a planned tree into a directory.
type Syncer struct {
	// Root is the library's boxes/ directory.
	Root string
	// LinkMode is how a projected file points at its blob (spec 3.5).
	LinkMode blob.LinkMode
	// Locate maps a blob key onto a local path, reporting false when the
	// content is not on local disk.
	Locate func(key string) (string, bool)
	Log    *slog.Logger
}

func (s *Syncer) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

// SyncDir makes one directory under Root match want.
//
// It is a diff, not a rebuild: a box holding twenty thousand photos must not
// have every link torn down and remade because one file was added. Running it
// twice must change nothing, which is the property that lets relink be the
// safe fallback for every projection problem (spec 3.4.1).
//
// The order is forced:
//
//  1. remove strays — files first, then directories deepest-first, or a
//     non-empty directory refuses to go
//  2. create missing directories, shallowest-first
//  3. create missing links
//  4. stamp directory mtimes, last of all — creating a child bumps its
//     parent's mtime, so any earlier stamp would be undone
func (s *Syncer) SyncDir(dir string, want []Entry) (Stats, error) {
	var stats Stats

	desired := make(map[string]Entry, len(want))
	for _, e := range want {
		desired[e.Path] = e
	}

	actual, err := s.walk(dir)
	if err != nil {
		return stats, err
	}

	// 1. Strays. Deepest first so a directory is empty by the time it goes.
	var strays []string
	for p := range actual {
		if _, ok := desired[p]; !ok {
			strays = append(strays, p)
		}
	}
	slices.Sort(strays)
	slices.Reverse(strays)
	for _, p := range strays {
		if err := os.RemoveAll(filepath.Join(s.Root, filepath.FromSlash(p))); err != nil {
			s.log().Error("could not remove a stray from the projection", "path", p, "err", err)
			stats.Failed++
			continue
		}
		stats.Removed++
	}

	// 2 and 3. Creations, in plan order, which is parents before children.
	for _, e := range want {
		if _, exists := actual[e.Path]; exists {
			continue
		}
		abs := filepath.Join(s.Root, filepath.FromSlash(e.Path))
		if e.IsDir {
			if err := os.MkdirAll(abs, 0o755); err != nil {
				s.log().Error("could not create a projected directory", "path", e.Path, "err", err)
				stats.Failed++
				continue
			}
			stats.Created++
			continue
		}
		src, ok := s.Locate(e.BlobKey)
		if !ok {
			s.log().Warn("the content behind a projected file is not on local disk",
				"path", e.Path, "blob", e.BlobKey)
			stats.Missing++
			continue
		}
		if err := blob.Link(s.LinkMode, src, abs); err != nil {
			s.log().Error("could not link a projected file", "path", e.Path, "err", err)
			stats.Failed++
			continue
		}
		stats.Created++
	}

	// 4. Directory mtimes, after every creation.
	for _, e := range want {
		if !e.IsDir || e.ModTime.IsZero() {
			continue
		}
		abs := filepath.Join(s.Root, filepath.FromSlash(e.Path))
		if err := os.Chtimes(abs, e.ModTime, e.ModTime); err != nil && !errors.Is(err, fs.ErrNotExist) {
			s.log().Warn("could not stamp a projected directory", "path", e.Path, "err", err)
		}
	}
	return stats, nil
}

// walk reads what is actually on disk under one directory, as slash paths
// relative to Root. A directory that does not exist yet is not an error — that
// is the first run.
func (s *Syncer) walk(dir string) (map[string]bool, error) {
	out := map[string]bool{}
	base := filepath.Join(s.Root, filepath.FromSlash(dir))
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		rel, err := filepath.Rel(s.Root, p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = true
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read the projection under %s: %w", dir, err)
	}
	return out, nil
}

// PruneRoot removes directories directly under Root that belong to no box.
//
// That is what a deleted box or an interrupted rename leaves behind. Dot-files
// are left alone: the projection's own dirty marker lives there.
func (s *Syncer) PruneRoot(keep map[string]bool) (int, error) {
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("read %s: %w", s.Root, err)
	}
	removed := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") || keep[e.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(s.Root, e.Name())); err != nil {
			return removed, fmt.Errorf("remove %s: %w", e.Name(), err)
		}
		removed++
	}
	return removed, nil
}

// Rename moves a box directory rather than relinking its contents.
//
// Renaming a directory is an O(1) metadata operation and leaves every hard
// link and reflink inside untouched — they are directory entries pointing at
// inodes, and renaming the parent touches only the parent's own entry
// (spec 3.4.1). Unlinking and rebuilding a directory of twenty thousand photos
// is the alternative, and it is a disaster.
func (s *Syncer) Rename(from, to string) error {
	src := filepath.Join(s.Root, filepath.FromSlash(from))
	dst := filepath.Join(s.Root, filepath.FromSlash(to))
	if _, err := os.Stat(src); errors.Is(err, fs.ErrNotExist) {
		return nil // nothing projected yet; the caller's sync will create it
	}
	if _, err := os.Stat(dst); err == nil {
		// The target is taken, which means the naming is more tangled than a
		// single rename can express. The caller falls back to a full relink.
		return fmt.Errorf("%s already exists", to)
	}
	return os.Rename(src, dst)
}
