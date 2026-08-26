package blob

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

const compareBlockSize = 128 * 1024

// compareAndHash reads the existing blob once and answers two different
// questions with that one pass:
//
//   - existingHex: does the existing blob still hash to what its name claims?
//     That is a corruption check.
//   - identical: is the candidate byte-for-byte the same content? That is a
//     collision check.
//
// Both are needed. Rehashing alone cannot detect a collision — when two
// different contents hash alike, which is the definition of a collision, the
// rehash result is identical to the deduplication case (spec 3.3).
//
// The existing file is always read to the end, even once a difference is
// known, because the corruption verdict needs its complete hash.
func compareAndHash(existingPath, candidatePath string) (identical bool, existingHex string, err error) {
	existing, err := os.Open(existingPath)
	if err != nil {
		return false, "", fmt.Errorf("open existing blob: %w", err)
	}
	defer existing.Close()

	candidate, err := os.Open(candidatePath)
	if err != nil {
		return false, "", fmt.Errorf("open candidate: %w", err)
	}
	defer candidate.Close()

	h := sha256.New()
	same := true

	eb := make([]byte, compareBlockSize)
	cb := make([]byte, compareBlockSize)

	for {
		en, eerr := io.ReadFull(existing, eb)
		if en > 0 {
			h.Write(eb[:en])
		}

		if same {
			cn, cerr := io.ReadFull(candidate, cb)
			if cn != en || !bytes.Equal(eb[:en], cb[:cn]) {
				same = false
			} else if cerr != nil && !isEOF(cerr) {
				return false, "", fmt.Errorf("read candidate: %w", cerr)
			}
		}

		if eerr != nil {
			if isEOF(eerr) {
				break
			}
			return false, "", fmt.Errorf("read existing blob: %w", eerr)
		}
	}

	if same {
		// The existing file is exhausted; the candidate must be too, otherwise
		// it is a longer file sharing a prefix.
		var probe [1]byte
		if n, _ := candidate.Read(probe[:]); n > 0 {
			same = false
		}
	}

	return same, hex.EncodeToString(h.Sum(nil)), nil
}

func isEOF(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}
