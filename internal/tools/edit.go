package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EditFileTool applies targeted edits using SEARCH/REPLACE format
type EditFileTool struct {
	WorkingDir string
}

// NewEditFileTool creates a new EditFileTool
func NewEditFileTool(workingDir string) *EditFileTool {
	return &EditFileTool{
		WorkingDir: workingDir,
	}
}

func (t *EditFileTool) Name() string {
	return "edit_file"
}

func (t *EditFileTool) Description() string {
	return `Apply targeted edits to a file using SEARCH/REPLACE format.

The edit should be in this format:
new code to replace with

You can include multiple SEARCH/REPLACE blocks in a single edit.`
}

func (t *EditFileTool) Category() ToolCategory {
	return CategoryWrite
}

func (t *EditFileTool) Parameters() []Parameter {
	return []Parameter{
		{
			Name:        "path",
			Type:        "string",
			Description: "The path to the file to edit (relative to working directory or absolute)",
			Required:    true,
		},
		{
			Name:        "edit",
			Type:        "string",
			Description: "The SEARCH/REPLACE edit to apply",
			Required:    true,
		},
	}
}

func (t *EditFileTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &ToolResult{
			Content: "Error: 'path' parameter is required and must be a string",
			IsError: true,
		}, nil
	}

	edit, ok := args["edit"].(string)
	if !ok {
		return &ToolResult{
			Content: "Error: 'edit' parameter is required and must be a string",
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
	content, err := os.ReadFile(absPath)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error reading file '%s': %v", path, err),
			IsError: true,
		}, nil
	}

	// Parse SEARCH/REPLACE blocks
	blocks, err := t.parseEditBlocks(edit)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error parsing edit: %v", err),
			IsError: true,
		}, nil
	}

	// Apply edits
	result := string(content)
	appliedCount := 0

	for _, block := range blocks {
		if !strings.Contains(result, block.Search) {
			return &ToolResult{
				Content: fmt.Sprintf("Error: SEARCH text not found in file:\n%s", block.Search),
				IsError: true,
			}, nil
		}

		result = strings.Replace(result, block.Search, block.Replace, 1)
		appliedCount++
	}

	// Write result back to file
	if err := os.WriteFile(absPath, []byte(result), 0644); err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error writing file '%s': %v", path, err),
			IsError: true,
		}, nil
	}

	return &ToolResult{
		Content: fmt.Sprintf("Successfully applied %d edit(s) to '%s'", appliedCount, path),
		IsError: false,
	}, nil
}

type editBlock struct {
	Search  string
	Replace string
}

func (t *EditFileTool) parseEditBlocks(edit string) ([]editBlock, error) {
	var blocks []editBlock

	// Split by SEARCH/REPLACE markers
	parts := strings.Split(edit, "<<<<<<< SEARCH")

	for i := 1; i < len(parts); i++ {
		part := parts[i]

		// Find the separator
		separatorIdx := strings.Index(part, "=======")
		if separatorIdx == -1 {
			return nil, fmt.Errorf("missing ======= separator in edit block %d", i)
		}

		// Find the end marker
		endIdx := strings.Index(part, ">>>>>>> REPLACE")
		if endIdx == -1 {
			return nil, fmt.Errorf("missing >>>>>>> REPLACE marker in edit block %d", i)
		}

		// Extract search and replace text
		search := strings.TrimSpace(part[:separatorIdx])
		replace := strings.TrimSpace(part[separatorIdx+7 : endIdx])

		blocks = append(blocks, editBlock{
			Search:  search,
			Replace: replace,
		})
	}

	if len(blocks) == 0 {
		return nil, fmt.Errorf("no valid SEARCH/REPLACE blocks found")
	}

	return blocks, nil
}
