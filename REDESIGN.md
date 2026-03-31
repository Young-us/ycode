# ycode Improvement Plan

Based on the analysis of opencode, the following are key areas for improvement.

## 1. Agent System Improvements

### 1.1 Multi-Agent Types
opencode implements multiple Agent types:
- `build`: Default Agent, executes tools
- `plan`: Plan mode, disabled edit tools
- `explore`: Fast codebase exploration
- `general`: General research Agent
- `compaction`: Context compression Agent
- `title`: Title generation Agent
- `summary`: Summary generation Agent

### 1.2 Permission System
opencode uses PermissionNext for fine-grained permission control:
```typescript
permission: PermissionNext.merge(
  defaults,
  PermissionNext.fromConfig({
    question: "allow",
    plan_enter: "allow",
  }),
  user,
)
```

Permission types: `allow`, `deny`, `ask`

### 1.3 Agent State Management
Use Instance.state() to manage Agent configuration, support hot reload

## 2. Skill System Improvements

### 2.1 Skill Directory Structure
Support multiple Skill directories:
- `~/.claude/skills/`
- `~/.agents/skills/`
- `.claude/` or `.agents/` under project directory
- Configured directories

### 2.2 Skill Permission Filtering
Filter available Skills based on current Agent permissions

### 2.3 Skill Discovery Mechanism
```go
// Async loading with cache
type State struct {
    skills   map[string]*Skill
    dirs     []string
    task     *sync.Once
    mu       sync.RWMutex
}
```

## 3. MCP Improvements

### 3.1 MCP Client State Management
```go
type MCPClient struct {
    Name    string
    Client  *mcp.Client
    Status  MCPStatus
    Tools   []ToolDefinition
    Timeout time.Duration
}
```

### 3.2 MCP Status Types
- `connected`: Connected
- `disabled`: Disabled
- `failed`: Connection failed
- `needs_auth`: Requires authentication
- `needs_client_registration`: Requires client registration

### 3.3 OAuth Support
- Support OAuth authentication for remote MCP servers
- Browser authorization flow
- Token storage and refresh

### 3.4 MCP Resource Management
- MCP Prompts: Can be invoked as commands
- MCP Resources: Readable resources
- Tool conversion: MCP Tool -> ycode Tool

## 4. LSP Improvements

### 4.1 LSP Server Management
```go
type LSPServer struct {
    Name      string
    Command   string
    Args      []string
    Enabled   bool
    Transport LSPTransport
}
```

### 4.2 LSP Features
- Diagnostics
- Hover information
- Goto Definition
- Find References
- Code Completion

## 5. Plugin System Improvements

### 5.1 Hook System
```go
type PluginHook interface {
    OnChatStart(ctx context.Context, session *Session)
    OnChatComplete(ctx context.Context, session *Session)
    OnToolExecute(ctx context.Context, tool Tool, args map[string]interface{})
    OnError(ctx context.Context, err error)
}
```

### 5.2 Plugin Lifecycle
1. Discovery: Discover plugins
2. Load: Load plugins
3. Init: Initialize plugins
4. Shutdown: Shutdown plugins

### 5.3 Built-in Plugins
- Codex Auth Plugin
- Copilot Auth Plugin
- GitLab Auth Plugin

## 6. Command System Improvements

### 6.1 Command Sources
- Built-in: Built-in commands
- MCP: MCP Prompts as commands
- Skill: Skills as commands
- Config: Commands from config file

### 6.2 Command Template Variables
- `$1, $2, ...`: Positional arguments
- `$ARGUMENTS`: All arguments
- Environment variables `$ENV_VAR`

### 6.3 Command Attributes
```go
type Command struct {
    Name        string
    Description string
    Agent       string      // Agent to use
    Model       string      // Model to use
    Source      string      // "command", "mcp", "skill"
    Subtask     bool        // Whether to run as subtask
    Hints       []string    // Argument hints
    Template    string      // Template content
}
```

## 7. UI Improvements

### 7.1 Improvement Areas
- Streaming text display
- Command palette (Cmd/Ctrl+P)
- Better error display
- Keyboard shortcuts
- MCP/LSP status indicators
- Token usage statistics

### 7.2 Status Bar Information
- Model information
- Token usage
- MCP server status
- LSP server status
- Agent mode

## 8. Implementation Priority

### Phase 1: Core Architecture
1. ✅ Agent system multi-type support
2. ✅ Permission system
3. ✅ Command system improvements

### Phase 2: Integration
1. MCP client complete implementation
2. LSP client complete implementation
3. Skill system enhancement

### Phase 3: Advanced Features
1. ✅ Plugin Hook system - Implemented in `internal/plugin/hooks.go` and `internal/plugin/manager.go`
2. ✅ UI enhancement - Implemented in `internal/ui/` directory:
   - `statusbar.go` - Status bar with model info, token usage, MCP/LSP status, agent mode
   - `command_palette.go` - Command palette (Cmd/Ctrl+P) for quick command execution
   - `streaming.go` - Streaming text display for real-time AI responses
   - `error_display.go` - Better error display with severity levels and stack traces
   - `shortcuts.go` - Keyboard shortcuts management
   - `tokens.go` - Token usage statistics display
   - `components.go` - UI components manager for integration
3. ✅ OAuth support - Implemented in `internal/mcp/oauth.go`:
   - OAuth flow for remote MCP servers
   - Browser authorization flow
   - Token storage and refresh
   - Multiple provider support (GitHub, GitLab, Google, Microsoft)

## 9. File Structure

```
internal/
├── agent/
│   ├── loop.go           # TAOR loop
│   ├── agent.go         # Agent definition
│   ├── permission.go    # Permission system
│   └── compact.go       # Compression Agent
├── command/
│   ├── command.go       # Command definition
│   ├── manager.go       # Command manager
│   ├── builtin.go       # Built-in commands
│   └── template/        # Command template
├── mcp/
│   ├── client.go        # MCP client
│   ├── oauth.go         # OAuth support ✅
│   └── tools.go         # Tool conversion
├── lsp/
│   ├── client.go        # LSP client
│   ├── diagnostics.go   # Diagnostics
│   └── completions.go   # Completion
├── skill/
│   ├── skill.go        # Skill definition
│   ├── discovery.go    # Skill discovery
│   └── permission.go   # Skill permission
├── plugin/
│   ├── plugin.go       # Plugin interface
│   ├── hooks.go        # Hook definition ✅
│   └── manager.go      # Plugin manager ✅
└── ui/
    ├── tui.go          # TUI main program
    ├── statusbar.go    # Status bar ✅
    ├── command_palette.go  # Command palette ✅
    ├── streaming.go    # Streaming text ✅
    ├── error_display.go    # Error display ✅
    ├── shortcuts.go    # Keyboard shortcuts ✅
    ├── tokens.go       # Token statistics ✅
    ├── components.go   # Component manager ✅
    └── styles/         # Styles
```

## 10. Phase 3 Implementation Summary

### Plugin Hook System (Already Complete)
The plugin hook system was already implemented in Phase 1 and is fully functional:
- **File**: `internal/plugin/hooks.go` - Defines all available hooks
- **File**: `internal/plugin/manager.go` - Hook registration and execution
- **Hooks Available**:
  - `on_chat_start` - Triggered when chat session begins
  - `on_chat_complete` - Triggered when chat response is generated
  - `on_tool_execute` - Triggered before tool execution
  - `on_tool_complete` - Triggered after tool execution
  - `on_error` - Triggered when error occurs
  - `on_agent_switch` - Triggered when switching agents
  - `on_file_read` - Triggered after reading file
  - `on_file_write` - Triggered before writing file
  - `on_command_execute` - Triggered before shell command
  - `on_startup` - Triggered on app startup
  - `on_shutdown` - Triggered on app shutdown

### UI Enhancement (New)
Created comprehensive UI components in `internal/ui/`:
1. **Status Bar** (`statusbar.go`)
   - Model information display
   - Token usage statistics (input/output/total)
   - MCP server status indicators (connected/disabled/failed)
   - LSP server status indicators
   - Agent mode display
   
2. **Command Palette** (`command_palette.go`)
   - Quick command execution via Cmd/Ctrl+P
   - Search/filter commands
   - Built-in commands integration
   - Keyboard navigation
   
3. **Streaming Text Display** (`streaming.go`)
   - Real-time streaming of AI responses
   - Cursor animation during streaming
   - Speed tracking and display
   - Line-by-line streaming support
   
4. **Error Display** (`error_display.go`)
   - Severity levels (info/warning/error/fatal)
   - Stack trace display
   - Error code classification
   - Timestamp display
   
5. **Keyboard Shortcuts** (`shortcuts.go`)
   - Global shortcut registry
   - Context-aware shortcuts
   - Shortcut management (register/unregister)
   - Key combination parsing
   
6. **Token Statistics** (`tokens.go`)
   - Session token tracking
   - Total usage display
   - Percentage-based progress bar
   - Cost estimation
   
7. **Components Manager** (`components.go`)
   - Centralized UI component management
   - Integration of all UI elements
   - Status bar updates
   - Streaming control

### OAuth Support (New)
Created OAuth implementation in `internal/mcp/oauth.go`:
- **OAuth Flow**: Full OAuth 2.0 authorization flow
- **Browser Launch**: Automatic browser launch for authorization
- **Token Storage**: Secure token storage with file permissions
- **Token Refresh**: Automatic token refresh when expired
- **Multiple Providers**: Support for GitHub, GitLab, Google, Microsoft
- **Local Server**: Temporary local server for OAuth callback
- **MCP Integration**: Seamless integration with MCP client

## 11. Usage Examples

### Using OAuth for MCP
```go
// Create OAuth manager
oauthMgr := mcp.NewOAuthManager()

// Configure provider
provider := mcp.ProviderGitHub
config := mcp.OAuthConfig{
    Provider:     provider,
    ClientID:     "your-client-id",
    ClientSecret: "your-client-secret",
    RedirectURL:  "http://localhost:8080/callback",
    Scopes:       []string{"repo", "user"},
}

// Start OAuth flow
token, err := oauthMgr.StartFlow(ctx, config)

// Use token with MCP client
client.SetOAuthToken(token)
```

### Using Status Bar
```go
// Create status bar
sb := ui.NewStatusBar()

// Set model info
sb.SetModel("claude-sonnet-4-20250514")

// Set token usage
sb.SetTokens(1500, 800)

// Set MCP status
sb.SetMCPStatus(map[string]ui.ServerStatus{
    "github": ui.StatusConnected,
    "gitlab": ui.StatusFailed,
})

// Set LSP status
sb.SetLSPStatus(map[string]ui.ServerStatus{
    "gopls": ui.StatusConnected,
})
```

### Using Command Palette
```go
// Create command palette
cp := ui.NewCommandPalette()

// Register commands
cp.RegisterCommand("exit", "Exit application", func() { app.Quit() })
cp.RegisterCommand("help", "Show help", func() { app.ShowHelp() })

// Open command palette (Cmd/Ctrl+P)
cp.Open()
```

### Using Streaming Display
```go
// Create streaming display
sd := ui.NewStreamingDisplay()

// Start streaming
sd.Start()

// Add content chunks
sd.AddChunk("Hello, ")
sd.AddChunk("world!")

// Stop streaming
sd.Stop()
```

## 12. Testing

All Phase 3 components should be tested:

```bash
# Test OAuth implementation
go test ./internal/mcp/oauth_test.go

# Test UI components
go test ./internal/ui/...

# Test integration
go test ./internal/...
```

## 13. Next Steps

Phase 3 is now **COMPLETE**. All advanced features have been implemented:
- ✅ Plugin Hook System
- ✅ UI Enhancement
- ✅ OAuth Support

Future improvements can focus on:
- Performance optimization
- Additional OAuth providers
- Advanced UI features (split panes, multi-tab)
- Plugin marketplace
- Skill templates