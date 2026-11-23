//go:build !windows

package main

import (
	"os"
	"syscall"
)

func ownProcesses() error { return nil }

// Replace the launcher so the host owns the debug adapter's lifetime and pipes.
func launch(path string, args []string) error {
	return syscall.Exec(path, append([]string{path}, args...), os.Environ())
}
