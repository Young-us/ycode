# Configuration Guide

This document provides detailed configuration options for ycode.

## Configuration Hierarchy

Configuration loads in order (later overrides earlier):

1. **Embedded defaults** - Default values in code (`internal/config/config.go`)
2. **Global config** - `~/.ycode/config.yaml`
3. **Project config** - `.ycode/project.yaml`
4. **Environment variables** - `ANTHROPIC_API_KEY`, `YCODE_*`

## Configuration File

### Minimal Configuration

```yaml
llm:
  api_key: ${ANTHROPIC_API_KEY}
  model: claude-sonnet-4-20250514
```

### Full Configuration

```yaml
# LLM Configuration
llm:
  api_key: ${ANTHROPIC_API_KEY}
  model: claude-sonnet-4-20250514
  max_tokens: "128k"  # Supports human-readable formats: 4k, 128k, 1M
  temperature: 0.7
  base_url: https://api.anthropic.com  # Optional: custom API endpoint

# UI Configuration
ui:
  theme: auto  # auto, light, dark
  streaming: true  # Enable streaming responses
  show_tokens: true  # Show token usage

# Permission Configuration
permissions:
  mode: confirm  # confirm, auto, deny
  rules:
    - tool: bash
      action: confirm
      patterns:
        - "rm -rf *"
    - tool: write_file
      action: auto
      patterns:
        - "*.log"

# Agent Configuration
agent:
  max_steps: 10  # Maximum agent loop iterations
  auto_compact: true  # Auto-compact conversation history
  compact_threshold: 0.8  # Compact when context reaches 80% of max
  multi_agent:
    enabled: true  # Enable multi-agent system
    max_agents: 5  # Maximum concurrent agents

# MCP (Model Context Protocol) Servers
mcp:
  servers:
    - name: example
      command: mcp-server
      args: []
      env: {}
      enabled: true

# LSP (Language Server Protocol) Servers
lsp:
  servers:
    - name: gopls
      command: gopls
      args: ["serve"]
      enabled: true
    - name: typescript
      command: typescript-language-server
      args: ["--stdio"]
      enabled: true

# Plugin Configuration
plugins:
  enabled: true
  directory: plugins/
  hot_reload: true  # Auto-reload plugins on change

# Sandbox Configuration
sandbox:
  enabled: true
  timeout: 30s
  memory_limit: 512MB
  max_file_size: 10MB
  network: false  # Disable network access
  allowed_paths:
    - ./
  blocked_commands:
    - rm -rf /
    - dd
    - mkfs

# Logging Configuration
logging:
  level: info  # debug, info, warn, error
  file: ~/.ycode/ycode.log
  max_size: 10MB
  max_backups: 3
  max_age: 30  # days
  compress: true
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ANTHROPIC_API_KEY` | Anthropic API key | - |
| `YCODE_CONFIG` | Custom config file path | `~/.ycode/config.yaml` |
| `YCODE_MODEL` | Default model | `claude-sonnet-4-20250514` |
| `YCODE_THEME` | UI theme | `auto` |
| `YCODE_LOG_LEVEL` | Log level | `info` |

## Permission Modes

### Confirm Mode (Default)
All operations require user confirmation:
```yaml
permissions:
  mode: confirm
```

### Auto Mode
Most operations are allowed automatically (dangerous operations still need confirmation):
```yaml
permissions:
  mode: auto
```

### Deny Mode
All operations are denied by default:
```yaml
permissions:
  mode: deny
```

### YOLO Flag
Use `--yolo` flag to auto-approve all operations:
```bash
ycode chat --yolo
```

## Sandbox Configuration

The sandbox provides safe command execution with resource limits:

```yaml
sandbox:
  enabled: true
  timeout: 30s  # Maximum execution time
  memory_limit: 512MB  # Maximum memory usage
  max_file_size: 10MB  # Maximum file size for read/write
  network: false  # Disable network access
  
  # Path restrictions
  allowed_paths:
    - ./
    - /tmp
  blocked_paths:
    - ~/.ssh
    - ~/.gnupg
  
  # Blocked commands
  blocked_commands:
    - rm -rf /
    - dd
    - mkfs
    - fdisk
```

## LLM Configuration

### Supported Models

- `claude-sonnet-4-20250514` - Latest Claude Sonnet (recommended)
- `claude-3-5-sonnet-20241022` - Claude 3.5 Sonnet
- `claude-3-5-haiku-20241022` - Claude 3.5 Haiku

### Token Limits

```yaml
llm:
  max_tokens: "128k"  # Human-readable format
```

Supported formats:
- `4k` = 4,000
- `32k` = 32,000
- `128k` = 128,000
- `1m` = 1,000,000

### Temperature

```yaml
llm:
  temperature: 0.7  # 0.0 to 1.0
```

- `0.0` - Deterministic, focused
- `0.5` - Balanced
- `1.0` - Creative, diverse

## Multi-Agent Configuration

```yaml
agent:
  multi_agent:
    enabled: true
    max_agents: 5  # Maximum concurrent agents
    
    # Agent-specific settings
    explorer:
      model: claude-sonnet-4-20250514
      max_steps: 5
      
    planner:
      model: claude-sonnet-4-20250514
      max_steps: 3
      
    architect:
      model: claude-sonnet-4-20250514
      max_steps: 10
```

## MCP Server Configuration

### Local MCP Server

```yaml
mcp:
  servers:
    - name: local-server
      command: /path/to/mcp-server
      args: ["--port", "8080"]
      env:
        DEBUG: "1"
      enabled: true
```

### Remote MCP Server with OAuth

```yaml
mcp:
  servers:
    - name: github
      url: https://api.github.com/mcp
      oauth:
        provider: github
        client_id: your-client-id
        client_secret: your-client-secret
        scopes:
          - repo
          - user
      enabled: true
```

## LSP Server Configuration

### Go (gopls)

```yaml
lsp:
  servers:
    - name: gopls
      command: gopls
      args: ["serve"]
      enabled: true
```

### TypeScript

```yaml
lsp:
  servers:
    - name: typescript
      command: typescript-language-server
      args: ["--stdio"]
      enabled: true
```

### Python (pyright)

```yaml
lsp:
  servers:
    - name: pyright
      command: pyright-langserver
      args: ["--stdio"]
      enabled: true
```

### Rust (rust-analyzer)

```yaml
lsp:
  servers:
    - name: rust-analyzer
      command: rust-analyzer
      enabled: true
```

## Plugin Configuration

```yaml
plugins:
  enabled: true
  directory: plugins/
  hot_reload: true
  
  # Plugin-specific settings
  settings:
    my-plugin:
      enabled: true
      option: value
```

## Logging Configuration

```yaml
logging:
  level: info  # debug, info, warn, error
  file: ~/.ycode/ycode.log
  max_size: 10MB
  max_backups: 3
  max_age: 30  # days
  compress: true
```

## Example Configurations

### Development Environment

```yaml
llm:
  model: claude-sonnet-4-20250514
  
permissions:
  mode: auto
  
agent:
  max_steps: 20
  
sandbox:
  enabled: false  # Disable for development
```

### Production Environment

```yaml
llm:
  model: claude-sonnet-4-20250514
  
permissions:
  mode: confirm
  
agent:
  max_steps: 10
  
sandbox:
  enabled: true
  timeout: 30s
  network: false
```

### High-Security Environment

```yaml
permissions:
  mode: deny
  rules:
    - tool: read_file
      action: confirm
    - tool: bash
      action: deny
      
sandbox:
  enabled: true
  network: false
  allowed_paths:
    - ./
    
audit:
  enabled: true
  log_all_operations: true
```