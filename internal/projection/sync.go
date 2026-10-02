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

// Add accumulates another sync's counts.
func (s *Stats) Add(o Stats) {
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

// Diff is what one directory needs to become correct: things to make, things
// to remove. Remove comes out deepest-first, ready to be applied in order.
type Diff struct {
	Create []Entry
	Remove []string
	// Repair identifies replacements, which are neither missing nor stray.
	Repair []LinkProblem
}

// Empty reports whether the projection already matches.
func (d Diff) Empty() bool { return len(d.Create) == 0 && len(d.Remove) == 0 }

// DiffDir computes what SyncDir would do, without doing any of it.
//
// verify's projection check is this function (spec 3.8). Sharing the diff with
// the thing that repairs is the whole point: two implementations of "what does
// boxes/ look like when it is right" would eventually disagree, and then
// neither the report nor the repair could be trusted.
func (s *Syncer) DiffDir(dir string, want []Entry) (Diff, error) {
	desired := make(map[string]Entry, len(want))
	for _, e := range want {
		desired[e.Path] = e
	}
	actual, err := s.walk(dir)
	if err != nil {
		return Diff{}, err
	}

	// Check identity/size as well as presence. Existing paths can still point
	// at the wrong blob or have the wrong filesystem type.
	problems, err := s.CheckLinks(want)
	if err != nil {
		return Diff{}, err
	}
	replace := make(map[string]bool)
	for _, p := range problems {
		replace[p.Path] = true
	}
	for _, e := range want {
		if isDir, exists := actual[e.Path]; exists && isDir != e.IsDir {
			if !replace[e.Path] {
				problems = append(problems, LinkProblem{e.Path, "wrong filesystem type"})
			}
			replace[e.Path] = true
		}
	}
	d := Diff{Repair: problems}
	for p := range actual {
		if _, ok := desired[p]; !ok || replace[p] {
			d.Remove = append(d.Remove, p)
		}
	}
	// Deepest first, so a directory is empty by the time it goes.
	slices.Sort(d.Remove)
	slices.Reverse(d.Remove)

	// Plan order, which is parents before children.
	for _, e := range want {
		if _, exists := actual[e.Path]; !exists || replace[e.Path] {
			d.Create = append(d.Create, e)
		}
	}
	return d, nil
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
	d, err := s.DiffDir(dir, want)
	if err != nil {
		return Stats{}, err
	}
	return s.apply(d, want), nil
}

// apply carries out a diff.
//
// want is passed alongside it because step 4 stamps every directory in the
// plan, not only the ones just created: a sync that merely removed a stray
// still has to put its parent's mtime back, and the removal is what disturbed
// it.
func (s *Syncer) apply(d Diff, want []Entry) Stats {
	var stats Stats

	// 1. Strays, deepest first.
	for _, p := range d.Remove {
		if err := os.RemoveAll(filepath.Join(s.Root, filepath.FromSlash(p))); err != nil {
			s.log().Error("could not remove a stray from the projection", "path", p, "err", err)
			stats.Failed++
			continue
		}
		stats.Removed++
	}

	// 2 and 3. Creations, in plan order, which is parents before children.
	for _, e := range d.Create {
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
	return stats
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
		out[filepath.ToSlash(rel)] = d.IsDir()
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read the projection under %s: %w", dir, err)
	}
	return out, nil
}

// StrayBoxDirs lists directories directly under Root that belong to no box.
//
// That is what a deleted box or an interrupted rename leaves behind. Dot-files
// are left alone: the projection's own dirty marker lives there.
//
// verify reports these and PruneRoot removes them, from this one list — the
// same reason DiffDir exists. A checker and a repairer with separate notions
// of "belongs to no box" is a pair that will one day disagree.
func (s *Syncer) StrayBoxDirs(keep map[string]bool) ([]string, error) {
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", s.Root, err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") || keep[e.Name()] {
			continue
		}
		out = append(out, e.Name())
	}
	return out, nil
}

// PruneRoot removes the directories StrayBoxDirs finds.
func (s *Syncer) PruneRoot(keep map[string]bool) (int, error) {
	strays, err := s.StrayBoxDirs(keep)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, name := range strays {
		if err := os.RemoveAll(filepath.Join(s.Root, name)); err != nil {
			return removed, fmt.Errorf("remove %s: %w", name, err)
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
