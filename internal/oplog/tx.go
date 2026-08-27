package oplog

import (
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

// Tx is everything Apply is allowed to do.
//
// The list is short on purpose. Apply receives a Tx and a payload and nothing
// else: no clock, no ID generator, no arbitrary SQL. That is how invariant 3
// is enforced structurally — a handler that wanted to read the wall clock
// would have to change this interface, and that change would be visible in
// review.
//
// store satisfies this interface structurally; it does not import oplog, and
// oplog does not import store. library is what puts the two together.
type Tx interface {
	// TS is the timestamp of the op being applied — ops.ts, not the wall
	// clock. Every created_at and updated_at written during Apply comes from
	// here, so a rebuild reproduces the same values (spec 2.3.1).
	TS() time.Time

	InsertBox(b domain.Box) error
	UpdateBox(b domain.Box) error
	DeleteBox(id domain.BoxID) error
	GetBox(id domain.BoxID) (domain.Box, bool, error)
	CountBoxTags(box domain.BoxID) (int, error)

	InsertFolder(f domain.Folder) error
	DeleteFoldersOfBox(id domain.BoxID) error

	InsertTag(t domain.Tag) error
	UpdateTag(t domain.Tag) error
	DeleteTag(id domain.TagID) error
	GetTag(id domain.TagID) (domain.Tag, bool, error)
	CountTagChildren(id domain.TagID) (int, error)
	CountTagUsage(id domain.TagID) (int, error)

	AddBoxTag(box domain.BoxID, tag domain.TagID, source string) error
	RemoveBoxTag(box domain.BoxID, tag domain.TagID) error
	HasBoxTag(box domain.BoxID, tag domain.TagID) (bool, error)
}
