package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileTool(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "line 1\nline 2\nline 3\nline 4\nline 5"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := NewReadFileTool(tmpDir)

	t.Run("read entire file", func(t *testing.T) {
		result, err := tool.Execute(context.Background(), map[string]interface{}{
			"path": "test.txt",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %s", result.Content)
		}
		if !strings.Contains(result.Content, "line 1") {
			t.Errorf("expected content to contain 'line 1', got: %s", result.Content)
		}
	})

	t.Run("read with line range", func(t *testing.T) {
		result, err := tool.Execute(context.Background(), map[string]interface{}{
			"path":       "test.txt",
			"start_line": float64(2),
			"end_line":   float64(4),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %s", result.Content)
		}
		if !strings.Contains(result.Content, "line 2") {
			t.Errorf("expected content to contain 'line 2', got: %s", result.Content)
		}
		if !strings.Contains(result.Content, "line 4") {
			t.Errorf("expected content to contain 'line 4', got: %s", result.Content)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		result, err := tool.Execute(context.Background(), map[string]interface{}{
			"path": "nonexistent.txt",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected error, got success")
		}
	})

	t.Run("directory traversal blocked", func(t *testing.T) {
		result, err := tool.Execute(context.Background(), map[string]interface{}{
			"path": "../../../etc/passwd",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected error for directory traversal, got success")
		}
	})
}

func TestReadFileToolBinaryDetection(t *testing.T) {
	tmpDir := t.TempDir()

	// Create binary file
	binaryFile := filepath.Join(tmpDir, "binary.bin")
	binaryContent := []byte{0x00, 0x01, 0x02, 0x03}
	err := os.WriteFile(binaryFile, binaryContent, 0644)
	if err != nil {
		t.Fatalf("failed to create binary file: %v", err)
	}

	tool := NewReadFileTool(tmpDir)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": "binary.bin",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Errorf("expected error for binary file, got success")
	}
}
