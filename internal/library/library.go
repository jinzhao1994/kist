package library

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
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
	tags   *TagTree
	// lock serializes writes against every other process sharing this library
	// — and, inside serve, against every other request (spec 2.7). Reads never
	// take it.
	lock  *lockfile.Locker
	log   *slog.Logger
	actor string
	now   func() time.Time
	hooks hooks
	// loc is the library's timezone, resolved once at Open. It interprets a
	// date range against capture instants and renders one back (spec 1.5.2).
	loc *time.Location
}

// hooks are test seams.
//
// They are unexported and nothing outside a test ever sets them. The
// alternative for testing interruption is a real signal, which is not
// something a test can aim precisely at one point in a loop.
type hooks struct {
	// afterFile runs after each candidate an import has processed.
	afterFile func()
}

// Option configures Open.
type Option func(*Library)

// WithClock replaces the clock. Tests use it to get reproducible timestamps,
// which matters because Rebuild's proof compares snapshots that contain them.
func WithClock(now func() time.Time) Option { return func(l *Library) { l.now = now } }

// WithActor records who is acting. v0.1 has one user, so this is mostly a
// label; it becomes meaningful when sharing arrives (spec 2.2).
func WithActor(actor string) Option { return func(l *Library) { l.actor = actor } }

// WithLogger sets the logger.
func WithLogger(log *slog.Logger) Option { return func(l *Library) { l.log = log } }

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
		actor:  "cli",
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
	if err := store.Migrate(l.db); err != nil {
		l.db.Close()
		return nil, err
	}
	if l.blobs, err = blob.NewDir(layout.Blobs(), l.log); err != nil {
		l.db.Close()
		return nil, err
	}
	if l.tags, err = LoadTagTree(l.db); err != nil {
		l.db.Close()
		return nil, err
	}
	if err := l.checkLinkMode(); err != nil {
		l.db.Close()
		return nil, err
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

// Tags is the in-memory tag tree.
func (l *Library) Tags() *TagTree { return l.tags }

// zone is the library's configured timezone (spec 1.5.2). It interprets a date
// range against capture instants and renders one back.
func (l *Library) zone() *time.Location { return l.loc }

// Box reads one box.
func (l *Library) Box(ctx context.Context, id domain.BoxID) (domain.Box, bool, error) {
	var (
		b     domain.Box
		found bool
	)
	err := l.readTx(ctx, func(vtx *store.ViewTx) error {
		var err error
		b, found, err = vtx.GetBox(id)
		return err
	})
	return b, found, err
}

// Boxes lists boxes.
func (l *Library) Boxes(ctx context.Context, opt ListBoxesOptions) ([]domain.Box, error) {
	return store.ListBoxes(l.db, opt)
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
		Source:    oplog.Source(r.Source),
		Actor:     r.Actor,
		Payload:   r.Payload,
		RevertsID: r.RevertsID,
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
