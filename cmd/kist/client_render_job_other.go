//go:build !windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"syscall"
	"time"

	"github.com/jinzhao1994/kist/internal/derive"
)

func prepareClientRenderer(command *osexec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.Setpgid = true
	command.SysProcAttr.Pgid = 0
}

func clientRenderContext(ctx context.Context) (context.Context, error) {
	if syscall.Getpgrp() != os.Getpid() {
		return nil, errors.New("render worker must own its process group")
	}
	return derive.WithOwnedProcessGroup(ctx), nil
}

func stopClientRenderer(command *osexec.Cmd) error {
	// The worker has not been reaped. Its PID cannot have been reused, including
	// after an abnormal exit. Never send another group signal after Wait.
	err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stop render group: %w", err)
	}
	return nil
}

func containClientRenderer(command *osexec.Cmd) (func() error, error) {
	group, err := syscall.Getpgid(command.Process.Pid)
	if err != nil {
		return nil, err
	}
	if group != command.Process.Pid {
		return nil, fmt.Errorf("render worker does not own process group %d", group)
	}
	return func() error {
		// Wait has already reaped our worker. When this service is an init/subreaper,
		// also reap adopted children from this group, never unrelated process groups.
		// Otherwise init reaps them. A successful signal delivery alone does not
		// prove that an external writer has stopped, so retain leases until empty.
		for {
			var status syscall.WaitStatus
			pid, err := syscall.Wait4(-group, &status, syscall.WNOHANG, nil)
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			if err != nil && !errors.Is(err, syscall.ECHILD) {
				return fmt.Errorf("reap render group: %w", err)
			}
			if pid > 0 {
				continue
			}
			err = syscall.Kill(-group, 0)
			if errors.Is(err, syscall.ESRCH) {
				return nil
			}
			// Darwin can report EPERM while only unreaped zombies remain. This
			// is not proof of termination: keep the leases and wait for ESRCH.
			if err != nil && !errors.Is(err, syscall.EPERM) {
				return fmt.Errorf("inspect render group: %w", err)
			}
			time.Sleep(time.Millisecond)
		}
	}, nil
}
