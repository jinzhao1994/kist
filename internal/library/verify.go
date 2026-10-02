package library

import (
	"context"
	"fmt"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/projection"
	"github.com/jinzhao1994/kist/internal/store"
)

// VerifyOptions controls how much work verify does.
type VerifyOptions struct {
	// Deep replays the whole log into a scratch copy and compares it with the
	// live materialized views. It is minutes on a large library, which is why
	// it is not the default: verify has to be cheap enough to run often
	// (spec 3.8).
	Deep bool
}

// Verify walks the library and reports what is wrong with it, without changing
// anything.
//
// Three layers — structure, content, projection — all of which run to
// completion. A check that stops at the first problem checks nothing, and a
// user runs verify precisely when they suspect the library is broken.
//
// It stats and does not read content. That line is what keeps verify cheap
// enough to put in cron, and reading is what scrub is for (spec 3.8).
func (l *Library) Verify(ctx context.Context, opt VerifyOptions) (integrity.Report, error) {
	var r integrity.Report

	// The log, views and projection must describe the same committed state.
	// Do not catch up here: verification reports a lag without repairing it.
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return r, err
	}
	defer release()

	// Crash residue first: it is the one thing here that is safe to remove,
	// and leaving it would sit next to genuine findings looking like one
	// (spec 3.9).
	l.cleanTmp()

	log, err := l.checkLog()
	if err != nil {
		return r, err
	}
	r.Merge(log)

	structural, err := integrity.CheckStructure(l.db)
	if err != nil {
		return r, err
	}
	r.Merge(structural)

	blobs, err := integrity.CheckBlobs(ctx, l.db, l.blobs)
	if err != nil {
		return r, err
	}
	r.Merge(blobs)

	proj, err := l.checkProjection(ctx)
	if err != nil {
		return r, err
	}
	r.Merge(proj)

	if opt.Deep {
		deep, err := l.verifyDeep(ctx)
		if err != nil {
			return r, err
		}
		r.Merge(deep)
	}
	return r, nil
}

// checkLog reads ops/ and compares its end with the database's.
//
// It reports and does not repair, which is the same line scrub and repair sit
// on either side of (spec 3.8). Catching up is a replay, so it needs Apply,
// and integrity structurally cannot reach it — which is why this lives here
// and the repair lives in Open (spec 2.2.2).
func (l *Library) checkLog() (integrity.Report, error) {
	var r integrity.Report

	problems, err := l.ops.Check()
	if err != nil {
		return r, err
	}
	r.Note("operations logged", 0)
	for _, p := range problems {
		r.Add(integrity.Finding{
			Kind:   integrity.LogCorrupt,
			Detail: p.String(),
			Fix:    "restore ops/ from a backup",
		})
	}

	last, ok, err := l.ops.Last()
	if err != nil {
		return r, err
	}
	var logID int64
	if ok {
		logID = last.ID
	}
	dbID, err := store.MaxOpID(l.db)
	if err != nil {
		return r, err
	}
	r.Checked["operations logged"] = int(logID)
	if logID > dbID {
		r.Add(integrity.Finding{
			Kind:   integrity.LogAhead,
			Detail: fmt.Sprintf("the log ends at op %d, the database at %d", logID, dbID),
			Fix:    "any kist command reopens the library and replays the tail",
		})
	}
	return r, nil
}

// checkProjection plans every box the way Relink would, and diffs it.
func (l *Library) checkProjection(ctx context.Context) (integrity.Report, error) {
	var r integrity.Report

	dirs, boxes, err := l.boxDirs()
	if err != nil {
		return r, err
	}
	s := l.syncer()
	r.Note("boxes", len(boxes))

	for _, b := range boxes {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		folders, err := store.FoldersOfBox(l.db, b.ID)
		if err != nil {
			return r, err
		}
		files, err := store.FilesOfBox(l.db, b.ID)
		if err != nil {
			return r, err
		}
		want, err := projection.Plan(dirs[b.ID], b, folders, files)
		if err != nil {
			// A box that cannot even be planned is a structural problem, and
			// the structural layer has already said so in its own words. Keep
			// going: the other boxes are still worth checking.
			l.log.Warn("could not plan a box's projection", "box", b.ID, "err", err)
			continue
		}
		one, err := integrity.CheckProjection(s, dirs[b.ID], want)
		if err != nil {
			return r, err
		}
		r.Merge(one)
	}

	keep := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		keep[d] = true
	}
	strays, err := s.StrayBoxDirs(keep)
	if err != nil {
		return r, err
	}
	for _, name := range strays {
		r.Add(integrity.Finding{Kind: integrity.LinkStray, Detail: name, Fix: "kist relink"})
	}
	return r, nil
}

// cleanTmp removes crash residue from blobs/tmp.
//
// It runs at the start of verify and of import, and deliberately not in Open:
// `kist ls` must not have write side effects. A read-only command that deletes
// files is the kind of behaviour nobody thinks of when something later goes
// wrong (spec 3.9).
func (l *Library) cleanTmp() {
	n, err := l.blobs.CleanTmp(l.now().Add(-blob.TmpGrace))
	if err != nil {
		l.log.Warn("could not clean blobs/tmp", "err", err)
		return
	}
	if n > 0 {
		l.log.Info("removed crash residue from blobs/tmp", "files", n)
	}
}
