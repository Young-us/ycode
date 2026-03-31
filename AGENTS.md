# ycode Agent Guidelines

Go AI coding assistant with TAOR pattern and multi-agent system.

## Build & Test

```bash
go build -v ./...       # Build
go test -v ./...        # Test
```

## Style

- **Imports**: stdlib → third-party → internal
- **Naming**: PascalCase (exported), camelCase (unexported)
- **Errors**: Use `internal/errors` package
- **Context**: First parameter, add timeout for external ops

## Architecture

```
internal/
├── agent/    # TAOR loop, orchestration
├── llm/      # LLM clients
├── tools/    # Tool definitions
└── ui/       # TUI components
```

**Agents**: Explorer/Planner/Reviewer (read-only), Coder/Debugger/Tester (read-write)
**Tools**: Basic (always), Write, LSP, Git

## Adding Tools

```go
func (t *MyTool) Name() string           { return "my_tool" }
func (t *MyTool) Category() ToolCategory { return CategoryBasic }
func (t *MyTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
    return &ToolResult{Content: "result"}, nil
}
```

Register in `internal/app/app.go`.

## Security

Validate paths, block dangerous commands, 10MB file limit, use timeouts.