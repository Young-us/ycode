# ycode Agent Guidelines

Coding guidelines for the ycode codebase (Go AI coding assistant).

## Project Overview

ycode is an AI-powered terminal coding assistant built with:
- **Go 1.25+** with Cobra CLI framework
- **Bubble Tea** for TUI
- **Viper** for configuration management
- **Anthropic API** for LLM integration
- **TAOR (Think-Act-Observe-Repeat)** agent loop pattern

## Build & Test Commands

```bash
# Build all packages
go build -v ./...

# Build binary
go build -o ycode ./cmd/ycode

# Run all tests
go test -v ./...

# Run single test file (note: must pass package path, not file directly)
go test -v ./internal/config

# Run single test function
go test -v ./internal/config -run TestLoadDefaultConfig

# Run tests matching pattern
go test -v -run "TestExecutor" ./...

# Run tests with coverage
go test -cover ./...

# Vet and format
go vet ./...
go fmt ./...

# Lint (CI uses golangci-lint)
golangci-lint run ./...
```

## Code Style Guidelines

### 1. Import Organization

Standard library first, then third-party, then internal (separated by blank lines):

```go
import (
    "context"
    "fmt"

    "github.com/spf13/viper"

    "github.com/Young-us/ycode/internal/config"
)
```

### 2. Naming Conventions

| Element | Convention | Example |
|---------|-----------|---------|
| Packages | lowercase, no underscores | `internal/tools` |
| Functions | PascalCase exported, camelCase unexported | `func NewBashTool()`, `func execute()` |
| Variables | camelCase | `workingDir`, `maxLines` |
| Constants | PascalCase exported, camelCase unexported | `ErrCodeConfigLoad`, `defaultTimeout` |
| Interfaces | PascalCase, -er suffix preferred | `Client`, `Tool` |
| Structs | PascalCase | `AnthropicClient`, `YCodeError` |

### 3. Error Handling

Use structured errors from `internal/errors/errors.go`:

```go
return nil, errors.New(errors.ErrCodeToolExecution, "failed to execute")
return nil, errors.Wrap(errors.ErrCodeFileRead, "cannot read config", err)
if errors.IsErrorCode(err, errors.ErrCodeToolTimeout) { ... }
```

**Rules**: Always handle errors explicitly. Use `fmt.Errorf("context: %w", err)` for wrapping. Return `(*ToolResult, error)` where `IsError` signals tool failures.

### 4. Context Usage

Always pass `context.Context` as first parameter. Use `context.WithTimeout` for time-limited operations.

### 5. Testing Patterns

Use table-driven tests, `t.TempDir()` for temp files, and handle cross-platform differences:

```go
func TestExecutor_Execute(t *testing.T) {
    executor := NewExecutor(t.TempDir())
    t.Run("simple command", func(t *testing.T) {
        if _, err := executor.Execute(context.Background(), "echo hello"); err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
    })
}

// Cross-platform command selection
if runtime.GOOS == "windows" {
    cmd = "ping -n 10 127.0.0.1"
} else {
    cmd = "sleep 10"
}
```

## Directory Structure

```
ycode/
├── cmd/ycode/          # CLI entry point
├── internal/
│   ├── agent/          # TAOR agent loop
│   ├── app/            # Application setup & tool registration
│   ├── config/         # Configuration management
│   ├── errors/         # Structured error types
│   ├── file/           # File operations
│   ├── git/            # Git integration
│   ├── llm/            # LLM clients (Anthropic)
│   ├── lsp/            # LSP integration
│   ├── mcp/            # MCP server integration
│   ├── shell/          # Shell execution
│   ├── skill/          # Skill system
│   ├── tools/          # Tool definitions (bash, read, write, edit, grep, glob, git)
│   └── ui/             # TUI components (Bubble Tea)
├── configs/            # Default configuration
├── skills/             # Skill plugins
└── scripts/            # Build/utility scripts
```

## Key Patterns

### Adding a New Tool

1. Create struct implementing `tools.Tool` interface (`Name()`, `Description()`, `Parameters()`, `Execute()`)
2. Add constructor `NewXTool(workingDir string) *XTool`
3. Register in `internal/app/app.go`: `toolManager.Register(tools.NewXTool(workDir))`

### Configuration Layers

Priority: Environment vars (`YCODE_*`) > Project (`.ycode/project.yaml`) > Global (`~/.ycode/config.yaml`) > Defaults (`configs/default.yaml`)

Config structures use `mapstructure` tags:
```go
type Config struct {
    LLM   LLMConfig   `mapstructure:"llm"`
    Tools ToolsConfig `mapstructure:"tools"`
}
```

## Security Considerations

- **Path traversal**: Always validate paths are within working directory
- **Command restrictions**: Maintain deny lists for dangerous commands
- **File size limits**: Enforce maximum file read sizes (10MB default)
- **Context timeouts**: All external operations must have timeouts
- **Binary detection**: Reject binary files when reading as text

## Integration Points

- **MCP Servers**: Configured in `configs/default.yaml` under `mcp.servers`. Stdio-based JSON-RPC 2.0.
- **LSP Servers**: Configured under `lsp.servers`. Provides code intelligence.
- **Skills**: Loaded from `skills/`, `~/.ycode/skills/`, `.ycode/skills/` directories.


