package shell

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Executor executes shell commands
type Executor struct {
	WorkingDir     string
	Timeout        time.Duration
	DeniedCommands []string
}

// NewExecutor creates a new shell executor
func NewExecutor(workingDir string) *Executor {
	return &Executor{
		WorkingDir: workingDir,
		Timeout:    30 * time.Second,
		DeniedCommands: []string{
			"rm -rf /",
			"sudo",
			"su -",
		},
	}
}

// Execute executes a shell command
func (e *Executor) Execute(ctx context.Context, command string) (string, error) {
	// Check for denied commands
	commandLower := strings.ToLower(command)
	for _, denied := range e.DeniedCommands {
		if strings.Contains(commandLower, strings.ToLower(denied)) {
			return "", fmt.Errorf("command contains denied pattern '%s'", denied)
		}
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	// Determine shell based on platform
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Use PowerShell on Windows
		cmd = exec.CommandContext(execCtx, "powershell", "-Command", command)
	} else {
		// Use bash on Unix-like systems
		cmd = exec.CommandContext(execCtx, "bash", "-c", command)
	}

	cmd.Dir = e.WorkingDir

	// Execute command
	output, err := cmd.CombinedOutput()

	// Handle timeout
	if execCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command timed out after %v", e.Timeout)
	}

	// Handle execution error
	if err != nil {
		errorMsg := fmt.Sprintf("Command failed: %v", err)
		if len(output) > 0 {
			errorMsg += fmt.Sprintf("\n\nOutput:\n%s", string(output))
		}
		return "", fmt.Errorf("%s", errorMsg)
	}

	// Return output
	result := strings.TrimSpace(string(output))
	if result == "" {
		result = "(command executed successfully with no output)"
	}

	return result, nil
}
