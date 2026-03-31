# ycode Agent Guidelines

Go AI coding assistant with TAOR pattern and multi-agent system.

## Build & Test

```bash
go build -v ./...       # Build
go test -v ./...        # Test all
go test -cover ./...    # Coverage
```

## Code Style

- **Imports**: stdlib → third-party → internal (blank line separated)
- **Naming**: PascalCase exported, camelCase unexported, interfaces with `-er` suffix
- **Errors**: Use `internal/errors` package
- **Context**: Always first parameter, use timeout for external ops

## Architecture

```
internal/
├── agent/    # TAOR loop, orchestration, classification
├── llm/      # LLM clients
├── tools/    # Tool definitions
└── ui/       # TUI components
```

**Multi-Agent**: Explorer/Planner/Reviewer (read-only), Coder/Debugger/Tester (read-write)

**Tool Categories**: Basic (always), Write, LSP, Git

## Adding Tools

```go
type MyTool struct{ workDir string }

func (t *MyTool) Name() string         { return "my_tool" }
func (t *MyTool) Category() ToolCategory { return CategoryBasic }
func (t *MyTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
    return &ToolResult{Content: "result"}, nil
}
```

Register in `internal/app/app.go`.

## Security

Validate paths, block dangerous commands, enforce file limits (10MB), use timeouts, reject binaries.