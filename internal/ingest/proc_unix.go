//go:build !windows

package ingest

import (
	"os/exec"
	"syscall"
	"time"
)

// setupProc puts the adapter in its own process group so a cancel also kills
// children the shell forked, which would otherwise keep the output pipe open.
func setupProc(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 2 * time.Second
}
