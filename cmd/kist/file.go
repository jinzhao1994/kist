package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/domain"
)

func newFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Work with individual files",
	}
	cmd.AddCommand(newFileTagCmd(false), newFileTagCmd(true))
	return cmd
}

// newFileTagCmd builds `file tag` and `file untag`, which differ only in
// direction.
func newFileTagCmd(remove bool) *cobra.Command {
	use, short := "tag <tag> <file-id>...", "Attach a tag to files"
	if remove {
		use, short = "untag <tag> <file-id>...", "Remove a tag from files"
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long: `File ids come from ` + "`kist find`" + `, which prints one per line.

Tagging many files is a single operation, so undoing it is a single undo.
Files that would not change are left out of that operation entirely.`,
		Args: cobra.MinimumNArgs(2),
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
			files := make([]domain.FileID, 0, len(args)-1)
			for _, ref := range args[1:] {
				id, err := domain.ParseID(ref)
				if err != nil {
					return fmt.Errorf("%q is not a file id; `kist find` prints them", ref)
				}
				files = append(files, domain.FileID{ID: id})
			}

			var n int
			if remove {
				n, err = l.UntagFiles(cmd.Context(), tag.ID, files)
			} else {
				n, err = l.TagFiles(cmd.Context(), tag.ID, files)
			}
			if err != nil {
				return err
			}
			verb := "tagged"
			if remove {
				verb = "untagged"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d file(s) with %s\n", verb, n, tag.Name)
			return nil
		},
	}
}
