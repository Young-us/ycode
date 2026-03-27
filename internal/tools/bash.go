package tools

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// BashTool executes shell commands
type BashTool struct {
	WorkingDir     string
	Timeout        time.Duration
	DeniedCommands []string
	plugins        PluginHookTrigger
}

// NewBashTool creates a new BashTool
func NewBashTool(workingDir string) *BashTool {
	return &BashTool{
		WorkingDir: workingDir,
		Timeout:    30 * time.Second,
		DeniedCommands: []string{
			"rm -rf /",
			"sudo",
			"su -",
		},
	}
}

// SetPluginManager sets the plugin manager for hook triggering
func (t *BashTool) SetPluginManager(plugins PluginHookTrigger) {
	t.plugins = plugins
}

func (t *BashTool) Name() string {
	return "bash"
}

func (t *BashTool) Description() string {
	return "Execute a shell command. Use this to run tests, build commands, or any other terminal operations."
}

func (t *BashTool) Category() ToolCategory {
	return CategoryWrite
}

func (t *BashTool) Parameters() []Parameter {
	return []Parameter{
		{
			Name:        "command",
			Type:        "string",
			Description: "The shell command to execute",
			Required:    true,
		},
	}
}

func (t *BashTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	command, ok := args["command"].(string)
	if !ok {
		return &ToolResult{
			Content: "Error: 'command' parameter is required and must be a string",
			IsError: true,
		}, nil
	}

	// Trigger on_command_execute hook before execution
	if t.plugins != nil && t.plugins.Enabled() {
		hookResult, err := t.plugins.Trigger(ctx, "on_command_execute", map[string]interface{}{
			"command":     command,
			"working_dir": t.WorkingDir,
		})
		if err != nil {
			return &ToolResult{
				Content: fmt.Sprintf("Plugin hook error: %v", err),
				IsError: true,
			}, nil
		}
		// Allow plugin to modify command
		if modifiedCommand, ok := hookResult["command"].(string); ok {
			command = modifiedCommand
		}
		// Allow plugin to skip execution
		if skip, ok := hookResult["skip"].(bool); ok && skip {
			return &ToolResult{Content: "Command skipped by plugin", IsError: false}, nil
		}
	}

	// Check for denied commands
	commandLower := strings.ToLower(command)
	for _, denied := range t.DeniedCommands {
		if strings.Contains(commandLower, strings.ToLower(denied)) {
			return &ToolResult{
				Content: fmt.Sprintf("Error: command contains denied pattern '%s'", denied),
				IsError: true,
			}, nil
		}
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, t.Timeout)
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

	cmd.Dir = t.WorkingDir

	// Execute command
	output, err := cmd.CombinedOutput()

	// Handle timeout
	if execCtx.Err() == context.DeadlineExceeded {
		return &ToolResult{
			Content: fmt.Sprintf("Error: command timed out after %v", t.Timeout),
			IsError: true,
		}, nil
	}

	result := strings.TrimSpace(string(output))

	// Handle execution error
	if err != nil {
		errorMsg := fmt.Sprintf("Command failed: %v", err)
		if result != "" {
			errorMsg += fmt.Sprintf("\n\nOutput:\n%s", result)
		}
		return &ToolResult{
			Content: errorMsg,
			IsError: true,
		}, nil
	}

	// Success
	if result == "" {
		result = "(command executed successfully with no output)"
	}

	return &ToolResult{
		Content: result,
		IsError: false,
	}, nil
}
