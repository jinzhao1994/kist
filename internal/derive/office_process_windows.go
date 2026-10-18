package derive

import (
	"os/exec"
	"strconv"
)

func officeProcess(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		// soffice may launch soffice.bin; terminate descendants before the launcher.
		_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
		return cmd.Process.Kill()
	}
}
