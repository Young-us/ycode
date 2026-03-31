# Architecture Overview

This document provides a detailed overview of ycode's architecture and design.

## Directory Structure

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
├── ast/           # Abstract Syntax Tree utilities
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
│   ├── manager.go       # Tool registration and execution
│   ├── permission_checker.go  # Unified permission checking
│   ├── read.go          # read_file tool
│   ├── write.go         # write_file tool
│   ├── edit.go          # edit_file tool
│   ├── bash.go          # bash command execution
│   ├── glob.go          # glob pattern matching
│   ├── grep.go          # grep content search
│   ├── git.go           # git operations
│   ├── lsp.go           # LSP tool
│   ├── web_search.go    # DuckDuckGo search
│   ├── web_fetch.go     # Web page fetch
│   └── diff.go          # LCS-based diff computation
└── ui/            # UI components (Bubble Tea + Lipgloss)
    └── action_bar.go    # Reusable action bar
```

## Core Design Patterns

### 1. Bubble Tea (Elm Architecture)

The TUI is built on the Model-Update-View pattern:

```go
type Model struct {
    // Application state
}

func (m Model) Init() tea.Cmd {
    // Initial commands
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Handle messages and update state
}

func (m Model) View() string {
    // Render the UI
}
```

### 2. Tool System

Tools are registered with the ToolManager and called by the AI:

```go
type Tool interface {
    Name() string
    Description() string
    Category() ToolCategory
    Parameters() []Parameter
    Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error)
}
```

**Tool Categories:**
- `CategoryBasic`: Always loaded (read, glob, grep, web)
- `CategoryWrite`: Write operations (write, edit, bash)
- `CategoryLSP`: LSP features (hover, definition, references)
- `CategoryGit`: Git operations (status, log, diff)

### 3. Plugin Architecture

Plugins extend functionality with hooks:

```go
type Hook interface {
    OnToolExecute(ctx context.Context, tool Tool, args map[string]interface{})
    OnToolComplete(ctx context.Context, result *ToolResult)
}
```

### 4. Skill System

Skills are reusable AI capabilities in `skills/` directory:

```
skills/
├── configure-lsp/SKILL.md
├── create-skill/SKILL.md
├── git-master/SKILL.md
├── manage-mcp/SKILL.md
└── manage-plugins/SKILL.md
```

## Agent System (TAOR Pattern)

The agent loop implements **Think-Act-Observe-Repeat**:

1. **Think**: Agent receives user input and context
2. **Act**: Agent decides which tools to use
3. **Observe**: Agent processes tool results
4. **Repeat**: Agent continues until task is complete

### Multi-Agent Orchestration

The orchestrator manages specialized agents:

| Agent | Role | Capabilities |
|-------|------|--------------|
| Explorer | Codebase navigation | Read-only, search, explore |
| Planner | Task breakdown | Read-only, plan, organize |
| Architect | System design | Design interfaces, structure |
| Coder | Implementation | Write, edit, refactor |
| Debugger | Bug analysis | Analyze, fix, test |
| Tester | Test writing | Write tests, verify |
| Reviewer | Code review | Read-only, review, suggest |
| Writer | Documentation | Write docs, comments |

### Intent Classification

The `IntentClassifier` analyzes user requests to:
- Determine if a request needs single or multiple agents
- Identify task dependencies for coordinated execution
- Support both independent parallel tasks and dependent sequential tasks

## Data Flow

```
User Input → App → Agent Loop → LLM → Tool Selection → Tool Execution → Result → Agent Loop → Response
                ↑                                                                    ↓
                └──────────────── Context Management ←──────────────────────────────┘
```

## Security Model

### Permission Levels

1. **Strict**: All operations require confirmation
2. **Normal**: Safe operations allowed, dangerous ones need confirmation
3. **Permissive**: Most operations allowed automatically

### Sandbox Execution

The sandbox provides:
- Resource limits (timeout, memory, file size)
- Network access control
- Path restrictions
- Command blocking
- Environment variable protection

### Audit Logging

All sensitive operations are logged:
- File writes
- Shell commands
- Permission decisions

## Configuration Hierarchy

Configuration loads in order (later overrides earlier):

1. **Embedded defaults** - Default values in code
2. `~/.ycode/config.yaml` - Global user configuration
3. `.ycode/project.yaml` - Project-specific configuration
4. Environment variables (`ANTHROPIC_API_KEY`, `YCODE_*`)

## Performance Considerations

### Parallel Tool Execution

Read-only tools are executed in parallel when multiple tool calls are received:
- Read-only: `read_file`, `glob`, `grep`, `git_*`, `lsp_*`, `web_*`
- Write tools: Executed sequentially

### Context Compaction

The compactor automatically summarizes conversation history when:
- Token usage exceeds threshold (default 80%)
- LLM generates summaries for old messages
- Recent messages are preserved

### Caching

- Intent classification results are cached
- File reads are cached for short durations
- LSP diagnostics are cached per file