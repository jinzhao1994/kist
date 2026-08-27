package domain

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Fold produces the comparison form of a piece of user text: NFC-normalized
// and lowercased.
//
// It exists because LIKE's case behaviour differs between engines — SQLite is
// case-insensitive for ASCII, Postgres is not — and v0.1's entire search is
// built on LIKE. Rather than inherit whichever behaviour the engine happens to
// have, kist owns the decision: written into a folded column, matched against
// a pattern put through this same function, compared byte for byte
// (spec 4.7). Search is therefore case-insensitive because we say so.
//
// Fold must stay idempotent: the query side feeds it patterns that may already
// be folded.
func Fold(s string) string {
	// Lowercase first, then normalize: a few characters denormalize when
	// lowercased, and NFC last guarantees the output is in normal form.
	return norm.NFC.String(strings.ToLower(norm.NFC.String(s)))
}

// Normalize puts user text into NFC, the one form the database stores
// (spec 3.4).
//
// It matters most for names that arrived from a filesystem: macOS hands back
// NFD, Linux hands back whatever was written. Without this, the same 中文 or
// accented name imported on two machines produces two rows whose name differs
// and whose name_fold does not — a UNIQUE constraint that fires for reasons
// nobody can see, and a projection that differs per platform.
func Normalize(s string) string { return norm.NFC.String(s) }
