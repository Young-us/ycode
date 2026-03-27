---
name: configure-lsp
description: Manage LSP (Language Server Protocol) servers for code intelligence features. Use when setting up, modifying, checking, or removing LSP servers.
triggers:
  - configure lsp
  - setup lsp
  - add lsp
  - remove lsp
  - delete lsp
  - language server
  - lsp config
  - lsp management
commands:
  - /configure-lsp
---

# LSP Management Guide

## Overview

This skill helps you manage LSP servers for ycode. LSP provides code intelligence features like:
- **Hover**: Type information and documentation
- **Definition**: Go to definition
- **References**: Find all references
- **Completion**: Auto-completion suggestions
- **Diagnostics**: Errors and warnings

## Management Actions

### 1. View Current LSP Configuration

Use the `/lsp` command in ycode to see:
- Configured LSP servers
- Server status (enabled/disabled)
- Server command and arguments

### 2. Add/Configure LSP Server

Edit configuration file and add server entry:

**Configuration files (priority order):**
1. `.ycode/project.yaml` (project-specific)
2. `~/.ycode/config.yaml` (user global)
3. `configs/default.yaml` (system defaults)

**Configuration format:**
```yaml
lsp:
  servers:
    - name: <server-name>
      command: <executable>
      args: ["<arg1>", "<arg2>"]
      enabled: true
```

### 3. Disable LSP Server

Set `enabled: false` in configuration:
```yaml
lsp:
  servers:
    - name: gopls
      command: gopls
      args: ["serve"]
      enabled: false  # Disabled
```

### 4. Remove LSP Server

Delete the server entry from configuration file:
```yaml
# Remove this entire block:
- name: old-server
  command: old-server
  args: []
  enabled: true
```

### 5. Restart LSP Server

After configuration changes, restart ycode to apply changes.

## Common LSP Servers

### Go (gopls)
```yaml
- name: gopls
  command: gopls
  args: ["serve"]
  enabled: true
```
Installation: `go install golang.org/x/tools/gopls@latest`

### TypeScript/JavaScript (typescript-language-server)
```yaml
- name: typescript
  command: typescript-language-server
  args: ["--stdio"]
  enabled: true
```
Installation: `npm install -g typescript-language-server typescript`

### Python (pyright)
```yaml
- name: pyright
  command: pyright-langserver
  args: ["--stdio"]
  enabled: true
```
Installation: `npm install -g pyright`

### Python (pylsp)
```yaml
- name: pylsp
  command: pylsp
  args: []
  enabled: true
```
Installation: `pip install python-lsp-server`

### Rust (rust-analyzer)
```yaml
- name: rust-analyzer
  command: rust-analyzer
  args: []
  enabled: true
```
Installation: `rustup component add rust-analyzer`

### C/C++ (clangd)
```yaml
- name: clangd
  command: clangd
  args: []
  enabled: true
```
Installation: Install via system package manager

### Java (jdtls)
```yaml
- name: jdtls
  command: jdtls
  args: []
  enabled: true
```

### Ruby (solargraph)
```yaml
- name: solargraph
  command: solargraph
  args: ["stdio"]
  enabled: true
```
Installation: `gem install solargraph`

### PHP (intelephense)
```yaml
- name: intelephense
  command: intelephense
  args: ["--stdio"]
  enabled: true
```
Installation: `npm install -g intelephense`

## Workflow Examples

### Adding a new LSP server

1. Check if LSP server is installed:
   ```bash
   which <command>
   ```

2. Install if needed (see installation commands above)

3. Add configuration to `.ycode/project.yaml` or `~/.ycode/config.yaml`

4. Restart ycode to apply changes

5. Verify with `/lsp` command

### Removing an LSP server

1. Open the configuration file containing the server

2. Delete the entire server block:
   ```yaml
   # Delete from here...
   - name: server-to-remove
     command: ...
     args: [...]
     enabled: true
   # ...to here
   ```

3. Save the file

4. Restart ycode

### Temporarily disabling an LSP server

1. Open configuration file

2. Change `enabled: true` to `enabled: false`

3. Restart ycode

## Troubleshooting

### LSP server fails to start

1. **Check if command exists in PATH:**
   ```bash
   which <command>
   ```

2. **Test command manually:**
   ```bash
   <command> --version
   ```

3. **Check arguments** - some LSP servers require `--stdio` argument

4. **Check logs** - ycode shows warnings for failed LSP connections

### No hover/definition working

1. Verify LSP server is running with `/lsp` command

2. Check file type is supported by the LSP server

3. Ensure the file is saved (some LSP servers need saved files)

### Multiple LSP servers for same language

Only one LSP server per language is recommended. If multiple are configured, disable the unused ones.

## Multi-Language Projects

Configure multiple LSP servers for different languages:

```yaml
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
    - name: pyright
      command: pyright-langserver
      args: ["--stdio"]
      enabled: true
```

## Quick Reference

| Action | How |
|--------|-----|
| View servers | `/lsp` command |
| Add server | Add entry to config file, restart ycode |
| Disable server | Set `enabled: false`, restart ycode |
| Remove server | Delete entry from config file, restart ycode |
| Test server | Run `<command> --version` in terminal |