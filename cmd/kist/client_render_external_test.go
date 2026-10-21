//go:build !windows

package main

import (
	"bufio"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
)

// The shell only forwards argv into the test binary using exec. The actual
// converter is a standalone process, supervised by the production render worker.
func TestClientRendererExternalProcessRecovery(t *testing.T) {
	for _, name := range []string{"cancellation", "parent-killed", "worker-killed"} {
		crash := name == "parent-killed"
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "client")
			c, err := library.CreateClient(t.Context(), root, library.ClientSettings{LibraryID: domain.NewID()})
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			box, err := c.CreateBox(t.Context(), "converter crash", "", domain.LocalDate{}, domain.LocalDate{})
			if err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(t.TempDir(), "video.mp4")
			if err = os.WriteFile(source, []byte("synthetic video input"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err = c.Import(t.Context(), source, library.ClientImportOptions{Box: box.ID}); err != nil {
				t.Fatal(err)
			}
			files, _, err := c.Files(t.Context(), query.Query{})
			if err != nil || len(files) != 1 {
				t.Fatal(files, err)
			}
			tools := t.TempDir()
			probe := "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"width\":80,\"height\":40}],\"format\":{\"duration\":\"1\"}}'\n"
			encoder := "#!/bin/sh\nfor output do :; done\nexport KIST_TEST_RENDER_OUTPUT=\"$output\"\nexport KIST_TEST_RENDER_PROCESS=converter\nexec \"$KIST_TEST_RENDER_BINARY\" -test.run=^TestClientRenderSubprocess$\n"
			for name, body := range map[string]string{"ffprobe": probe, "ffmpeg": encoder} {
				if err = os.WriteFile(filepath.Join(tools, name), []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			if err = listener.(*net.TCPListener).SetDeadline(time.Now().Add(8 * time.Second)); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("KIST_TEST_RENDER_BINARY", os.Args[0])
			t.Setenv("KIST_TEST_RENDER_SOCKET", listener.Addr().String())
			t.Setenv("KIST_TEST_RENDER_ROOT", root)
			t.Setenv("KIST_TEST_RENDER_ORIGINAL", files[0].BlobKey)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			var stop func()
			if crash {
				parent := renderHelperCommand(ctx, "render-parent")
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
				go func() {
					_, err := c.GenerateDerived(ctx, files[0].BlobKey, derive.Video, func(ctx context.Context, request library.ClientRenderRequest) (derive.Result, error) {
						return runClientRendererCommand(ctx, request, renderHelperCommand(ctx, "worker"))
					})
					done <- err
				}()
				stop = cancel
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
			output, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			output = strings.TrimSpace(output)
			parentID, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			workerPID, err := strconv.Atoi(strings.TrimSpace(parentID))
			if err != nil {
				t.Fatal(err)
			}
			if name == "worker-killed" {
				worker, err := os.FindProcess(workerPID)
				if err != nil {
					t.Fatal(err)
				}
				defer worker.Release()
				stop = func() {
					if err := worker.Kill(); err != nil {
						t.Fatal(err)
					}
				}
			}
			// A second client observes the live workspace and cannot prune it.
			if _, err = c.PruneCache(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err = os.Stat(output); err != nil {
				t.Fatal("pruned active writer", err)
			}
			usage, err := c.CacheStatus(t.Context())
			if err != nil || usage.PartialBytes == 0 || usage.ReservedBytes == 0 {
				t.Fatal(usage, err)
			}
			stop()
			if _, err = reader.ReadByte(); err == nil {
				t.Fatal("converter kept its socket open")
			} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				t.Fatal("external converter survived parent cancellation/crash", err)
			}
			if !crash {
				select {
				case err = <-done:
					if name == "worker-killed" && err == nil {
						t.Fatal("killed worker reported success")
					}
					if name == "cancellation" && !errors.Is(err, context.Canceled) {
						t.Fatal(err)
					}
				case <-time.After(8 * time.Second):
					t.Fatal("render did not return after joining converter")
				}
			}
			deadline := time.Now().Add(8 * time.Second)
			for {
				usage, err = c.PruneCache(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				if usage.PartialBytes == 0 && usage.ReservedBytes == 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("workspace lease/budget leaked", usage)
				}
				time.Sleep(10 * time.Millisecond)
			}
			if _, err = os.Stat(output); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("scratch output survived cleanup", err)
			}
			original, err := c.OpenOriginal(t.Context(), files[0].BlobKey)
			if err != nil {
				t.Fatal("pending original was lost", err)
			}
			original.Close()
			if _, preview, err := c.OpenDerived(t.Context(), files[0].BlobKey, derive.Video); !errors.Is(err, library.ErrNotCached) {
				if preview != nil {
					preview.Close()
				}
				t.Fatal("partial preview was published", err)
			}
		})
	}
}

func TestClientRendererParentCrashClosesControlPipe(t *testing.T) {
	root := t.TempDir()
	parent := renderHelperCommand(t.Context(), "parent")
	parent.Env = append(parent.Env, "KIST_TEST_RENDER_ROOT="+root)
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = parent.Process.Kill() }()
	waitRenderMarker(t, root, "worker-ready")
	if err := parent.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = parent.Wait()
	waitRenderMarker(t, root, "worker-stopped")
}
