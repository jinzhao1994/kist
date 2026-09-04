package domain

import (
	"errors"
	"fmt"
	"time"
)

// ErrNoBlobKey means a file was built without content behind it.
var ErrNoBlobKey = errors.New("file has no blob key")

// File is one stored thing: exactly one blob, in exactly one folder.
//
// It does not know its box, for the same reason a Folder does not: the answer
// is the box whose root its folder chain ends at, and one copy of an answer
// cannot disagree with itself (spec 1.4).
//
// BlobKey is a string rather than a blob.Key because domain must not depend on
// anything else in this repository — blob.Key is a named string type, so the
// conversion at the boundary costs nothing.
//
// CapturedAt is when the photo was taken, as a unix instant. It is empty for
// every file until the extraction pipeline fills it in (M3), and files with no
// CapturedAt sort last (spec 4.5.1).
//
// It is a conclusion, not an observation: the kernel resolves it from several
// metadata rows (EXIF's DateTimeOriginal, a video's creation time, file:mtime)
// plus a timezone, and records the result here (spec 1.5.2). No extractor sets
// it, which is a trust boundary as much as a layering one.
type File struct {
	ID         FileID
	FolderID   FolderID
	Name       string
	NameFold   string
	BlobKey    string
	CapturedAt Instant
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewFile validates and folds. Timestamps are left zero: they belong to the op
// that creates the file (spec 2.3.1).
func NewFile(id FileID, folder FolderID, name, blobKey string) (File, error) {
	name = Normalize(name)
	if err := ValidateName(name); err != nil {
		return File{}, err
	}
	if blobKey == "" {
		return File{}, fmt.Errorf("%s: %w", name, ErrNoBlobKey)
	}
	return File{
		ID:       id,
		FolderID: folder,
		Name:     name,
		NameFold: Fold(name),
		BlobKey:  blobKey,
	}, nil
}
