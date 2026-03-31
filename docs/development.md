# Development Guide

This guide covers building, testing, and contributing to ycode.

## Prerequisites

- Go 1.25+
- Git
- Anthropic API key

## Building

```bash
# Clone repository
git clone https://github.com/Young-us/ycode.git
cd ycode

# Download dependencies
go mod download

# Build
go build -o ycode ./cmd/ycode

# Run
./ycode chat
```

## Testing

```bash
# Run all tests
go test ./...

# Run with verbose
go test -v ./...

# Run specific package
go test ./internal/tools/...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Code Style

### Import Organization

Standard library first, then third-party, then internal (separated by blank lines):

```go
import (
    "context"
    "fmt"

    "github.com/spf13/viper"

    "github.com/Young-us/ycode/internal/config"
)
```

### Naming Conventions

| Element | Convention | Example |
|---------|-----------|---------|
| Packages | lowercase, no underscores | `internal/tools` |
| Functions | PascalCase exported, camelCase unexported | `NewBashTool()`, `execute()` |
| Variables | camelCase | `workingDir`, `maxLines` |
| Constants | PascalCase exported, camelCase unexported | `ErrCodeConfigLoad` |
| Interfaces | PascalCase, -er suffix preferred | `Client`, `Tool` |
| Structs | PascalCase | `AnthropicClient` |

### Error Handling

Use structured errors from `internal/errors`:

```go
// Create new error
return nil, errors.New(errors.ErrCodeToolExecution, "failed to execute")

// Wrap error
return nil, errors.Wrap(errors.ErrCodeFileRead, "cannot read config", err)

// Check error type
if errors.IsErrorCode(err, errors.ErrCodeToolTimeout) { ... }
```

**Rules**:
- Always handle errors explicitly
- Use `fmt.Errorf("context: %w", err)` for wrapping
- Return `(*ToolResult, error)` for tools

### Context Usage

Always pass `context.Context` as first parameter:

```go
func (t *MyTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
    // Use WithTimeout for time-limited operations
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    // ...
}
```

## Adding New Tools

### 1. Create Tool File

Create file in `internal/tools/my_tool.go`:

```go
package tools

import "context"

type MyTool struct {
    workDir string
}

func NewMyTool(workDir string) *MyTool {
    return &MyTool{workDir: workDir}
}

func (t *MyTool) Name() string {
    return "my_tool"
}

func (t *MyTool) Description() string {
    return "Description of what this tool does"
}

func (t *MyTool) Category() ToolCategory {
    return CategoryBasic // or CategoryWrite, CategoryLSP, CategoryGit
}

func (t *MyTool) Parameters() []Parameter {
    return []Parameter{
        {
            Name:        "input",
            Type:        "string",
            Description: "Input parameter description",
            Required:    true,
        },
    }
}

func (t *MyTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
    input, ok := args["input"].(string)
    if !ok {
        return nil, errors.New(errors.ErrCodeInvalidParam, "input must be a string")
    }
    
    // Implement tool logic
    
    return &ToolResult{
        Content: "result",
        IsError: false,
    }, nil
}
```

### 2. Register Tool

Register in `internal/app/app.go`:

```go
toolManager.Register(tools.NewMyTool(workDir))
```

### 3. Add Tests

Create `internal/tools/my_tool_test.go`:

```go
package tools

import (
    "context"
    "testing"
)

func TestMyTool_Execute(t *testing.T) {
    tool := NewMyTool(t.TempDir())
    
    result, err := tool.Execute(context.Background(), map[string]interface{}{
        "input": "test",
    })
    
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    if result.IsError {
        t.Error("expected success, got error")
    }
}
```

## Testing Patterns

### Table-Driven Tests

```go
func TestExecutor_Execute(t *testing.T) {
    tests := []struct {
        name    string
        cmd     string
        wantErr bool
    }{
        {"simple echo", "echo hello", false},
        {"invalid command", "nonexistent", true},
    }
    
    executor := NewExecutor(t.TempDir())
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := executor.Execute(context.Background(), tt.cmd)
            if (err != nil) != tt.wantErr {
                t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Cross-Platform Tests

Handle platform differences:

```go
func TestPath(t *testing.T) {
    if runtime.GOOS == "windows" {
        // Windows-specific test
    } else {
        // Unix-specific test
    }
}
```

## Debugging

### Enable Debug Logging

```bash
# Set log level
export YCODE_LOG_LEVEL=debug

# Or in config
logger:
  level: debug
  output: stdout
```

### Verbose Mode

```bash
./ycode chat --verbose
```

## Linting

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run

# Fix issues
golangci-lint run --fix
```

## Code Formatting

```bash
# Format code
gofmt -w .

# Check formatting
gofmt -d .
```

## Project Structure

```
ycode/
├── cmd/                    # Command-line interface
│   └── ycode/             # Main entry point
├── internal/              # Internal packages
│   ├── agent/            # Agent loop and orchestration
│   ├── app/              # Application state and TUI
│   ├── command/          # Command definitions
│   ├── config/           # Configuration management
│   ├── errors/           # Error types
│   ├── llm/              # LLM client
│   ├── lsp/              # LSP integration
│   ├── mcp/              # MCP protocol
│   ├── plugin/           # Plugin system
│   ├── sandbox/          # Sandbox execution
│   ├── session/          # Session management
│   ├── shell/            # Shell execution
│   ├── skill/            # Skill system
│   ├── tools/            # Tool implementations
│   └── ui/               # UI components
├── configs/              # Configuration examples
├── docs/                 # Documentation
├── skills/               # Built-in skills
└── plugins/              # Built-in plugins
```

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for contribution guidelines.