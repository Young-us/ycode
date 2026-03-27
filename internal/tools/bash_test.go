package tools

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestBashTool(t *testing.T) {
	tool := NewBashTool(t.TempDir())

	t.Run("execute simple command", func(t *testing.T) {
		var cmd string
		if runtime.GOOS == "windows" {
			cmd = "echo hello"
		} else {
			cmd = "echo hello"
		}

		result, err := tool.Execute(context.Background(), map[string]interface{}{
			"command": cmd,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %s", result.Content)
		}
		if result.Content != "hello" {
			t.Errorf("expected 'hello', got '%s'", result.Content)
		}
	})

	t.Run("denied command blocked", func(t *testing.T) {
		result, err := tool.Execute(context.Background(), map[string]interface{}{
			"command": "rm -rf /",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected error for denied command, got success")
		}
	})

	t.Run("timeout command", func(t *testing.T) {
		tool.Timeout = 100 * time.Millisecond

		var cmd string
		if runtime.GOOS == "windows" {
			cmd = "ping -n 10 127.0.0.1"
		} else {
			cmd = "sleep 10"
		}

		result, err := tool.Execute(context.Background(), map[string]interface{}{
			"command": cmd,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected error for timeout, got success")
		}
	})
}
