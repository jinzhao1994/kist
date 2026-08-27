package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/library"
)

func newFindCmd() *cobra.Command {
	var (
		name    string
		tags    []string
		notTags []string
		box     string
		limit   int
	)

	cmd := &cobra.Command{
		Use:   "find",
		Short: "Search for files by name and tag",
		Long: `find matches file names as a case-insensitive substring, and intersects
tag conditions: --tag can be given several times and a file has to carry all of
them. A tag includes everything below it in the tree, so --tag 人物 finds
files tagged with any person.

--not-tag excludes.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			opt := library.ListFilesOptions{Text: name, Limit: limit}
			if box != "" {
				b, err := l.ResolveBox(cmd.Context(), box)
				if err != nil {
					return err
				}
				opt.Box = &b.ID
			}
			// An unknown tag name is a mistake worth naming: silently
			// returning nothing looks exactly like "you have no such files".
			if opt.AllOf, err = l.ResolveTagRefs(tags); err != nil {
				return err
			}
			for _, ref := range notTags {
				tag, err := l.ResolveTag(ref)
				if err != nil {
					return err
				}
				opt.NoneOf = append(opt.NoneOf, l.Tags().Descendants(tag.ID)...)
			}

			files, _, err := l.Files(cmd.Context(), opt)
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, f := range files {
				// The path inside the box, not just the name: a bare file name
				// is not enough to go find the thing under boxes/.
				path, err := l.FilePath(cmd.Context(), f)
				if err != nil {
					return err
				}
				fmt.Fprintf(w, "%s\t%s\n", f.ID, path)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "match this substring of the file name")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "require this tag (repeatable; includes its descendants)")
	cmd.Flags().StringArrayVar(&notTags, "not-tag", nil, "exclude this tag (repeatable)")
	cmd.Flags().StringVar(&box, "box", "", "only files in this box")
	cmd.Flags().IntVarP(&limit, "limit", "n", 50, "how many results to show")
	return cmd
}
