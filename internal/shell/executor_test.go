package shell

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestExecutor_Execute(t *testing.T) {
	executor := NewExecutor(t.TempDir())

	t.Run("simple command", func(t *testing.T) {
		var cmd string
		if runtime.GOOS == "windows" {
			cmd = "echo hello"
		} else {
			cmd = "echo hello"
		}

		output, err := executor.Execute(context.Background(), cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output != "hello" {
			t.Errorf("expected 'hello', got '%s'", output)
		}
	})

	t.Run("denied command", func(t *testing.T) {
		_, err := executor.Execute(context.Background(), "rm -rf /")
		if err == nil {
			t.Error("expected error for denied command")
		}
	})

	t.Run("timeout command", func(t *testing.T) {
		executor.Timeout = 100 * time.Millisecond

		var cmd string
		if runtime.GOOS == "windows" {
			cmd = "ping -n 10 127.0.0.1"
		} else {
			cmd = "sleep 10"
		}

		_, err := executor.Execute(context.Background(), cmd)
		if err == nil {
			t.Error("expected error for timeout command")
		}
	})
}
