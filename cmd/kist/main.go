// Command kist is the kist CLI.
package main

import (
	"os"
)

// exitCoder is an error that names the status to exit with. verify uses it to
// separate "the mirror is stale" from "your content is damaged", which is the
// distinction a cron job needs to act on.
type exitCoder interface{ ExitCode() int }

func main() {
	err := newRootCmd().Execute()
	if err == nil {
		return
	}
	writeError(os.Stderr, err)
	os.Exit(exitCodeFor(err))
}
