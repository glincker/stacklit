//go:build windows

package ingest

import (
	"os/exec"
	"time"
)

// setupProc has no process groups to use here, so it only bounds the wait.
func setupProc(cmd *exec.Cmd) {
	cmd.WaitDelay = 2 * time.Second
}
