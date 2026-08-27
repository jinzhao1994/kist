package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
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

			q := query.Query{Text: name, Limit: limit}
			if box != "" {
				b, err := l.ResolveBox(cmd.Context(), box)
				if err != nil {
					return err
				}
				q.Boxes = []domain.BoxID{b.ID}
			}
			// An unknown tag name is a mistake worth naming: silently
			// returning nothing looks exactly like "you have no such files".
			required, err := l.TagCondition(tags)
			if err != nil {
				return err
			}
			excluded, err := l.ExcludeTags(notTags)
			if err != nil {
				return err
			}
			q.Tags = combineTagConditions(required, excluded)

			files, _, err := l.Files(cmd.Context(), q)
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

// combineTagConditions puts the required and excluded halves together.
//
// Either may be nil, and nil is not the same as an expression matching
// nothing: it means there was no condition of that kind. Building a TagAll
// unconditionally would work too, but it would lose that distinction in the
// one place a reader looks for it.
func combineTagConditions(required, excluded query.TagExpr) query.TagExpr {
	switch {
	case required == nil && excluded == nil:
		return nil
	case excluded == nil:
		return required
	case required == nil:
		return excluded
	default:
		return query.TagAll{required, excluded}
	}
}
