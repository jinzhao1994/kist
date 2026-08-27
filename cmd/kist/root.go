package main

import (
	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/buildinfo"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "kist",
		Short: "Self-hosted personal archive",
		Long: `kist is a self-hosted personal archive.

Files live in boxes — one trip, one hospital visit, one batch of paperwork —
and a global tag tree cuts across them for retrieval.`,

		// main prints the error once, on stderr. Without these, cobra prints it
		// too, plus the full usage text, for errors that have nothing to do
		// with usage.
		SilenceUsage:  true,
		SilenceErrors: true,

		// The default "completion" command is noise in --help until we
		// actually document shell completion.
		CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},

		Version: buildinfo.Get().String(),
	}

	// buildinfo.Info.String() is already a complete line; cobra's default
	// template would prefix it with "kist version ".
	root.SetVersionTemplate("{{.Version}}\n")

	root.PersistentFlags().StringVar(&libraryFlag, "library", "",
		"path to the kist library (default ~/kist)")

	root.AddCommand(newInitCmd())
	root.AddCommand(newBoxCmd())
	root.AddCommand(newTagCmd())
	root.AddCommand(newImportCmd())
	root.AddCommand(newLsCmd())
	root.AddCommand(newFindCmd())
	root.AddCommand(newFileCmd())
	root.AddCommand(newRelinkCmd())
	root.AddCommand(newVerifyCmd())
	root.AddCommand(newScrubCmd())
	root.AddCommand(newRepairCmd())
	root.AddCommand(newRemoteCmd())
	root.AddCommand(newRebuildCmd())
	root.AddCommand(newLogCmd())
	root.AddCommand(newUndoCmd())
	root.AddCommand(newVersionCmd())
	return root
}
