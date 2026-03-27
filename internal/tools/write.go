package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteFileTool creates or overwrites files
type WriteFileTool struct {
	WorkingDir string
	plugins    PluginHookTrigger
}

// NewWriteFileTool creates a new WriteFileTool
func NewWriteFileTool(workingDir string) *WriteFileTool {
	return &WriteFileTool{
		WorkingDir: workingDir,
	}
}

// SetPluginManager sets the plugin manager for hook triggering
func (t *WriteFileTool) SetPluginManager(plugins PluginHookTrigger) {
	t.plugins = plugins
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Create a new file or overwrite an existing file with the given content."
}

func (t *WriteFileTool) Category() ToolCategory {
	return CategoryWrite
}

func (t *WriteFileTool) Parameters() []Parameter {
	return []Parameter{
		{
			Name:        "path",
			Type:        "string",
			Description: "The path to the file to write (relative to working directory or absolute)",
			Required:    true,
		},
		{
			Name:        "content",
			Type:        "string",
			Description: "The content to write to the file",
			Required:    true,
		},
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &ToolResult{
			Content: "Error: 'path' parameter is required and must be a string",
			IsError: true,
		}, nil
	}

	content, ok := args["content"].(string)
	if !ok {
		return &ToolResult{
			Content: "Error: 'content' parameter is required and must be a string",
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

	// Trigger on_file_write hook before writing
	if t.plugins != nil && t.plugins.Enabled() {
		hookResult, err := t.plugins.Trigger(ctx, "on_file_write", map[string]interface{}{
			"path":     path,
			"abs_path": absPath,
			"content":  content,
		})
		if err != nil {
			return &ToolResult{
				Content: fmt.Sprintf("Plugin hook error: %v", err),
				IsError: true,
			}, nil
		}
		// Allow plugin to modify content
		if modifiedContent, ok := hookResult["content"].(string); ok {
			content = modifiedContent
		}
		// Allow plugin to skip write
		if skip, ok := hookResult["skip"].(bool); ok && skip {
			return &ToolResult{Content: "Write skipped by plugin", IsError: false}, nil
		}
	}

	// Create parent directories if they don't exist
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error creating directory '%s': %v", dir, err),
			IsError: true,
		}, nil
	}

	// Check if file exists (for reporting)
	fileExists := false
	if _, err := os.Stat(absPath); err == nil {
		fileExists = true
	}

	// Write file
	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error writing file '%s': %v", path, err),
			IsError: true,
		}, nil
	}

	// Report result
	if fileExists {
		return &ToolResult{
			Content: fmt.Sprintf("File '%s' overwritten successfully", path),
			IsError: false,
		}, nil
	}

	return &ToolResult{
		Content: fmt.Sprintf("File '%s' created successfully", path),
		IsError: false,
	}, nil
}
