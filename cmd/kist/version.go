package main

import (
	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/buildinfo"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.Println(buildinfo.Get())
			return nil
		},
	}
}
