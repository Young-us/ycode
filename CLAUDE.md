# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) for working with code in this repository.

## Project Overview

ycode is an AI-powered terminal coding assistant that uses Claude and compatible AI models via the Anthropic protocol. It provides a rich TUI experience for interacting with AI to perform file operations, shell commands, git workflows, and more through natural language.

## Build and Test Commands

```bash
# Build
go build ./...

# Build specific binary
go build -o ycode ./cmd/ycode

# Run
go run ./cmd/ycode

# Run with specific command
go run ./cmd/ycode chat --help

# Test all packages
go test ./...

# Test with coverage
go test -cover ./...

# Test specific package
go test ./internal/agent/...

# Lint
golangci-lint run

# Format code
gofmt -w .
```

## Architecture Overview

```
cmd/ycode/main.go          # Entry point
internal/
├── agent/         # AI agent orchestration and message handling
│   ├── loop.go          # TAOR agent loop with streaming support
│   ├── orchestrator.go  # Multi-agent orchestration with specialized agent types
│   ├── compactor.go     # History compaction for context management
│   ├── classifier.go    # Intent classification for semantic analysis
│   ├── workflow.go      # Plan mode types and state management
│   └── permission.go    # Agent permission system
├── app/           # Main application state and TUI (Bubble Tea)
├── ast/           # Abstract Syntax Tree utilities (placeholder)
├── audit/         # Audit logging and sensitive operation management
├── command/       # CLI command definitions (Cobra) and slash command manager
├── config/        # Configuration management (Viper)
├── errors/        # Custom error types and handling
├── llm/           # LLM client abstraction (Anthropic API)
├── logger/        # Logging utilities
├── lsp/           # Language Server Protocol integration
├── mcp/           # Model Context Protocol support
│   ├── client.go        # MCP client with JSON-RPC 2.0
│   └── oauth.go         # OAuth 2.0 authentication for MCP servers
├── plugin/        # Plugin architecture and loading
├── sandbox/       # Sandbox environment for safe command execution
├── session/       # Session management
├── shell/         # Shell command execution
├── skill/         # Skill system (reusable AI capabilities)
├── tools/         # AI tools (file, shell, git, web, etc.)
│   ├── tool.go          # Tool interface definition
│   ├── manager.go       # Tool registration and execution with plugin hook support
│   ├── permission_checker.go  # Unified permission checking
│   ├── read.go          # read_file tool
│   ├── write.go         # write_file tool
│   ├── edit.go          # edit_file tool
│   ├── bash.go          # bash command execution with sandbox support
│   ├── glob.go          # glob pattern matching
│   ├── grep.go          # grep content search
│   ├── git.go           # git operations (status, log, diff)
│   ├── lsp.go           # LSP tool (hover, definition, references, completion)
│   ├── web_search.go    # DuckDuckGo search (no API key required)
│   ├── web_fetch.go     # Web page fetch and markdown conversion
│   └── diff.go          # LCS-based diff computation utility
└── ui/            # UI components (Bubble Tea + Lipgloss)
    ├── action_bar.go    # Reusable action bar for user decisions
```

**Key Design Patterns:**
- **Bubble Tea (Elm Architecture)**: TUI built on Model-Update-View pattern
- **Tool System**: Extensible tools registered with the agent for AI capabilities
- **Plugin Architecture**: Loadable plugins for extending functionality with hook support
- **Skill System**: Reusable, configurable AI capabilities in `skills/` directory
- **Sandbox System**: Safe command execution with resource limits and restrictions

## Core Architecture

### Agent Loop (TAOR Pattern)
The agent loop implements Think-Act-Observe-Repeat:
- `internal/agent/loop.go`: Core agent loop with streaming support
- `internal/agent/orchestrator.go`: Multi-agent orchestration with specialized agent types
- `internal/agent/compactor.go`: History compaction for context management with real token tracking
- `internal/agent/classifier.go`: Intent classification for semantic analysis

### Multi-Agent System
The orchestrator manages specialized agents:
- **Explorer**: Codebase navigation and search (read-only)
- **Planner**: Task breakdown and planning (read-only)
- **Architect**: System design and interfaces
- **Coder**: Code implementation and modification
- **Debugger**: Bug analysis and fixing
- **Tester**: Test writing and verification
- **Reviewer**: Code quality review (read-only)
- **Writer**: Documentation creation

### Intent Classification
The `IntentClassifier` (`internal/agent/classifier.go`) performs semantic analysis to:
- Determine if a request needs single or multiple agents
- Identify task dependencies for coordinated execution
- Support both independent parallel tasks and dependent sequential tasks
- Cache classification results for efficiency

### Tool System
Tools are registered with the `ToolManager` and called by the AI:
- `internal/tools/tool.go`: Tool interface definition
- `internal/tools/manager.go`: Tool registration and execution with plugin hook support
- Tool categories: Basic, Write, LSP, Git

**Tool Categories:**
- `CategoryBasic`: Always loaded (read_file, glob, grep, web_search, web_fetch)
- `CategoryWrite`: Write operations (write_file, edit_file, bash)
- `CategoryLSP`: LSP features (hover, definition, references, completion)
- `CategoryGit`: Git operations (status, log, diff)

### Web Tools
The project includes built-in web capabilities:
- `web_search`: Search the web using DuckDuckGo (no API key required)
- `web_fetch`: Fetch and convert web pages to markdown

### LLM Integration
- `internal/llm/client.go`: Client interface
- `internal/llm/anthropic.go`: Anthropic API implementation with streaming
- Supports extended thinking via `thinking_delta` events
- Automatic retry with exponential backoff (max 3 retries)
- Real token tracking via API response headers

### Configuration Hierarchy
Configuration loads in order (later overrides earlier):
1. **Embedded defaults**: Default values are set in code (`internal/config/config.go`), NOT from `configs/default.yaml`
2. `~/.ycode/config.yaml` - Global user configuration
3. `.ycode/project.yaml` - Project-specific configuration
4. Environment variables (`ANTHROPIC_API_KEY`, `YCODE_*`)

## Key Files and Entry Points

| File | Purpose |
|------|---------|
| `cmd/ycode/main.go` | Application entry point, initializes Cobra commands |
| `internal/app/app.go` | Main TUI application, command setup, and initialization |
| `internal/agent/loop.go` | TAOR agent loop with streaming support |
| `internal/agent/orchestrator.go` | Multi-agent orchestration |
| `internal/agent/classifier.go` | Intent classification for semantic analysis |
| `internal/agent/compactor.go` | Conversation compaction with token tracking |
| `internal/config/config.go` | Configuration loading and management |
| `internal/llm/anthropic.go` | LLM API client for Anthropic protocol |
| `internal/tools/manager.go` | Tool registration and execution |
| `internal/lsp/client.go` | LSP client with JSON-RPC 2.0 protocol support |

## Development Guidelines

### Code Organization
- Follow standard Go project layout with `cmd/` and `internal/` packages
- Each `internal/` package has a single responsibility
- Keep package APIs minimal and focused

### TUI Development (Bubble Tea)
- Models implement `tea.Model` interface (`Init`, `Update`, `View`)
- Use `lipgloss` for styling, `bubbles` for UI components
- Messages are defined as types in each package

### Adding New Tools
1. Create tool in `internal/tools/` implementing the `Tool` interface
2. Define `Name()`, `Description()`, `Parameters()`, `Execute()`, `Category()`
3. Register in `internal/app/app.go` during initialization
4. Tool will be automatically available to the AI

**Example Tool Structure:**
```go
type MyTool struct {
    workDir string
}

func (t *MyTool) Name() string { return "my_tool" }
func (t *MyTool) Description() string { return "Description of tool" }
func (t *MyTool) Category() ToolCategory { return CategoryBasic }
func (t *MyTool) Parameters() []Parameter { /* ... */ }
func (t *MyTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) { /* ... */ }
```

### Skills
Skills are reusable AI capabilities stored in `skills/` directory:
- Each skill has a `SKILL.md` with YAML frontmatter defining triggers and commands
- Skills can be loaded dynamically from multiple directories:
  - `skills/` - Built-in skills
  - `~/.ycode/skills/` - User global skills
  - `.ycode/skills/` - Project-specific skills

**Available Skills:**
- `configure-lsp`: Manage LSP servers
- `create-skill`: Create new skills
- `git-master`: Git workflow automation
- `manage-mcp`: MCP server management
- `manage-plugins`: Plugin management

## Dependencies

| Library | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework with subcommands |
| `github.com/spf13/viper` | Configuration management |
| `github.com/charmbracelet/bubbletea` | TUI framework (Elm architecture) |
| `github.com/charmbracelet/lipgloss` | Styling and layout for TUI |
| `github.com/charmbracelet/bubbles` | Pre-built TUI components |
| `github.com/charmbracelet/glamour` | Markdown rendering |
| `gopkg.in/yaml.v3` | YAML parsing |

## Configuration Example

```yaml
llm:
  api_key: ${ANTHROPIC_API_KEY}
  model: claude-sonnet-4-20250514
  max_tokens: "128k"  # Supports human-readable formats (4k, 128k, 1M)

ui:
  theme: auto
  streaming: true

permissions:
  mode: confirm  # confirm, auto, deny

agent:
  max_steps: 10
  auto_compact: true
  compact_threshold: 0.8
  multi_agent:
    enabled: true
    max_agents: 5

mcp:
  servers:
    - name: example
      command: mcp-server
      args: []
      enabled: true

lsp:
  servers:
    - name: gopls
      command: gopls
      args: ["serve"]
      enabled: true

plugins:
  enabled: true
  directory: plugins/
  hot_reload: true
```

## Important Implementation Notes

### Streaming Response Handling
The LLM client streams responses via SSE (Server-Sent Events):
- `message_start`: Contains initial token usage
- `content_block_delta`: Text content and thinking deltas
- `message_delta`: Final output token count
- Handle `thinking_delta` events for extended thinking support

### Permission System
Tools check permissions before execution:
- `strict` mode: All operations require confirmation
- `normal` mode: Safe operations allowed, dangerous ones need confirmation
- `permissive` mode: Most operations allowed automatically
- Permission rules can target specific tools and file patterns
- Use `--yolo` flag to auto-approve all tool executions

### Parallel Tool Execution
Read-only tools are executed in parallel when multiple tool calls are received:
- Read-only tools: `read_file`, `glob`, `grep`, `git_status`, `git_log`, `git_diff`, `lsp_*`, `web_search`, `web_fetch`
- Write tools are executed sequentially

### Plugin Hooks
Tools can trigger plugin hooks:
- `on_tool_execute`: Called before tool execution (can modify args or skip)
- `on_tool_complete`: Called after tool execution (async)

### MCP Integration
Model Context Protocol servers can be configured to provide additional tools:
- Supports stdio-based MCP servers (JSON-RPC 2.0)
- Tools from MCP servers are automatically registered
- Use `manage-mcp` skill for configuration

### MCP OAuth Support
The OAuth module (`internal/mcp/oauth.go`) provides OAuth 2.0 authentication for MCP servers:
- Supports PKCE (Proof Key for Code Exchange) for public clients
- Token persistence with automatic refresh
- Browser-based authorization flow with callback server
- Token storage in `~/.ycode/oauth_tokens.json`

### LSP Integration
The LSP client (`internal/lsp/client.go`) provides:
- Full JSON-RPC 2.0 protocol support with Content-Length headers
- File synchronization (didOpen, didChange, didSave, didClose)
- Code intelligence: hover, definition, references, completion
- Diagnostics handling with callbacks
- File watcher for automatic document synchronization
- Support for multiple language servers

### Compaction System
The compactor (`internal/agent/compactor.go`) provides:
- Real token tracking from API responses
- Threshold-based auto-compaction (default 75%)
- LLM-generated summaries in Chinese
- Smart compaction with configurable keepRecent
- Quick summary fallback for fast compaction

### Diff Utility
The diff module (`internal/tools/diff.go`) provides:
- LCS (Longest Common Subsequence) based diff computation
- Line-based diff with add/delete/equal operations
- ANSI-colored and plain text formatting
- Statistics (additions, deletions, unchanged)

### Sandbox System
The sandbox (`internal/sandbox/`) provides safe command execution:
- Resource limits: timeout, memory, file size
- Network access control
- Path restrictions (allowed/blocked paths)
- Command blocking for dangerous operations
- Environment variable protection (API keys, secrets)
- Platform-specific implementations (Unix/Windows)

### Audit System
The audit system (`internal/audit/`) tracks sensitive operations:
- Logs all file writes, shell commands, and permission decisions
- Sensitive operation detection and confirmation flow
- YOLO mode for auto-approving all operations

### Plan Mode
The orchestrator supports a plan mode workflow (`internal/agent/workflow.go`):
1. `GeneratePlan`: LLM creates execution plan with steps, files, commands, risk assessment
2. User reviews and can modify/reject steps
3. `ExecutePlan`: Executes approved steps with appropriate agent
4. Supports iterative plan refinement based on feedback

### ActionBar Component
The ActionBar (`internal/ui/action_bar.go`) is a reusable TUI component for user decisions:
- Used in plan mode for confirming/modifying/canceling plans
- Used in audit system for approving sensitive operations
- Supports keyboard shortcuts (y/n/m) and visual styling
- Includes input mode for modification suggestions

### Sandbox Documentation
See `docs/sandbox-guide.md` for detailed sandbox configuration guide (in Chinese):
- Security features overview
- Configuration options explained
- Usage scenarios and best practices
- Troubleshooting tips

## Code Style Guidelines

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
| Functions | PascalCase exported, camelCase unexported | `func NewBashTool()`, `func execute()` |
| Variables | camelCase | `workingDir`, `maxLines` |
| Constants | PascalCase exported, camelCase unexported | `ErrCodeConfigLoad`, `defaultTimeout` |
| Interfaces | PascalCase, -er suffix preferred | `Client`, `Tool` |
| Structs | PascalCase | `AnthropicClient`, `YCodeError` |

### Error Handling
Use structured errors from `internal/errors/errors.go`:
```go
return nil, errors.New(errors.ErrCodeToolExecution, "failed to execute")
return nil, errors.Wrap(errors.ErrCodeFileRead, "cannot read config", err)
if errors.IsErrorCode(err, errors.ErrCodeToolTimeout) { ... }
```

**Rules**: Always handle errors explicitly. Use `fmt.Errorf("context: %w", err)` for wrapping. Return `(*ToolResult, error)` where `IsError` signals tool failures.

### Context Usage
Always pass `context.Context` as first parameter. Use `context.WithTimeout` for time-limited operations.

### Testing Patterns
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
```

## Security Considerations

- **Path traversal**: Always validate paths are within working directory
- **Command restrictions**: Maintain deny lists for dangerous commands
- **File size limits**: Enforce maximum file read sizes (10MB default)
- **Context timeouts**: All external operations must have timeouts
- **Binary detection**: Reject binary files when reading as text

## Prerequisites

- Go 1.25.0+ (as specified in go.mod)
- Git

## Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test ./internal/tools/...

# Run a specific test
go test -v ./internal/tools/... -run TestBash

# Run tests with coverage
go test -cover ./...
```