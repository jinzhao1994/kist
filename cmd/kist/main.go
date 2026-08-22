// Command kist is the kist CLI.
//
// M0 ships only "version" — enough to prove the single-binary build works on
// every target platform. The real commands (init, import, find, ...) land in M1.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "kist:", err)
		os.Exit(1)
	}
}
