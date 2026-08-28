package library

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jinzhao1994/kist/internal/domain"
)

var (
	// ErrNoSuchOp means the history has no op with that id.
	ErrNoSuchOp = errors.New("no such operation")
	// ErrNotALibrary means the path is not a kist library.
	ErrNotALibrary = errors.New("not a kist library")
	// ErrFolderNotInBox means a folder exists but belongs to another box.
	ErrFolderNotInBox = errors.New("that folder is in a different box")
	// ErrInvariantViolated means the library contradicts itself: a file in a
	// folder that does not exist, a box with no root, two tags with the same
	// folded name. It is not a bug in the caller and not an I/O failure — it
	// is damage, and `kist verify` is the thing that describes it (spec 3.8).
	ErrInvariantViolated = errors.New("the library contradicts itself")
)

// The types below are the errors that carry data a caller needs to act on.
//
// A UI showing "cannot undo: something changed" is a dead end; one showing
// "op 51 is in the way" with 51 as a link is not (spec 2.5). Parsing that id
// back out of an English sentence is not a contract, so the id travels as a
// field and internal/errcodes lifts it onto the wire.
//
// None of them import errcodes. The kernel does not know there is a wire
// format; errcodes reads these with errors.As from above (spec 2.9).

// BlockedUndoError is a strict-mode refusal, naming the op in the way.
//
// BlockingOp is zero when the conflict could not be attributed to one op — the
// state moved but no single later op explains it. The sentence degrades with
// it, and so does the field.
type BlockedUndoError struct {
	OpID       int64
	BlockingOp int64
}

func (e *BlockedUndoError) Error() string {
	if e.BlockingOp == 0 {
		return fmt.Sprintf("cannot undo op %d: the library has changed since it ran", e.OpID)
	}
	return fmt.Sprintf(
		"cannot undo op %d: op %d changed the same box afterwards; undo that first if you want this one back",
		e.OpID, e.BlockingOp)
}

func (e *BlockedUndoError) Unwrap() error { return ErrUndoBlocked }

// AlreadyRevertedError names the op that already undid this one.
type AlreadyRevertedError struct {
	OpID       int64
	RevertedBy int64
}

func (e *AlreadyRevertedError) Error() string {
	return fmt.Sprintf("op %d was already undone by op %d", e.OpID, e.RevertedBy)
}

func (e *AlreadyRevertedError) Unwrap() error { return ErrAlreadyReverted }

// Candidate is one of the things an ambiguous reference matched.
type Candidate struct {
	ID   string
	Name string
}

// AmbiguousRefError carries what the reference matched, so that a UI can offer
// the choice instead of asking the user to retype something more specific.
type AmbiguousRefError struct {
	Ref        string
	Kind       string // "box", "tag" — what was being looked up
	Candidates []Candidate
}

func (e *AmbiguousRefError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%q matches %d %ss; use an id instead:", e.Ref, len(e.Candidates), e.Kind)
	for _, c := range e.Candidates {
		fmt.Fprintf(&sb, "\n  %s  %s", c.ID, c.Name)
	}
	return sb.String()
}

func (e *AmbiguousRefError) Unwrap() error { return ErrAmbiguousRef }

// NameTakenError names what already holds the name, so a client can offer to
// go look at it rather than only reporting a refusal.
type NameTakenError struct {
	Name       string
	ExistingID string
}

func (e *NameTakenError) Error() string {
	return fmt.Sprintf("%q is already a tag (%s)", e.Name, e.ExistingID)
}

func (e *NameTakenError) Unwrap() error { return ErrTagNameTaken }

// boxCandidates renders matched boxes for AmbiguousRefError.
func boxCandidates(boxes []domain.Box) []Candidate {
	out := make([]Candidate, len(boxes))
	for i, b := range boxes {
		out[i] = Candidate{ID: b.ID.String(), Name: b.Name}
	}
	return out
}
