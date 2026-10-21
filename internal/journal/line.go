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
	Request   *Request        `json:"request,omitempty"`
	Sync      *Sync           `json:"sync,omitempty"`
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
	l.V = 2
	if l.Sync != nil {
		l.V = Version
		if !l.Sync.Valid() {
			return nil, fmt.Errorf("%w: invalid sync metadata", ErrCorrupt)
		}
	}
	if l.Request != nil && !l.Request.Valid() {
		return nil, fmt.Errorf("%w: invalid request metadata", ErrCorrupt)
	}
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
	if l.V < 1 || l.V > Version {
		return Line{}, fmt.Errorf("%w: line format v%d, this build writes v%d",
			ErrUnsupportedVersion, l.V, Version)
	}
	if l.Request != nil && (l.V < 2 || !l.Request.Valid()) {
		return Line{}, fmt.Errorf("%w: invalid request metadata", ErrCorrupt)
	}
	if l.Sync != nil && (l.V < 3 || !l.Sync.Valid()) {
		return Line{}, fmt.Errorf("%w: invalid sync metadata", ErrCorrupt)
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

// Request binds a client request to the event that completed it. It is part of
// the checksummed envelope, not an in-memory response cache.
type Request struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

// Valid limits keys to opaque ASCII identifiers and SHA-256 hex digests.
func (r Request) Valid() bool {
	if len(r.ID) < 8 || len(r.ID) > 128 || len(r.Digest) != 64 {
		return false
	}
	for _, c := range r.ID {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	_, err := hex.DecodeString(r.Digest)
	return err == nil
}

// Sync preserves the identity of an offline operation and its acceptance time.
// Digest binds the original intent, even when an explicit resolution is needed.
type Sync struct {
	ID         string `json:"id"`
	Digest     string `json:"digest"`
	AcceptedAt string `json:"accepted_at"`
}

func (s Sync) Valid() bool {
	if !(Request{ID: s.ID, Digest: s.Digest}).Valid() {
		return false
	}
	t, err := ParseTime(s.AcceptedAt)
	return err == nil && !t.IsZero() && FormatTime(t) == s.AcceptedAt
}

// CommitTimestamp selects the durable acceptance time for log placement.
// Legacy and locally authored events retain their original timestamp.
func (l Line) CommitTimestamp() string {
	if l.Sync != nil {
		return l.Sync.AcceptedAt
	}
	return l.TS
}
