package main

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/jinzhao1994/kist/internal/errcodes"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/lockfile"
	"github.com/jinzhao1994/kist/internal/wire"
)

// exitTempFail is sysexits.h's EX_TEMPFAIL: "the operation failed, but it was
// not a permanent failure — try again".
//
// It is the one class that gets its own exit code, because it is the one a
// cron job can act on without parsing anything: a `kist import` that lost a
// race for the write lock should be retried, and one that was given a bad name
// should not (spec 2.9).
const exitTempFail = 75

// exitCodeFor turns an error into an exit status.
//
// Everything except a temporary failure is 1. verify, scrub and repair use 1
// and 2 for something else entirely — how bad what they *found* was — and
// those are deliberately not merged: a command that ran fine and found your
// passport scan damaged has to be distinguishable from a command that could
// not run.
func exitCodeFor(err error) int {
	var coder exitCoder
	if errors.As(err, &coder) {
		return coder.ExitCode()
	}
	if errcodes.ClassOf(err) == errcodes.ClassUnavailable {
		return exitTempFail
	}
	return 1
}

// renderError builds the wire form, filling in whatever fields this particular
// failure carries.
func renderError(err error) wire.Error {
	out := wire.Error{
		Code:    string(errcodes.Of(err)),
		Class:   errcodes.ClassOf(err).String(),
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

// writeError puts a failure where the caller can find it.
//
// In --json mode it goes to stderr and stdout stays empty. That is what makes
// `kist find --json > out.json` safe: the file is either a result or nothing,
// never an error object waiting to be parsed as data (spec 2.9).
func writeError(w io.Writer, err error) {
	if !jsonFlag {
		io.WriteString(w, "kist: "+err.Error()+"\n")
		return
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(renderError(err))
}
