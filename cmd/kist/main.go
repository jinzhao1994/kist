// Command kist is the kist CLI.
package main

import (
	"errors"
	"fmt"
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
	fmt.Fprintln(os.Stderr, "kist:", err)

	var coder exitCoder
	if errors.As(err, &coder) {
		os.Exit(coder.ExitCode())
	}
	os.Exit(1)
}
