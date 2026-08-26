// Package blob is kist's content-addressed store. It knows nothing about the
// database — the only thing linking the two is the key.
package blob

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// AlgoSHA256 is the only hash in v0.1. SHA-256 was chosen over faster options
// because a user can verify it themselves with `shasum -a 256` — an archive's
// trustworthiness comes from being independently checkable (spec 3.3).
const AlgoSHA256 = "sha256"

// Key addresses a blob: "sha256:<64 lowercase hex>", plus ".N" when a hash
// collision forced a second object under the same digest.
//
// The algorithm prefix lives in the key itself, not just in the path, so a key
// read out of the database is enough to find the object without knowing which
// algorithm era it came from.
type Key string

// NewKey builds a key. suffix 0 means no suffix.
func NewKey(algo, hex string, suffix int) Key {
	if suffix == 0 {
		return Key(algo + ":" + hex)
	}
	return Key(algo + ":" + hex + "." + strconv.Itoa(suffix))
}

// ParseKey validates and returns a key. Everything that is not exactly the
// accepted shape is rejected: a key is an address, and a malformed address
// that gets stored is a dangling reference nobody notices until restore time.
func ParseKey(s string) (Key, error) {
	algo, rest, ok := strings.Cut(s, ":")
	if !ok {
		return "", fmt.Errorf("parse blob key %q: missing algorithm prefix", s)
	}
	if algo != AlgoSHA256 {
		return "", fmt.Errorf("parse blob key %q: unknown algorithm %q", s, algo)
	}

	hex, suffixStr, hasSuffix := strings.Cut(rest, ".")
	if len(hex) != 64 {
		return "", fmt.Errorf("parse blob key %q: hash is %d characters, want 64", s, len(hex))
	}
	for i := 0; i < len(hex); i++ {
		c := hex[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return "", fmt.Errorf("parse blob key %q: hash must be lowercase hex", s)
		}
	}

	if hasSuffix {
		n, err := strconv.Atoi(suffixStr)
		if err != nil || n < 1 || suffixStr != strconv.Itoa(n) {
			return "", fmt.Errorf("parse blob key %q: collision suffix must be a positive integer", s)
		}
	}
	return Key(s), nil
}

func (k Key) Algo() string {
	algo, _, _ := strings.Cut(string(k), ":")
	return algo
}

// Hex returns the bare digest, without algorithm prefix or collision suffix.
// This is what goes in the blobs.sha256 column, which is deliberately not
// unique: a collision produces two rows with the same digest (spec 3.3).
func (k Key) Hex() string {
	_, rest, _ := strings.Cut(string(k), ":")
	hex, _, _ := strings.Cut(rest, ".")
	return hex
}

// Suffix returns the collision counter, 0 when there is none.
func (k Key) Suffix() int {
	_, rest, _ := strings.Cut(string(k), ":")
	_, s, ok := strings.Cut(rest, ".")
	if !ok {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

// RelPath is the only legal way to turn a key into a path: "sha256/ab/cd/<hex>".
// Two levels of sharding keep directory sizes manageable; the shard is derived
// from the hash rather than stored, so there is one source of truth.
func (k Key) RelPath() string {
	hex := k.Hex()
	name := hex
	if s := k.Suffix(); s != 0 {
		name += "." + strconv.Itoa(s)
	}
	return filepath.Join(k.Algo(), hex[0:2], hex[2:4], name)
}
