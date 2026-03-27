---
name: manage-mcp
description: Manage MCP (Model Context Protocol) servers for ycode. Use when setting up, adding, removing, enabling, disabling, or checking MCP servers.
triggers:
  - configure mcp
  - setup mcp
  - add mcp
  - remove mcp
  - delete mcp
  - mcp config
  - mcp management
  - mcp server
  - list mcp
  - enable mcp
  - disable mcp
commands:
  - /manage-mcp
---

# MCP Management Guide

## Overview

This skill helps you manage MCP (Model Context Protocol) servers for ycode. MCP servers extend ycode with external tools and capabilities like:

- **Filesystem**: Read/write files in specific directories
- **GitHub**: Interact with GitHub repositories, issues, PRs
- **Fetch**: Fetch web content
- **Memory**: Persistent memory across sessions
- **Database**: Query databases (PostgreSQL, SQLite, etc.)
- **Browser**: Web automation with Playwright

## Management Actions

### 1. View Current MCP Configuration

Use the `/mcp` command in ycode to see:
- Configured MCP servers
- Server status (connected/disconnected)
- Number of tools provided by each server

Or manually check configuration files:
- `.ycode/project.yaml` (project-specific)
- `~/.ycode/config.yaml` (user global)
- `configs/default.yaml` (system defaults)

### 2. Add MCP Server

Edit configuration file and add server entry under `mcp.servers`:

**Configuration format:**
```yaml
mcp:
  servers:
    - name: <server-name>
      command: <executable>
      args: ["<arg1>", "<arg2>"]
      enabled: true
```

**Example - Add Filesystem MCP:**
```yaml
mcp:
  servers:
    - name: filesystem
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/directory"]
      enabled: true
```

### 3. Remove MCP Server

Delete the entire server entry from configuration file:
```yaml
# Remove this entire block:
- name: server-to-remove
  command: npx
  args: ["-y", "@modelcontextprotocol/server-example"]
  enabled: true
```

### 4. Enable/Disable MCP Server

Set `enabled: true` or `enabled: false`:
```yaml
mcp:
  servers:
    - name: filesystem
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/path"]
      enabled: false  # Disabled - won't start on ycode launch
```

### 5. Test MCP Server Connection

Test if an MCP server can start correctly:
```bash
# Test stdio-based server
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | npx -y @modelcontextprotocol/server-everything stdio
```

If successful, you'll see a JSON response with `protocolVersion` and `capabilities`.

## Common MCP Servers

### Filesystem (No API Key Required)
Provides file read/write access to specified directories.
```yaml
- name: filesystem
  command: npx
  args: ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/project"]
  enabled: true
```
Installation: `npm install -g @modelcontextprotocol/server-filesystem`

### GitHub (Requires GitHub Token)
Interact with GitHub repositories, issues, and pull requests.
```yaml
- name: github
  command: npx
  args: ["-y", "@modelcontextprotocol/server-github"]
  enabled: true
```
Installation: `npm install -g @modelcontextprotocol/server-github`
Setup: Set environment variable `GITHUB_PERSONAL_ACCESS_TOKEN`

### Memory (No API Key Required)
Persistent memory across sessions using knowledge graph.
```yaml
- name: memory
  command: npx
  args: ["-y", "@modelcontextprotocol/server-memory"]
  enabled: true
```
Installation: `npm install -g @modelcontextprotocol/server-memory`

### Everything (No API Key Required)
Official test server with multiple tools for testing MCP functionality.
```yaml
- name: everything
  command: npx
  args: ["-y", "@modelcontextprotocol/server-everything", "stdio"]
  enabled: true
```
Installation: `npm install -g @modelcontextprotocol/server-everything`

### Playwright (No API Key Required)
Browser automation for web scraping and testing.
```yaml
- name: playwright
  command: npx
  args: ["-y", "@playwright/mcp-server"]
  enabled: true
```
Installation: `npm install -g @playwright/mcp-server`

### Sequential Thinking (No API Key Required)
Step-by-step reasoning and problem solving.
```yaml
- name: sequential-thinking
  command: npx
  args: ["-y", "@modelcontextprotocol/server-sequential-thinking"]
  enabled: true
```
Installation: `npm install -g @modelcontextprotocol/server-sequential-thinking`

### PostgreSQL (Requires Connection String)
Query PostgreSQL databases.
```yaml
- name: postgres
  command: npx
  args: ["-y", "@modelcontextprotocol/server-postgres", "postgresql://user:pass@host/db"]
  enabled: true
```
Installation: `npm install -g @modelcontextprotocol/server-postgres`

### SQLite (No API Key Required)
Query SQLite databases.
```yaml
- name: sqlite
  command: npx
  args: ["-y", "@modelcontextprotocol/server-sqlite", "/path/to/database.db"]
  enabled: true
```
Installation: `npm install -g @modelcontextprotocol/server-sqlite`

## Workflow Examples

### Adding a New MCP Server

1. **Choose an MCP server** from the list above or find others at:
   - https://github.com/modelcontextprotocol/servers
   - https://mcpmarket.com

2. **Install the server package** (if using npm):
   ```bash
   npm install -g <package-name>
   ```

3. **Add configuration** to `.ycode/project.yaml` or `~/.ycode/config.yaml`:
   ```yaml
   mcp:
     servers:
       - name: my-server
         command: npx
         args: ["-y", "<package-name>"]
         enabled: true
   ```

4. **Set required environment variables** (if needed):
   ```bash
   # Linux/macOS
   export API_KEY="your-key"
   
   # Windows
   set API_KEY=your-key
   ```

5. **Restart ycode** to apply changes

6. **Verify connection** with `/mcp` command - should show `[*]` for connected servers

### Removing an MCP Server

1. Open the configuration file containing the server

2. Delete the entire server block:
   ```yaml
   # Delete from here...
   - name: server-to-remove
     command: npx
     args: [...]
     enabled: true
   # ...to here
   ```

3. Save the file

4. Restart ycode

### Temporarily Disabling an MCP Server

1. Open configuration file

2. Change `enabled: true` to `enabled: false`

3. Restart ycode

### Testing Before Adding

Before adding a server to ycode, test it manually:
```bash
# For npx-based servers
npx -y @modelcontextprotocol/server-everything stdio

# For globally installed servers
server-command --help
```

## Troubleshooting

### MCP Server Fails to Connect

1. **Check if command exists:**
   ```bash
   which npx
   npx --version
   ```

2. **Test server manually:**
   ```bash
   npx -y <package-name>
   ```

3. **Check server logs** - ycode shows warnings for failed connections:
   ```
   Warning: failed to start MCP server <name>: <error>
   ```

4. **Verify package is installed:**
   ```bash
   npm list -g | grep <package-name>
   ```

### MCP Server Connected But Tools Not Working

1. **Check server status** with `/mcp` command

2. **Verify tool names** - MCP tools are prefixed with `mcp_<server-name>_<tool-name>`

3. **Check required environment variables** are set

4. **Restart ycode** to refresh tool list

### Slow MCP Server Startup

1. **First-time npm install** may be slow - subsequent starts use cache

2. **Check network connection** for downloading packages

3. **Pre-install packages globally:**
   ```bash
   npm install -g <package-name>
   ```

### Permission Errors

1. **Filesystem MCP** - Ensure ycode has read/write access to specified directory

2. **GitHub MCP** - Verify token has required scopes (repo, read:org, etc.)

3. **Database MCP** - Check connection string and credentials

## Quick Reference

| Action | How |
|--------|-----|
| View servers | `/mcp` command |
| Add server | Add entry to config file, restart ycode |
| Disable server | Set `enabled: false`, restart ycode |
| Remove server | Delete entry from config file, restart ycode |
| Test server | Run `npx -y <package>` in terminal |
| Check status | Look for `[*]` (connected) or `[ ]` (disconnected) |

## MCP Server Registry

Find more MCP servers at:
- **Official Servers**: https://github.com/modelcontextprotocol/servers
- **MCP Market**: https://mcpmarket.com
- **MCP Directory**: https://mcpdirectory.app
- **Glama MCP**: https://glama.ai/mcp/servers

## Configuration Priority

MCP servers are loaded in this order (later overrides earlier):
1. `configs/default.yaml` - System defaults
2. `~/.ycode/config.yaml` - User global config
3. `.ycode/project.yaml` - Project-specific config

Use `.ycode/project.yaml` for project-specific MCP servers that should only be active in that project.
