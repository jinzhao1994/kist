package main

import (
	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func newVerifyCmd() *cobra.Command {
	var deep bool

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Check the library for damage, without changing anything",
		Long: `Verify walks the database, the content store and boxes/, and reports what
does not add up. It never repairs anything.

It stats files rather than reading them, so it is cheap enough to run often.
Content that has quietly rotted — the right size, the wrong bytes — is
invisible to verify by design; that is what "kist scrub" is for.

  --deep  also replays the whole operation log into a scratch copy and compares
          it with the current tables. This is the strongest check there is, and
          it takes minutes on a large library.

Exit status:

  0  nothing wrong, or notes only
  1  boxes/ has drifted — "kist relink" rebuilds it
  2  content or the database is damaged`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			r, err := l.Verify(cmd.Context(), library.VerifyOptions{Deep: deep})
			if err != nil {
				return err
			}
			if err := emit(cmd, report(r), r.Write); err != nil {
				return err
			}
			return verifyExit(r.Worst())
		},
	}
	cmd.Flags().BoolVar(&deep, "deep", false, "also replay the log and compare it with the current tables")
	return cmd
}

// verifyExit turns the worst severity into an exit code.
//
// Warnings and errors are kept apart because this is the command people put in
// cron, and "your mirror is stale" and "your passport scan is gone" are not
// the same night. Info-level findings exit 0: orphan blobs are a normal state
// for a v0.1 library (spec 3.9), and a check that cries wolf gets removed.
func verifyExit(worst integrity.Severity) error {
	switch worst {
	case integrity.SeverityWarn:
		return &exitError{code: 1, msg: "the projection has drifted"}
	case integrity.SeverityError:
		return &exitError{code: 2, msg: "the library is damaged"}
	}
	return nil
}

// exitError carries an exit code out of a command. The report has already been
// printed, so main prints the message without framing and uses the code.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }
func (e *exitError) ExitCode() int { return e.code }

// report renders an integrity report for JSON.
//
// It is built from Groups rather than from Write's output, so the two cannot
// disagree about a library (spec 2.8) — Groups is the same content in the same
// order, as data.
func report(r integrity.Report) wire.Report {
	groups := r.Groups()
	out := wire.Report{
		Findings: make([]wire.Finding, len(groups)),
		Worst:    r.Worst().String(),
		Checked:  r.Checked,
	}
	for i, g := range groups {
		out.Findings[i] = wire.Finding{
			Kind:      g.Kind.String(),
			Severity:  g.Severity.String(),
			Headline:  g.Headline,
			Count:     g.Count,
			Examples:  g.Examples,
			Truncated: g.Truncated,
			Fix:       g.Fix,
		}
	}
	return out
}
