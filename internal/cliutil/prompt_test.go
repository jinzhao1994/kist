package cliutil

import (
	"bytes"
	"strings"
	"testing"
)

func TestAskReturnsTheAnswer(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("/mnt/archive\n"), &out, false)

	got, err := p.Ask("Library location", "~/kist")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got != "/mnt/archive" {
		t.Errorf("Ask = %q, want %q", got, "/mnt/archive")
	}
	if !strings.Contains(out.String(), "~/kist") {
		t.Errorf("prompt %q does not show the default", out.String())
	}
}

func TestAskFallsBackToDefaultOnBareEnter(t *testing.T) {
	p := NewPrompter(strings.NewReader("\n"), &bytes.Buffer{}, false)

	got, err := p.Ask("Library location", "~/kist")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got != "~/kist" {
		t.Errorf("Ask = %q, want the default %q", got, "~/kist")
	}
}

func TestAskTrimsWhitespace(t *testing.T) {
	p := NewPrompter(strings.NewReader("  /mnt/archive  \n"), &bytes.Buffer{}, false)
	got, _ := p.Ask("Library location", "~/kist")
	if got != "/mnt/archive" {
		t.Errorf("Ask = %q, want it trimmed", got)
	}
}

func TestAutoModeNeverReadsInput(t *testing.T) {
	// --yes and a non-terminal stdin both land here. It must be the same code
	// path the interactive user takes, or the tested path is not the shipped
	// one (spec 3.1).
	in := strings.NewReader("this must not be consumed\n")
	p := NewPrompter(in, &bytes.Buffer{}, true)

	got, err := p.Ask("Library location", "~/kist")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got != "~/kist" {
		t.Errorf("Ask = %q, want the default %q", got, "~/kist")
	}
	if in.Len() == 0 {
		t.Error("auto mode consumed stdin")
	}
}

func TestConfirm(t *testing.T) {
	tests := []struct {
		input string
		def   bool
		want  bool
	}{
		{"y\n", false, true},
		{"Y\n", false, true},
		{"yes\n", false, true},
		{"n\n", true, false},
		{"no\n", true, false},
		{"\n", true, true}, // bare enter takes the default
		{"\n", false, false},
	}
	for _, tt := range tests {
		p := NewPrompter(strings.NewReader(tt.input), &bytes.Buffer{}, false)
		got, err := p.Confirm("Create it?", tt.def)
		if err != nil {
			t.Fatalf("Confirm(%q): %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("Confirm(input=%q, default=%v) = %v, want %v", tt.input, tt.def, got, tt.want)
		}
	}
}

func TestConfirmRejectsGibberishUntilItGetsAnAnswer(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("maybe\nsure\ny\n"), &out, false)

	got, err := p.Confirm("Create it?", false)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if !got {
		t.Error("Confirm = false, want true from the final y")
	}
}

func TestAskFailsOnClosedInput(t *testing.T) {
	p := NewPrompter(strings.NewReader(""), &bytes.Buffer{}, false)
	if _, err := p.Ask("Library location", "~/kist"); err == nil {
		t.Error("Ask on empty input = nil error, want a failure")
	}
}
