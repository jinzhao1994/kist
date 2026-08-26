// Package cliutil holds the pieces of the command line that are worth testing
// on their own.
package cliutil

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Prompter asks questions with defaults, in the style of ssh-keygen: every
// question has an answer ready, so holding down Enter completes the flow.
//
// auto makes every question return its default without touching the input.
// Both --yes and a non-terminal stdin set it, which keeps the scripted path
// and the interactive path the same code.
type Prompter struct {
	in   *bufio.Reader
	out  io.Writer
	auto bool
}

func NewPrompter(in io.Reader, out io.Writer, auto bool) *Prompter {
	return &Prompter{in: bufio.NewReader(in), out: out, auto: auto}
}

// Auto reports whether this prompter answers on the user's behalf.
func (p *Prompter) Auto() bool { return p.auto }

// Printf writes to the prompter's output.
func (p *Prompter) Printf(format string, args ...any) {
	fmt.Fprintf(p.out, format, args...)
}

// Ask asks for a value, returning def when the user just presses Enter.
func (p *Prompter) Ask(question, def string) (string, error) {
	if p.auto {
		return def, nil
	}
	fmt.Fprintf(p.out, "%s [%s]: ", question, def)

	line, err := p.in.ReadString('\n')
	if err != nil && (line == "" || !errors.Is(err, io.EOF)) {
		return "", fmt.Errorf("read answer: %w", err)
	}
	if answer := strings.TrimSpace(line); answer != "" {
		return answer, nil
	}
	return def, nil
}

// Confirm asks a yes/no question, repeating until it gets one of them.
func (p *Prompter) Confirm(question string, def bool) (bool, error) {
	if p.auto {
		return def, nil
	}
	hint := "y/N"
	if def {
		hint = "Y/n"
	}
	for {
		fmt.Fprintf(p.out, "%s [%s]: ", question, hint)

		line, err := p.in.ReadString('\n')
		if err != nil && (line == "" || !errors.Is(err, io.EOF)) {
			return false, fmt.Errorf("read answer: %w", err)
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "":
			return def, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintln(p.out, "Please answer y or n.")
		}
	}
}
