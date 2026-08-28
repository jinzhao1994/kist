package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/wire"
)

func newTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage the tag tree",
		Long: `Tags cut across boxes. Names are unique across the whole tree, so a
tag can be referred to by name from anywhere.`,
	}
	cmd.AddCommand(newTagNewCmd(), newTagTreeCmd(), newTagRenameCmd(), newTagMoveCmd(), newTagRemoveCmd())
	return cmd
}

func newTagNewCmd() *cobra.Command {
	var parent string

	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a tag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			var parentID *domain.TagID
			if parent != "" {
				p, err := l.ResolveTag(parent)
				if err != nil {
					return err
				}
				parentID = &p.ID
			}

			tag, err := l.CreateTag(cmd.Context(), args[0], parentID)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", tag.ID, tag.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&parent, "parent", "", "put it under this tag")
	return cmd
}

func newTagTreeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tree",
		Short: "Show the tag tree",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			// Printing the whole tree is the one thing that really wants the
			// whole tree; everything else asks about one tag or one subtree
			// and goes to the database for it (spec 1.6).
			tree, totals, err := l.TagTree()
			if err != nil {
				return err
			}
			// The count is the subtree total, which is what a tag selector
			// shows next to a node (spec 1.6).
			var build func(tags []domain.Tag) []wire.Tag
			build = func(tags []domain.Tag) []wire.Tag {
				out := make([]wire.Tag, len(tags))
				for i, tag := range tags {
					out[i] = wire.NewTag(tag, totals[tag.ID])
					out[i].Children = build(tree.Children(tag.ID))
				}
				return out
			}
			out := wire.TagTree{Tags: build(tree.Roots())}

			return emit(cmd, out, func(w io.Writer) error {
				var print func(tags []wire.Tag, depth int)
				print = func(tags []wire.Tag, depth int) {
					for _, tag := range tags {
						fmt.Fprintf(w, "%s%s (%d)\n",
							strings.Repeat("  ", depth), tag.Name, tag.Count)
						print(tag.Children, depth+1)
					}
				}
				print(out.Tags, 0)
				return nil
			})
		},
	}
}

func newTagRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <tag> <new-name>",
		Short: "Rename a tag",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			tag, err := l.ResolveTag(args[0])
			if err != nil {
				return err
			}
			if err := l.RenameTag(cmd.Context(), tag.ID, args[1]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s is now %s\n", tag.Name, args[1])
			return nil
		},
	}
}

func newTagMoveCmd() *cobra.Command {
	var parent string

	cmd := &cobra.Command{
		Use:   "move <tag> --parent <tag>",
		Short: "Move a tag under a different parent",
		Long:  `Pass --parent "" to move it to the top level.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("parent") {
				return fmt.Errorf(`--parent is required; pass --parent "" to move it to the top level`)
			}
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			tag, err := l.ResolveTag(args[0])
			if err != nil {
				return err
			}
			var parentID *domain.TagID
			if parent != "" {
				p, err := l.ResolveTag(parent)
				if err != nil {
					return err
				}
				parentID = &p.ID
			}
			if err := l.MoveTag(cmd.Context(), tag.ID, parentID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "moved %s\n", tag.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&parent, "parent", "", `the new parent, or "" for the top level`)
	return cmd
}

func newTagRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <tag>",
		Short: "Delete a tag",
		Long: `Only tags with no children and nothing attached can be deleted.
kist does not cascade: losing a whole branch of your classification to one
mistyped command is not a recoverable kind of mistake.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			tag, err := l.ResolveTag(args[0])
			if err != nil {
				return err
			}
			if err := l.DeleteTag(cmd.Context(), tag.ID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", tag.Name)
			return nil
		},
	}
}
