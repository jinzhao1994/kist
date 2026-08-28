package wire

// Error is a failure, as a client reads it.
//
// Code is the contract; Message is prose that will be reworded, so anything
// deciding what to do must decide on Code (spec 2.9). Class is the coarse
// grouping HTTP status and exit codes come from — a client that does not know
// a specific code can still act on the class.
//
// The fields below Class are the data particular codes carry. They are named
// fields rather than a free-form details object for the reason invariant 4
// gives: a bag of JSON is cargo nobody can query, and anything a client is
// meant to read should be a column. They are optional and omitted when unset;
// when there are more than a handful, this splits into one type per code.
type Error struct {
	Code    string `json:"code"`
	Class   string `json:"class"`
	Message string `json:"message"`

	// BlockingOp is the op standing in the way of an undo (undo_blocked).
	BlockingOp int64 `json:"blocking_op,omitempty"`
	// RevertedBy is the op that already undid this one (already_reverted).
	RevertedBy int64 `json:"reverted_by,omitempty"`
	// Candidates are what an ambiguous reference matched (ambiguous_ref).
	Candidates []Ref `json:"candidates,omitempty"`
	// ExistingID holds the name already taken (tag_name_taken).
	ExistingID string `json:"existing_id,omitempty"`
	// Holder describes the process holding the write lock (library_busy). It
	// is diagnostic only: correctness comes from the kernel's lock, never from
	// this text (spec 2.7).
	Holder string `json:"holder,omitempty"`
}

// Ref names one thing a reference could have meant.
type Ref struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
