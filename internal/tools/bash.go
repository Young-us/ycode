package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Young-us/ycode/internal/sandbox"
)

// BashTool executes shell commands
type BashTool struct {
	WorkingDir string
	Timeout    time.Duration
	plugins    PluginHookTrigger
	sandbox    *sandbox.Sandbox
}

// NewBashTool creates a new BashTool
func NewBashTool(workingDir string) *BashTool {
	return &BashTool{
		WorkingDir: workingDir,
		Timeout:    30 * time.Second,
	}
}

// NewBashToolWithSandbox creates a new BashTool with sandbox enabled
func NewBashToolWithSandbox(workingDir string, sandboxConfig *sandbox.Config) *BashTool {
	tool := &BashTool{
		WorkingDir: workingDir,
		Timeout:    30 * time.Second,
	}

	if sandboxConfig != nil && sandboxConfig.Enabled {
		tool.sandbox = sandbox.New(workingDir, sandboxConfig)
		// Sync timeout from sandbox config
		if sandboxConfig.Timeout > 0 {
			tool.Timeout = sandboxConfig.Timeout
		}
	}

	return tool
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

	// If sandbox is enabled, use sandbox execution
	if t.sandbox != nil && t.sandbox.IsEnabled() {
		return t.executeWithSandbox(ctx, command)
	}

	// Otherwise, use legacy execution (backward compatibility)
	return t.executeLegacy(ctx, command)
}

// executeWithSandbox executes command using sandbox
func (t *BashTool) executeWithSandbox(ctx context.Context, command string) (*ToolResult, error) {
	result, err := t.sandbox.ExecuteShell(ctx, command)
	if err != nil {
		// Check if it's a sandbox policy violation
		if strings.Contains(err.Error(), "blocked") || strings.Contains(err.Error(), "dangerous") {
			return &ToolResult{
				Content: fmt.Sprintf("Security violation: %v", err),
				IsError: true,
			}, nil
		}

		// Other execution errors
		errorMsg := fmt.Sprintf("Command failed: %v", err)
		if result != nil && result.Stdout != "" {
			errorMsg += fmt.Sprintf("\n\nOutput:\n%s", result.Stdout)
		}
		if result != nil && result.Stderr != "" {
			errorMsg += fmt.Sprintf("\n\nError:\n%s", result.Stderr)
		}
		return &ToolResult{
			Content: errorMsg,
			IsError: true,
		}, nil
	}

	// Success - build output
	output := ""
	if result.Stdout != "" {
		output += result.Stdout
	}
	if result.Stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += result.Stderr
	}

	// Add execution metadata if relevant
	if result.TimedOut {
		return &ToolResult{
			Content: fmt.Sprintf("Command timed out after %v", t.Timeout),
			IsError: true,
		}, nil
	}

	// Check exit code
	if result.ExitCode != 0 {
		if output == "" {
			output = "(command failed with no output)"
		}
		return &ToolResult{
			Content: fmt.Sprintf("Exit code: %d\n%s", result.ExitCode, output),
			IsError: true,
		}, nil
	}

	// Success
	if output == "" {
		output = "(command executed successfully with no output)"
	}

	// Add duration info for long-running commands
	if result.Duration > 1*time.Second {
		output += fmt.Sprintf("\n\nDuration: %v", result.Duration)
	}

	return &ToolResult{
		Content: output,
		IsError: false,
	}, nil
}

// executeLegacy executes command without sandbox (backward compatibility)
func (t *BashTool) executeLegacy(ctx context.Context, command string) (*ToolResult, error) {
	// Check for basic denied commands (legacy behavior)
	deniedCommands := []string{"rm -rf /", "sudo", "su -"}
	commandLower := strings.ToLower(command)
	for _, denied := range deniedCommands {
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

	// Execute command
	result, err := t.sandbox.ExecuteShell(execCtx, command)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error: %v", err),
			IsError: true,
		}, nil
	}

	// Build output
	output := ""
	if result.Stdout != "" {
		output += result.Stdout
	}
	if result.Stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += result.Stderr
	}

	// Check for timeout
	if result.TimedOut {
		return &ToolResult{
			Content: fmt.Sprintf("Error: command timed out after %v", t.Timeout),
			IsError: true,
		}, nil
	}

	// Check exit code
	if result.ExitCode != 0 {
		if output == "" {
			output = "(command failed with no output)"
		}
		return &ToolResult{
			Content: fmt.Sprintf("Exit code: %d\n%s", result.ExitCode, output),
			IsError: true,
		}, nil
	}

	// Success
	if output == "" {
		output = "(command executed successfully with no output)"
	}

	return &ToolResult{
		Content: output,
		IsError: false,
	}, nil
}