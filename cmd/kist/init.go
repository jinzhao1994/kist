package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/cliutil"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func newInitCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Create a new library",
		Long: `Create a new kist library.

Run without arguments to be asked where it should go; every question has a
default, so holding down Enter is a complete answer.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var given string
			if len(args) == 1 {
				given = args[0]
			}
			return runInit(cmd, given, yes)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "accept every default without asking")
	return cmd
}

func runInit(cmd *cobra.Command, given string, yes bool) error {
	// Only --yes answers on the user's behalf. Deriving it from "stdin is not
	// a terminal" was the first design here and it was wrong: a piped stdin is
	// never a terminal, so the interactive flow would silently ignore
	// everything the user typed and create the library at the default path.
	// It also made the tested path a different one from the shipped path,
	// which is precisely what spec 3.1 asks us to avoid. Reading a
	// non-terminal stdin cannot hang — the writer closes and we see EOF — so
	// the hazard it was guarding against does not exist.
	// In --json mode the conversation moves to stderr: stdout carries one
	// document and nothing else (spec 2.8). The questions still get asked, so
	// an interactive `kist init --json` works; a script passes --yes as it
	// would anyway.
	prose := cmd.OutOrStdout()
	if jsonFlag {
		prose = cmd.ErrOrStderr()
	}
	p := cliutil.NewPrompter(cmd.InOrStdin(), prose, yes)

	path := given
	if path == "" {
		def, err := defaultLibraryPath()
		if err != nil {
			return err
		}
		path, err = p.Ask("Library location", def)
		if err != nil {
			return withInputHint(err)
		}
	}
	path, err := expandHome(path)
	if err != nil {
		return err
	}

	// Show what is known before creating anything: a few hundred gigabytes
	// landing on the system disk is a real trap (spec 3.1). The link mode is
	// not knowable yet — probing it needs the directories to exist — so it is
	// reported afterwards.
	free, freeKnown := library.FreeSpaceAt(nearestExistingDir(path))

	p.Printf("\n  Location   %s\n", path)
	if freeKnown {
		p.Printf("  Free space %s\n", humanBytes(free))
	} else {
		p.Printf("  Free space unknown\n")
	}
	p.Printf("\n")

	create, err := p.Confirm("Create the library here?", true)
	if err != nil {
		return withInputHint(err)
	}
	if !create {
		p.Printf("Nothing was created.\n")
		// Nothing was created, so there is no result. JSON says so with an
		// empty path rather than with no document at all — a script that got
		// nothing on stdout could not tell this from a crash.
		return emitResult(cmd, wire.InitResult{})
	}

	res, err := library.Create(cmd.Context(), path)
	if err != nil {
		return err
	}

	// Every line of this goes through the prompter, which --json has already
	// pointed at stderr. init's human output is a conversation rather than a
	// result line, so it is not emit's human branch: the advice at the end is
	// as much part of it as the questions were.
	p.Printf("  Link mode  %s — %s\n\n", res.Probe.LinkMode, linkModeExplanation(res.Probe.LinkMode))
	p.Printf("Library created at %s.\n\n", res.Layout.Root)
	p.Printf("Next: `kist box new <name>` to make a box, then `kist import --box <name> <path>`\n")
	p.Printf("to put files in it. `kist verify` checks at any time that nothing has gone wrong.\n")

	return emitResult(cmd, wire.InitResult{Path: res.Layout.Root, LinkMode: string(res.Probe.LinkMode)})
}

// nearestExistingDir walks up until it finds a directory that exists, so that
// free space can be measured for a library path that has not been created yet.
func nearestExistingDir(path string) string {
	for {
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return path
		}
		path = parent
	}
}

func linkModeExplanation(m blob.LinkMode) string {
	switch m {
	case blob.LinkReflink:
		return "boxes/ shares storage with blobs/ and costs almost nothing"
	case blob.LinkHardlink:
		return "boxes/ shares storage with blobs/ and costs almost nothing"
	case blob.LinkSymlink:
		return "boxes/ holds symlinks; some tools will not follow them"
	case blob.LinkCopy:
		return "this storage cannot deduplicate, so boxes/ will double the space used"
	default:
		return "unknown"
	}
}

func defaultLibraryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, "kist"), nil
}

func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~"+string(os.PathSeparator)) && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, strings.TrimPrefix(path[1:], "/")), nil
}

// withInputHint turns "stdin ran out" into something the user can act on. It
// is the expected failure when init is run from a script that forgot --yes.
func withInputHint(err error) error {
	if !errors.Is(err, io.EOF) {
		return err
	}
	return fmt.Errorf("%w: no more input to read; pass the path as an argument and --yes to run without questions", err)
}

func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
