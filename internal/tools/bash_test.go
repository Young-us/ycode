package tools

import (
	"context"
	"testing"
	"time"

	"github.com/Young-us/ycode/internal/sandbox"
)

func TestBashToolBasic(t *testing.T) {
	tool := NewBashTool(t.TempDir())
	
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo hello",
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}
	
	if !contains(result.Content, "hello") {
		t.Errorf("expected output to contain 'hello', got: %s", result.Content)
	}
}

func TestBashToolWithSandbox(t *testing.T) {
	sandboxConfig := &sandbox.Config{
		Enabled:         true,
		Timeout:         10 * time.Second,
		MaxMemoryMB:     256,
		AllowNetwork:    false,
		BlockedCommands: []string{"rm -rf"},
		RestrictedEnvVars: []string{"API_KEY"},
	}
	
	tool := NewBashToolWithSandbox(t.TempDir(), sandboxConfig)
	
	// Test safe command
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo safe",
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}
	
	if !contains(result.Content, "safe") {
		t.Errorf("expected output to contain 'safe', got: %s", result.Content)
	}
}

func TestBashToolBlockedCommand(t *testing.T) {
	sandboxConfig := &sandbox.Config{
		Enabled:         true,
		Timeout:         10 * time.Second,
		BlockedCommands: []string{"rm -rf"},
	}
	
	tool := NewBashToolWithSandbox(t.TempDir(), sandboxConfig)
	
	// Test blocked command
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "rm -rf /test",
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if !result.IsError {
		t.Fatalf("expected error for blocked command, got success")
	}
	
	if !contains(result.Content, "blocked") {
		t.Errorf("expected error message about blocked command, got: %s", result.Content)
	}
}

func TestBashToolTimeout(t *testing.T) {
	sandboxConfig := &sandbox.Config{
		Enabled: true,
		Timeout: 1 * time.Second, // Very short timeout
	}
	
	tool := NewBashToolWithSandbox(t.TempDir(), sandboxConfig)
	
	// Test command that should timeout
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "sleep 5",
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if !result.IsError {
		t.Fatalf("expected timeout error, got success")
	}
	
	if !contains(result.Content, "timeout") {
		t.Errorf("expected timeout error message, got: %s", result.Content)
	}
}

func TestBashToolDisabledSandbox(t *testing.T) {
	sandboxConfig := &sandbox.Config{
		Enabled: false, // Sandbox disabled
	}
	
	tool := NewBashToolWithSandbox(t.TempDir(), sandboxConfig)
	
	// Even dangerous patterns should work when sandbox is disabled
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo test",
	})
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if result.IsError {
		t.Fatalf("expected success when sandbox disabled, got error: %s", result.Content)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}