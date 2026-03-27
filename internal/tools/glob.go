package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GlobTool finds files matching a pattern
type GlobTool struct {
	WorkingDir string
	MaxResults int
}

// NewGlobTool creates a new GlobTool
func NewGlobTool(workingDir string) *GlobTool {
	return &GlobTool{
		WorkingDir: workingDir,
		MaxResults: 100,
	}
}

func (t *GlobTool) Name() string {
	return "glob"
}

func (t *GlobTool) Description() string {
	return "Find files matching a glob pattern. Supports *, **, ?, and [abc] patterns."
}

func (t *GlobTool) Category() ToolCategory {
	return CategoryBasic
}

func (t *GlobTool) Parameters() []Parameter {
	return []Parameter{
		{
			Name:        "pattern",
			Type:        "string",
			Description: "The glob pattern to match (e.g., '**/*.go', 'src/**/*.js')",
			Required:    true,
		},
		{
			Name:        "path",
			Type:        "string",
			Description: "The directory to search in (default: working directory)",
			Required:    false,
		},
	}
}

func (t *GlobTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return &ToolResult{
			Content: "Error: 'pattern' parameter is required and must be a string",
			IsError: true,
		}, nil
	}

	// Get search path
	searchPath := t.WorkingDir
	if path, ok := args["path"].(string); ok && path != "" {
		searchPath = filepath.Join(t.WorkingDir, path)
	}

	// Security: prevent directory traversal
	relPath, err := filepath.Rel(t.WorkingDir, searchPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return &ToolResult{
			Content: fmt.Sprintf("Error: path is outside working directory"),
			IsError: true,
		}, nil
	}

	// Find matching files
	matches, err := t.findMatches(searchPath, pattern)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error finding matches: %v", err),
			IsError: true,
		}, nil
	}

	// Sort results
	sort.Strings(matches)

	// Limit results
	truncated := false
	if len(matches) > t.MaxResults {
		matches = matches[:t.MaxResults]
		truncated = true
	}

	// Format output
	if len(matches) == 0 {
		return &ToolResult{
			Content: fmt.Sprintf("No files found matching pattern '%s'", pattern),
			IsError: false,
		}, nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d file(s) matching '%s':\n\n", len(matches), pattern))

	for _, match := range matches {
		// Make path relative to working directory
		relPath, _ := filepath.Rel(t.WorkingDir, match)
		output.WriteString(fmt.Sprintf("  %s\n", relPath))
	}

	if truncated {
		output.WriteString(fmt.Sprintf("\n... (%d more results truncated)\n", len(matches)-t.MaxResults))
	}

	return &ToolResult{
		Content: output.String(),
		IsError: false,
	}, nil
}

func (t *GlobTool) findMatches(root, pattern string) ([]string, error) {
	var matches []string

	// Use filepath.Walk for recursive search
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden directories
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") && path != root {
			return filepath.SkipDir
		}

		// Skip common ignored directories
		if info.IsDir() {
			switch info.Name() {
			case "node_modules", "vendor", "__pycache__", ".git":
				return filepath.SkipDir
			}
		}

		// Get relative path from root
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}

		// Match pattern
		matched, err := matchPattern(pattern, relPath)
		if err != nil {
			return nil
		}

		if matched && !info.IsDir() {
			matches = append(matches, path)
		}

		return nil
	})

	return matches, err
}

// matchPattern matches a glob pattern against a path
func matchPattern(pattern, path string) (bool, error) {
	// Normalize paths
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		return matchDoubleStar(pattern, path)
	}

	// Use filepath.Match for simple patterns
	return filepath.Match(pattern, path)
}

// matchDoubleStar handles ** glob patterns
func matchDoubleStar(pattern, path string) (bool, error) {
	// Split pattern by **
	parts := strings.Split(pattern, "**")

	if len(parts) == 1 {
		// No **, use simple match
		return filepath.Match(pattern, path)
	}

	// Match prefix
	prefix := strings.TrimSuffix(parts[0], "/")
	if prefix != "" {
		if !strings.HasPrefix(path, prefix) {
			return false, nil
		}
	}

	// Match suffix
	suffix := strings.TrimPrefix(parts[1], "/")
	if suffix != "" {
		pathParts := strings.Split(path, "/")
		for i := range pathParts {
			subPath := strings.Join(pathParts[i:], "/")
			matched, err := filepath.Match(suffix, subPath)
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
		return false, nil
	}

	return true, nil
}
