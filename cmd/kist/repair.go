package main

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/library"
)

func newRepairCmd() *cobra.Command {
	var (
		yes  bool
		deep bool
	)

	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Fetch damaged or missing content back from a remote",
		Long: `Repair asks each configured remote for the content this library is missing,
verifies what comes back against its hash, and puts it in place.

The verification is why a remote can be an external drive or a network share
you would not otherwise hand your archive to: content that does not hash to
what was asked for is discarded, and the damaged local copy is left alone.

It writes only to the content store, and never deletes anything.

By default it is a dry run that reports what it would do, and it does not
download to find out — it checks that a remote has content under the right key,
of the right size. Pass --yes to actually repair.

  --deep  re-hash local content first, so that files which rotted without
          changing size are repaired too. Use this after "kist scrub".

Remotes are configured by hand in kist.toml:

    [[remote]]
    name = "backup"
    path = "/Volumes/Archive/kist/blobs"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			res, err := l.Repair(cmd.Context(), library.RepairOptions{Apply: yes, Deep: deep})
			if errors.Is(err, library.ErrNoRemotes) {
				printNoRemotes(cmd)
				return &exitError{code: 2, msg: "there is damaged content and nowhere to repair it from"}
			}
			if err != nil {
				return err
			}
			return printRepair(cmd, res)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "actually write the repaired content")
	cmd.Flags().BoolVar(&deep, "deep", false, "re-hash local content first, to catch rot that kept the size")
	return cmd
}

func printRepair(cmd *cobra.Command, res library.RepairResult) error {
	w := cmd.OutOrStdout()

	for name, err := range res.Unreachable {
		fmt.Fprintf(cmd.ErrOrStderr(), "remote %q cannot be read: %v\n", name, err)
	}
	if len(res.Broken) == 0 {
		fmt.Fprintln(w, "nothing needs repairing")
		return nil
	}

	verb := "would repair"
	if !res.DryRun {
		verb = "repaired"
	}
	fmt.Fprintf(w, "%d damaged, %s %d\n", len(res.Broken), verb, len(res.Repaired))
	for _, r := range res.Repaired {
		fmt.Fprintf(w, "    %s  from %s\n", describeContent(r), r.Remote)
	}

	if len(res.Unfixable) > 0 {
		fmt.Fprintf(w, "\nno remote has a good copy of these — this content is gone:\n")
		for _, r := range res.Unfixable {
			fmt.Fprintf(w, "    %s\n", describeContent(r))
		}
	}
	if res.DryRun && len(res.Repaired) > 0 {
		fmt.Fprintln(w, "\nthis was a dry run; rerun with --yes to write it")
	}

	if len(res.Unfixable) > 0 {
		return &exitError{code: 2, msg: "some content could not be repaired"}
	}
	if res.DryRun {
		// Damage is still damage until somebody acts on it, so a script must
		// be able to notice that a dry run found something.
		return &exitError{code: 1, msg: "damaged content was found but not repaired"}
	}
	return nil
}

// describeContent names one blob the way a person recognises it: the files
// that need it, with the digest shortened behind them. The digest still has to
// be there — it is what you would grep a backup index for — but it is not what
// tells you whether this matters.
func describeContent(r library.RepairSource) string {
	key := r.Key
	if i := strings.IndexByte(key, ':'); i >= 0 && len(key) > i+13 {
		key = key[:i+13] + "…"
	}
	switch {
	case len(r.Files) == 0:
		return "(no file uses this content) — " + key
	case len(r.Files) <= 3:
		return strings.Join(r.Files, ", ") + " — " + key
	default:
		return fmt.Sprintf("%s and %d more — %s", strings.Join(r.Files[:3], ", "), len(r.Files)-3, key)
	}
}

func printNoRemotes(cmd *cobra.Command) {
	fmt.Fprint(cmd.ErrOrStderr(), `This library has damaged content and no remote to repair it from.

A remote is any other copy of the same content — an external drive, a network
share, a second machine. Add one to kist.toml:

    [[remote]]
    name = "backup"
    path = "/Volumes/Archive/kist/blobs"

The usual way to make one is the same command that backs the library up:

    rsync -a ~/kist/blobs/ /Volumes/Archive/kist/blobs/
`)
}

func newRemoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Show the other copies kist can repair from",
	}
	cmd.AddCommand(newRemoteListCmd())
	return cmd
}

func newRemoteListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the configured remotes and whether they can be read",
		Long: `Remotes are added by hand in kist.toml — there is no "remote add", because
adding one is something you do once and the file says at the top that hand
edits are read back:

    [[remote]]
    name = "backup"
    path = "/Volumes/Archive/kist/blobs"

This command exists so a mistyped path says so today, rather than on the day
something is damaged.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			remotes := l.CheckRemotes()
			if len(remotes) == 0 {
				fmt.Fprint(cmd.OutOrStdout(), `No remotes are configured, so "kist repair" has nowhere to fetch from.

Add one to kist.toml:

    [[remote]]
    name = "backup"
    path = "/Volumes/Archive/kist/blobs"
`)
				return nil
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tPATH\tSTATUS")
			for _, r := range remotes {
				status := fmt.Sprintf("%d blobs", r.Blobs)
				if r.Err != nil {
					status = "cannot be read: " + r.Err.Error()
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Name, r.Path, status)
			}
			return tw.Flush()
		},
	}
}
