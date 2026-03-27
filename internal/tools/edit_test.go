package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEditFileTool(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := `line 1
line 2
line 3
line 4
line 5`
	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := NewEditFileTool(tmpDir)

	t.Run("single edit", func(t *testing.T) {
		edit := `<<<<<<< SEARCH
line 2
=======
modified line 2
>>>>>>> REPLACE`

		result, err := tool.Execute(context.Background(), map[string]interface{}{
			"path": "test.txt",
			"edit": edit,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %s", result.Content)
		}

		// Verify edit was applied
		content, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if !contains(string(content), "modified line 2") {
			t.Errorf("expected file to contain 'modified line 2', got: %s", string(content))
		}
	})

	t.Run("search not found", func(t *testing.T) {
		edit := `<<<<<<< SEARCH
nonexistent text
=======
replacement
>>>>>>> REPLACE`

		result, err := tool.Execute(context.Background(), map[string]interface{}{
			"path": "test.txt",
			"edit": edit,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected error for search not found, got success")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInMiddle(s, substr)))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
