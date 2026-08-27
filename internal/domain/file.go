package domain

import (
	"errors"
	"fmt"
	"time"
)

// ErrNoBlobKey means a file was built without content behind it.
var ErrNoBlobKey = errors.New("file has no blob key")

// File is one stored thing: exactly one blob, in exactly one folder, in
// exactly one box.
//
// BlobKey is a string rather than a blob.Key because domain must not depend on
// anything else in this repository — blob.Key is a named string type, so the
// conversion at the boundary costs nothing.
//
// CapturedAt is the photo's own idea of when it was taken: variable-precision
// local wall time with no timezone (spec 1.3), extracted from EXIF in M3. It
// is empty for every file until then, and files with no CapturedAt sort last
// (spec 4.5.1).
type File struct {
	ID         FileID
	BoxID      BoxID
	FolderID   FolderID
	Name       string
	NameFold   string
	BlobKey    string
	CapturedAt LocalDate
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewFile validates and folds. Timestamps are left zero: they belong to the op
// that creates the file (spec 2.3.1).
func NewFile(id FileID, box BoxID, folder FolderID, name, blobKey string) (File, error) {
	name = Normalize(name)
	if err := ValidateName(name); err != nil {
		return File{}, err
	}
	if blobKey == "" {
		return File{}, fmt.Errorf("%s: %w", name, ErrNoBlobKey)
	}
	return File{
		ID:       id,
		BoxID:    box,
		FolderID: folder,
		Name:     name,
		NameFold: Fold(name),
		BlobKey:  blobKey,
	}, nil
}
