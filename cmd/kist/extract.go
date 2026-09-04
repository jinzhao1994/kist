package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

const extractStopMessage = "stopping — the batches already recorded will be kept; press Ctrl-C again to discard them"

func newExtractCmd() *cobra.Command {
	var (
		extractor string
		force     bool
		quiet     bool
	)

	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Read metadata out of files and record what was found",
		Long: `extract reads each file and records what it observed: EXIF for photographs,
and the file's own size and modification time for everything else.

It also works out each file's capture time, which is what "find --sort captured"
orders by. That is kist's own conclusion rather than any single tag: it prefers
EXIF's DateTimeOriginal, falls back to the file's modification time, and needs a
timezone to turn a camera's wall clock into a moment. If the file carries one,
it is used; otherwise the library's configured timezone is assumed, and
"kist file show" says so rather than pretending.

Files are processed in batches, and each batch is one operation you can undo.
Ctrl-C stops after the batch in flight.

Re-running is cheap. A file is only read again when an extractor's version has
changed, so an ordinary re-run over an unchanged library does nothing at all.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			sig := make(chan os.Signal, 2)
			signal.Notify(sig, os.Interrupt)
			defer signal.Stop(sig)
			ctx, cancel := interruptContext(cmd.Context(), sig, extractStopMessage,
				func(msg string) { fmt.Fprintln(cmd.ErrOrStderr(), msg) },
				os.Exit)
			defer cancel()

			var progress func(int)
			if !quiet {
				progress = func(done int) {
					fmt.Fprintf(cmd.ErrOrStderr(), "\r%d files", done)
				}
			}
			res, err := l.Extract(ctx, library.ExtractOptions{Extractor: extractor, Force: force}, progress)
			if err != nil {
				return err
			}
			if !quiet {
				fmt.Fprintln(cmd.ErrOrStderr())
			}
			out := wire.ExtractResult{
				OpIDs:       res.OpIDs,
				Extracted:   res.Count(library.FileImported),
				Skipped:     res.Count(library.FileSkipped),
				Failed:      res.Count(library.FileFailed),
				Interrupted: res.Interrupted,
				Files:       make([]wire.FileOutcome, len(res.Files)),
			}
			for i, f := range res.Files {
				out.Files[i] = wire.FileOutcome{Name: f.Name, Status: f.Status.String()}
				if f.Err != nil {
					out.Files[i].Error = f.Err.Error()
				}
			}
			if err := emit(cmd, out, func(w io.Writer) error {
				printExtractSummary(w, res)
				return nil
			}); err != nil {
				return err
			}

			if res.Count(library.FileFailed) > 0 {
				return errSomeFilesFailed
			}
			if res.Interrupted {
				return errInterrupted
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&extractor, "extractor", "", "run only this extractor")
	cmd.Flags().BoolVar(&force, "force", false, "re-read every file, even ones already at the current version")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "no progress output")
	return cmd
}

// printExtractSummary reports what happened, in the same shape import uses:
// a count per outcome, and every failure named (spec 3.7.1 ③).
func printExtractSummary(w io.Writer, res *library.ExtractResult) {
	fmt.Fprintf(w, "extracted %d, skipped %d, %d operations\n",
		res.Count(library.FileImported), res.Count(library.FileSkipped), len(res.OpIDs))

	failed := res.Count(library.FileFailed)
	if failed == 0 {
		return
	}
	fmt.Fprintf(w, "\nfailed: %d\n", failed)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, f := range res.Files {
		if f.Status == library.FileFailed {
			fmt.Fprintf(tw, "  %s\t%v\n", f.Name, f.Err)
		}
	}
	tw.Flush()
}

func newFileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <file-id>",
		Short: "Show everything known about one file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			raw, err := domain.ParseID(args[0])
			if err != nil {
				return fmt.Errorf("%q is not a file id; `kist find` prints them", args[0])
			}
			id := domain.FileID{ID: raw}
			file, ok, err := l.File(cmd.Context(), id)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("no file with id %s", args[0])
			}
			box, path, err := l.FileLocation(cmd.Context(), file)
			if err != nil {
				return err
			}
			all, err := l.FileMetadata(cmd.Context(), id)
			if err != nil {
				return err
			}
			// Tombstones are how the library remembers a key you removed; the
			// point of removing it is not to see it again.
			rows := make([]extract.Row, 0, len(all))
			for _, r := range all {
				if !r.Tombstone {
					rows = append(rows, r)
				}
			}
			info, err := l.CapturedInfo(cmd.Context(), file)
			if err != nil {
				return err
			}

			out := wire.FileDetail{
				File:         wire.NewFile(file, box.String(), path, l.Zone()),
				CapturedFrom: info.From,
				CapturedZone: info.Zone,
				Extracted:    info.Extracted,
				Metadata:     make([]wire.Metadata, len(rows)),
			}
			for i, r := range rows {
				out.Metadata[i] = wire.Metadata{
					Key: r.Key, Value: r.Value, Type: string(r.Type), Source: r.Source,
				}
			}

			return emit(cmd, out, func(w io.Writer) error {
				fmt.Fprintf(w, "%s\n%s\n\n", file.ID, path)
				l.DescribeCapturedAt(cmd.Context(), w, file)

				if len(rows) == 0 {
					fmt.Fprintln(w, "\nno metadata — nothing this build can read out of this file")
					return nil
				}
				fmt.Fprintln(w)
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				for _, r := range rows {
					source := r.Source
					if r.Source == extract.UserSource {
						source = "you"
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Key, r.Value, source)
				}
				return tw.Flush()
			})
		},
	}
}
