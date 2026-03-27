package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteFileTool(tmpDir)

	t.Run("create new file", func(t *testing.T) {
		result, err := tool.Execute(context.Background(), map[string]interface{}{
			"path":    "test.txt",
			"content": "Hello, World!",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %s", result.Content)
		}

		// Verify file was created
		content, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
		if err != nil {
			t.Fatalf("failed to read created file: %v", err)
		}
		if string(content) != "Hello, World!" {
			t.Errorf("expected 'Hello, World!', got '%s'", string(content))
		}
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		result, err := tool.Execute(context.Background(), map[string]interface{}{
			"path":    "test.txt",
			"content": "Updated content",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %s", result.Content)
		}

		// Verify file was updated
		content, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
		if err != nil {
			t.Fatalf("failed to read updated file: %v", err)
		}
		if string(content) != "Updated content" {
			t.Errorf("expected 'Updated content', got '%s'", string(content))
		}
	})

	t.Run("create nested directories", func(t *testing.T) {
		result, err := tool.Execute(context.Background(), map[string]interface{}{
			"path":    "nested/dir/file.txt",
			"content": "Nested file",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %s", result.Content)
		}

		// Verify file was created
		content, err := os.ReadFile(filepath.Join(tmpDir, "nested/dir/file.txt"))
		if err != nil {
			t.Fatalf("failed to read nested file: %v", err)
		}
		if string(content) != "Nested file" {
			t.Errorf("expected 'Nested file', got '%s'", string(content))
		}
	})

	t.Run("directory traversal blocked", func(t *testing.T) {
		result, err := tool.Execute(context.Background(), map[string]interface{}{
			"path":    "../../../etc/passwd",
			"content": "malicious",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected error for directory traversal, got success")
		}
	})
}
