package oplog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Op is one row of the ops table: the authoritative record of one change.
//
// Ops are never updated and never deleted. Undo appends an inverse op that
// points back through RevertsID (spec 2.4).
type Op struct {
	ID        int64
	TS        time.Time
	Action    Action
	Origin    Origin
	Payload   []byte
	RevertsID *int64
	// RootOpID is the first op of the group this one belongs to, when a single
	// command wrote several; nil on that first op, and on every op written
	// alone. It says how the op came about rather than what it did, so no
	// handler is given it: an Apply that could read it would be an Apply whose
	// result depends on how the write was invoked (invariant 3).
	RootOpID *int64
}

// Marshal encodes a typed payload struct.
//
// Payloads are stored as TEXT json rather than a binary form: nothing ever
// queries into them (invariant 4), so a parser's speed buys nothing, while
// being able to read the audit log with a plain SQL client is worth a lot on
// the day the history looks wrong (spec 2.2.1).
func Marshal(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode op payload: %w", err)
	}
	return b, nil
}

// Payload decodes a payload into its typed struct. Unknown fields are an
// error: a payload written by a newer kist may mean something this build would
// only half-apply, and half-applying a change to an archive is worse than
// refusing it.
func Payload[T any](b []byte) (T, error) {
	var v T
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, fmt.Errorf("decode op payload: %w", err)
	}
	return v, nil
}
