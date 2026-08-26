package blob

import (
	"context"
	"io"
)

// Info describes a stored blob.
type Info struct {
	Key  Key
	Size int64
}

// Outcome says what Put actually did. Callers need the distinction: an import
// summary reports how much was deduplicated, and the last two are events worth
// shouting about.
type Outcome int

const (
	// OutcomeStored: the content was new.
	OutcomeStored Outcome = iota
	// OutcomeDeduped: identical content was already present.
	OutcomeDeduped
	// OutcomeRepaired: a blob was on disk under this name but no longer hashed
	// to it. It has been replaced with the freshly verified content.
	OutcomeRepaired
	// OutcomeCollided: different content hashed to an existing blob's digest.
	// Stored under a ".N" suffix; the existing blob is untouched.
	OutcomeCollided
)

func (o Outcome) String() string {
	switch o {
	case OutcomeStored:
		return "stored"
	case OutcomeDeduped:
		return "deduped"
	case OutcomeRepaired:
		return "repaired"
	case OutcomeCollided:
		return "collided"
	default:
		return "unknown"
	}
}

// PutResult is what Put returns.
type PutResult struct {
	Info
	Outcome Outcome
}

// Store is content-addressed blob storage. v0.1 has two implementations: the
// library's own directory, and a remote directory used as a backup and repair
// source (spec 3.10).
type Store interface {
	// Put stores the reader's content and returns the key it was addressed
	// under. It is idempotent for identical content.
	//
	// The returned key is authoritative: a collision suffix is decided here,
	// and the caller must write that key into the op payload rather than
	// recomputing it later (spec 2.3.1).
	Put(ctx context.Context, r io.Reader) (PutResult, error)

	Get(ctx context.Context, k Key) (io.ReadSeekCloser, error)
	Stat(ctx context.Context, k Key) (Info, error)
	Delete(ctx context.Context, k Key) error

	// LocalPath returns a filesystem path for the blob, if this store has one.
	// The link farm under boxes/ only works for local blobs; once an object
	// moves to cold storage this returns false and the projection has to put
	// a stub there instead (spec 3.10). v0.1 is all local.
	LocalPath(k Key) (string, bool)
}
