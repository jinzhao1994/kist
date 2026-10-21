//go:build !windows

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

func TestClientRendererGroupJoinsGrandchildren(t *testing.T) {
	for _, mode := range []string{"cancel", "worker-exit", "worker-crash"} {
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
			t.Setenv("KIST_TEST_RENDER_GRANDCHILD", "1")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			workerMode := "job-worker"
			if mode != "cancel" {
				workerMode = "job-" + mode
			}
			go func() {
				_, err := runClientRendererCommand(ctx, library.ClientRenderRequest{Root: root}, renderHelperCommand(ctx, workerMode))
				done <- err
			}()
			conn, err := listener.Accept()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err = conn.SetReadDeadline(time.Now().Add(8 * time.Second)); err != nil {
				t.Fatal(err)
			}
			reader := bufio.NewReader(conn)
			if ready, err := reader.ReadString('\n'); err != nil || ready != "ready\n" {
				t.Fatal(ready, err)
			}
			if mode == "cancel" {
				cancel()
			}
			if _, err = reader.ReadByte(); !errors.Is(err, io.EOF) {
				t.Fatal("grandchild survived worker termination", err)
			}
			select {
			case err := <-done:
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
				t.Fatal("caller did not join the entire process group")
			}
		})
	}
}
