package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/wire"
)

func newFindCmd() *cobra.Command {
	var (
		name    string
		tags    []string
		notTags []string
		box     string
		sort    string
		cursor  string
		asc     bool
		since   string
		until   string
		limit   int
	)

	cmd := &cobra.Command{
		Use:   "find",
		Short: "Search for files by name and tag",
		Long: `find matches file names as a case-insensitive substring, and intersects
tag conditions: --tag can be given several times and a file has to carry all of
them. A tag includes everything below it in the tree, so --tag 人物 finds
files tagged with any person.

--not-tag excludes.

  --sort   captured (default), imported, name, updated, type, or size
  --asc    oldest or A-Z first, instead of newest or Z-A

Time filtering takes --since and --until. Give the dates as precisely as you
like — "2019", "2019-08", "2019-08-12" — and each one is read as the whole
period it names: --since 2019-03 --until 2019-06 means from the first moment
of March to the last of June.

A file's capture time is a real instant, not a period (it comes from EXIF, or
failing that from the file's own timestamp), so a file is either inside that
range or it is not. Run ` + "`kist extract`" + ` after importing to read
capture times out of photographs; until then a file is dated by when its bytes
were last written.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			sortSpec, err := parseSort(sort, asc)
			if err != nil {
				return err
			}
			q := query.Query{Text: name, Limit: limit, Sort: sortSpec}
			if q.Time, err = parseRange(since, until); err != nil {
				return err
			}
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

			q.Cursor = cursor

			files, next, err := l.Files(cmd.Context(), q)
			if err != nil {
				return err
			}

			// The path inside the box, not just the name: a bare file name is
			// not enough to go find the thing under boxes/.
			out := wire.FileList{Files: make([]wire.File, len(files)), Cursor: next}
			for i, f := range files {
				box, path, err := l.FileLocation(cmd.Context(), f)
				if err != nil {
					return err
				}
				out.Files[i] = wire.NewFile(f, box.String(), path, l.Zone())
			}

			return emit(cmd, out, func(w io.Writer) error {
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				for _, f := range out.Files {
					fmt.Fprintf(tw, "%s\t%s\n", f.ID, f.Path)
				}
				if err := tw.Flush(); err != nil {
					return err
				}
				// A cursor is only useful to a script, and a script uses
				// --json. Saying "there is more" is useful to a person.
				if out.Cursor != "" {
					fmt.Fprintf(w, "\n(more results; raise --limit or use --json to page)\n")
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "match this substring of the file name")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "require this tag (repeatable; includes its descendants)")
	cmd.Flags().StringArrayVar(&notTags, "not-tag", nil, "exclude this tag (repeatable)")
	cmd.Flags().StringVar(&box, "box", "", "only files in this box")
	cmd.Flags().StringVar(&sort, "sort", "captured", "order by captured, imported, name, updated, type or size")
	cmd.Flags().BoolVar(&asc, "asc", false, "oldest or A-Z first")
	cmd.Flags().StringVar(&since, "since", "", "only files captured on or after this date (2019, 2019-08, 2019-08-12)")
	cmd.Flags().StringVar(&until, "until", "", "only files captured on or before this date")
	cmd.Flags().IntVarP(&limit, "limit", "n", 50, "how many results to show")
	cmd.Flags().StringVar(&cursor, "cursor", "", "continue from a previous page (see the cursor in --json output)")
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

// parseSort turns the flag into a SortSpec.
//
// An unknown value names the valid ones. The contract's own error is "unknown
// sort key 99", which is true and useless to somebody who typed --sort date.
func parseSort(name string, asc bool) (query.SortSpec, error) {
	keys := map[string]query.SortKey{
		"captured": query.SortCaptured,
		"imported": query.SortImported,
		"updated":  query.SortUpdated,
		"name":     query.SortName,
		"type":     query.SortType,
		"size":     query.SortSize,
	}
	key, ok := keys[name]
	if !ok {
		return query.SortSpec{}, fmt.Errorf("unknown --sort %q; use captured, imported, name, updated, type or size", name)
	}
	return query.SortSpec{Key: key, Asc: asc}, nil
}

// parseRange turns --since and --until into a DateRange, or nil if neither was
// given.
func parseRange(since, until string) (*query.DateRange, error) {
	if since == "" && until == "" {
		return nil, nil
	}
	var r query.DateRange
	var err error
	if since != "" {
		if r.From, err = domain.ParseLocalDate(since); err != nil {
			return nil, fmt.Errorf("--since: %w", err)
		}
	}
	if until != "" {
		if r.To, err = domain.ParseLocalDate(until); err != nil {
			return nil, fmt.Errorf("--until: %w", err)
		}
	}
	return &r, nil
}
