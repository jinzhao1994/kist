package oplog

import (
	"errors"
	"fmt"
)

// Handler implements one action.
type Handler interface {
	// Apply performs the change. It must be a pure function of (tx, payload):
	// no generated IDs, no clock reads, no decisions that depend on the
	// current state of the library (invariant 3).
	//
	// Verifying that the current state matches the payload's recorded "before"
	// and failing if it does not IS allowed — that is an assertion, not a
	// decision, and it is how strict undo is implemented (spec 2.5).
	Apply(tx Tx, payload []byte) error

	// Inverse returns the action and payload that undo this op.
	//
	// It returns an action as well as a payload because the inverse of
	// create_box is delete_box — a different handler entirely.
	Inverse(payload []byte) (Action, []byte, error)

	// Targets extracts the index-table rows from the payload alone.
	Targets(payload []byte) (Targets, error)
}

// ErrUnknownAction means this build has no handler for an action found in the
// log.
var ErrUnknownAction = errors.New("unknown op action")

var registry = map[Action]Handler{}

// Register wires a handler to an action. It panics on a duplicate: two
// handlers for one action is a programming error that must not survive to
// runtime, and every call site is an init function.
func Register(a Action, h Handler) {
	if _, exists := registry[a]; exists {
		panic("oplog: duplicate handler for action " + string(a))
	}
	registry[a] = h
	implementedActions = append(implementedActions, a)
}

// Lookup finds the handler for an action.
//
// The error wording matters (spec 2.2). Reading an op this build does not
// understand is not corruption — it is a library that a newer kist has
// written to, and the user needs to be told exactly that rather than shown a
// crash.
func Lookup(a Action) (Handler, error) {
	h, ok := registry[a]
	if !ok {
		return nil, fmt.Errorf("op action %q is not known to this build; the library was probably written by a newer kist: %w", a, ErrUnknownAction)
	}
	return h, nil
}
