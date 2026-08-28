package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func newRelinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "relink",
		Short: "Rebuild boxes/ from the database",
		Long: `boxes/ is a projection: a human-readable mirror of what the database
says, built out of links rather than copies. Nothing in it is data.

relink makes it match again. It is safe to run at any time, it only creates
links, and running it twice changes nothing the second time — so it is the
answer whenever boxes/ looks wrong, including after it has been edited or
deleted by hand.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			wasDirty := l.ProjectionDirty()
			stats, err := l.Relink(cmd.Context())
			if err != nil {
				return err
			}
			return emit(cmd, relinkResult(stats, wasDirty), func(w io.Writer) error {
				fmt.Fprintf(w, "%d box(es): created %d, removed %d\n",
					stats.Boxes, stats.Created, stats.Removed)
				if stats.Missing > 0 {
					fmt.Fprintf(w,
						"%d file(s) could not be linked: their content is not on local disk\n", stats.Missing)
				}
				if stats.Failed > 0 {
					fmt.Fprintf(w, "%d file(s) failed; see the log above\n", stats.Failed)
				}
				if wasDirty {
					fmt.Fprintln(w, "the stale-projection warning is cleared")
				}
				return nil
			})
		},
	}
}

// relinkResult is shared with rebuild, which relinks as its last step.
func relinkResult(stats library.RelinkStats, wasDirty bool) wire.RelinkResult {
	return wire.RelinkResult{
		Boxes:    stats.Boxes,
		Created:  stats.Created,
		Removed:  stats.Removed,
		Missing:  stats.Missing,
		Failed:   stats.Failed,
		WasDirty: wasDirty,
	}
}
