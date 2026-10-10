package store

import (
	"errors"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// IsStateConflict identifies relational constraints that a valid historical
// inverse can encounter after the referenced entities or names have changed.
// Other database errors (including CHECK and NOT NULL failures) stay internal.
func IsStateConflict(err error) bool {
	var dbErr *sqlite.Error
	if !errors.As(err, &dbErr) {
		return false
	}
	switch dbErr.Code() {
	case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY, sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
		return true
	}
	return false
}
