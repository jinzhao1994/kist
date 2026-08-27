package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxNameRunes bounds box, folder, tag and file names. The limit counts runes,
// not bytes: a byte limit would let an English name be three times longer than
// a Chinese one for no reason anybody could explain.
const MaxNameRunes = 255

var (
	ErrEmptyName        = errors.New("name is empty")
	ErrNameHasEdgeSpace = errors.New("name starts or ends with whitespace")
	ErrNameTrailingDot  = errors.New("name ends with a dot")
	ErrNameTooLong      = errors.New("name is too long")
)

// ValidateName rejects names that would produce a projected directory nobody
// can tell apart from its neighbour (spec 1.4).
//
// Normalization does not do this job: Fold deliberately does not trim, because
// trimming would make "a b" and "ab" collide. Rejecting at the door is the
// only place where the user still knows what they typed.
func ValidateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%q: %w", name, ErrEmptyName)
	}
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("%q: %w", name, ErrNameHasEdgeSpace)
	}
	// A trailing dot is legal on POSIX and silently dropped by Windows, which
	// means the same library would hold different names on different machines
	// (spec 3.4).
	if strings.HasSuffix(name, ".") {
		return fmt.Errorf("%q: %w", name, ErrNameTrailingDot)
	}
	if utf8.RuneCountInString(name) > MaxNameRunes {
		return fmt.Errorf("%q is %d characters, limit is %d: %w",
			name, utf8.RuneCountInString(name), MaxNameRunes, ErrNameTooLong)
	}
	return nil
}
