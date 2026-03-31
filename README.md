# ycode

An AI-powered terminal coding assistant with multi-agent system.

## Features

- 🤖 **Multi-Agent System** - 8 specialized agents (Explorer, Planner, Architect, Coder, Debugger, Tester, Reviewer, Writer)
- 🔧 **Rich Tool Ecosystem** - File operations, shell commands, git, LSP, web search
- 🔒 **Security First** - Sandbox execution, permission control, audit logging
- 🔌 **Extensible** - MCP protocol, plugins, skills system
- 💻 **Modern TUI** - Streaming responses, syntax highlighting

## Quick Start

```bash
# Build
go build -o ycode ./cmd/ycode

# Run
./ycode chat

# Or with API key
ANTHROPIC_API_KEY=your_key ./ycode chat
```

## Configuration

Configuration files (priority: later overrides earlier):
1. `~/.ycode/config.yaml` - Global config
2. `.ycode/project.yaml` - Project config
3. Environment variables (`ANTHROPIC_API_KEY`, `YCODE_*`)

Minimal configuration:
```yaml
llm:
  api_key: ${ANTHROPIC_API_KEY}
  model: claude-sonnet-4-20250514
```

See [configs/example.yaml](configs/example.yaml) for full configuration options.

## Documentation

- [Architecture](docs/architecture.md) - System design and components
- [Configuration](docs/configuration.md) - Detailed configuration guide
- [Tools](docs/tools.md) - Available tools and usage
- [Multi-Agent](docs/multi-agent.md) - Agent types and orchestration
- [Security](docs/security.md) - Sandbox and permissions
- [Development](docs/development.md) - Build, test, and contribute

## Requirements

- Go 1.25+
- Anthropic API key

## License

MIT