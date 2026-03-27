package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadFileTool reads file contents
type ReadFileTool struct {
	WorkingDir string
	MaxLines   int
	plugins    PluginHookTrigger
}

// NewReadFileTool creates a new ReadFileTool
func NewReadFileTool(workingDir string) *ReadFileTool {
	return &ReadFileTool{
		WorkingDir: workingDir,
		MaxLines:   1000, // Default max lines
	}
}

// SetPluginManager sets the plugin manager for hook triggering
func (t *ReadFileTool) SetPluginManager(plugins PluginHookTrigger) {
	t.plugins = plugins
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file. Use this to examine code, configuration files, or any text file."
}

func (t *ReadFileTool) Category() ToolCategory {
	return CategoryBasic
}

func (t *ReadFileTool) Parameters() []Parameter {
	return []Parameter{
		{
			Name:        "path",
			Type:        "string",
			Description: "The path to the file to read (relative to working directory or absolute)",
			Required:    true,
		},
		{
			Name:        "start_line",
			Type:        "integer",
			Description: "Start reading from this line number (1-indexed, optional)",
			Required:    false,
		},
		{
			Name:        "end_line",
			Type:        "integer",
			Description: "Stop reading at this line number (1-indexed, optional)",
			Required:    false,
		},
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &ToolResult{
			Content: "Error: 'path' parameter is required and must be a string",
			IsError: true,
		}, nil
	}

	// Resolve path
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(t.WorkingDir, path)
	}

	// Security: prevent directory traversal
	relPath, err := filepath.Rel(t.WorkingDir, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return &ToolResult{
			Content: fmt.Sprintf("Error: path '%s' is outside working directory", path),
			IsError: true,
		}, nil
	}

	// Check if file exists
	info, err := os.Stat(absPath)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error: cannot access file '%s': %v", path, err),
			IsError: true,
		}, nil
	}

	// Check if it's a directory
	if info.IsDir() {
		return &ToolResult{
			Content: fmt.Sprintf("Error: '%s' is a directory, not a file", path),
			IsError: true,
		}, nil
	}

	// Check file size (max 10MB)
	if info.Size() > 10*1024*1024 {
		return &ToolResult{
			Content: fmt.Sprintf("Error: file '%s' is too large (%d bytes). Maximum size is 10MB", path, info.Size()),
			IsError: true,
		}, nil
	}

	// Read file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error reading file '%s': %v", path, err),
			IsError: true,
		}, nil
	}

	// Check if binary
	if isBinary(content) {
		return &ToolResult{
			Content: fmt.Sprintf("Error: '%s' appears to be a binary file and cannot be read as text", path),
			IsError: true,
		}, nil
	}

	// Split into lines
	lines := strings.Split(string(content), "\n")

	// Apply line range if specified
	startLine := 1
	endLine := len(lines)

	if start, ok := args["start_line"].(float64); ok && start > 0 {
		startLine = int(start)
	}
	if end, ok := args["end_line"].(float64); ok && end > 0 {
		endLine = int(end)
	}

	// Validate line range
	if startLine > len(lines) {
		return &ToolResult{
			Content: fmt.Sprintf("Error: start_line %d exceeds file length (%d lines)", startLine, len(lines)),
			IsError: true,
		}, nil
	}

	if endLine > len(lines) {
		endLine = len(lines)
	}

	if startLine > endLine {
		return &ToolResult{
			Content: fmt.Sprintf("Error: start_line %d is greater than end_line %d", startLine, endLine),
			IsError: true,
		}, nil
	}

	// Apply max lines limit
	if endLine-startLine+1 > t.MaxLines {
		endLine = startLine + t.MaxLines - 1
	}

	// Extract lines
	selectedLines := lines[startLine-1 : endLine]

	// Format output with line numbers
	var output strings.Builder
	for i, line := range selectedLines {
		lineNum := startLine + i
		fmt.Fprintf(&output, "%4d | %s\n", lineNum, line)
	}

	result := output.String()

	// Add truncation notice if needed
	if endLine < len(lines) {
		result += fmt.Sprintf("\n... (%d more lines)", len(lines)-endLine)
	}

	// Trigger on_file_read hook
	if t.plugins != nil && t.plugins.Enabled() {
		t.plugins.Trigger(ctx, "on_file_read", map[string]interface{}{
			"path":      path,
			"abs_path":  absPath,
			"content":   result,
			"line_count": len(selectedLines),
		})
	}

	return &ToolResult{
		Content: result,
		IsError: false,
	}, nil
}

// isBinary checks if content appears to be binary
func isBinary(content []byte) bool {
	if len(content) == 0 {
		return false
	}

	// Check for null bytes (common in binary files)
	for _, b := range content[:min(8192, len(content))] {
		if b == 0 {
			return true
		}
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
