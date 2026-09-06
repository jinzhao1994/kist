package library

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/store"
)

var (
	// ErrAlreadyLibrary means root already holds a kist library.
	ErrAlreadyLibrary = errors.New("already a kist library")
	// ErrNotEmpty means root holds something else. kist will not lay a library
	// over a user's files: boxes/ gets rebuilt wholesale by `kist relink`, and
	// mixing the two would put that rebuild next to things it does not own.
	ErrNotEmpty = errors.New("directory is not empty")
)

// Probe is what init found out about the chosen location.
type Probe struct {
	LinkMode  blob.LinkMode
	FreeBytes uint64
	FreeKnown bool
}

// CreateResult describes the library that was just built.
type CreateResult struct {
	Layout Layout
	Probe  Probe
	Config *config.Config
}

// Create builds a new library at root.
//
// The order matters: directories first, then probe (which needs both blobs/
// and boxes/ to exist), then the database, then the config — which records
// what the probe found.
func Create(_ context.Context, root string) (*CreateResult, error) {
	l, err := prepareRoot(root)
	if err != nil {
		return nil, err
	}

	for _, dir := range []string{l.Blobs(), l.Boxes(), l.Derived()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create library layout: %w", err)
		}
	}
	if _, err := blob.NewDir(l.Blobs(), slog.Default()); err != nil {
		return nil, err
	}
	// The other authority (invariant 1). Created here rather than lazily on
	// the first write so that a fresh library is a complete one on disk.
	if _, err := journal.NewDir(l.Ops()); err != nil {
		return nil, err
	}

	mode, err := blob.DetectLinkMode(l.Blobs(), l.Boxes())
	if err != nil {
		return nil, fmt.Errorf("probe link capability: %w", err)
	}
	free, freeKnown := freeSpace(l.Root)

	db, err := store.Open(l.DB())
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		return nil, err
	}

	if err := os.WriteFile(l.Notice(), []byte(noticeText), 0o644); err != nil {
		return nil, fmt.Errorf("write notice: %w", err)
	}

	cfg := config.Default(mode)
	if err := config.Save(l.ConfigPath(), cfg); err != nil {
		return nil, err
	}

	return &CreateResult{
		Layout: l,
		Probe:  Probe{LinkMode: mode, FreeBytes: free, FreeKnown: freeKnown},
		Config: cfg,
	}, nil
}

func prepareRoot(root string) (Layout, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Layout{}, fmt.Errorf("resolve library path: %w", err)
	}
	l := Layout{Root: abs}

	entries, err := os.ReadDir(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return Layout{}, fmt.Errorf("create library directory: %w", err)
		}
	case err != nil:
		return Layout{}, fmt.Errorf("inspect %s: %w", abs, err)
	case isLibrary(abs):
		return Layout{}, fmt.Errorf("%s: %w", abs, ErrAlreadyLibrary)
	case len(entries) > 0:
		return Layout{}, fmt.Errorf("%s: %w", abs, ErrNotEmpty)
	}
	return l, nil
}

const noticeText = `This directory is a kist library.

WHAT IS AUTHORITATIVE — do not edit, move, or rename:

  blobs/     your files, stored by content hash and kept read-only
  kist.db    every operation ever performed on this library

WHAT IS A PROJECTION — safe to browse, pointless to edit:

  boxes/     a human-readable view of blobs/. kist rebuilds it wholesale, so
             changes made here are not read back and will be overwritten.
             If it looks wrong, run: kist relink
  derived/   thumbnails and previews, regenerated on demand

TO BACK THIS UP:

  rsync -a --exclude boxes/ --exclude derived/ <this directory> <destination>

boxes/ and derived/ are both reconstructible from the two authoritative
things above, so backing them up only costs space.
`

// FreeSpaceAt reports the bytes available at path, and whether that could be
// determined at all. init shows this before creating anything, so it has to be
// callable without a library.
func FreeSpaceAt(path string) (uint64, bool) { return freeSpace(path) }
