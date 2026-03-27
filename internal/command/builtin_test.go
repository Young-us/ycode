package command

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeProject(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "ycode-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test directory structure
	// tmpDir/
	//   main.go
	//   go.mod
	//   .git/ (hidden)
	//   node_modules/ (should be skipped)
	//   internal/
	//     foo.go
	//   .hidden/ (should be skipped)

	// Create files
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create directories
	if err := os.MkdirAll(filepath.Join(tmpDir, "internal"), 0755); err != nil {
		t.Fatalf("Failed to create internal dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "internal", "foo.go"), []byte("package internal"), 0644); err != nil {
		t.Fatalf("Failed to create internal/foo.go: %v", err)
	}

	// Create directories that should be skipped
	if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "node_modules"), 0755); err != nil {
		t.Fatalf("Failed to create node_modules dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, ".hidden"), 0755); err != nil {
		t.Fatalf("Failed to create .hidden dir: %v", err)
	}

	// Create files in hidden directories (should not be counted)
	if err := os.WriteFile(filepath.Join(tmpDir, ".git", "config"), []byte("git config"), 0644); err != nil {
		t.Fatalf("Failed to create .git/config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "node_modules", "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create node_modules/package.json: %v", err)
	}

	// Run analyzeProject
	info, err := analyzeProject(tmpDir)
	if err != nil {
		t.Fatalf("analyzeProject failed: %v", err)
	}

	// Verify results
	t.Logf("Project info: Name=%s, Language=%s, Type=%s", info.Name, info.Language, info.Type)
	t.Logf("FileCount=%d, DirCount=%d", info.FileCount, info.DirCount)
	t.Logf("Directories: %v", info.Directories)
	t.Logf("KeyFiles: %v", info.KeyFiles)
	t.Logf("MainFiles: %v", info.MainFiles)

	// Check file count (should be 3: main.go, go.mod, internal/foo.go)
	// Files in .git and node_modules should not be counted
	if info.FileCount != 3 {
		t.Errorf("Expected FileCount=3, got %d", info.FileCount)
	}

	// Check directory count (should be 1: internal/)
	// Root directory should not be counted
	// .git, node_modules, .hidden should be skipped
	if info.DirCount != 1 {
		t.Errorf("Expected DirCount=1, got %d", info.DirCount)
	}

	// Check that only "internal" is in Directories
	if len(info.Directories) != 1 {
		t.Errorf("Expected 1 directory in Directories, got %d: %v", len(info.Directories), info.Directories)
	}

	// Check language detection
	if info.Language != "Go" {
		t.Errorf("Expected Language='Go', got %s", info.Language)
	}

	// Check key files
	if !info.HasGit {
		t.Errorf("Expected HasGit=true")
	}

	// Check that go.mod and main.go are in KeyFiles
	keyFileSet := make(map[string]bool)
	for _, f := range info.KeyFiles {
		keyFileSet[f] = true
	}
	if !keyFileSet["go.mod"] {
		t.Errorf("Expected 'go.mod' in KeyFiles")
	}
	if !keyFileSet["main.go"] {
		t.Errorf("Expected 'main.go' in KeyFiles")
	}

	// Check that main.go is in MainFiles
	mainFileSet := make(map[string]bool)
	for _, f := range info.MainFiles {
		mainFileSet[f] = true
	}
	if !mainFileSet["main.go"] {
		t.Errorf("Expected 'main.go' in MainFiles")
	}
}

func TestGenerateClaudeMd(t *testing.T) {
	info := &ProjectInfo{
		Name:        "test-project",
		Language:    "Go",
		Type:        "go",
		FileCount:   10,
		DirCount:    5,
		HasReadme:   true,
		HasGit:      true,
		MainFiles:   []string{"main.go"},
		KeyFiles:    []string{"go.mod", "README.md"},
		Directories: []string{"internal", "pkg"},
		BuildCmd:    "go build ./...",
		TestCmd:     "go test ./...",
		RunCmd:      "go run main.go",
	}

	content := generateClaudeMd("/tmp/test", info)

	// Verify content
	if content == "" {
		t.Fatal("generateClaudeMd returned empty content")
	}

	t.Logf("Generated CLAUDE.md:\n%s", content)

	// Check for expected sections
	expectedSections := []string{
		"# CLAUDE.md",
		"## Build and Test Commands",
		"## Project Overview",
		"## Key Files",
		"## Main Entry Points",
		"## Directory Structure",
		"## Development Notes",
		"test-project",
		"go build ./...",
		"go test ./...",
	}

	for _, section := range expectedSections {
		if !contains(content, section) {
			t.Errorf("Expected content to contain '%s'", section)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}