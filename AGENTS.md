# ycode Agent Guidelines

Coding guidelines for the ycode codebase (Go AI coding assistant).

## Project Overview

ycode is an AI-powered terminal coding assistant built with:
- **Go 1.25+** with Cobra CLI framework
- **Bubble Tea** for TUI
- **Viper** for configuration management
- **Anthropic API** for LLM integration
- **TAOR (Think-Act-Observe-Repeat)** agent loop pattern
- **Multi-Agent System** with specialized agent types

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
├── cmd/ycode/              # CLI entry point
├── internal/
│   ├── agent/              # TAOR agent loop and orchestration
│   │   ├── loop.go         # Core agent loop with streaming
│   │   ├── orchestrator.go # Multi-agent orchestration
│   │   ├── classifier.go   # Intent classification
│   │   ├── compactor.go    # History compaction
│   │   ├── workflow.go     # Plan mode workflow
│   │   └── permission.go   # Permission management
│   ├── app/                # Application setup & TUI
│   ├── audit/              # Audit logging
│   ├── command/            # CLI commands (Cobra)
│   ├── config/             # Configuration management
│   ├── errors/             # Structured error types
│   ├── llm/                # LLM clients (Anthropic)
│   ├── lsp/                # LSP integration
│   ├── mcp/                # MCP server integration
│   │   ├── client.go       # MCP client
│   │   └── oauth.go        # OAuth 2.0 authentication
│   ├── plugin/             # Plugin system
│   ├── sandbox/            # Sandbox for safe execution
│   ├── session/            # Session management
│   ├── shell/              # Shell execution
│   ├── skill/              # Skill system
│   ├── tools/              # Tool definitions
│   │   ├── tool.go         # Tool interface
│   │   ├── manager.go      # Tool manager
│   │   ├── read.go         # File reading
│   │   ├── write.go        # File writing
│   │   ├── edit.go         # File editing
│   │   ├── bash.go         # Shell commands
│   │   ├── glob.go         # Pattern matching
│   │   ├── grep.go         # Content search
│   │   ├── git.go          # Git operations
│   │   ├── lsp.go          # LSP tools
│   │   ├── web_search.go   # Web search
│   │   ├── web_fetch.go    # Web fetch
│   │   └── diff.go         # Diff computation
│   └── ui/                 # TUI components (Bubble Tea)
│       ├── tui_modern.go   # Main TUI
│       ├── action_bar.go   # Action bar
│       └── diff_viewer.go  # Diff viewer
├── configs/                # Example configuration
├── docs/                   # Documentation
├── skills/                 # Skill templates
└── plugins/                # Plugin directory
```

## Core Architecture

### TAOR Pattern

The agent loop implements **Think-Act-Observe-Repeat**:

1. **Think**: Analyze user request, formulate execution plan
2. **Act**: Execute tool calls
3. **Observe**: Process tool results
4. **Repeat**: Continue until task complete or user input needed

### Multi-Agent System

The orchestrator manages specialized agents:

| Agent | Role | Permissions |
|-------|------|-------------|
| Explorer | Codebase navigation | Read-only |
| Planner | Task planning | Read-only |
| Architect | System design | Read-only |
| Coder | Code implementation | Read-write |
| Debugger | Bug fixing | Read-write |
| Tester | Test writing | Read-write |
| Reviewer | Code review | Read-only |
| Writer | Documentation | Read-write |

### Intent Classification

The `IntentClassifier` (`internal/agent/classifier.go`) provides:
- Single vs. multi-agent task detection
- Task dependency identification
- Parallel vs. sequential execution planning
- Result caching for efficiency

### Tool System

Tools implement the `Tool` interface:

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() []Parameter
    Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error)
    Category() ToolCategory
}
```

**Tool Categories**:
- `CategoryBasic`: Always loaded (read_file, glob, grep, web_search, web_fetch)
- `CategoryWrite`: Write operations (write_file, edit_file, bash)
- `CategoryLSP`: LSP features (hover, definition, references, completion)
- `CategoryGit`: Git operations (status, log, diff)

### Permission System

Three permission modes:
- **strict**: All operations require confirmation
- **normal**: Safe operations allowed, dangerous need confirmation
- **permissive**: Most operations allowed automatically

## Key Patterns

### Adding a New Tool

1. Create struct implementing `tools.Tool` interface
2. Add constructor `NewXTool(workingDir string) *XTool`
3. Register in `internal/app/app.go`: `toolManager.Register(tools.NewXTool(workDir))`

Example:

```go
package tools

type MyTool struct {
    workDir string
}

func (t *MyTool) Name() string { return "my_tool" }
func (t *MyTool) Description() string { return "Description of tool" }
func (t *MyTool) Category() ToolCategory { return CategoryBasic }
func (t *MyTool) Parameters() []Parameter {
    return []Parameter{
        {Name: "input", Type: "string", Description: "Input", Required: true},
    }
}
func (t *MyTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
    // Implementation
    return &ToolResult{Content: "result"}, nil
}
```

### Configuration Layers

Priority (later overrides earlier):
1. **Embedded defaults**: Default values in code (`internal/config/config.go`)
2. **Global**: `~/.ycode/config.yaml`
3. **Project**: `.ycode/project.yaml`
4. **Environment**: `YCODE_*` and `ANTHROPIC_API_KEY`

Config structures use `mapstructure` tags:
```go
type Config struct {
    LLM   LLMConfig   `mapstructure:"llm"`
    Tools ToolsConfig `mapstructure:"tools"`
}
```

### Sandbox System

The sandbox (`internal/sandbox/`) provides safe command execution:
- Resource limits: timeout, memory, file size
- Network access control
- Path restrictions
- Command blocking
- Environment variable protection

## Security Considerations

- **Path traversal**: Always validate paths are within working directory
- **Command restrictions**: Maintain deny lists for dangerous commands
- **File size limits**: Enforce maximum file read sizes (10MB default)
- **Context timeouts**: All external operations must have timeouts
- **Binary detection**: Reject binary files when reading as text
- **Permission checks**: Use `PermissionChecker` for sensitive operations

## Integration Points

- **MCP Servers**: Configured in config file under `mcp.servers`. Stdio-based JSON-RPC 2.0.
- **LSP Servers**: Configured under `lsp.servers`. Provides code intelligence.
- **Skills**: Loaded from `skills/`, `~/.ycode/skills/`, `.ycode/skills/` directories.
- **Plugins**: Loaded from `plugins/` directory with hot reload support.