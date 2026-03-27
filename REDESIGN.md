# ycode 改进计划

基于 opencode 的分析，以下是需要改进的关键领域。

## 1. Agent 系统改进

### 1.1 多 Agent 类型
opencode 实现了多种 Agent 类型:
- `build`: 默认 Agent，执行工具
- `plan`: 计划模式，禁止编辑工具
- `explore`: 快速探索代码库
- `general`: 通用研究 Agent
- `compaction`: 上下文压缩 Agent
- `title`: 标题生成 Agent
- `summary`: 摘要生成 Agent

### 1.2 权限系统
opencode 使用 PermissionNext 实现细粒度权限控制:
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

权限类型: `allow`, `deny`, `ask`

### 1.3 Agent 状态管理
使用 Instance.state() 管理 Agent 配置，支持热重载

## 2. Skill 系统改进

### 2.1 Skill 目录结构
支持多个 Skill 目录:
- `~/.claude/skills/`
- `~/.agents/skills/`
- 项目目录下的 `.claude/` 或 `.agents/`
- 配置指定的目录

### 2.2 Skill 权限过滤
根据当前 Agent 权限过滤可用的 Skills

### 2.3 Skill 发现机制
```go
// 异步加载，带缓存
type State struct {
    skills   map[string]*Skill
    dirs     []string
    task     *sync.Once
    mu       sync.RWMutex
}
```

## 3. MCP 改进

### 3.1 MCP 客户端状态管理
```go
type MCPClient struct {
    Name    string
    Client  *mcp.Client
    Status  MCPStatus
    Tools   []ToolDefinition
    Timeout time.Duration
}
```

### 3.2 MCP 状态类型
- `connected`: 已连接
- `disabled`: 禁用
- `failed`: 连接失败
- `needs_auth`: 需要认证
- `needs_client_registration`: 需要客户端注册

### 3.3 OAuth 支持
- 支持远程 MCP 服务器的 OAuth 认证
- 浏览器授权流程
- Token 存储和刷新

### 3.4 MCP 资源管理
- MCP Prompts: 可作为命令调用
- MCP Resources: 可读取的资源
- Tool 转换: MCP Tool -> ycode Tool

## 4. LSP 改进

### 4.1 LSP 服务器管理
```go
type LSPServer struct {
    Name      string
    Command   string
    Args      []string
    Enabled   bool
    Transport LSPTransport
}
```

### 4.2 LSP 功能
- 诊断 (Diagnostics)
- 悬停信息 (Hover)
- 跳转定义 (Goto Definition)
- 查找引用 (Find References)
- 代码补全 (Completion)

## 5. Plugin 系统改进

### 5.1 Hook 系统
```go
type PluginHook interface {
    OnChatStart(ctx context.Context, session *Session)
    OnChatComplete(ctx context.Context, session *Session)
    OnToolExecute(ctx context.Context, tool Tool, args map[string]interface{})
    OnError(ctx context.Context, err error)
}
```

### 5.2 Plugin 生命周期
1. Discovery: 发现插件
2. Load: 加载插件
3. Init: 初始化插件
4. Shutdown: 关闭插件

### 5.3 内置插件
- Codex Auth Plugin
- Copilot Auth Plugin
- GitLab Auth Plugin

## 6. Command 系统改进

### 6.1 命令来源
- Built-in: 内置命令
- MCP: MCP Prompts 作为命令
- Skill: Skills 作为命令
- Config: 配置文件中的命令

### 6.2 命令模板变量
- `$1, $2, ...`: 位置参数
- `$ARGUMENTS`: 所有参数
- 环境变量 `$ENV_VAR`

### 6.3 命令属性
```go
type Command struct {
    Name        string
    Description string
    Agent       string      // 使用的 Agent
    Model       string      // 使用的模型
    Source      string      // "command", "mcp", "skill"
    Subtask     bool        // 是否作为子任务
    Hints       []string    // 参数提示
    Template    string      // 模板内容
}
```

## 7. UI 改进

### 7.1 改进区域
- 流式文本显示
- 命令面板 (Cmd/Ctrl+P)
- 更好的错误显示
- 键盘快捷键
- MCP/LSP 状态指示器
- Token 使用统计

### 7.2 状态栏信息
- 模型信息
- Token 使用情况
- MCP 服务器状态
- LSP 服务器状态
- Agent 模式

## 8. 实现优先级

### Phase 1: 核心架构
1. ✅ Agent 系统多类型支持
2. ✅ Permission 系统
3. ✅ Command 系统改进

### Phase 2: 集成
1. MCP 客户端完整实现
2. LSP 客户端完整实现
3. Skill 系统增强

### Phase 3: 高级功能
1. Plugin Hook 系统
2. UI 增强
3. OAuth 支持

## 9. 文件结构建议

```
internal/
├── agent/
│   ├── loop.go           # TAOR 循环
│   ├── agent.go         # Agent 定义
│   ├── permission.go    # 权限系统
│   └── compact.go       # 压缩 Agent
├── command/
│   ├── command.go       # Command 定义
│   ├── manager.go       # Command 管理器
│   ├── builtin.go       # 内置命令
│   └── template/        # 命令模板
├── mcp/
│   ├── client.go        # MCP 客户端
│   ├── oauth.go        # OAuth 支持
│   └── tools.go        # Tool 转换
├── lsp/
│   ├── client.go        # LSP 客户端
│   ├── diagnostics.go  # 诊断
│   └── completions.go   # 补全
├── skill/
│   ├── skill.go        # Skill 定义
│   ├── discovery.go    # Skill 发现
│   └── permission.go   # Skill 权限
├── plugin/
│   ├── plugin.go       # Plugin 接口
│   ├── hooks.go        # Hook 定义
│   └── manager.go      # Plugin 管理器
└── ui/
    ├── tui.go          # TUI 主程序
    ├── component/       # UI 组件
    └── styles/         # 样式
```
