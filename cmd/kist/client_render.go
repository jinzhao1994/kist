package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	osexec "os/exec"
	"os/signal"
	"syscall"

	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/spf13/cobra"
)

type clientRenderReply struct {
	Result derive.Result `json:"result"`
	Code   string        `json:"code,omitempty"`
}

func newClientRenderCmd() *cobra.Command {
	return &cobra.Command{Use: "render-worker", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		var request library.ClientRenderRequest
		decoder := json.NewDecoder(io.LimitReader(cmd.InOrStdin(), 64<<10))
		if err := decoder.Decode(&request); err != nil {
			return err
		}
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		// EOF is the parent's lifetime signal. A crash closes the pipe just as an
		// explicit cancellation does; do not rely on a reusable numeric process ID.
		go func() { _, _ = io.Copy(io.Discard, cmd.InOrStdin()); cancel() }()
		client, err := library.OpenClient(ctx, request.Root)
		if err != nil {
			return err
		}
		defer client.Close()
		result, err := client.RenderWorkspace(ctx, request)
		reply := clientRenderReply{Result: result}
		if err != nil {
			reply.Code = derive.Code(err)
			if errors.Is(err, library.ErrCacheQuota) {
				reply.Code = "cache_quota"
			}
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(reply)
	}}
}

// runClientRenderer owns the pipe's write end. Cancellation closes it rather
// than killing the supervisor before it can stop and join converter processes.
func runClientRenderer(ctx context.Context, request library.ClientRenderRequest) (derive.Result, error) {
	executable, err := os.Executable()
	if err != nil {
		return derive.Result{}, err
	}
	return runClientRendererCommand(ctx, request, osexec.CommandContext(ctx, executable, "client", "render-worker"))
}

func runClientRendererCommand(ctx context.Context, request library.ClientRenderRequest, command *osexec.Cmd) (derive.Result, error) {
	var reply clientRenderReply
	input, control, err := os.Pipe()
	if err != nil {
		return reply.Result, err
	}
	defer input.Close()
	defer control.Close()
	command.Stdin = input
	command.Cancel = func() error { return control.Close() }
	// No forced supervisor kill on cancellation: it holds the scratch lease
	// until its context-aware renderer has joined the external conversion tool.
	command.WaitDelay = 0
	output, err := command.StdoutPipe()
	if err != nil {
		return reply.Result, err
	}
	command.Stderr = io.Discard
	if err = command.Start(); err != nil {
		return reply.Result, err
	}
	input.Close()
	cleanup, err := containClientRenderer(command)
	if err != nil {
		control.Close()
		_ = command.Process.Kill()
		_ = command.Wait()
		return reply.Result, err
	}
	if err = json.NewEncoder(control).Encode(request); err != nil {
		control.Close()
		_ = command.Wait()
		cleanupErr := cleanup()
		return reply.Result, errors.Join(err, cleanupErr)
	}
	decodeErr := json.NewDecoder(io.LimitReader(output, 64<<10)).Decode(&reply)
	// Closing after a complete reply is safe: rendering has already stopped.
	control.Close()
	waitErr := command.Wait()
	cleanupErr := cleanup()
	if cleanupErr != nil {
		return reply.Result, cleanupErr
	}
	if ctx.Err() != nil {
		return reply.Result, ctx.Err()
	}
	if decodeErr != nil {
		return reply.Result, decodeErr
	}
	if waitErr != nil {
		return reply.Result, waitErr
	}
	if reply.Code == "cache_quota" {
		return reply.Result, library.ErrCacheQuota
	}
	if reply.Code != "" {
		return reply.Result, &derive.Failure{Code: reply.Code, Cause: errors.New("local preview conversion failed")}
	}
	return reply.Result, nil
}
