package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

// importStopMessage is what the first Ctrl-C prints. It has to say that the
// finished files are kept, because the whole point of spec 3.7.1 ① is that
// stopping an import does not throw away what it already did — and a user who
// does not know that will re-run from scratch.
const importStopMessage = "stopping — the files already imported will be kept; press Ctrl-C again to discard them"

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
			ctx, cancel := interruptContext(cmd.Context(), sig, importStopMessage,
				func(msg string) { fmt.Fprintln(cmd.ErrOrStderr(), msg) },
				os.Exit)
			defer cancel()

			// Collected rather than printed as they go: --json has to produce
			// one document, and an import of three directories is three ops.
			// A person still wants each summary, so the human renderer walks
			// the same list.
			var (
				out         wire.ImportResults
				results     []*library.ImportResult
				failed      bool
				interrupted bool
			)
			for _, src := range args {
				res, err := l.Import(ctx, src, opt)
				if err != nil {
					return err
				}
				// One group per invocation, not per path: `kist import a/ b/ c/`
				// is three ops today, and a chunked import will make even one
				// path several. The first op that actually gets written heads
				// the group — a path that imported nothing writes no op, so
				// which one that is cannot be decided up front.
				if opt.RootOp == nil && res.OpID != 0 {
					root := res.OpID
					opt.RootOp = &root
				}
				out.Imports = append(out.Imports, importResult(src, res))
				results = append(results, res)
				if res.Count(library.FileFailed) > 0 {
					failed = true
				}
				if res.Interrupted {
					interrupted = true
					break
				}
			}

			err = emit(cmd, out, func(w io.Writer) error {
				for i, res := range results {
					printImportSummary(w, args[i], res)
				}
				return nil
			})
			if err != nil {
				return err
			}

			// Stopping is still not finishing, so a script has to be able to
			// tell — but it is a different thing from a file having gone
			// wrong, and it gets a different message.
			if failed {
				// A script has to be able to notice (spec 3.7.1 ③).
				return errSomeFilesFailed
			}
			if interrupted {
				return errInterrupted
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

// errInterrupted marks a run the user stopped. Non-zero so a script notices,
// but not phrased as a failure: nothing went wrong.
var errInterrupted = &quietError{"stopped before finishing"}

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

// importResult renders one source's outcome.
func importResult(src string, res *library.ImportResult) wire.ImportResult {
	out := wire.ImportResult{
		Source:      src,
		OpID:        res.OpID,
		Imported:    res.Count(library.FileImported),
		Skipped:     res.Count(library.FileSkipped),
		Failed:      res.Count(library.FileFailed),
		Interrupted: res.Interrupted,
		Remaining:   res.Remaining,
		Files:       make([]wire.FileOutcome, len(res.Files)),
	}
	for i, f := range res.Files {
		out.Files[i] = wire.FileOutcome{
			Source:  f.Source,
			Path:    f.Rel,
			Name:    f.Name,
			Status:  f.Status.String(),
			Outcome: f.Outcome.String(),
		}
		if f.Err != nil {
			out.Files[i].Error = f.Err.Error()
		}
	}
	return out
}
