package journal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// timeLayout is the stored form of an instant: fixed-width RFC3339 with all
// nine nanosecond digits.
//
// time.RFC3339Nano cannot be used: it drops trailing zeros, so the width would
// depend on the value. Width does not carry ordering here — the file is read
// in line order, which is id order (spec 3.1.1) — it is for the human reading
// the file, which is the whole reason the log is text.
const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

// FormatTime renders an instant for storage.
func FormatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

// ParseTime reads one back.
func ParseTime(s string) (time.Time, error) {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: bad timestamp %q: %v", ErrCorrupt, s, err)
	}
	return t.UTC(), nil
}

// Line is one op as it appears on disk.
//
// Field order is part of the format: Sum is computed by re-encoding the line
// with Sum empty, and encoding/json writes fields in declaration order. Moving
// a field changes every checksum this build would compute, so don't.
//
// Payload is carried as raw bytes and never inspected. That is invariant 4
// seen from this side: the log moves the payload, the index tables answer
// questions about it.
type Line struct {
	V         int             `json:"v"`
	ID        int64           `json:"id"`
	TS        string          `json:"ts"`
	Action    string          `json:"action"`
	Origin    string          `json:"origin"`
	RevertsID *int64          `json:"reverts_id"`
	RootOpID  *int64          `json:"root_op_id"`
	Payload   json.RawMessage `json:"payload"`
	Sum       string          `json:"sum"`
}

// Time is the line's timestamp as an instant.
func (l Line) Time() (time.Time, error) { return ParseTime(l.TS) }

// checksum is the sha256 of the line with Sum cleared.
//
// Hashing a re-encoding rather than the raw bytes is deliberate. Hashing the
// bytes would mean defining "the bytes except the sum field", which in
// practice means requiring sum to be last and slicing it off — a rule that
// holds until someone reorders the struct. Re-encoding has no such rule: it
// only requires that decode-then-encode reproduces the input, which is what
// TestRoundTripIsByteStable pins.
func checksum(l Line) (string, error) {
	l.Sum = ""
	b, err := json.Marshal(l)
	if err != nil {
		return "", fmt.Errorf("checksum op %d: %w", l.ID, err)
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// MarshalLine encodes a line, filling in V and Sum.
func MarshalLine(l Line) ([]byte, error) {
	l.V = Version
	sum, err := checksum(l)
	if err != nil {
		return nil, err
	}
	l.Sum = sum
	b, err := json.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("encode op %d: %w", l.ID, err)
	}
	return b, nil
}

// UnmarshalLine decodes a line and verifies its checksum.
func UnmarshalLine(b []byte) (Line, error) {
	var l Line
	if err := json.Unmarshal(b, &l); err != nil {
		return Line{}, fmt.Errorf("%w: cannot decode line: %v", ErrCorrupt, err)
	}
	if l.V != Version {
		return Line{}, fmt.Errorf("%w: line format v%d, this build writes v%d",
			ErrUnsupportedVersion, l.V, Version)
	}
	want, err := checksum(l)
	if err != nil {
		return Line{}, err
	}
	if l.Sum != want {
		return Line{}, fmt.Errorf("%w: op %d does not match its checksum", ErrCorrupt, l.ID)
	}
	return l, nil
}
