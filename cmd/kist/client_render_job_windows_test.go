package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/library"
)

func TestClientRendererWindowsJobTerminatesDescendants(t *testing.T) {
	for _, mode := range []string{"cancel", "worker-exit", "worker-crash", "parent-crash"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			if err = listener.(*net.TCPListener).SetDeadline(time.Now().Add(8 * time.Second)); err != nil {
				t.Fatal(err)
			}
			t.Setenv("KIST_TEST_RENDER_ROOT", root)
			t.Setenv("KIST_TEST_RENDER_SOCKET", listener.Addr().String())
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			stop := cancel
			if mode == "parent-crash" {
				parent := renderHelperCommand(ctx, "job-parent")
				if err = parent.Start(); err != nil {
					t.Fatal(err)
				}
				defer func() { _ = parent.Process.Kill() }()
				stop = func() {
					if err := parent.Process.Kill(); err != nil {
						t.Fatal(err)
					}
					_ = parent.Wait()
				}
			} else {
				workerMode := "job-worker"
				if mode == "worker-exit" || mode == "worker-crash" {
					workerMode = "job-" + mode
				}
				go func() {
					_, err := runClientRendererCommand(ctx, library.ClientRenderRequest{Root: root}, renderHelperCommand(ctx, workerMode))
					done <- err
				}()
			}
			conn, err := listener.Accept()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err = conn.SetReadDeadline(time.Now().Add(8 * time.Second)); err != nil {
				t.Fatal(err)
			}
			reader := bufio.NewReader(conn)
			if line, err := reader.ReadString('\n'); err != nil || line != "ready\n" {
				t.Fatal(line, err)
			}
			if mode == "cancel" || mode == "parent-crash" {
				stop()
			}
			if _, err = reader.ReadByte(); !errors.Is(err, io.EOF) {
				// Windows can report a reset for a process forcibly terminated with an
				// open TCP socket. A timeout means it survived the job, which is a failure.
				if timeout, ok := err.(net.Error); err == nil || ok && timeout.Timeout() {
					t.Fatal("converter survived job termination", err)
				}
			}
			if mode != "parent-crash" {
				select {
				case err = <-done:
					if mode == "cancel" && !errors.Is(err, context.Canceled) {
						t.Fatal(err)
					}
					if mode == "worker-crash" && err == nil {
						t.Fatal("worker crash reported success")
					}
					if mode == "worker-exit" && err != nil {
						t.Fatal(err)
					}
				case <-time.After(8 * time.Second):
					t.Fatal("caller did not finish after job termination")
				}
			}
		})
	}
}
