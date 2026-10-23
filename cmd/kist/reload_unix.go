//go:build !windows

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/jinzhao1994/kist/internal/api"
)

func watchConfigReload(ctx context.Context, server *api.Server, log io.Writer) func() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP)
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-signals:
				if err := server.ReloadSettings(ctx); err != nil {
					fmt.Fprintf(log, "Configuration reload failed; previous runtime settings remain active: %v\n", err)
				} else {
					fmt.Fprintln(log, "Configuration reloaded (listener unchanged).")
				}
			}
		}
	}()
	return func() { signal.Stop(signals); cancel(); <-done }
}
