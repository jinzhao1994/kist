package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/library"
)

// interruptExitCode is the conventional 128+SIGINT.
const interruptExitCode = 130

// interruptContext turns the first SIGINT into a graceful stop and the second
// into an exit.
//
// The policy is spec 3.7.1 ①: the first Ctrl-C lets the files already imported
// commit as one op, and only the second gives up on them. library knows
// nothing about signals — it just watches the context — which is what lets
// this be tested by pushing values into a channel instead of sending a real
// signal at the test binary.
func interruptContext(
	parent context.Context,
	sig <-chan os.Signal,
	notify func(string),
	exit func(int),
) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		first := false
		for {
			select {
			case <-done:
				return
			case _, ok := <-sig:
				if !ok {
					return
				}
				if !first {
					first = true
					notify("stopping — the files already imported will be kept; press Ctrl-C again to discard them")
					cancel()
					continue
				}
				exit(interruptExitCode)
				return
			}
		}
	}()
	return ctx, func() {
		close(done)
		cancel()
	}
}

func newImportCmd() *cobra.Command {
	var (
		box    string
		into   string
		hidden bool
	)

	cmd := &cobra.Command{
		Use:   "import <path>...",
		Short: "Put files into a box",
		Long: `Import copies files into the library's content store and records them in a box.

The box has to exist first: import never invents one, and never guesses which
one you meant.

Ctrl-C stops after the file in flight; everything imported up to that point is
kept as a single operation you can undo. Press it twice to discard that too.

Re-running the same import is safe. A file already in the target folder with
the same name and the same content is skipped, so an interrupted import
converges instead of accumulating copies.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			b, err := l.ResolveBox(cmd.Context(), box)
			if err != nil {
				return err
			}
			opt := library.ImportOptions{Box: b.ID, Hidden: hidden}
			if into != "" {
				f, err := l.ResolveFolder(cmd.Context(), b.ID, into)
				if err != nil {
					return err
				}
				opt.Into = f.ID
			}

			sig := make(chan os.Signal, 2)
			signal.Notify(sig, os.Interrupt)
			defer signal.Stop(sig)
			ctx, cancel := interruptContext(cmd.Context(), sig,
				func(msg string) { fmt.Fprintln(cmd.ErrOrStderr(), msg) },
				os.Exit)
			defer cancel()

			failed := false
			for _, src := range args {
				res, err := l.Import(ctx, src, opt)
				if err != nil {
					return err
				}
				printImportSummary(cmd.OutOrStdout(), src, res)
				if res.Count(library.FileFailed) > 0 {
					failed = true
				}
				if res.Interrupted {
					break
				}
			}
			if failed {
				// A script has to be able to notice (spec 3.7.1 ③).
				return errSomeFilesFailed
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&box, "box", "", "the box to import into (id or name)")
	cmd.Flags().StringVar(&into, "into", "", "a folder inside the box, as a path")
	cmd.Flags().BoolVar(&hidden, "hidden", false, "include files and directories whose name starts with a dot")
	cmd.MarkFlagRequired("box")
	return cmd
}

// errSomeFilesFailed makes the exit code non-zero without printing a second
// message: the per-file report above already said what went wrong.
var errSomeFilesFailed = &quietError{"some files were not imported"}

// quietError is an error main prints as-is, with no "kist:" framing, because
// the command has already explained itself.
type quietError struct{ msg string }

func (e *quietError) Error() string { return e.msg }

// printImportSummary reports what happened, per file.
//
// Skipped and failed are kept apart on purpose. A re-run after an interrupted
// import is almost entirely skips, and that is success — it must not read like
// failure (spec 3.7.1 ②). A failure, on the other hand, has to be named: a
// summary that says only "done" makes a partial failure invisible, which is
// the one thing per-file results exist to prevent.
func printImportSummary(w io.Writer, src string, res *library.ImportResult) {
	imported := res.Count(library.FileImported)
	skipped := res.Count(library.FileSkipped)
	failed := res.Count(library.FileFailed)

	fmt.Fprintf(w, "%s: imported %d, skipped %d, failed %d\n", src, imported, skipped, failed)

	if failed > 0 {
		fmt.Fprintln(w, "\nfailed:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, f := range res.Files {
			if f.Status == library.FileFailed {
				fmt.Fprintf(tw, "  %s\t%v\n", f.Source, f.Err)
			}
		}
		tw.Flush()
	}
	if res.Interrupted {
		fmt.Fprintf(w, "\nstopped early: %d files were not looked at. "+
			"Everything above is committed and can be undone as one operation.\n", res.Remaining)
	}
}
