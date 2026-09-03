package oplog

import (
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
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
//
// TouchBoxes and TouchFolders are deliberately absent even though ViewTx has
// them: propagating updated_at is the write path's rule, not a handler's
// (spec 1.5.1 ②). Putting them here would invite a handler to do it, and then
// a new op type could forget to.
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
	CountBoxFiles(id domain.BoxID) (int, error)

	InsertFolder(f domain.Folder) error
	GetFolder(id domain.FolderID) (domain.Folder, bool, error)
	DeleteFolder(id domain.FolderID) error
	CountFolderFiles(id domain.FolderID) (int, error)
	CountFolderChildren(id domain.FolderID) (int, error)

	InsertFile(f domain.File) error
	DeleteFile(id domain.FileID) error
	GetFile(id domain.FileID) (domain.File, bool, error)
	CountFileTags(id domain.FileID) (int, error)

	// UpsertBlob maintains the blobs index. It is an upsert rather than an
	// insert because Rebuild replays imports against a blobs table it never
	// cleared — see the implementation's comment in store.
	UpsertBlob(key, sha256 string, size int64) error

	InsertTag(t domain.Tag) error
	UpdateTag(t domain.Tag) error
	DeleteTag(id domain.TagID) error
	GetTag(id domain.TagID) (domain.Tag, bool, error)
	CountTagChildren(id domain.TagID) (int, error)
	CountTagUsage(id domain.TagID) (int, error)

	AddBoxTag(box domain.BoxID, tag domain.TagID) error
	RemoveBoxTag(box domain.BoxID, tag domain.TagID) error
	HasBoxTag(box domain.BoxID, tag domain.TagID) (bool, error)

	// AddFileTag records that source claims this tag on this file. The tag
	// itself is present once; the claim is one row per source (spec 1.6.1).
	AddFileTag(file domain.FileID, tag domain.TagID, source string) error
	// RemoveFileTag withdraws one source's claim, and drops the tag when no
	// claim is left. Naming the source is what makes it the exact inverse of
	// AddFileTag — withdrawing every claim is a different, future operation
	// (a person overruling an extractor, spec 1.6.1 ④).
	RemoveFileTag(file domain.FileID, tag domain.TagID, source string) error
	HasFileTag(file domain.FileID, tag domain.TagID) (bool, error)

	// SetFileMetadata writes one observation, replacing any row with the same
	// key. source is the extractor's id (spec 1.7.6 ①), which is how a re-run
	// of one extractor leaves every other extractor's rows alone.
	SetFileMetadata(file domain.FileID, kv extract.KV, source string) error
	DeleteFileMetadata(file domain.FileID, key string) error
	// TombstoneFileMetadata records that a person removed this key. It is a
	// row rather than the absence of one, so a re-extraction cannot put the
	// value back (spec 1.7.7).
	TombstoneFileMetadata(file domain.FileID, key string) error
	// GetFileMetadata reads one slot, tombstone included, for the before-state
	// assertion that makes an edit undoable.
	GetFileMetadata(file domain.FileID, key string) (extract.Row, bool, error)

	// SetFileExtraction records that an extractor at a version has seen this
	// file. It is what makes "which files still need extracting" an indexed
	// query rather than a scan of payloads (spec 1.7.6 ③, invariant 4).
	SetFileExtraction(file domain.FileID, extractor string, version int) error
	DeleteFileExtraction(file domain.FileID, extractor string) error

	// SetCapturedAt writes the kernel's conclusion about when a file was
	// captured (spec 1.5.2). It is here rather than on InsertFile because it
	// is resolved long after the import, and re-resolved whenever its inputs
	// change.
	SetCapturedAt(file domain.FileID, at domain.Instant) error
}
