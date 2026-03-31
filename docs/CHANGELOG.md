# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Intent classification for intelligent agent routing
- Plan mode workflow with user approval
- OAuth 2.0 support for MCP servers
- Sandbox system for safe command execution
- Web tools: `web_search` and `web_fetch`
- ActionBar component for interactive user decisions
- Unified permission checker for all tools
- Audit logging for sensitive operations

### Changed
- Refactored orchestrator with specialized agent types
- Improved multi-agent coordination
- Enhanced tool permission system

### Removed
- Deprecated files: `multi.go`, `tui_final.go`, `fileops.go`

## [0.1.0] - 2025-01-15

### Added
- Initial release of ycode
- **Core Features**
  - TAOR (Think-Act-Observe-Repeat) agent loop pattern
  - Multi-agent orchestration system
  - Streaming LLM response support
  - Interactive TUI with Bubble Tea
  
- **Tools**
  - File operations: `read_file`, `write_file`, `edit_file`
  - Search: `glob`, `grep`
  - Shell: `bash` command execution
  - Git: `git_status`, `git_log`, `git_diff`
  - LSP: `hover`, `definition`, `references`, `completion`
  - Web: `web_search`, `web_fetch`
  
- **Agent Types**
  - Explorer: Code navigation and search
  - Planner: Task breakdown and planning
  - Architect: System design and interfaces
  - Coder: Code implementation
  - Debugger: Bug analysis and fixing
  - Tester: Test writing and verification
  - Reviewer: Code quality review
  - Writer: Documentation creation
  
- **Security**
  - Sandbox execution with resource limits
  - Permission modes: confirm, auto, deny
  - Path traversal protection
  - Command deny lists
  - File size limits
  - Sensitive data masking
  
- **Extensions**
  - MCP (Model Context Protocol) support
  - Plugin system with hot reload
  - Skill system for reusable capabilities
  
- **Configuration**
  - Multi-layer configuration (defaults, global, project, env)
  - YAML configuration files
  - Environment variable support
  - Human-readable token limits (4k, 128k, 1M)
  
- **UI Features**
  - Streaming response display
  - Markdown and code syntax highlighting
  - Diff view for file changes
  - Interactive confirmation dialogs
  - Auto-completion for commands

### Technical Details
- Go 1.25+ support
- Anthropic Claude API integration
- Bubble Tea TUI framework
- Cobra CLI framework
- Viper configuration management
- LSP JSON-RPC 2.0 client
- MCP JSON-RPC 2.0 client

## Version History Summary

| Version | Date | Description |
|---------|------|-------------|
| 0.1.0 | 2025-01-15 | Initial release with core features |
| Unreleased | - | Multi-agent improvements, OAuth, Sandbox |

---

**Note**: This project is in active development. Breaking changes may occur before v1.0.0.