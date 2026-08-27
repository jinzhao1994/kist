package main

import (
	"fmt"

	"github.com/spf13/cobra"
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
			fmt.Fprintf(cmd.OutOrStdout(), "%d box(es): created %d, removed %d\n",
				stats.Boxes, stats.Created, stats.Removed)
			if stats.Missing > 0 {
				fmt.Fprintf(cmd.OutOrStdout(),
					"%d file(s) could not be linked: their content is not on local disk\n", stats.Missing)
			}
			if stats.Failed > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%d file(s) failed; see the log above\n", stats.Failed)
			}
			if wasDirty {
				fmt.Fprintln(cmd.OutOrStdout(), "the stale-projection warning is cleared")
			}
			return nil
		},
	}
}
