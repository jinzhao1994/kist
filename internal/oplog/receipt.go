package oplog

import "errors"

// ErrNotUndoable identifies bookkeeping events with no business effect to undo.
var ErrNotUndoable = errors.New("this operation has no business change to undo")

type requestReceipt struct{}

func init() { Register(ActionRequestReceipt, requestReceipt{}) }

func (requestReceipt) Apply(_ Tx, payload []byte) error {
	_, err := Payload[struct{}](payload)
	return err
}
func (requestReceipt) Inverse([]byte) (Action, []byte, error) { return "", nil, ErrNotUndoable }
func (requestReceipt) Targets(payload []byte) (Targets, error) {
	_, err := Payload[struct{}](payload)
	return Targets{}, err
}
