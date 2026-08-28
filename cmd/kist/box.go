package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func newBoxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "box",
		Short: "Create and edit boxes",
		Long: `A box is one activity, one trip, one batch of paperwork — the unit
everything else hangs off.`,
	}
	cmd.AddCommand(newBoxNewCmd(), newBoxListCmd(), newBoxEditCmd(), newBoxTagCmd(), newBoxUntagCmd())
	return cmd
}

// parseDate turns a flag value into a variable-precision date. An empty string
// means "not set", which is a legitimate state (spec 1.3).
func parseDate(s string) (domain.LocalDate, error) {
	if s == "" {
		return domain.LocalDate{}, nil
	}
	return domain.ParseLocalDate(s)
}

func newBoxNewCmd() *cobra.Command {
	var start, end, description string

	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a box",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			startAt, err := parseDate(start)
			if err != nil {
				return err
			}
			endAt, err := parseDate(end)
			if err != nil {
				return err
			}

			b, err := l.CreateBox(cmd.Context(), args[0], description, startAt, endAt)
			if err != nil {
				return err
			}
			return emit(cmd, wire.BoxResult{Box: wire.NewBox(b)}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s  %s\n", b.ID, b.Name)
				return err
			})
		},
	}
	cmd.Flags().StringVar(&start, "start", "", "when it started: 2019, 2019-08, 2019-08-12, or 2019-08-12T14:30")
	cmd.Flags().StringVar(&end, "end", "", "when it ended, same forms as --start")
	cmd.Flags().StringVar(&description, "description", "", "a note about this box")
	return cmd
}

func newBoxListCmd() *cobra.Command {
	var byName bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List boxes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			boxes, err := l.Boxes(cmd.Context(), library.ListBoxesOptions{SortByName: byName})
			if err != nil {
				return err
			}
			return emit(cmd, wire.BoxList{Boxes: wire.NewBoxes(boxes)}, func(w io.Writer) error {
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				for _, b := range boxes {
					when := b.StartAt.String()
					if when == "" {
						when = "-"
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\n", b.ID, when, b.Name)
				}
				return tw.Flush()
			})
		},
	}
	// Default is most-recently-active first; --sort-name is byte order
	// (spec 4.8).
	cmd.Flags().BoolVar(&byName, "sort-name", false, "sort by name instead of recent activity")
	return cmd
}

func newBoxEditCmd() *cobra.Command {
	var name, start, end, description string

	cmd := &cobra.Command{
		Use:   "edit <box>",
		Short: "Change a box's name, dates or note",
		Long:  "<box> is an id, an id prefix, or an exact name.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			b, err := l.ResolveBox(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			// Unset flags keep the current value, so editing one field does
			// not quietly blank the others.
			fields := library.BoxFields{
				Name: b.Name, Description: b.Description, StartAt: b.StartAt, EndAt: b.EndAt,
			}
			if cmd.Flags().Changed("name") {
				fields.Name = name
			}
			if cmd.Flags().Changed("description") {
				fields.Description = description
			}
			if cmd.Flags().Changed("start") {
				if fields.StartAt, err = parseDate(start); err != nil {
					return err
				}
			}
			if cmd.Flags().Changed("end") {
				if fields.EndAt, err = parseDate(end); err != nil {
					return err
				}
			}

			updated, err := l.UpdateBox(cmd.Context(), b.ID, fields)
			if err != nil {
				return err
			}
			return emit(cmd, wire.BoxResult{Box: wire.NewBox(updated)}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s  %s\n", updated.ID, updated.Name)
				return err
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new name")
	cmd.Flags().StringVar(&start, "start", "", "new start date, or empty to clear")
	cmd.Flags().StringVar(&end, "end", "", "new end date, or empty to clear")
	cmd.Flags().StringVar(&description, "description", "", "new note")
	return cmd
}

func newBoxTagCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tag <box> <tag>...",
		Short: "Attach tags to a box",
		Args:  cobra.MinimumNArgs(2),
		RunE:  func(cmd *cobra.Command, args []string) error { return runBoxTag(cmd, args, true) },
	}
}

func newBoxUntagCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "untag <box> <tag>...",
		Short: "Remove tags from a box",
		Args:  cobra.MinimumNArgs(2),
		RunE:  func(cmd *cobra.Command, args []string) error { return runBoxTag(cmd, args, false) },
	}
}

func runBoxTag(cmd *cobra.Command, args []string, attach bool) error {
	l, err := openLibrary(cmd)
	if err != nil {
		return err
	}
	defer l.Close()

	b, err := l.ResolveBox(cmd.Context(), args[0])
	if err != nil {
		return err
	}

	// One op per tag: a batch op carries one tag and many boxes (spec 2.2.1),
	// so tagging one box with three tags is three ops. The CLI shape here is
	// the mirror image of the batching the data model is built for; the web UI
	// will use the other direction.
	box := wire.NewBox(b)
	out := wire.TagChanges{Box: &box, Attach: attach}
	names := make([]string, 0, len(args)-1)
	for _, ref := range args[1:] {
		tag, err := l.ResolveTag(ref)
		if err != nil {
			return err
		}
		var n int
		if attach {
			n, err = l.TagBoxes(cmd.Context(), tag.ID, []domain.BoxID{b.ID})
		} else {
			n, err = l.UntagBoxes(cmd.Context(), tag.ID, []domain.BoxID{b.ID})
		}
		if err != nil {
			return err
		}
		out.Changes = append(out.Changes, wire.TagChange{Tag: wire.NewTag(tag), Changed: n})
		names = append(names, tag.Name)
	}

	return emit(cmd, out, func(w io.Writer) error {
		for i, c := range out.Changes {
			var verb string
			switch {
			case c.Changed > 0 && attach:
				verb = "tagged"
			case c.Changed > 0:
				verb = "untagged"
			case attach:
				verb = "already had"
			default:
				verb = "did not have"
			}
			if _, err := fmt.Fprintf(w, "%s %s %s\n", b.Name, verb, names[i]); err != nil {
				return err
			}
		}
		return nil
	})
}
