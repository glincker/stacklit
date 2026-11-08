//go:build unix

package main

import (
	"os/exec"
	"runtime"
	"syscall"
)

// maxRSSBytes returns the peak resident set size of a finished process.
func maxRSSBytes(cmd *exec.Cmd) int64 {
	ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		return -1
	}
	if runtime.GOOS == "darwin" {
		return int64(ru.Maxrss) // bytes
	}
	return int64(ru.Maxrss) * 1024 // kilobytes on Linux and the BSDs
}
