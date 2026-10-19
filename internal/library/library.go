package library

import (
	"context"
	"database/sql"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/lockfile"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// Library is an open kist library, and the only thing in this program that can
// write to one (invariant 2). api and cmd hold one of these; neither can reach
// store's write side, because the write path is unexported.
type Library struct {
	layout Layout
	cfg    *config.Config
	db     *sql.DB
	blobs  *blob.Dir
	// ops is the authoritative log. The database is a projection of it
	// (invariant 1); everything written here lands in ops first.
	ops *journal.Dir
	// lock serializes writes against every other process sharing this library
	// — and, inside serve, against every other request (spec 2.7). Verify
	// also takes it to compare a stable state; ordinary queries do not.
	lock  *lockfile.Locker
	log   *slog.Logger
	now   func() time.Time
	hooks hooks
	// loc is the library's timezone, resolved once at Open. It interprets a
	// date range against capture instants and renders one back (spec 1.5.2).
	loc *time.Location
	// noRecovery suppresses Open's catch-up. See WithoutRecovery.
	noRecovery bool
}

// hooks are test seams.
//
// They are unexported and nothing outside a test ever sets them. The
// alternative for testing interruption is a real signal, which is not
// something a test can aim precisely at one point in a loop.
type hooks struct {
	// afterFile runs after each candidate an import has processed.
	afterFile func()
	// afterAppend runs between the two phases of a write: the line is in the
	// log and flushed, nothing has applied it. Returning an error is the only
	// way to aim a crash at exactly that instant (spec 2.2.2).
	afterAppend func() error
	// afterDeepReplay runs before comparing the scratch and live databases.
	afterDeepReplay func()
}

// Option configures Open.
type Option func(*Library)

// WithClock replaces the clock. Tests use it to get reproducible timestamps,
// which matters because Rebuild's proof compares snapshots that contain them.
func WithClock(now func() time.Time) Option { return func(l *Library) { l.now = now } }

// WithLogger sets the logger.
func WithLogger(log *slog.Logger) Option { return func(l *Library) { l.log = log } }

// WithoutRecovery stops Open from replaying a log tail the database has not
// applied yet.
//
// verify, scrub and repair open this way. Catching up writes to kist.db and
// takes the write lock, and those three promise to do neither (spec 3.8) —
// a verify in cron that quietly finished someone else's interrupted write
// would be reporting on a library it had just changed. They report the lag
// instead, as integrity.LogAhead.
func WithoutRecovery() Option { return func(l *Library) { l.noRecovery = true } }

// Open opens an existing library.
func Open(root string, opts ...Option) (*Library, error) {
	layout, err := OpenLayout(root)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(layout.ConfigPath())
	if err != nil {
		return nil, err
	}

	l := &Library{
		layout: layout,
		cfg:    cfg,
		log:    slog.Default(),
		now:    time.Now,
		lock:   lockfile.New(layout.LockPath()),
	}
	for _, opt := range opts {
		opt(l)
	}

	if l.loc, err = cfg.Library.Zone(); err != nil {
		return nil, err
	}
	if l.db, err = store.Open(layout.DB()); err != nil {
		return nil, err
	}
	releaseMigration, err := l.lock.Acquire(context.Background())
	if err != nil {
		l.db.Close()
		return nil, err
	}
	err = store.Migrate(l.db)
	releaseMigration()
	if err != nil {
		l.db.Close()
		return nil, err
	}
	// Blob publication is independent of the journal/database commit lock.
	// Every library instance uses the same file, including repair-only opens.
	blobPlacement := lockfile.New(filepath.Join(layout.Root, "blobs.lock"))
	if l.blobs, err = blob.NewDir(layout.Blobs(), l.log, blob.WithPlacementLock(blobPlacement.Acquire)); err != nil {
		l.db.Close()
		return nil, err
	}
	if l.ops, err = journal.NewDir(layout.Ops()); err != nil {
		l.db.Close()
		return nil, err
	}
	if err := l.checkLinkMode(); err != nil {
		l.db.Close()
		return nil, err
	}
	// A crash can leave the database one op behind the log. Repairing it here
	// rather than on the next write is what keeps read-only commands honest:
	// otherwise kist ls would quietly omit the last import until something
	// wrote (spec 2.2.2).
	if !l.noRecovery {
		if err := l.catchUp(context.Background()); err != nil {
			l.db.Close()
			return nil, err
		}
	}
	return l, nil
}

func (l *Library) Close() error {
	if l.db == nil {
		return nil
	}
	return l.db.Close()
}

// Layout exposes where things live, for commands that print paths.
func (l *Library) Layout() Layout { return l.layout }

// TagTree reads the whole tag tree, together with each tag's subtree total.
//
// This is the only thing in the library that reads every tag, and it is read
// per call rather than kept on Library — see TagTree's own documentation for
// why both of those are deliberate.
func (l *Library) TagTree() (*TagTree, map[domain.TagID]int, error) {
	tree, counts, err := l.TagTreeCounts(context.Background())
	totals := make(map[domain.TagID]int, len(counts))
	for id, count := range counts {
		totals[id] = count.Boxes + count.Files
	}
	return tree, totals, err
}

// zone is the library's configured timezone (spec 1.5.2). It interprets a date
// range against capture instants and renders one back.
func (l *Library) zone() *time.Location { return l.loc }

// Zone is the timezone this library renders instants in.
//
// It is exported because a capture time is an instant and rendering it is the
// caller's decision (spec 1.5.2): cmd needs it to put an offset on the JSON,
// and api will need it for the same reason.
func (l *Library) Zone() *time.Location { return l.loc }

// Box reads one box.
func (l *Library) Box(ctx context.Context, id domain.BoxID) (domain.Box, bool, error) {
	var (
		b     domain.Box
		found bool
	)
	err := l.readTx(ctx, func(vtx *store.ViewTx) error {
		var err error
		b, found, err = store.DisplayBox(vtx.Queryer(), id)
		return err
	})
	return b, found, err
}

// Boxes lists boxes.
func (l *Library) Boxes(ctx context.Context, opt ListBoxesOptions) ([]domain.Box, error) {
	var boxes []domain.Box
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		if opt.Tag != nil {
			_, found, err := store.TagByID(tx.Queryer(), *opt.Tag)
			if err != nil {
				return err
			}
			if !found {
				return ErrNoSuchTag
			}
		}
		var err error
		boxes, err = store.ListBoxes(tx.Queryer(), opt)
		return err
	})
	return boxes, err
}

// BoxTags returns the tags attached to a box.
func (l *Library) BoxTags(ctx context.Context, id domain.BoxID) ([]domain.TagID, error) {
	return store.BoxTags(l.db, id)
}

// Ops reads the history.
//
// The returned ops carry no payload: the history view never renders one, and
// an import's payload runs to megabytes (spec 2.2.1). Undo reads the whole row
// separately.
func (l *Library) Ops(ctx context.Context, opt ListOpsOptions) ([]Op, error) {
	rows, err := store.ListOps(l.db, opt)
	if err != nil {
		return nil, err
	}
	out := make([]Op, len(rows))
	for i, r := range rows {
		out[i] = toOp(r)
	}
	return out, nil
}

// RevertedBy reports which op undid this one, if any.
func (l *Library) RevertedBy(ctx context.Context, id int64) (int64, bool, error) {
	return store.RevertedBy(l.db, id)
}

// toOp converts store's row into oplog's type. The two packages do not import
// each other; library is the seam, and this function is the price of that.
func toOp(r store.OpRow) oplog.Op {
	return oplog.Op{
		ID:        r.ID,
		TS:        r.TS,
		Action:    oplog.Action(r.Action),
		Origin:    oplog.Origin(r.Origin),
		Payload:   r.Payload,
		RevertsID: r.RevertsID,
		RootOpID:  r.RootOpID,
	}
}

// store.ViewTx satisfies oplog.Tx structurally — store does not import oplog,
// so neither package can state that relationship. library is where the two
// meet, so this is the only place the compiler can be asked to check it.
// Without this line, a missing method would surface as a confusing error deep
// inside write() instead of right here.
var _ oplog.Tx = (*store.ViewTx)(nil)

// The types below let cmd and api speak to library without importing store or
// oplog at all.
//
// They are aliases, not copies: there is one definition of each, and no
// conversion layer that could drift. What they buy is the import boundary —
// a command that cannot name store's package cannot call store's write side,
// which is invariant 2 expressed as something the compiler and
// internal/arch's test can both check.
type (
	// BoxFields are a box's user-editable fields.
	BoxFields = oplog.BoxFields
	// ListBoxesOptions controls the box listing.
	ListBoxesOptions = store.ListBoxesOptions
	// ListOpsOptions filters the history view.
	ListOpsOptions = store.ListOpsOptions
	// Op is one entry in the history.
	Op = oplog.Op
)
