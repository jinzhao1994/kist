// Package domain holds kist's core types. It depends on nothing else in this
// repository — everything else may depend on it.
package domain

import (
	"database/sql/driver"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ID is a UUIDv7: random, but time-ordered in its high bits so that index
// locality stays good as the library grows (spec 3.12). Stored as BLOB(16),
// never as a 36-character string.
type ID [16]byte

// NewID returns a fresh UUIDv7.
func NewID() ID {
	return ID(uuid.Must(uuid.NewV7()))
}

// ParseID reads the canonical hyphenated form.
func ParseID(s string) (ID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return ID{}, fmt.Errorf("parse id %q: %w", s, err)
	}
	return ID(u), nil
}

func (id ID) String() string { return uuid.UUID(id).String() }

// IsZero reports whether id is the zero value, which is never a valid ID.
func (id ID) IsZero() bool { return id == ID{} }

func (id ID) Value() (driver.Value, error) { return append([]byte(nil), id[:]...), nil }

func (id *ID) Scan(src any) error {
	switch v := src.(type) {
	case []byte:
		if len(v) != 16 {
			return fmt.Errorf("scan id: got %d bytes, want 16", len(v))
		}
		copy(id[:], v)
		return nil
	case nil:
		return errors.New("scan id: got NULL")
	default:
		return fmt.Errorf("scan id: cannot read %T", src)
	}
}

func (id ID) MarshalText() ([]byte, error) { return []byte(id.String()), nil }

func (id *ID) UnmarshalText(b []byte) error {
	parsed, err := ParseID(string(b))
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// Entity IDs are distinct types so the compiler catches a FolderID passed
// where a BoxID belongs. Embedding rather than defining (`type BoxID ID`)
// promotes every method above, including the pointer-receiver Scan.
type (
	BoxID    struct{ ID }
	FolderID struct{ ID }
	FileID   struct{ ID }
	TagID    struct{ ID }
	// GroupID marks the ops one command wrote. It names nothing that has a
	// row of its own — a group with no attributes is not an entity — so the
	// only place it appears is ops.group_id (spec 2.6).
	GroupID struct{ ID }
)

func NewBoxID() BoxID       { return BoxID{NewID()} }
func NewFolderID() FolderID { return FolderID{NewID()} }
func NewFileID() FileID     { return FileID{NewID()} }
func NewTagID() TagID       { return TagID{NewID()} }
