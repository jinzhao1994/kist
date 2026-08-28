package main

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/wire"
)

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls [<box>] [<path>]",
		Short: "List boxes, or what is inside one",
		Long: `With no arguments, ls lists the boxes. With a box it lists that box's
top level, and with a path it lists that folder.

Directories come first, then files; both in byte order, which is the order
every other part of kist uses so that two views of the same library never
disagree.`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()
			warnIfProjectionStale(cmd, l)

			if len(args) == 0 {
				return listBoxes(cmd, l)
			}
			b, err := l.ResolveBox(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			path := ""
			if len(args) == 2 {
				path = args[1]
			}
			return listFolder(cmd, l, b, path)
		},
	}
}

func listBoxes(cmd *cobra.Command, l *library.Library) error {
	boxes, err := l.Boxes(cmd.Context(), library.ListBoxesOptions{SortByName: true})
	if err != nil {
		return err
	}
	return emit(cmd, wire.BoxList{Boxes: wire.NewBoxes(boxes)}, func(w io.Writer) error {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, b := range boxes {
			date := b.StartAt.String()
			if date == "" {
				date = "—"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\n", b.ID, date, b.Name)
		}
		return tw.Flush()
	})
}

func listFolder(cmd *cobra.Command, l *library.Library, b domain.Box, path string) error {
	folder, err := l.ResolveFolder(cmd.Context(), b.ID, path)
	if err != nil {
		return err
	}
	folders, err := l.Folders(cmd.Context(), b.ID)
	if err != nil {
		return err
	}
	var subs []domain.Folder
	for _, f := range folders {
		if f.ParentID != nil && *f.ParentID == folder.ID {
			subs = append(subs, f)
		}
	}
	slices.SortFunc(subs, func(x, y domain.Folder) int { return cmp.Compare(x.Name, y.Name) })

	files, _, err := l.Files(cmd.Context(), query.Query{Folder: &folder.ID, Sort: query.SortSpec{Key: query.SortName, Asc: true}})
	if err != nil {
		return err
	}
	slices.SortFunc(files, func(x, y domain.File) int { return cmp.Compare(x.Name, y.Name) })

	out := wire.FolderList{
		Box:     wire.NewBox(b),
		Folder:  wire.NewFolder(folder),
		Folders: make([]wire.Folder, len(subs)),
		Files:   make([]wire.File, len(files)),
	}
	for i, f := range subs {
		out.Folders[i] = wire.NewFolder(f)
	}
	for i, f := range files {
		// No path: inside one folder the name is the whole answer, and
		// resolving a path costs a walk up the chain per file.
		out.Files[i] = wire.NewFile(f, "", l.Zone())
	}

	return emit(cmd, out, func(w io.Writer) error {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, f := range subs {
			fmt.Fprintf(tw, "%s\t%s/\n", f.UpdatedAt.Format("2006-01-02 15:04"), f.Name)
		}
		for _, f := range files {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", f.UpdatedAt.Format("2006-01-02 15:04"), f.Name, f.ID)
		}
		return tw.Flush()
	})
}

// warnIfProjectionStale puts the dirty marker in front of a person.
//
// spec 3.4.1 requires it to be visible; a marker nobody is shown is a marker
// that lets boxes/ go stale until someone happens to run verify.
func warnIfProjectionStale(cmd *cobra.Command, l *library.Library) {
	if l.ProjectionDirty() {
		fmt.Fprintln(cmd.ErrOrStderr(),
			"warning: boxes/ is behind the database. Run `kist relink` to rebuild it.")
	}
}
