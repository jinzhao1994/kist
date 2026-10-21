package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net"
	"os"
	osexec "os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
)

func renderHelperCommand(ctx context.Context, mode string) *osexec.Cmd {
	cmd := osexec.CommandContext(ctx, os.Args[0], "-test.run=^TestClientRenderSubprocess$")
	cmd.Env = append(os.Environ(), "KIST_TEST_RENDER_PROCESS="+mode)
	return cmd
}

func TestClientRenderSubprocess(t *testing.T) {
	mode := os.Getenv("KIST_TEST_RENDER_PROCESS")
	if mode == "" {
		return
	}
	// Bound helper lifetime even if a broken pipe implementation would leak it.
	time.AfterFunc(15*time.Second, func() { os.Exit(3) })
	switch mode {
	case "job-parent":
		_, _ = runClientRendererCommand(context.Background(), library.ClientRenderRequest{Root: os.Getenv("KIST_TEST_RENDER_ROOT")}, renderHelperCommand(context.Background(), "job-worker"))
	case "job-worker", "job-worker-exit", "job-worker-crash":
		var request library.ClientRenderRequest
		if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
			os.Exit(1)
		}
		childMode := "job-converter"
		if os.Getenv("KIST_TEST_RENDER_GRANDCHILD") == "1" {
			childMode = "job-converter-parent"
		}
		child := renderHelperCommand(context.Background(), childMode)
		if err := child.Start(); err != nil {
			os.Exit(1)
		}
		waitRenderMarker(t, request.Root, "job-converter-ready")
		if mode == "job-worker-crash" {
			os.Exit(7)
		}
		if mode == "job-worker" {
			_, _ = io.Copy(io.Discard, os.Stdin)
		}
		_ = json.NewEncoder(os.Stdout).Encode(clientRenderReply{})
		// Deliberately leave the child alive: the enclosing job must reap it.
	case "job-converter-parent":
		child := renderHelperCommand(context.Background(), "job-converter")
		if err := child.Run(); err != nil {
			os.Exit(1)
		}
	case "job-converter":
		conn, err := net.Dial("tcp", os.Getenv("KIST_TEST_RENDER_SOCKET"))
		if err != nil {
			os.Exit(1)
		}
		defer conn.Close()
		if _, err = io.WriteString(conn, "ready\n"); err != nil {
			os.Exit(1)
		}
		if err = os.WriteFile(filepath.Join(os.Getenv("KIST_TEST_RENDER_ROOT"), "job-converter-ready"), []byte("ready"), 0600); err != nil {
			os.Exit(1)
		}
		for {
			time.Sleep(time.Second)
		}
	case "converter":
		conn, err := net.Dial("tcp", os.Getenv("KIST_TEST_RENDER_SOCKET"))
		if err != nil {
			os.Exit(1)
		}
		defer conn.Close()
		output := os.Getenv("KIST_TEST_RENDER_OUTPUT")
		file, err := os.Create(output)
		if err != nil {
			os.Exit(1)
		}
		defer file.Close()
		if _, err = file.Write([]byte("partial preview")); err != nil {
			os.Exit(1)
		}
		if _, err = io.WriteString(conn, output+"\n"); err != nil {
			os.Exit(1)
		}
		if _, err = fmt.Fprintln(conn, os.Getppid()); err != nil {
			os.Exit(1)
		}
		// The parent test observes socket EOF when the real renderer kills this
		// external process. No PID polling or shell child lifetime assumptions.
		for {
			time.Sleep(time.Second)
		}
	case "render-parent":
		ctx := context.Background()
		c, err := library.OpenClient(ctx, os.Getenv("KIST_TEST_RENDER_ROOT"))
		if err != nil {
			os.Exit(1)
		}
		defer c.Close()
		render := func(ctx context.Context, request library.ClientRenderRequest) (derive.Result, error) {
			return runClientRendererCommand(ctx, request, renderHelperCommand(ctx, "worker"))
		}
		_, _ = c.GenerateDerived(ctx, os.Getenv("KIST_TEST_RENDER_ORIGINAL"), derive.Video, render)
	case "worker":
		if err := newClientRenderCmd().Execute(); err != nil {
			os.Exit(1)
		}
	case "control":
		var request library.ClientRenderRequest
		if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
			os.Exit(1)
		}
		if err := os.WriteFile(filepath.Join(request.Root, "worker-ready"), []byte("ready"), 0600); err != nil {
			os.Exit(1)
		}
		_, _ = io.Copy(io.Discard, os.Stdin)
		// Simulate joining converter cleanup before reporting completion.
		if err := os.WriteFile(filepath.Join(request.Root, "worker-stopped"), []byte("stopped"), 0600); err != nil {
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(clientRenderReply{Code: "cancelled"})
	case "parent":
		request := library.ClientRenderRequest{Root: os.Getenv("KIST_TEST_RENDER_ROOT")}
		_, _ = runClientRendererCommand(context.Background(), request, renderHelperCommand(context.Background(), "control"))
	default:
		os.Exit(2)
	}
	os.Exit(0)
}

func waitRenderMarker(t *testing.T, root, name string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("missing worker marker %s", name)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestClientRendererCancellationJoinsSupervisor(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := runClientRendererCommand(ctx, library.ClientRenderRequest{Root: root}, renderHelperCommand(ctx, "control"))
		done <- err
	}()
	waitRenderMarker(t, root, "worker-ready")
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("worker not joined")
	}
	if _, err := os.Stat(filepath.Join(root, "worker-stopped")); err != nil {
		t.Fatal("returned before supervisor cleanup", err)
	}
}

func TestClientRendererConvertsPendingPhotoInSubprocess(t *testing.T) {
	ctx := t.Context()
	c, err := library.CreateClient(ctx, filepath.Join(t.TempDir(), "client"), library.ClientSettings{LibraryID: domain.NewID()})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	box, err := c.CreateBox(ctx, "pending photo", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "photo.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = png.Encode(file, image.NewRGBA(image.Rect(0, 0, 80, 40)))
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Import(ctx, path, library.ClientImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files, _, err := c.Files(ctx, query.Query{})
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	renderer := func(ctx context.Context, request library.ClientRenderRequest) (derive.Result, error) {
		return runClientRendererCommand(ctx, request, renderHelperCommand(ctx, "worker"))
	}
	d, err := c.GenerateDerived(ctx, files[0].BlobKey, derive.Thumbnail, renderer)
	if err != nil {
		t.Fatal(err)
	}
	if d.Job.Result.Width != 80 || d.Job.Result.Height != 40 {
		t.Fatal(d)
	}
	_, reader, err := c.OpenDerived(ctx, files[0].BlobKey, derive.Thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	config, format, err := image.DecodeConfig(reader)
	if err != nil || format != "jpeg" || config.Width != 80 {
		t.Fatal(config, format, err)
	}
}
