// Package errcodes turns an error into a stable machine-readable code.
//
// It is spec 2.9's contract half. The code is what a client matches on; the
// message is prose that will be reworded, and matching on it is how a script
// breaks the day somebody improves a sentence — the same reasoning that gave
// integrity.Kind a machine name separate from its headline.
//
// It sits above library rather than below it. That is deliberate: a code is
// part of the wire format, and the wire format belongs to the boundary, not to
// the kernel. Nothing in library, oplog, domain or blob knows this package
// exists.
//
// Of is a function rather than a registry populated by init(). Two reasons.
// Registration is structurally impossible for two of the packages here —
// domain and query may import nothing (spec 2.6), and ErrNameTooLong and
// ErrCursorSortMismatch live in them. And this repository has already been
// bitten by init()-time registration once: the exif extractor's init never
// ran, because nothing imported the package, and `kist extract` reported
// "extracted 0" on a library full of photographs with no hint why. A function
// has no "did registration happen" failure mode.
package errcodes

import (
	"errors"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/lockfile"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/query"
)

// Code is a stable name for one kind of failure. These never change.
type Code string

// Class is the coarse grouping a code belongs to. HTTP status and exit codes
// are derived from it, which is why the code list can grow without either of
// those having to be revisited (spec 2.9).
type Class int

const (
	// ClassInternal is a bug or an I/O failure. It is the zero value because
	// an error nobody classified is, by definition, one nobody expected.
	ClassInternal Class = iota
	// ClassNotFound is "the thing you named is not here".
	ClassNotFound
	// ClassInvalid is "your input is not well formed".
	ClassInvalid
	// ClassAmbiguous is "your reference matches several things".
	ClassAmbiguous
	// ClassConflict is "your input is fine, the current state does not allow
	// it".
	ClassConflict
	// ClassUnavailable is "not right now; retrying may work".
	ClassUnavailable
	// ClassDamaged is "the library itself is inconsistent". Kept apart from
	// ClassInternal because they are two different sentences to a user: "your
	// library needs kist repair" and "this is a bug, please report it".
	ClassDamaged
)

func (c Class) String() string {
	switch c {
	case ClassNotFound:
		return "not_found"
	case ClassInvalid:
		return "invalid"
	case ClassAmbiguous:
		return "ambiguous"
	case ClassConflict:
		return "conflict"
	case ClassUnavailable:
		return "unavailable"
	case ClassDamaged:
		return "damaged"
	default:
		return "internal"
	}
}

// The codes. Adding one means adding a line to table below; the completeness
// test walks the source of every package listed there and fails if a sentinel
// has no code.
const (
	CodeMediaNotReady   Code = "media_not_ready"
	CodeInvalidRequest  Code = "invalid_request"
	CodeRequestConflict Code = "request_conflict"
	CodeRequestScope    Code = "request_scope"
	CodeNotUndoable     Code = "not_undoable"
	CodeInternal        Code = "internal"
	CodeWriteCommitted  Code = "write_committed"
	CodeWriteUnknown    Code = "write_outcome_unknown"

	CodeSavedSearchNotFound Code = "saved_search_not_found"
	CodeBoxNotFound         Code = "box_not_found"
	CodeTagNotFound         Code = "tag_not_found"
	CodeFileNotFound        Code = "file_not_found"
	CodeMetadataNotFound    Code = "metadata_not_found"
	CodeFolderNotFound      Code = "folder_not_found"
	CodeOpNotFound          Code = "op_not_found"
	CodeUnknownAction       Code = "unknown_action"
	CodeNotALibrary         Code = "not_a_library"

	CodeNameEmpty      Code = "name_empty"
	CodeNameEdgeSpace  Code = "name_edge_space"
	CodeNameTrailigDot Code = "name_trailing_dot"
	CodeNameTooLong    Code = "name_too_long"
	CodeBadCursor      Code = "bad_cursor"
	CodeNoBlobKey      Code = "no_blob_key"

	CodeAmbiguousRef Code = "ambiguous_ref"

	CodeTagNameTaken    Code = "tag_name_taken"
	CodeTagInUse        Code = "tag_in_use"
	CodeTagCycle        Code = "tag_cycle"
	CodeUndoBlocked     Code = "undo_blocked"
	CodeAlreadyReverted Code = "already_reverted"
	CodeAlreadyLibrary  Code = "already_library"
	CodeNotEmpty        Code = "directory_not_empty"
	CodeFolderWrongBox  Code = "folder_in_another_box"

	CodeLibraryBusy    Code = "library_busy"
	CodeNoRemotes      Code = "no_remotes"
	CodeUnsupportedLnk Code = "unsupported_link_mode"

	CodeContentMismatch Code = "content_mismatch"
	CodeImportIntegrity Code = "upload_integrity_mismatch"
	CodeBlobIntegrity   Code = "blob_integrity_mismatch"
	CodeInvariantBroken Code = "invariant_violated"
	CodeStaleBefore     Code = "stale_before"

	CodeLogCorrupt Code = "log_corrupt"
	CodeLogMissing Code = "log_missing"
	CodeLogTooNew  Code = "log_format_too_new"
)

// table maps sentinels to codes, in the order Of tries them.
//
// Order matters where errors nest: library.ErrUndoBlocked wraps nothing, but
// oplog.ErrStaleBefore is what write returns underneath it, so the specific
// one has to come first.
var table = []struct {
	err  error
	code Code
}{
	{library.ErrMediaNotReady, CodeMediaNotReady},
	{library.ErrInvalidRequest, CodeInvalidRequest},
	{library.ErrRequestConflict, CodeRequestConflict},
	{library.ErrRequestScope, CodeRequestScope},
	{oplog.ErrNotUndoable, CodeNotUndoable},
	{library.ErrUndoBlocked, CodeUndoBlocked},
	{library.ErrAlreadyReverted, CodeAlreadyReverted},
	{library.ErrNoSuchSavedSearch, CodeSavedSearchNotFound},
	{library.ErrNoSuchBox, CodeBoxNotFound},
	{library.ErrNoSuchTag, CodeTagNotFound},
	{library.ErrNoSuchFile, CodeFileNotFound},
	{library.ErrNoSuchMetadata, CodeMetadataNotFound},
	{library.ErrNoSuchFolder, CodeFolderNotFound},
	{library.ErrNoSuchOp, CodeOpNotFound},
	{library.ErrAmbiguousRef, CodeAmbiguousRef},
	{library.ErrTagNameTaken, CodeTagNameTaken},
	{library.ErrTagCycle, CodeTagCycle},
	{library.ErrAlreadyLibrary, CodeAlreadyLibrary},
	{library.ErrNotEmpty, CodeNotEmpty},
	{library.ErrNoRemotes, CodeNoRemotes},
	{library.ErrNotALibrary, CodeNotALibrary},
	{library.ErrFolderNotInBox, CodeFolderWrongBox},
	{library.ErrInvariantViolated, CodeInvariantBroken},
	{library.ErrNoJournal, CodeLogMissing},

	{journal.ErrCorrupt, CodeLogCorrupt},
	{journal.ErrUnsupportedVersion, CodeLogTooNew},

	{oplog.ErrTagInUse, CodeTagInUse},
	{oplog.ErrStaleBefore, CodeStaleBefore},
	{oplog.ErrUnknownAction, CodeUnknownAction},

	{domain.ErrEmptyName, CodeNameEmpty},
	{domain.ErrNameHasEdgeSpace, CodeNameEdgeSpace},
	{domain.ErrNameTrailingDot, CodeNameTrailigDot},
	{domain.ErrNameTooLong, CodeNameTooLong},
	{domain.ErrNoBlobKey, CodeNoBlobKey},

	{query.ErrCursorSortMismatch, CodeBadCursor},

	{blob.ErrContentMismatch, CodeContentMismatch},
	{blob.ErrIntegrityMismatch, CodeBlobIntegrity},
	{library.ErrImportIntegrity, CodeImportIntegrity},
	{blob.ErrUnsupportedLink, CodeUnsupportedLnk},
}

// classes maps a code to its class. A code missing from here is internal,
// which is checked by the same test that checks completeness.
var classes = map[Code]Class{
	CodeMediaNotReady:       ClassConflict,
	CodeInvalidRequest:      ClassInvalid,
	CodeRequestConflict:     ClassConflict,
	CodeRequestScope:        ClassInternal,
	CodeNotUndoable:         ClassConflict,
	CodeWriteCommitted:      ClassConflict,
	CodeWriteUnknown:        ClassConflict,
	CodeSavedSearchNotFound: ClassNotFound,
	CodeBoxNotFound:         ClassNotFound,
	CodeTagNotFound:         ClassNotFound,
	CodeFileNotFound:        ClassNotFound,
	CodeMetadataNotFound:    ClassNotFound,
	CodeFolderNotFound:      ClassNotFound,
	CodeOpNotFound:          ClassNotFound,
	CodeUnknownAction:       ClassNotFound,
	CodeNotALibrary:         ClassNotFound,

	CodeNameEmpty:      ClassInvalid,
	CodeNameEdgeSpace:  ClassInvalid,
	CodeNameTrailigDot: ClassInvalid,
	CodeNameTooLong:    ClassInvalid,
	CodeBadCursor:      ClassInvalid,
	CodeNoBlobKey:      ClassInvalid,

	CodeAmbiguousRef: ClassAmbiguous,

	CodeTagNameTaken:    ClassConflict,
	CodeTagInUse:        ClassConflict,
	CodeTagCycle:        ClassConflict,
	CodeUndoBlocked:     ClassConflict,
	CodeAlreadyReverted: ClassConflict,
	CodeAlreadyLibrary:  ClassConflict,
	CodeNotEmpty:        ClassConflict,
	CodeStaleBefore:     ClassConflict,
	CodeFolderWrongBox:  ClassConflict,

	CodeLibraryBusy:    ClassUnavailable,
	CodeNoRemotes:      ClassUnavailable,
	CodeUnsupportedLnk: ClassUnavailable,

	CodeContentMismatch: ClassDamaged,
	CodeImportIntegrity: ClassInvalid,
	CodeBlobIntegrity:   ClassInvalid,
	CodeInvariantBroken: ClassDamaged,
	CodeLogCorrupt:      ClassDamaged,
	CodeLogMissing:      ClassDamaged,
	// Not damaged and not the caller's fault: the library is fine, this binary
	// is too old to read it. ClassConflict is exactly "your input is fine, the
	// current state does not allow it".
	CodeLogTooNew: ClassConflict,
}

// Of returns the code for an error.
//
// Anything with no code is CodeInternal — the two hundred-odd places that wrap
// a database or filesystem error do not need one, because "the disk is full"
// and "the database is locked" are the same thing to a client: nothing you can
// do, go look at the log.
func Of(err error) Code {
	if err == nil {
		return ""
	}
	var outcome *library.WriteOutcomeError
	if errors.As(err, &outcome) {
		if outcome.OpID != 0 {
			return CodeWriteCommitted
		}
		return CodeWriteUnknown
	}
	// Checked by type rather than by sentinel: BusyError carries the holder
	// diagnostic, and there is no separate sentinel to match (spec 2.7).
	var busy *lockfile.BusyError
	if errors.As(err, &busy) {
		return CodeLibraryBusy
	}
	for _, e := range table {
		if errors.Is(err, e.err) {
			return e.code
		}
	}
	return CodeInternal
}

// ClassOf returns the class of an error, which is what HTTP status and exit
// codes are built from.
func ClassOf(err error) Class { return classes[Of(err)] }
