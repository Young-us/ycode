# ycode 插件系统

## 概述

ycode 支持两种类型的插件：
1. **脚本插件** - Python (.py)、Lua (.lua)、JavaScript (.js)
2. **原生插件** - Go 编译的动态库 (.so/.dylib/.dll)

## 插件目录

ycode 按以下顺序扫描三个插件目录：

| 目录 | 类型 | 用途 | 优先级 |
|------|------|------|--------|
| `plugins/` | 系统内置 | ycode 自带的插件 | 最低 |
| `~/.ycode/plugins/` | 全局用户 | 用户安装的插件 | 中等 |
| `.ycode/plugins/` | 项目级 | 项目特定的插件 | 最高 |

**加载顺序**：系统 → 全局 → 项目（后加载的可以覆盖先加载的同名插件）

### 目录结构示例

```
ycode/
├── plugins/                    # 系统内置插件
│   ├── logging_plugin.py
│   └── stats_plugin.js
│
├── ~/.ycode/
│   └── plugins/               # 全局用户插件
│       ├── notification_plugin.lua
│       └── custom_plugin.py
│
└── your-project/
    └── .ycode/
        └── plugins/           # 项目级插件
            └── project_plugin.py
```

## 安装插件

### 脚本插件（推荐）

1. 将插件文件复制到插件目录：
   ```bash
   # 用户级插件
   cp my_plugin.py ~/.ycode/plugins/
   
   # 或项目级插件
   cp my_plugin.py .ycode/plugins/
   ```

2. 启用插件系统：
   ```yaml
   # configs/default.yaml 或 ~/.ycode/config.yaml
   plugins:
     enabled: true
     directory: ~/.ycode/plugins
   ```

3. 重启 ycode

### 原生插件

原生插件需要使用与 ycode 相同的 Go 版本编译：
```bash
go build -buildmode=plugin -o my_plugin.so my_plugin.go
cp my_plugin.so ~/.ycode/plugins/
```

## 插件格式

### 插件元数据

在插件文件开头添加元数据注释，包括钩子声明：

**Python:**
```python
#!/usr/bin/env python3
# name: my-plugin
# version: 1.0.0
# description: My awesome plugin
# hooks: on_tool_execute, on_file_read, on_error
# priority: 50
```

**Lua:**
```lua
-- name: my-plugin
-- version: 1.0.0
-- description: My awesome plugin
-- hooks: on_tool_execute, on_file_read, on_error
-- priority: 50
```

**JavaScript:**
```javascript
/**
 * name: my-plugin
 * version: 1.0.0
 * description: My awesome plugin
 * hooks: on_tool_execute, on_file_read, on_error
 * priority: 50
 */
```

### 元数据字段

| 字段 | 必需 | 说明 |
|------|------|------|
| `name` | 否 | 插件名称（默认：文件名） |
| `version` | 否 | 版本号（默认：1.0.0） |
| `description` | 否 | 插件描述 |
| `hooks` | 否 | 要注册的钩子列表，逗号分隔（默认：on_tool_execute） |
| `priority` | 否 | 钩子优先级，数字越小越先执行（默认：100） |

### 插件输入

插件通过以下环境变量接收信息：

| 环境变量 | 说明 |
|---------|------|
| `PLUGIN_ARGS` | 钩子参数（JSON 格式） |
| `PLUGIN_HOOK` | 当前触发的钩子名称 |
| `PLUGIN_NAME` | 插件名称 |

```python
# Python 示例
import json
import os

args = json.loads(os.environ.get("PLUGIN_ARGS", "{}"))
hook_name = os.environ.get("PLUGIN_HOOK", "")
plugin_name = os.environ.get("PLUGIN_NAME", "")

# 根据不同钩子执行不同逻辑
if hook_name == "on_tool_execute":
    tool_name = args.get("tool_name")
    print(f"Tool executed: {tool_name}")
elif hook_name == "on_file_read":
    file_path = args.get("path")
    print(f"File read: {file_path}")
```

## 可用钩子

插件可以在以下钩子注册处理器：

| 钩子名称 | 触发时机 |
|---------|---------|
| `on_chat_start` | 新会话开始时 |
| `on_chat_complete` | 生成响应后 |
| `on_tool_execute` | 工具执行前 |
| `on_tool_complete` | 工具执行后 |
| `on_error` | 发生错误时 |
| `on_agent_switch` | 切换 agent 时 |
| `on_file_read` | 读取文件后 |
| `on_file_write` | 写入文件前 |
| `on_command_execute` | 执行命令前 |
| `on_startup` | 应用启动时 |
| `on_shutdown` | 应用关闭时 |

## 示例插件

### logging_plugin.py

记录所有工具执行到日志文件：

```python
#!/usr/bin/env python3
# name: logging-plugin
# version: 1.0.0
# description: Logs all tool executions

import json
import os
from datetime import datetime
from pathlib import Path

def main():
    args = json.loads(os.environ.get("PLUGIN_ARGS", "{}"))
    
    # 记录日志
    log_dir = Path.home() / ".ycode" / "plugin-logs"
    log_dir.mkdir(parents=True, exist_ok=True)
    
    with open(log_dir / "tool-executions.log", "a") as f:
        f.write(json.dumps({
            "timestamp": datetime.now().isoformat(),
            "tool": args.get("tool_name"),
            "args": args.get("args")
        }) + "\n")
    
    print(json.dumps(args))

if __name__ == "__main__":
    main()
```

### stats_plugin.js

收集工具使用统计：

```javascript
#!/usr/bin/env node
/**
 * name: stats-plugin
 * version: 1.0.0
 * description: Collects tool usage statistics
 */

const fs = require('fs');
const path = require('path');

function main() {
    const args = JSON.parse(process.env.PLUGIN_ARGS || '{}');
    const toolName = args.tool_name;
    
    // 更新统计
    const statsFile = path.join(
        process.env.HOME, '.ycode', 'plugin-stats', 'usage.json'
    );
    
    let stats = {};
    try {
        stats = JSON.parse(fs.readFileSync(statsFile, 'utf8'));
    } catch (e) {}
    
    stats[toolName] = (stats[toolName] || 0) + 1;
    
    fs.mkdirSync(path.dirname(statsFile), { recursive: true });
    fs.writeFileSync(statsFile, JSON.stringify(stats, null, 2));
    
    console.log(JSON.stringify(args));
}

main();
```

## 目录结构

```
~/.ycode/plugins/
├── logging_plugin.py      # Python 插件
├── notification_plugin.lua # Lua 插件
├── stats_plugin.js         # JavaScript 插件
└── native_plugin.so        # 原生插件（可选）

.ycode/plugins/             # 项目级插件
└── project_plugin.py
```

## 故障排除

### 插件未加载

1. 检查插件系统是否启用：
   ```yaml
   plugins:
     enabled: true
   ```

2. 检查插件目录是否存在：
   ```bash
   ls ~/.ycode/plugins/
   ```

3. 查看 ycode 启动日志

### 插件执行失败

1. 确保脚本解释器已安装：
   ```bash
   python3 --version  # Python 插件
   lua -v             # Lua 插件
   node --version     # JavaScript 插件
   ```

2. 检查插件文件权限：
   ```bash
   chmod +x ~/.ycode/plugins/*.py
   ```

3. 手动测试插件：
   ```bash
   PLUGIN_ARGS='{"tool_name":"test"}' python3 ~/.ycode/plugins/logging_plugin.py
   ```

## 最佳实践

1. **保持插件简单** - 每个插件只做一件事
2. **错误处理** - 捕获异常并返回有意义的错误
3. **日志记录** - 记录插件执行情况
4. **性能** - 避免长时间运行的操作
5. **安全性** - 不要执行不可信的代码
