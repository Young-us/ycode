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
go run ./cmd/ycode --help

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
├── app/           # Main application state and TUI (Bubble Tea)
├── ast/           # Abstract Syntax Tree utilities
├── audit/         # Audit logging system
├── command/       # CLI command definitions (Cobra)
├── config/        # Configuration management (Viper)
├── errors/        # Custom error types and handling
├── llm/           # LLM client abstraction (Anthropic API)
├── logger/        # Logging utilities
├── lsp/           # Language Server Protocol integration
├── mcp/           # Model Context Protocol support
├── plugin/        # Plugin architecture and loading
├── sandbox/       # Sandbox environment for safe execution
├── session/       # Session management
├── shell/         # Shell command execution
├── skill/         # Skill system (reusable AI capabilities)
├── tools/         # AI tools (file, shell, git, etc.)
└── ui/            # UI components (Bubble Tea + Lipgloss)
```

**Key Design Patterns:**
- **Bubble Tea (Elm Architecture)**: TUI built on Model-Update-View pattern
- **Tool System**: Extensible tools registered with the agent for AI capabilities
- **Plugin Architecture**: Loadable plugins for extending functionality
- **Skill System**: Reusable, configurable AI capabilities in `skills/` directory

## Core Architecture

### Agent Loop (TAOR Pattern)
The agent loop implements Think-Act-Observe-Repeat:
- `internal/agent/loop.go`: Core agent loop with streaming support
- `internal/agent/orchestrator.go`: Multi-agent orchestration with specialized agent types
- `internal/agent/compactor.go`: History compaction for context management

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

### Tool System
Tools are registered with the `ToolManager` and called by the AI:
- `internal/tools/tool.go`: Tool interface definition
- `internal/tools/manager.go`: Tool registration and execution
- Tool categories: Basic (read/glob/grep), Write, LSP, AST, Git

### LLM Integration
- `internal/llm/client.go`: Client interface
- `internal/llm/anthropic.go`: Anthropic API implementation with streaming
- Supports extended thinking via `thinking_delta` events
- Automatic retry with exponential backoff

### Configuration Hierarchy
Configuration loads in order (later overrides earlier):
1. `configs/default.yaml` - Default configuration
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
| `internal/config/config.go` | Configuration loading and management |
| `internal/llm/anthropic.go` | LLM API client for Anthropic protocol |
| `internal/tools/manager.go` | Tool registration and execution |
| `configs/default.yaml` | Default configuration file |

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

### Tool Categories
- `CategoryBasic`: Always loaded (read_file, glob, grep)
- `CategoryWrite`: Loaded on demand (write_file, edit_file, bash)
- `CategoryLSP`: LSP features (hover, definition, references)
- `CategoryAST`: AST operations
- `CategoryGit`: Git operations

### Skills
Skills are reusable AI capabilities stored in `skills/` directory:
- Each skill has a `SKILL.md` with YAML frontmatter defining triggers and commands
- Skills can be loaded dynamically from multiple directories
- See `skills/git-master/SKILL.md` for a comprehensive example

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
  max_tokens: "128k"  # Supports human-readable formats

ui:
  theme: auto
  streaming: true

permissions:
  mode: confirm  # confirm, auto, deny

agent:
  max_steps: 10
  auto_compact: true
  compact_threshold: 0.8

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

### Parallel Tool Execution
Read-only tools are executed in parallel when multiple tool calls are received:
- Read-only tools: `read_file`, `glob`, `grep`, `git_status`, `git_log`, `git_diff`, `lsp_*`, `ast_search`
- Write tools are executed sequentially