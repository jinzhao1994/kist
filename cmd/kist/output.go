package main

import (
	"encoding/json"
	"io"

	"github.com/spf13/cobra"
)

// jsonFlag is the value of --json, shared by every command that prints.
var jsonFlag bool

// emit renders a command's result, either as JSON or for a person.
//
// One flag on the root rather than a flag each command has to remember: a
// --json that some commands have and others do not is worse than none, because
// a script cannot tell the difference between "this command has no JSON form"
// and "you spelled the flag wrong" (spec 2.8).
//
// human writes to the given writer instead of taking the command, so that a
// renderer cannot accidentally print in JSON mode by reaching for
// cmd.OutOrStdout() itself.
func emit(cmd *cobra.Command, v any, human func(io.Writer) error) error {
	if !jsonFlag {
		return human(cmd.OutOrStdout())
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	// Indented: jq reformats anyway, and the other reader is a person piping
	// into less.
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
