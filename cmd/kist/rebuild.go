package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/cliutil"
	"github.com/jinzhao1994/kist/internal/wire"
)

func newRebuildCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Throw away the derived tables and replay the operation log",
		Long: `Rebuild discards every table kist derives — boxes, folders, files, tags — and
rebuilds them by replaying the operation log from the beginning.

That is safe by construction: the log and the content store are the authority,
and everything else exists to make them fast to query. It is what to run when
"kist verify --deep" reports that a table no longer matches the log.

It relinks boxes/ afterwards, because those tables are what boxes/ is built
from.

Replaying takes minutes on a large library, so it asks first.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			ops, err := l.OpCount(cmd.Context())
			if err != nil {
				return err
			}

			if !yes {
				p := cliutil.NewPrompter(cmd.InOrStdin(), cmd.OutOrStdout(), false)
				p.Printf("This replays %d operations and rebuilds every derived table.\n", ops)
				p.Printf("The operation log and the content store are not touched.\n\n")
				ok, err := p.Confirm("Rebuild now?", true)
				if err != nil {
					return err
				}
				if !ok {
					return emit(cmd, wire.RebuildResult{Ops: ops}, func(w io.Writer) error {
						_, err := fmt.Fprintln(w, "nothing was changed")
						return err
					})
				}
			}

			wasDirty := l.ProjectionDirty()
			if err := l.Rebuild(cmd.Context()); err != nil {
				return err
			}

			// The tables boxes/ is projected from were just rebuilt, so the
			// projection has to follow. A rebuild that leaves boxes/ wrong is
			// a command people learn not to trust.
			stats, err := l.Relink(cmd.Context())
			if err != nil {
				return err
			}

			out := wire.RebuildResult{
				Confirmed: true,
				Ops:       ops,
				Relink:    relinkResult(stats, wasDirty),
			}
			return emit(cmd, out, func(w io.Writer) error {
				fmt.Fprintf(w, "replayed %d operations\n", ops)
				fmt.Fprintf(w, "relinked %d boxes: created %d, removed %d\n",
					stats.Boxes, stats.Created, stats.Removed)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "do not ask before rebuilding")
	return cmd
}
