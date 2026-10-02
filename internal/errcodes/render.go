package errcodes

import (
	"errors"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/lockfile"
	"github.com/jinzhao1994/kist/internal/wire"
)

// Render converts a kernel error to the shared CLI and HTTP wire contract.
func Render(err error) wire.Error {
	out := wire.Error{
		Code:    string(Of(err)),
		Class:   ClassOf(err).String(),
		Message: err.Error(),
	}

	var outcome *library.WriteOutcomeError
	if errors.As(err, &outcome) {
		out.OpID = outcome.OpID
	}
	var blocked *library.BlockedUndoError
	if errors.As(err, &blocked) {
		out.BlockingOp = blocked.BlockingOp
	}
	var reverted *library.AlreadyRevertedError
	if errors.As(err, &reverted) {
		out.RevertedBy = reverted.RevertedBy
	}
	var ambiguous *library.AmbiguousRefError
	if errors.As(err, &ambiguous) {
		out.Candidates = make([]wire.Ref, len(ambiguous.Candidates))
		for i, c := range ambiguous.Candidates {
			out.Candidates[i] = wire.Ref{ID: c.ID, Name: c.Name}
		}
	}
	var taken *library.NameTakenError
	if errors.As(err, &taken) {
		out.ExistingID = taken.ExistingID
	}
	var busy *lockfile.BusyError
	if errors.As(err, &busy) {
		out.Holder = busy.Holder
	}
	return out
}
