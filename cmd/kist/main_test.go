package main

import (
	"bytes"
	"strings"
	"testing"
)

// exec runs the root command with args and returns stdout+stderr and the error.
func exec(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)

	// Execute must run before the buffer is read; return operands are
	// evaluated left to right.
	err := root.Execute()
	return out.String(), err
}

func TestVersion(t *testing.T) {
	// The subcommand and the flag must agree — they are two doors to the same
	// buildinfo, and it is easy to let one drift.
	sub, err := exec(t, "version")
	if err != nil {
		t.Fatalf("kist version: %v", err)
	}
	flag, err := exec(t, "--version")
	if err != nil {
		t.Fatalf("kist --version: %v", err)
	}

	if sub != flag {
		t.Errorf("`version` printed %q but `--version` printed %q", sub, flag)
	}
	if !strings.HasPrefix(sub, "kist ") {
		t.Errorf("version output = %q, want it to start with %q", sub, "kist ")
	}
}

func TestVersionRejectsArgs(t *testing.T) {
	if _, err := exec(t, "version", "extra"); err == nil {
		t.Error("kist version extra: got nil error, want a usage error")
	}
}

func TestUnknownCommand(t *testing.T) {
	out, err := exec(t, "frobnicate")
	if err == nil {
		t.Fatal("got nil error, want an error")
	}
	if !strings.Contains(err.Error(), "frobnicate") {
		t.Errorf("error = %q, want it to name the unknown command", err)
	}
	// SilenceUsage/SilenceErrors are load-bearing: main is the only place that
	// prints the error.
	if out != "" {
		t.Errorf("command wrote %q itself, want main to be the only printer", out)
	}
}

func TestHelpListsVersion(t *testing.T) {
	out, err := exec(t, "--help")
	if err != nil {
		t.Fatalf("kist --help: %v", err)
	}
	if !strings.Contains(out, "version") {
		t.Errorf("help = %q, want it to list the version command", out)
	}
	if strings.Contains(out, "completion") {
		t.Errorf("help = %q, want the default completion command hidden", out)
	}
}

// execStdin runs the root command with args, feeding stdin from a string.
func execStdin(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(args)

	err := root.Execute()
	return out.String(), err
}
