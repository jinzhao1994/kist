//go:build !windows

package derive

import (
	"context"
	"os/exec"
	"syscall"
)

func officeProcess(ctx context.Context, cmd *exec.Cmd) {
	if ctx.Value(ownedProcessGroupKey{}) == true {
		// Do not let soffice escape the worker's containment group. On timeout or
		// cancellation, kill that entire owned group, including this worker. Its
		// parent still holds the original/workspace leases and joins the group.
		cmd.Cancel = func() error { return syscall.Kill(-syscall.Getpgrp(), syscall.SIGKILL) }
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
