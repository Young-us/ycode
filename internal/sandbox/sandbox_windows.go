// +build windows

package sandbox

import (
	"os/exec"
)

// killProcess kills a process on Windows
func killProcess(pid int) error {
	return exec.Command("taskkill", "/F", "/PID", string(rune(pid))).Run()
}

// setupProcessPlatform configures process for Windows
func setupProcessPlatform(cmd *exec.Cmd) {
	// Use HideWindow to prevent console window popup
	cmd.SysProcAttr = nil // Use default settings on Windows
	// Note: More advanced process control on Windows would require
	// using the golang.org/x/sys/windows package for CREATE_NO_WINDOW
}