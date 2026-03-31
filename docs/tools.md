# Tools System

The ycode tools system provides a rich set of tools for AI agents to interact with the codebase, execute commands, and access external resources.

## Tool Categories

### CategoryBasic

Always loaded tools for read-only operations:

- **read_file**: Read file contents with line range support
- **glob**: Find files matching glob patterns
- **grep**: Search file contents with regex support
- **git_status**: Check git repository status
- **git_log**: View git commit history
- **git_diff**: View git diffs
- **web_search**: Search the web (DuckDuckGo)
- **web_fetch**: Fetch web pages and convert to markdown
- **lsp**: Language server protocol operations

### CategoryWrite

Tools for write operations:

- **write_file**: Create or overwrite files
- **edit_file**: Apply targeted edits to files
- **bash**: Execute shell commands

### CategoryLSP

Language server protocol features:

- **hover**: Show type information
- **definition**: Go to definition
- **references**: Find all references
- **completion**: Get code completions

### CategoryGit

Git operations:

- **status**: Repository status
- **log**: Commit history
- **diff**: View changes

## Tool Interface

All tools implement the `Tool` interface:

```go
type Tool interface {
    Name() string
    Description() string
    Category() ToolCategory
    Parameters() []Parameter
    Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error)
}
```

### Parameter Definition

```go
type Parameter struct {
    Name        string   `json:"name"`
    Type        string   `json:"type"`
    Description string   `json:"description"`
    Required    bool     `json:"required"`
    Enum        []string `json:"enum,omitempty"`
}
```

### Tool Result

```go
type ToolResult struct {
    Content string `json:"content"`
    IsError bool   `json:"is_error"`
}
```

## Adding a New Tool

### 1. Create Tool File

Create a new file in `internal/tools/`:

```go
// internal/tools/my_tool.go
package tools

import (
    "context"
    "github.com/Young-us/ycode/internal/errors"
)

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
    return CategoryBasic
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
    
    // Implement tool logic here
    
    return &ToolResult{
        Content: "result",
        IsError: false,
    }, nil
}
```

### 2. Register Tool

Register the tool in `internal/app/app.go`:

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

## Parallel Execution

Read-only tools are executed in parallel when multiple tool calls are received:

- **Parallel**: read_file, glob, grep, git_status, git_log, git_diff, lsp_*, web_search, web_fetch
- **Sequential**: write_file, edit_file, bash

## Permission Checking

Tools use the unified permission checker:

```go
func (t *MyTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
    // Check permission
    if err := t.permissionChecker.Check(ctx, PermissionRead, path); err != nil {
        return nil, err
    }
    
    // Execute tool logic
    // ...
}
```

Permission modes:

- **strict**: All operations require confirmation
- **normal**: Safe operations allowed, dangerous ones need confirmation
- **permissive**: Most operations allowed automatically

## Plugin Hooks

Tools can trigger plugin hooks:

- `on_tool_execute`: Called before tool execution (can modify args or skip)
- `on_tool_complete`: Called after tool execution (async)

## MCP Tools

Tools from MCP servers are automatically registered:

1. MCP server provides tool definitions
2. Tool manager registers MCP tools
3. AI can use MCP tools like built-in tools

## Error Handling

Tools should use structured errors:

```go
import "github.com/Young-us/ycode/internal/errors"

// Create new error
return nil, errors.New(errors.ErrCodeToolExecution, "failed to execute")

// Wrap error
return nil, errors.Wrap(errors.ErrCodeFileRead, "cannot read file", err)

// Check error type
if errors.IsErrorCode(err, errors.ErrCodeToolTimeout) {
    // Handle timeout
}
```

## Security Considerations

- **Path validation**: Ensure paths are within working directory
- **Command restrictions**: Block dangerous commands
- **File size limits**: Maximum file read size (10MB default)
- **Context timeout**: All operations have timeouts
- **Binary detection**: Reject binary files when reading as text