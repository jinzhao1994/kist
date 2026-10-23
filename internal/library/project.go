package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/projection"
	"github.com/jinzhao1994/kist/internal/store"
)

// dirtyMarker records that boxes/ is behind the database.
//
// It is a file rather than a row because it describes the filesystem, and
// because it has to survive `kist rebuild` — which discards every materialized
// view and would otherwise take the warning with it.
const dirtyMarker = ".kist-projection-dirty"

// RelinkStats is what a relink did.
type RelinkStats struct {
	Boxes int
	projection.Stats
}

// ProjectionDirty reports whether boxes/ is known to be out of step.
//
// spec 3.4.1 requires this to be visible: without it the projection goes stale
// silently until someone happens to run verify.
func (l *Library) ProjectionDirty() bool {
	_, err := os.Stat(filepath.Join(l.layout.Boxes(), dirtyMarker))
	return err == nil
}

func (l *Library) markDirty(cause error) {
	body := fmt.Sprintf("%s\n%v\n\nboxes/ is behind the database. Run `kist relink` to rebuild it.\n",
		time.Now().UTC().Format(time.RFC3339), cause)
	if err := os.WriteFile(filepath.Join(l.layout.Boxes(), dirtyMarker), []byte(body), 0o644); err != nil {
		l.log.Error("could not record that the projection is stale", "err", err)
	}
}

func (l *Library) clearDirty() {
	if err := os.Remove(filepath.Join(l.layout.Boxes(), dirtyMarker)); err != nil && !os.IsNotExist(err) {
		l.log.Error("could not clear the projection's stale marker", "err", err)
	}
}

func (l *Library) syncer() *projection.Syncer {
	return &projection.Syncer{
		Root:     l.layout.Boxes(),
		LinkMode: l.settings.Load().config.Storage.LinkMode,
		Locate:   func(key string) (string, bool) { return l.blobs.LocalPath(blobKey(key)) },
		Log:      l.log,
	}
}

// boxDirs computes the on-disk name of every box.
//
// It needs all of them because a collision is broken across the whole set: one
// box changing its date can change another box's directory name (spec 3.4.1).
// One row per box is cheap; files are never loaded here.
func (l *Library) boxDirs() (map[domain.BoxID]string, []domain.Box, error) {
	boxes, err := store.ListBoxes(l.db, store.ListBoxesOptions{})
	if err != nil {
		return nil, nil, err
	}
	named := make([]projection.Named, len(boxes))
	for i, b := range boxes {
		named[i] = projection.Named{ID: b.ID.ID, Name: projection.BoxDirName(b)}
	}
	resolved := projection.Resolve(named)

	out := make(map[domain.BoxID]string, len(boxes))
	for _, b := range boxes {
		out[b.ID] = resolved[b.ID.ID]
	}
	return out, boxes, nil
}

// Relink rebuilds boxes/ from the database.
//
// It is the correctness backstop for the entire projection layer: incremental
// maintenance is an optimization, and any time that gets complicated — a
// rename chain, a sharing violation, a half-finished move — the answer is to
// fall back here (spec 3.4.1). It only creates links, so it is cheap enough to
// be the answer.
//
// Memory is bounded by the largest box rather than by the library: box names
// need global knowledge to disambiguate, but that is one small row each, and
// files are loaded a box at a time.
func (l *Library) Relink(ctx context.Context) (RelinkStats, error) {
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return RelinkStats{}, err
	}
	defer release()
	return l.relink(ctx)
}

// relink is Relink without the lock, for the callers that already hold it.
//
// The split exists because project falls back here, and project runs inside
// write — taking the lock again there would deadlock against ourselves, which
// the in-process semaphore would report five seconds later as "another kist
// process is writing". The wrong answer and a slow one.
func (l *Library) relink(ctx context.Context) (RelinkStats, error) {
	var stats RelinkStats

	if err := os.MkdirAll(l.layout.Boxes(), 0o755); err != nil {
		return stats, fmt.Errorf("create boxes/: %w", err)
	}
	dirs, boxes, err := l.boxDirs()
	if err != nil {
		return stats, err
	}
	syncer := l.syncer()

	for _, b := range boxes {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		s, err := l.syncBox(syncer, b, dirs[b.ID])
		if err != nil {
			return stats, err
		}
		stats.Stats.Add(s)
		stats.Boxes++
	}

	keep := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		keep[d] = true
	}
	removed, err := syncer.PruneRoot(keep)
	if err != nil {
		return stats, err
	}
	stats.Removed += removed

	l.clearDirty()
	return stats, nil
}

// syncBox plans and writes one box's subtree.
func (l *Library) syncBox(s *projection.Syncer, b domain.Box, dir string) (projection.Stats, error) {
	folders, err := store.FoldersOfBox(l.db, b.ID)
	if err != nil {
		return projection.Stats{}, err
	}
	files, err := store.FilesOfBox(l.db, b.ID)
	if err != nil {
		return projection.Stats{}, err
	}
	sets, err := store.Filesets(l.db)
	if err != nil {
		return projection.Stats{}, err
	}
	want, err := projection.Plan(dir, b, folders, files, sets...)
	if err != nil {
		return projection.Stats{}, err
	}
	return s.SyncDir(dir, want)
}

// project brings boxes/ back in step after a committed write.
//
// It runs after the transaction, never inside it (spec 3.7 step 5): the
// database is already correct, and a projection failure is a stale mirror, not
// data loss. The caller logs and sets the dirty marker rather than failing the
// write — telling a user their import did not happen, when it did, would make
// them run it again.
//
// Renames move the directory instead of relinking its contents. Anything more
// tangled than "this box's name changed" — a collision chain that moved
// another box, a rename that failed — falls back to a full Relink, which is
// the rule spec 3.4.1 sets out.
//
// before is what the projected names were when this write started. It is a
// parameter rather than a field because a field would be this process's
// opinion of a directory tree that every other process can also write to
// (spec 2.7): the projection is shared state, so "what it was before" has to
// be read, under the lock, as part of the write that is about to change it.
func (l *Library) project(ctx context.Context, changed []domain.BoxID, before map[domain.BoxID]string) error {
	if len(changed) == 0 {
		return nil
	}
	if err := os.MkdirAll(l.layout.Boxes(), 0o755); err != nil {
		return fmt.Errorf("create boxes/: %w", err)
	}

	dirs, boxes, err := l.boxDirs()
	if err != nil {
		return err
	}
	touched := make(map[domain.BoxID]bool, len(changed))
	for _, id := range changed {
		touched[id] = true
	}

	// A box nobody touched whose name moved anyway means the collision suffix
	// spread (spec 3.4.1). That is the "complicated" case; take the backstop.
	for id, name := range dirs {
		if old, ok := before[id]; ok && old != name && !touched[id] {
			_, err := l.relink(ctx)
			return err
		}
	}

	syncer := l.syncer()

	// Boxes that no longer exist — an undone create_box, say — leave their
	// directory behind.
	for id, old := range before {
		if _, ok := dirs[id]; !ok {
			if err := os.RemoveAll(filepath.Join(l.layout.Boxes(), old)); err != nil {
				return err
			}
		}
	}

	byID := make(map[domain.BoxID]domain.Box, len(boxes))
	for _, b := range boxes {
		byID[b.ID] = b
	}
	for _, id := range changed {
		b, ok := byID[id]
		if !ok {
			continue // deleted above
		}
		if old, had := before[id]; had && old != dirs[id] {
			if err := syncer.Rename(old, dirs[id]); err != nil {
				l.log.Warn("falling back to a full relink", "box", id, "err", err)
				_, err := l.relink(ctx)
				return err
			}
		}
		if _, err := l.syncBox(syncer, b, dirs[id]); err != nil {
			return err
		}
	}
	return nil
}

// blobKey converts library's plain string back into blob's named type. domain
// stores the key as text so that it can stay dependency-free; this is one of
// the two places the conversion happens.
func blobKey(s string) blob.Key { return blob.Key(s) }

// checkLinkMode verifies an explicit mode or selects an available default.
func (l *Library) checkLinkMode() error {
	if err := os.MkdirAll(l.layout.Boxes(), 0o755); err != nil {
		return fmt.Errorf("create boxes/: %w", err)
	}
	if mode := l.settings.Load().config.Storage.LinkMode; mode != "" {
		if err := blob.CheckLinkMode(l.blobs.Root(), l.layout.Boxes(), mode); err != nil {
			return fmt.Errorf("configured link mode %s is unavailable: %w", mode, err)
		}
		return nil
	}
	mode, err := blob.DetectLinkMode(l.blobs.Root(), l.layout.Boxes())
	if err != nil {
		return err
	}
	l.settings.Load().config.Storage.LinkMode = mode
	return nil
}
