package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/buildinfo"
	"github.com/jinzhao1994/kist/internal/wire"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			i := buildinfo.Get()
			out := wire.Version{
				Version:  i.Version,
				Commit:   i.Commit,
				Dirty:    i.Dirty,
				Go:       i.Go,
				Platform: i.Platform,
			}
			return emit(cmd, out, func(w io.Writer) error {
				_, err := fmt.Fprintln(w, i)
				return err
			})
		},
	}
}
