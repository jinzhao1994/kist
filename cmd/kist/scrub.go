package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/library"
)

// scrubStopMessage is what the first Ctrl-C prints. A scrub has no work to
// commit, but it does have findings, and the report is printed either way.
const scrubStopMessage = "stopping — what has been checked so far will still be reported; press Ctrl-C again to quit"

func newScrubCmd() *cobra.Command {
	var quiet bool

	cmd := &cobra.Command{
		Use:   "scrub",
		Short: "Re-hash every file in the content store and report what has rotted",
		Long: `Scrub reads every byte the library holds and checks it still hashes to its own
name. That is how bit rot and bad sectors are found: they leave the file the
right size, so "kist verify" cannot see them.

It reads the whole archive, so it takes as long as that sounds. Ctrl-C stops
it and still prints what was found, and the report says plainly that the run
did not finish — "nothing wrong" and "nothing wrong so far" are different
answers.

Nothing is repaired and nothing is deleted, damaged content included: it may
be the only copy left. "kist repair" fetches good copies from a remote.

Exit status:

  0  everything read back correctly
  1  the run did not finish
  2  something is damaged`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openForChecks(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			sig := make(chan os.Signal, 2)
			signal.Notify(sig, os.Interrupt)
			defer signal.Stop(sig)
			ctx, cancel := interruptContext(cmd.Context(), sig, scrubStopMessage,
				func(msg string) { fmt.Fprintln(cmd.ErrOrStderr(), msg) },
				os.Exit)
			defer cancel()

			progress := scrubProgress(cmd, quiet)
			r, err := l.Scrub(ctx, progress)
			if err != nil {
				return err
			}
			if err := emit(cmd, report(r), r.Write); err != nil {
				return err
			}
			return verifyExit(r.Worst())
		},
	}
	cmd.Flags().BoolVar(&quiet, "quiet", false, "do not print progress")
	return cmd
}

// scrubProgress writes a single self-overwriting line to stderr.
//
// stderr rather than stdout, so that piping the report somewhere does not
// collect a thousand progress lines with it. The carriage return is only used
// when stderr is a terminal; redirected, it degrades to a line every so often,
// because a log file full of \r is unreadable.
func scrubProgress(cmd *cobra.Command, quiet bool) func(library.ScrubProgress) {
	if quiet {
		return nil
	}
	w := cmd.ErrOrStderr()
	tty := isTerminal(w)
	return func(p library.ScrubProgress) {
		switch {
		case tty:
			fmt.Fprintf(w, "\rchecked %d/%d", p.Done, p.Total)
			if p.Done == p.Total {
				fmt.Fprintln(w)
			}
		case p.Done == p.Total || p.Done%1000 == 0:
			fmt.Fprintf(w, "checked %d/%d\n", p.Done, p.Total)
		}
	}
}

// isTerminal reports whether w is a character device. It is deliberately
// minimal: getting this wrong costs a slightly uglier log, so it is not worth
// a dependency.
func isTerminal(w any) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
