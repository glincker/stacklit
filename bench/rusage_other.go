//go:build !unix

package main

import "os/exec"

// maxRSSBytes is unavailable on this platform.
func maxRSSBytes(cmd *exec.Cmd) int64 { return -1 }
