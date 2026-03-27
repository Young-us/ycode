---
name: manage-plugins
description: Manage ycode plugins - configure, create, enable, disable, reload, and troubleshoot plugins. Use when setting up, adding, removing, or managing ycode plugins.
triggers:
  - configure plugin
  - setup plugin
  - add plugin
  - remove plugin
  - delete plugin
  - plugin config
  - plugin management
  - list plugins
  - enable plugin
  - disable plugin
  - reload plugin
  - create plugin
  - hot reload
  - plugin hooks
commands:
  - /manage-plugins
---

# Plugin Management Guide

## Overview

ycode supports a powerful plugin system that allows extending functionality through hooks. Plugins can intercept and modify behavior at various lifecycle points.

## Plugin Types

### 1. Script Plugins (Recommended)

Script-based plugins are easy to create and don't require compilation:

- **Python** (`.py`) - Most flexible, use `py` or `python3`
- **JavaScript** (`.js`) - Node.js runtime required
- **Lua** (`.lua`) - Lightweight scripting option

### 2. Native Go Plugins (Advanced)

Native plugins (`.so`, `.dylib`, `.dll`) offer maximum performance but require:
- Same Go version as ycode
- Platform support (Linux/macOS preferred)
- Export `NewPlugin` function

**Recommendation**: Use script plugins for most use cases.

## Plugin Directories

Plugins are loaded in this order (later directories override earlier):

| Priority | Directory | Purpose |
|----------|-----------|---------|
| 1 | `plugins/` | System built-in plugins (shipped with ycode) |
| 2 | `~/.ycode/plugins/` | Global user plugins (available everywhere) |
| 3 | `.ycode/plugins/` | Project-specific plugins |
| 4 | Custom | Configured via `plugins.directory` setting |

## Configuration

### Enable/Disable Plugin System

Edit `.ycode/project.yaml` or `~/.ycode/config.yaml`:

```yaml
plugins:
  enabled: true        # Enable or disable plugin system
  directory: ""        # Custom plugin directory (optional)
  hot_reload: true     # Auto-reload plugins on file change
```

### Configuration Priority

1. `configs/default.yaml` - System defaults
2. `~/.ycode/config.yaml` - User global config
3. `.ycode/project.yaml` - Project-specific config

## Available Hooks

Plugins can register handlers for these hooks:

| Hook | Description |
|------|-------------|
| `on_chat_start` | Triggered when a new chat session begins |
| `on_chat_complete` | Triggered when a chat response is generated |
| `on_tool_execute` | Triggered before a tool is executed |
| `on_tool_complete` | Triggered after a tool execution completes |
| `on_error` | Triggered when an error occurs |
| `on_agent_switch` | Triggered when switching between agents |
| `on_file_read` | Triggered after reading a file |
| `on_file_write` | Triggered before writing to a file |
| `on_command_execute` | Triggered before executing a shell command |
| `on_startup` | Triggered when the application starts |
| `on_shutdown` | Triggered when the application shuts down |

## Creating a Plugin

### Python Plugin Example

Create file `~/.ycode/plugins/my_plugin.py`:

```python
#!/usr/bin/env python3
# name: my-plugin
# version: 1.0.0
# description: My custom plugin for ycode
# hooks: on_tool_execute, on_file_read
# priority: 50

import json
import os
import sys

def main():
    # Read input from environment
    hook_name = os.environ.get("PLUGIN_HOOK", "")
    args_json = os.environ.get("PLUGIN_ARGS", "{}")
    plugin_name = os.environ.get("PLUGIN_NAME", "unknown")

    # Parse arguments
    try:
        args = json.loads(args_json)
    except json.JSONDecodeError:
        args = {}

    # Process based on hook type
    if hook_name == "on_tool_execute":
        tool_name = args.get("tool_name", "")
        print(f"[{plugin_name}] Tool executing: {tool_name}", file=sys.stderr)

        # Optionally modify args
        # args["custom_flag"] = True

    elif hook_name == "on_file_read":
        path = args.get("path", "")
        print(f"[{plugin_name}] File read: {path}", file=sys.stderr)

        # Optionally modify content
        # if "content" in args:
        #     args["content"] = args["content"].upper()

    # Return modified args as JSON
    print(json.dumps(args))

if __name__ == "__main__":
    main()
```

### JavaScript Plugin Example

Create file `~/.ycode/plugins/my_plugin.js`:

```javascript
// name: my-plugin
// version: 1.0.0
// description: My JavaScript plugin
// hooks: on_chat_complete
// priority: 100

function main() {
    const hookName = process.env.PLUGIN_HOOK || "";
    const argsJson = process.env.PLUGIN_ARGS || "{}";
    const pluginName = process.env.PLUGIN_NAME || "unknown";

    let args = {};
    try {
        args = JSON.parse(argsJson);
    } catch (e) {
        // Use default empty args
    }

    if (hookName === "on_chat_complete") {
        const response = args.response || "";
        console.error(`[${pluginName}] Chat response length: ${response.length}`);

        // Optionally modify response
        // args.response = response.toUpperCase();
    }

    // Output modified args
    console.log(JSON.stringify(args));
}

main();
```

### Lua Plugin Example

Create file `~/.ycode/plugins/my_plugin.lua`:

```lua
-- name: my-plugin
-- version: 1.0.0
-- description: My Lua plugin
-- hooks: on_command_execute
-- priority: 75

local json = require("dkjson")  -- Requires dkjson library

local hookName = os.getenv("PLUGIN_HOOK") or ""
local argsJson = os.getenv("PLUGIN_ARGS") or "{}"
local pluginName = os.getenv("PLUGIN_NAME") or "unknown"

local args = json.decode(argsJson) or {}

if hookName == "on_command_execute" then
    local command = args.command or ""
    print("[" .. pluginName .. "] Command: " .. command, io.stderr)

    -- Optionally modify command
    -- args.command = command .. " --verbose"
end

print(json.encode(args))
```

## Plugin Metadata

Plugins must declare metadata in comments at the top of the file:

| Field | Required | Description |
|-------|----------|-------------|
| `name:` | No | Plugin identifier (defaults to filename) |
| `version:` | No | Plugin version (defaults to 1.0.0) |
| `description:` | No | Human-readable description |
| `hooks:` | No | Comma-separated list of hooks to register |
| `priority:` | No | Execution priority (lower = earlier, default 100) |

## Management Actions

### 1. List Loaded Plugins

Use the `/plugins` command in ycode chat:
```
/plugins
```

Or check the logs during startup for plugin loading messages.

### 2. Create a New Plugin

1. Choose plugin directory:
   - Global: `~/.ycode/plugins/`
   - Project: `.ycode/plugins/`

2. Create plugin file with proper metadata comments

3. Make it executable (Linux/macOS):
   ```bash
   chmod +x ~/.ycode/plugins/my_plugin.py
   ```

4. Restart ycode or use hot-reload

### 3. Enable/Disable Plugin System

Edit configuration file:
```yaml
plugins:
  enabled: false  # Disable plugin system
```

### 4. Enable Hot-Reload

Enable in configuration:
```yaml
plugins:
  hot_reload: true
```

When enabled, modifying plugin files automatically reloads them without restarting ycode.

### 5. Disable a Specific Plugin

Options:
- Delete the plugin file
- Move it outside the plugin directories
- Rename with a different extension (e.g., `.py.disabled`)

### 6. Reload Plugin Manually

Use the reload command:
```
/reload
```

This reloads all skills and plugins.

## Plugin Development Tips

### Testing Plugins

1. Test the script independently first:
   ```bash
   PLUGIN_HOOK=on_tool_execute PLUGIN_ARGS='{"tool_name":"bash"}' python3 ~/.ycode/plugins/test.py
   ```

2. Check ycode logs for plugin messages

3. Use `print(..., file=sys.stderr)` for debug messages (won't interfere with JSON output)

### Error Handling

- Plugin errors are logged but don't crash ycode
- If a plugin returns invalid JSON, original args are used
- Use try/except blocks to handle errors gracefully

### Priority System

Lower priority values execute first:
- Priority 10: Runs first, can intercept and modify early
- Priority 50: Default for most plugins
- Priority 100: Runs later, can see changes from earlier plugins

### Multiple Hooks

Register for multiple hooks:
```python
# hooks: on_file_read, on_file_write, on_command_execute
```

## Workflow Examples

### Adding a Logging Plugin

1. Create the plugin:
   ```bash
   mkdir -p ~/.ycode/plugins
   cat > ~/.ycode/plugins/logger.py << 'EOF'
   #!/usr/bin/env python3
   # name: activity-logger
   # version: 1.0.0
   # description: Logs all tool executions to a file
   # hooks: on_tool_execute
   # priority: 10

   import json
   import os
   from datetime import datetime

   def main():
       hook = os.environ.get("PLUGIN_HOOK", "")
       args_json = os.environ.get("PLUGIN_ARGS", "{}")

       args = json.loads(args_json)

       if hook == "on_tool_execute":
           tool = args.get("tool_name", "unknown")
           log_file = os.path.expanduser("~/.ycode/activity.log")
           with open(log_file, "a") as f:
               f.write(f"{datetime.now().isoformat()} - Tool: {tool}\n")

       print(json.dumps(args))

   if __name__ == "__main__":
       main()
   EOF
   chmod +x ~/.ycode/plugins/logger.py
   ```

2. Restart ycode

3. Check logs:
   ```bash
   cat ~/.ycode/activity.log
   ```

### Adding a Code Formatter Plugin

1. Create the plugin:
   ```bash
   cat > ~/.ycode/plugins/formatter.py << 'EOF'
   #!/usr/bin/env python3
   # name: code-formatter
   # version: 1.0.0
   # description: Auto-format code before writing
   # hooks: on_file_write
   # priority: 50

   import json
   import os
   import subprocess

   def main():
       args = json.loads(os.environ.get("PLUGIN_ARGS", "{}"))

       if "path" in args and "content" in args:
           path = args["path"]
           content = args["content"]

           # Format Python files
           if path.endswith(".py"):
               try:
                   result = subprocess.run(
                       ["black", "--quiet", "-"],
                       input=content,
                       capture_output=True,
                       text=True
                   )
                   if result.returncode == 0:
                       args["content"] = result.stdout
               except:
                   pass

       print(json.dumps(args))

   if __name__ == "__main__":
       main()
   EOF
   chmod +x ~/.ycode/plugins/formatter.py
   ```

### Creating a Notification Plugin

```bash
cat > ~/.ycode/plugins/notify.py << 'EOF'
#!/usr/bin/env python3
# name: desktop-notify
# version: 1.0.0
# description: Send desktop notifications on chat complete
# hooks: on_chat_complete
# priority: 100

import json
import os
import subprocess

def main():
    args = json.loads(os.environ.get("PLUGIN_ARGS", "{}"))
    hook = os.environ.get("PLUGIN_HOOK", "")

    if hook == "on_chat_complete":
        # Send notification (Linux)
        subprocess.run([
            "notify-send",
            "ycode",
            "Chat response completed"
        ], capture_output=True)

        # macOS alternative:
        # subprocess.run([
        #     "osascript", "-e",
        #     'display notification "Chat completed" with title "ycode"'
        # ])

    print(json.dumps(args))

if __name__ == "__main__":
    main()
EOF
chmod +x ~/.ycode/plugins/notify.py
```

## Troubleshooting

### Plugin Not Loading

1. Check file permissions:
   ```bash
   ls -la ~/.ycode/plugins/
   chmod +x ~/.ycode/plugins/*.py
   ```

2. Verify interpreter is installed:
   ```bash
   which python3
   python3 --version
   ```

3. Check plugin syntax:
   ```bash
   python3 -m py_compile ~/.ycode/plugins/my_plugin.py
   ```

4. Test manually:
   ```bash
   PLUGIN_HOOK=test PLUGIN_ARGS='{}' python3 ~/.ycode/plugins/my_plugin.py
   ```

### Plugin Errors

Check ycode output for error messages:
- Plugin errors are logged with `[PLUGIN]` prefix
- Errors don't stop ycode from running

### Hot-Reload Not Working

1. Verify hot-reload is enabled in config:
   ```yaml
   plugins:
     hot_reload: true
   ```

2. Check file modification time changes

3. Use `/reload` command to manually reload

### Script Output Issues

If plugin returns invalid JSON:
- Original args are preserved
- Check for extra print statements
- Use `stderr` for debug output

## Quick Reference

| Action | How |
|--------|-----|
| List plugins | `/plugins` command |
| Add plugin | Create file in `~/.ycode/plugins/` |
| Remove plugin | Delete the plugin file |
| Reload plugins | `/reload` command |
| Enable system | Set `plugins.enabled: true` |
| Hot reload | Set `plugins.hot_reload: true` |
| Test plugin | Run script with `PLUGIN_HOOK` env var |

## Plugin Ideas

- **Activity Logger**: Track all tool usage
- **Code Formatter**: Auto-format files on write
- **Security Scanner**: Check for sensitive data before write
- **Custom Notifier**: Desktop notifications
- **Auto-commenter**: Add comments to generated code
- **Lint Integration**: Run linters on file changes
- **Git Hooks**: Integrate with git operations
- **Translation**: Translate responses
- **Response Filter**: Filter or transform AI responses

## Resources

- Plugin hooks are defined in: `internal/plugin/hooks.go`
- Plugin manager: `internal/plugin/manager.go`
- Configuration: `.ycode/project.yaml` or `~/.ycode/config.yaml`