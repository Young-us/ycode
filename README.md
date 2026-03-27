# ycode

AI-powered coding assistant for the terminal.

[![CI](https://github.com/Young-us/ycode/actions/workflows/ci.yml/badge.svg)](https://github.com/Young-us/ycode/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Young-us/ycode)](https://goreportcard.com/report/github.com/Young-us/ycode)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Overview

ycode is an AI-powered coding assistant that helps you code faster through natural language commands. It understands your codebase, executes tasks, and integrates with your development workflow.

## Features

- 🤖 **AI-Powered**: Uses Claude and compatible AI models via Anthropic protocol
- 💻 **Terminal-First**: Rich TUI experience built with Bubble Tea
- 📁 **File Operations**: Read, write, and edit files with AI assistance
- 🔧 **Shell Integration**: Execute commands and scripts
- 🔄 **Git Workflow**: Commit, branch, and manage version control
- 🔌 **Extensible**: Plugin architecture for custom tools

## Installation

### From Source

```bash
go install github.com/Young-us/ycode/cmd/ycode@latest
```

### Pre-built Binaries

Download from [GitHub Releases](https://github.com/Young-us/ycode/releases)

### Package Managers

```bash
# Homebrew (macOS/Linux)
brew install Young-us/tap/ycode

# Scoop (Windows)
scoop bucket add Young-us https://github.com/Young-us/scoop-bucket
scoop install ycode
```

## Quick Start

### 1. Set up your API key

You can set your Anthropic API key in three ways:

**Option A: Environment Variable (Recommended)**
```bash
# Linux/macOS
export ANTHROPIC_API_KEY="your-api-key"

# Windows (Command Prompt)
set ANTHROPIC_API_KEY=your-api-key

# Windows (PowerShell)
$env:ANTHROPIC_API_KEY="your-api-key"
```

**Option B: Global Configuration File**
Create `~/.ycode/config.yaml`:
```yaml
llm:
  api_key: your-api-key
  model: claude-sonnet-4-20250514
  # max_tokens supports human-readable formats: 4096, "4k", "128k", "1M"
  max_tokens: "4k"
```

**Option C: Project Configuration File**
Create `.ycode/project.yaml` in your project root:
```yaml
llm:
  api_key: ${ANTHROPIC_API_KEY}
  max_tokens: "128k"
```

### 2. Start a chat session

```bash
ycode chat
```

### 3. Give instructions in natural language

```
> Read the main.go file and explain what it does
> Add error handling to the processFile function
> Run the tests and fix any failures
```

## Usage

### Commands

```bash
ycode chat              # Start interactive chat session
ycode chat --no-ui      # Run without TUI (simple text mode)
ycode chat --yolo       # Auto-approve all tool executions
ycode version           # Print version information
```

### Configuration

Create a `.ycode/project.yaml` in your project root:

```yaml
project:
  name: my-project
  
instructions: |
  This is a Go project.
  Follow Go best practices and conventions.
  
tools:
  bash:
    timeout: 60s
```

Global configuration: `~/.ycode/config.yaml`

```yaml
llm:
  provider: anthropic
  api_key: ${ANTHROPIC_API_KEY}
  model: claude-sonnet-4-20250514
  
ui:
  theme: auto
  streaming: true
```

## Architecture

```
┌─────────────────────────────────────────┐
│            CLI (Cobra)                  │
├─────────────────────────────────────────┤
│         TUI (Bubble Tea)                │
├─────────────────────────────────────────┤
│         Agent Loop (TAOR)               │
├─────────────────────────────────────────┤
│    Tools │ LLM Client │ Services       │
└─────────────────────────────────────────┘
```

## Development

### Prerequisites

- Go 1.21+
- Git

### Building

```bash
git clone https://github.com/Young-us/ycode.git
cd ycode
go build -o ycode ./cmd/ycode
```

### Testing

```bash
go test ./...
```

### Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## Roadmap

### MVP (Current)
- [x] CLI with Cobra
- [x] Agent loop (TAOR pattern)
- [x] ReadFile and Bash tools
- [ ] Anthropic API client
- [ ] Bubble Tea TUI
- [ ] WriteFile, EditFile tools
- [ ] Git integration
- [ ] Configuration system

### Post-MVP
- [ ] Plugin system
- [ ] Multi-agent collaboration
- [ ] MCP integration
- [ ] AST parsing (Tree-sitter)
- [ ] LSP integration
- [ ] IDE plugins (VS Code, JetBrains)
- [ ] Web UI

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- Inspired by [Claude Code](https://github.com/anthropics/claude-code)
- TUI built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- CLI built with [Cobra](https://github.com/spf13/cobra)
