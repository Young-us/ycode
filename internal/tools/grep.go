package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GrepTool searches file contents
type GrepTool struct {
	WorkingDir string
	MaxResults int
}

// NewGrepTool creates a new GrepTool
func NewGrepTool(workingDir string) *GrepTool {
	return &GrepTool{
		WorkingDir: workingDir,
		MaxResults: 50,
	}
}

func (t *GrepTool) Name() string {
	return "grep"
}

func (t *GrepTool) Description() string {
	return "Search for a pattern in file contents. Supports regular expressions."
}

func (t *GrepTool) Category() ToolCategory {
	return CategoryBasic
}

func (t *GrepTool) Parameters() []Parameter {
	return []Parameter{
		{
			Name:        "pattern",
			Type:        "string",
			Description: "The pattern to search for (supports regex)",
			Required:    true,
		},
		{
			Name:        "path",
			Type:        "string",
			Description: "The directory to search in (default: working directory)",
			Required:    false,
		},
		{
			Name:        "file_pattern",
			Type:        "string",
			Description: "Filter files by glob pattern (e.g., '*.go', '*.js')",
			Required:    false,
		},
		{
			Name:        "case_sensitive",
			Type:        "boolean",
			Description: "Whether the search is case sensitive (default: true)",
			Required:    false,
		},
	}
}

type grepResult struct {
	File    string
	Line    int
	Content string
}

func (t *GrepTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
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

	// Get file pattern
	filePattern := "*"
	if fp, ok := args["file_pattern"].(string); ok && fp != "" {
		filePattern = fp
	}

	// Get case sensitivity
	caseSensitive := true
	if cs, ok := args["case_sensitive"].(bool); ok {
		caseSensitive = cs
	}

	// Security: prevent directory traversal
	relPath, err := filepath.Rel(t.WorkingDir, searchPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return &ToolResult{
			Content: fmt.Sprintf("Error: path is outside working directory"),
			IsError: true,
		}, nil
	}

	// Compile regex
	regexPattern := pattern
	if !caseSensitive {
		regexPattern = "(?i)" + regexPattern
	}

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error: invalid regex pattern: %v", err),
			IsError: true,
		}, nil
	}

	// Search files
	results, err := t.searchFiles(searchPath, filePattern, re)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error searching files: %v", err),
			IsError: true,
		}, nil
	}

	// Format output
	if len(results) == 0 {
		return &ToolResult{
			Content: fmt.Sprintf("No matches found for pattern '%s'", pattern),
			IsError: false,
		}, nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d match(es) for pattern '%s':\n\n", len(results), pattern))

	currentFile := ""
	for _, result := range results {
		// Print file header if changed
		if result.File != currentFile {
			if currentFile != "" {
				output.WriteString("\n")
			}
			relPath, _ := filepath.Rel(t.WorkingDir, result.File)
			output.WriteString(fmt.Sprintf("%s:\n", relPath))
			currentFile = result.File
		}

		output.WriteString(fmt.Sprintf("  %d: %s\n", result.Line, result.Content))
	}

	return &ToolResult{
		Content: output.String(),
		IsError: false,
	}, nil
}

func (t *GrepTool) searchFiles(root, filePattern string, re *regexp.Regexp) ([]grepResult, error) {
	var results []grepResult

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

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check file pattern
		matched, err := filepath.Match(filePattern, info.Name())
		if err != nil || !matched {
			return nil
		}

		// Skip binary files (simple check)
		if isBinaryFile(path) {
			return nil
		}

		// Search file
		fileResults, err := t.searchFile(path, re)
		if err != nil {
			return nil // Skip files that can't be read
		}

		results = append(results, fileResults...)

		// Limit total results
		if len(results) >= t.MaxResults {
			return filepath.SkipAll
		}

		return nil
	})

	return results, err
}

func (t *GrepTool) searchFile(path string, re *regexp.Regexp) ([]grepResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var results []grepResult
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if re.MatchString(line) {
			results = append(results, grepResult{
				File:    path,
				Line:    lineNum,
				Content: strings.TrimSpace(line),
			})
		}
	}

	return results, scanner.Err()
}

func isBinaryFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	// Read first 512 bytes
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil {
		return false
	}

	// Check for null bytes
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}

	return false
}
