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
	ActionUploadFiles          Action = "upload_files"
	ActionUndoUploadFiles      Action = "undo_upload_files"
	ActionRenameEntry          Action = "rename_entry"
	ActionDeleteEntryContents  Action = "delete_entry_contents"
	ActionRestoreEntryContents Action = "restore_entry_contents"
	ActionCreateSavedSearch    Action = "create_saved_search"
	ActionUpdateSavedSearch    Action = "update_saved_search"
	ActionDeleteSavedSearch    Action = "delete_saved_search"
	ActionRequestReceipt       Action = "request_receipt"
	ActionCreateBox            Action = "create_box"
	ActionDeleteBox            Action = "delete_box"
	ActionUpdateBox            Action = "update_box"
	ActionDeleteBoxContents    Action = "delete_box_contents"
	ActionRestoreBoxContents   Action = "restore_box_contents"

	ActionCreateTag Action = "create_tag"
	ActionDeleteTag Action = "delete_tag"
	ActionRenameTag Action = "rename_tag"
	ActionMoveTag   Action = "move_tag"

	ActionTagBoxes   Action = "tag_boxes"
	ActionUntagBoxes Action = "untag_boxes"

	ActionCreateFolder Action = "create_folder"
	ActionDeleteFolder Action = "delete_folder"

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
// It is the second half of the updated_at rule (spec 1.5.1). Origin says
// whether a person did it; this says whether there was anything to see. Tags
// and metadata are labels on content rather than content, and a directory
// whose mtime moved because a photo inside it was tagged is a lie told to
// Finder.
func (a Action) ChangesContent() bool {
	switch a {
	case ActionUploadFiles, ActionUndoUploadFiles, ActionRenameEntry, ActionDeleteEntryContents, ActionRestoreEntryContents, ActionCreateBox, ActionDeleteBox, ActionUpdateBox, ActionDeleteBoxContents, ActionRestoreBoxContents,
		ActionImportFiles, ActionDeleteFiles, ActionCreateFolder, ActionDeleteFolder:
		return true
	}
	return false
}

// Origin is what kind of executor produced an op.
//
// Not "source": that word is taken by file_metadata.source and
// file_tag_sources.source, which name a specific extractor. These are two
// axes, and one import proves it — the op's origin is user, while the rows it
// writes carry source "file". Neither derives from the other.
//
// It has to stay a closed class rather than an extractor id, because
// PropagatesUpdatedAt runs during a rebuild: classifying "exif" would mean
// consulting a registry that lives in Go, changes between versions, and no
// longer lists an extractor once it is uninstalled. A replay would then depend
// on which plugins happen to be installed.
type Origin string

const (
	// OriginUser: a person, through the CLI or the web UI.
	OriginUser Origin = "user"
	// OriginExtractor: the extraction pipeline (EXIF and friends, M3).
	OriginExtractor Origin = "extractor"
	// OriginMaintenance: relink / rebuild / gc / scrub repairs.
	OriginMaintenance Origin = "maintenance"
	// OriginSync: replicated from another device. Nothing produces this yet.
	OriginSync Origin = "sync"
)

// AllOrigins is every origin an op may carry.
//
// Unlike AllActions, which reads the handler registry, this is a written list:
// a Origin has no handler to register itself with. A test pins it to the
// declared constants, because a list a person has to remember to update is a
// list that eventually disagrees with the thing it describes.
func AllOrigins() []Origin {
	return []Origin{OriginUser, OriginExtractor, OriginMaintenance, OriginSync}
}

// Valid reports whether s is one of the declared origins.
//
// write() checks it for the same reason it looks up the action: the op log is
// append-only, so a row with an origin nothing recognises cannot be corrected
// later — and PropagatesUpdatedAt would silently answer false for it, which
// looks exactly like a deliberate "no".
func (s Origin) Valid() bool {
	for _, known := range AllOrigins() {
		if s == known {
			return true
		}
	}
	return false
}

// PropagatesUpdatedAt reports whether ops from this origin should bump
// updated_at up the folder chain (spec 1.5.1 ②).
//
// updated_at answers "has anything happened under here lately", and it is only
// useful if it means human activity. Re-running an extractor over the whole
// library would otherwise stamp a decade of archive as modified this morning,
// which does not degrade the field so much as delete it.
//
// The set is {user} today and becomes {user, sync} when another device's
// person can act on this library.
func (s Origin) PropagatesUpdatedAt() bool {
	return s == OriginUser
}
