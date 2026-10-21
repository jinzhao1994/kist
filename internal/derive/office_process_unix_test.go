//go:build !windows

package derive

import (
	"context"
	"os/exec"
	"testing"
)

func TestOfficeKeepsOwnedWorkerGroup(t *testing.T) {
	ordinary := exec.CommandContext(t.Context(), "unused")
	officeProcess(t.Context(), ordinary)
	if ordinary.SysProcAttr == nil || !ordinary.SysProcAttr.Setpgid {
		t.Fatal("ordinary converter lost its isolated group")
	}
	worker := exec.CommandContext(t.Context(), "unused")
	officeProcess(WithOwnedProcessGroup(context.Background()), worker)
	if worker.SysProcAttr != nil || worker.Cancel == nil {
		t.Fatal("worker converter escaped its owned group")
	}
}
