package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/wire"
)

func newFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Work with individual files",
	}
	cmd.AddCommand(newFileTagCmd(false), newFileTagCmd(true), newFileShowCmd(), newFileMetadataCmd())
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
			out := wire.TagChanges{
				Attach:  !remove,
				Changes: []wire.TagChange{{Tag: wire.NewTag(tag), Changed: n}},
			}
			return emit(cmd, out, func(w io.Writer) error {
				verb := "tagged"
				if remove {
					verb = "untagged"
				}
				_, err := fmt.Fprintf(w, "%s %d file(s) with %s\n", verb, n, tag.Name)
				return err
			})
		},
	}
}

// newFileMetadataCmd groups the commands that edit stored metadata. Only `rm`
// exists in v0.1; writing `user:` values is v0.2 (spec 1.7).
func newFileMetadataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metadata",
		Short: "Work with a file's stored metadata",
	}
	cmd.AddCommand(newFileMetadataRmCmd())
	return cmd
}

func newFileMetadataRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <file-id> <key>",
		Short: "Remove one metadata key from a file",
		Long: `Keys are shown by ` + "`kist file show`" + `, namespace included: "exif:Make".

The key stays removed. A later ` + "`kist extract`" + ` will not put it back, because
the library records that you took it out rather than that it was never there.
` + "`kist undo`" + ` restores it.`,
		Args: cobra.ExactArgs(2),
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

			removed, err := l.RemoveMetadata(cmd.Context(), id, args[1])
			if err != nil {
				return err
			}
			out := wire.MetadataRemoval{File: id.String(), Key: args[1], Removed: removed}
			return emit(cmd, out, func(w io.Writer) error {
				if !removed {
					_, err := fmt.Fprintf(w, "%s was already removed from this file\n", args[1])
					return err
				}
				_, err := fmt.Fprintf(w, "removed %s\n", args[1])
				return err
			})
		},
	}
}
