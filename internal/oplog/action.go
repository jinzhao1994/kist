// Package oplog defines what an operation is: its action, its payload, how it
// is applied, and how it is inverted.
//
// It knows nothing about databases. Everything it can do to a library goes
// through the Tx interface in this package, which is deliberately small — that
// is what makes invariant 3 ("Apply must be a pure function") a property of
// the type system rather than of code review.
package oplog

// Action is the discriminator stored in ops.action.
//
// It is TEXT in the database with no CHECK constraint and no enum, on purpose
// (spec 2.2): the set grows with every feature, and an append-only history
// table must stay readable after an action is retired. Strictness lives here,
// in Go, where a retired action can still be read and reported.
type Action string

const (
	ActionCreateBox Action = "create_box"
	ActionDeleteBox Action = "delete_box"
	ActionUpdateBox Action = "update_box"

	ActionCreateTag Action = "create_tag"
	ActionDeleteTag Action = "delete_tag"
	ActionRenameTag Action = "rename_tag"
	ActionMoveTag   Action = "move_tag"

	ActionTagBoxes   Action = "tag_boxes"
	ActionUntagBoxes Action = "untag_boxes"

	ActionImportFiles Action = "import_files"
	ActionDeleteFiles Action = "delete_files"

	ActionExtractMetadata Action = "extract_metadata"
	ActionSetMetadata     Action = "set_metadata"

	ActionTagFiles   Action = "tag_files"
	ActionUntagFiles Action = "untag_files"
)

// implementedActions is appended to by each handler file's init, so it always
// describes what this build can actually apply — not what the constants above
// hope for. A constant with no handler is exactly the mistake that would
// otherwise only show up on a user's library.
var implementedActions []Action

// AllActions lists every action this build implements.
func AllActions() []Action {
	out := make([]Action, len(implementedActions))
	copy(out, implementedActions)
	return out
}

// ChangesContent reports whether an action alters what boxes/ mirrors: the
// files and folders themselves, or a box's name.
//
// It is the second half of the updated_at rule (spec 1.5.1). Source says
// whether a person did it; this says whether there was anything to see. Tags
// and metadata are labels on content rather than content, and a directory
// whose mtime moved because a photo inside it was tagged is a lie told to
// Finder.
func (a Action) ChangesContent() bool {
	switch a {
	case ActionCreateBox, ActionDeleteBox, ActionUpdateBox,
		ActionImportFiles, ActionDeleteFiles:
		return true
	}
	return false
}

// Source is what kind of executor produced an op.
type Source string

const (
	// SourceUser: a person, through the CLI or the web UI.
	SourceUser Source = "user"
	// SourceExtractor: the extraction pipeline (EXIF and friends, M3).
	SourceExtractor Source = "extractor"
	// SourceMaintenance: relink / rebuild / gc / scrub repairs.
	SourceMaintenance Source = "maintenance"
	// SourceSync: replicated from another device. Nothing produces this yet.
	SourceSync Source = "sync"
)

// PropagatesUpdatedAt reports whether ops from this source should bump
// updated_at up the folder chain (spec 1.5.1 ②).
//
// updated_at answers "has anything happened under here lately", and it is only
// useful if it means human activity. Re-running an extractor over the whole
// library would otherwise stamp a decade of archive as modified this morning,
// which does not degrade the field so much as delete it.
//
// The set is {user} today and becomes {user, sync} when another device's
// person can act on this library.
func (s Source) PropagatesUpdatedAt() bool {
	return s == SourceUser
}
