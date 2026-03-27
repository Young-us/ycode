// +build !windows

package sandbox

import (
	"os/exec"
	"syscall"
)

// killProcess kills a process on Unix-like systems
func killProcess(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}

// setupProcessPlatform configures process for Unix-like systems
func setupProcessPlatform(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}