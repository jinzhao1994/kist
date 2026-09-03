package main

import (
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func newLogCmd() *cobra.Command {
	var (
		limit int
		box   string
	)

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show what has happened to this library",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			opt := library.ListOpsOptions{Limit: limit}
			if box != "" {
				b, err := l.ResolveBox(cmd.Context(), box)
				if err != nil {
					return err
				}
				opt.Box = &b.ID
			}

			ops, err := l.Ops(cmd.Context(), opt)
			if err != nil {
				return err
			}

			out := wire.OpList{Ops: make([]wire.Op, len(ops))}
			for i, op := range ops {
				// Marking reverted ops is what reverts_id being an indexed
				// column is for (spec 2.2).
				row := wire.Op{
					ID:        op.ID,
					TS:        op.TS,
					Action:    string(op.Action),
					Origin:    string(op.Origin),
					RevertsID: op.RevertsID,
				}
				if by, ok, err := l.RevertedBy(cmd.Context(), op.ID); err != nil {
					return err
				} else if ok {
					row.UndoneBy = &by
				}
				out.Ops[i] = row
			}

			return emit(cmd, out, func(w io.Writer) error {
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				for _, op := range out.Ops {
					status := ""
					if op.UndoneBy != nil {
						status = fmt.Sprintf("(undone by %d)", *op.UndoneBy)
					}
					if op.RevertsID != nil {
						status = fmt.Sprintf("(undoes %d)", *op.RevertsID)
					}
					fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
						op.ID, op.TS.Format("2006-01-02 15:04:05"), op.Action, op.Origin, status)
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "how many ops to show")
	cmd.Flags().StringVar(&box, "box", "", "only ops that touched this box")
	return cmd
}

func newUndoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undo <op-id>",
		Short: "Undo an operation",
		Long: `Undo appends the inverse of an operation; it does not erase anything.
The undo is itself an operation and can be undone in turn.

If later changes touched the same things, the undo is refused rather than
partly applied, and the operation in the way is named.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("%q is not an operation id; `kist log` lists them", args[0])
			}
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			undoID, err := l.Undo(cmd.Context(), opID)
			if err != nil {
				return err
			}
			out := wire.UndoResult{Undone: opID, UndoneBy: undoID}
			return emit(cmd, out, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "op %d undone by op %d\n", opID, undoID)
				return err
			})
		},
	}
}
