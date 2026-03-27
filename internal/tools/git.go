package tools

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// GitTool provides Git operations
type GitTool struct {
	WorkingDir string
}

// NewGitTool creates a new GitTool
func NewGitTool(workingDir string) *GitTool {
	return &GitTool{
		WorkingDir: workingDir,
	}
}

func (t *GitTool) Name() string {
	return "git"
}

func (t *GitTool) Description() string {
	return "Execute Git commands. Supports status, diff, log, add, commit, branch, and more."
}

func (t *GitTool) Category() ToolCategory {
	return CategoryBasic // Git is always available as a basic tool
}

func (t *GitTool) Parameters() []Parameter {
	return []Parameter{
		{
			Name:        "command",
			Type:        "string",
			Description: "The Git command to execute (e.g., 'status', 'diff', 'log', 'add .', 'commit -m \"message\"')",
			Required:    true,
		},
	}
}

func (t *GitTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	command, ok := args["command"].(string)
	if !ok {
		return &ToolResult{
			Content: "Error: 'command' parameter is required and must be a string",
			IsError: true,
		}, nil
	}

	// Parse command
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return &ToolResult{
			Content: "Error: empty command",
			IsError: true,
		}, nil
	}

	// Build git command
	gitArgs := append([]string{parts[0]}, parts[1:]...)

	// Execute git command
	cmd := exec.CommandContext(ctx, "git", gitArgs...)
	cmd.Dir = t.WorkingDir

	// Handle Windows
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "git", gitArgs...)
		cmd.Dir = t.WorkingDir
	}

	// Run command
	output, err := cmd.CombinedOutput()

	// Handle error
	if err != nil {
		errorMsg := fmt.Sprintf("Git command failed: %v", err)
		if len(output) > 0 {
			errorMsg += fmt.Sprintf("\n\nOutput:\n%s", string(output))
		}
		return &ToolResult{
			Content: errorMsg,
			IsError: true,
		}, nil
	}

	// Format output
	result := strings.TrimSpace(string(output))
	if result == "" {
		result = "(command executed successfully with no output)"
	}

	return &ToolResult{
		Content: result,
		IsError: false,
	}, nil
}
