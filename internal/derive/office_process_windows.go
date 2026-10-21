package derive

import (
	"context"
	"os/exec"
	"strconv"
)

func officeProcess(_ context.Context, cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		// soffice may launch soffice.bin; terminate descendants before the launcher.
		_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
		return cmd.Process.Kill()
	}
}
