package main

import (
	"context"
	"os"
)

// interruptExitCode is the conventional 128+SIGINT.
const interruptExitCode = 130

// interruptContext turns the first SIGINT into a graceful stop and the second
// into an exit.
//
// The policy comes from spec 3.7.1 ① for import — the first Ctrl-C lets the
// files already imported commit as one op, and only the second gives up on
// them — and scrub follows it for the same reason: a run stopped after four
// hours still checked four hours' worth, and discarding that because somebody
// wanted their terminal back is not a kindness.
//
// stopping is the sentence printed on the first signal. It differs per command
// because "what is kept" differs: import has files to commit, scrub has
// findings to print.
//
// library knows nothing about signals; it just watches the context. That is
// what lets this be tested by pushing values into a channel instead of sending
// a real signal at the test binary.
func interruptContext(
	parent context.Context,
	sig <-chan os.Signal,
	stopping string,
	notify func(string),
	exit func(int),
) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		first := false
		for {
			select {
			case <-done:
				return
			case _, ok := <-sig:
				if !ok {
					return
				}
				if !first {
					first = true
					notify(stopping)
					cancel()
					continue
				}
				exit(interruptExitCode)
				return
			}
		}
	}()
	return ctx, func() {
		close(done)
		cancel()
	}
}
