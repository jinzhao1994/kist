package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/store"
)

// verifyDeep replays the whole log into a scratch copy and compares it with
// the live materialized views.
//
// This is the only production check of invariant 1 — everything but ops can be
// rebuilt from ops. Until M2a that was asserted only in a test, which proves
// it about the code but says nothing about the library in front of you.
//
// The scratch database is built from ops/, not copied from kist.db. That is
// what makes this a second opinion rather than a tautology: when the log lived
// in the database, the replay read the same rows it was checking, so a
// corrupted ops table replayed into itself and matched (spec 3.8).
//
// It lives next to kist.db rather than in the system temp directory: it is as
// large as the database, and the disk holding a personal archive is the one
// with room for it. Temp directories are also routinely a different
// filesystem, and on some platforms a memory-backed one.
//
// Replaying into a scratch database rather than in place is not caution, it is
// the contract: verify does not write to the library (spec 3.8). In place
// would also mean a verify interrupted halfway leaves the library with half
// its views rebuilt.
func (l *Library) verifyDeep(ctx context.Context) (integrity.Report, error) {
	var r integrity.Report
	sizes, err := store.FolderSizeDrift(l.db)
	if err != nil {
		return r, err
	}
	for _, finding := range sizes {
		r.Add(integrity.Finding{Kind: integrity.ViewDrift, Detail: "folder size: " + finding.Detail, Fix: "kist rebuild"})
	}

	scratch := filepath.Join(filepath.Dir(l.layout.DB()), fmt.Sprintf(".kist-verify-%d.db", os.Getpid()))
	removeDatabase(scratch) // residue from a run that was killed
	defer removeDatabase(scratch)

	copyDB, err := store.Open(scratch)
	if err != nil {
		return r, err
	}
	if err := store.Migrate(copyDB); err != nil {
		copyDB.Close()
		return r, fmt.Errorf("prepare the scratch database: %w", err)
	}
	if err := rebuildInto(ctx, copyDB, l.ops); err != nil {
		copyDB.Close()
		return r, fmt.Errorf("replay the log into the scratch database: %w", err)
	}
	// Closed before the comparison, not deferred: in WAL mode an open database
	// keeps its changes in a -wal file, and ATTACH would then read something
	// that is not the finished replay.
	if err := copyDB.Close(); err != nil {
		return r, err
	}

	if l.hooks.afterDeepReplay != nil {
		l.hooks.afterDeepReplay()
	}
	drift, err := store.DiffViews(ctx, l.db, scratch)
	if err != nil {
		return r, err
	}
	r.Note("tables replayed", len(store.RebuildableTables()))
	for _, d := range drift {
		r.Add(integrity.Finding{
			Kind: integrity.ViewDrift,
			Detail: fmt.Sprintf("%s: %d row(s) only in the library, %d only in the replay",
				d.Table, d.OnlyInLive, d.OnlyInRebuilt),
			Fix: "kist rebuild",
		})
		for _, e := range d.Examples {
			r.Add(integrity.Finding{Kind: integrity.ViewDrift, Detail: "    " + e, Fix: "kist rebuild"})
		}
	}
	return r, nil
}

// removeDatabase deletes a SQLite database and the two files WAL mode leaves
// beside it. Missing any of them turns a scratch copy into litter that grows
// by one file every time verify --deep is run.
func removeDatabase(path string) {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		os.Remove(p)
	}
}
