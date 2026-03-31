# Ycode

**AI-Powered Terminal Coding Assistant**

Ycode 是一个基于 Claude 和兼容 AI 模型的智能终端编程助手，通过自然语言交互帮助你完成代码编写、重构、调试等任务。

[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## ✨ 核心特性

### 🤖 多智能体系统
- **8 种专业化智能体**: Explorer、Planner、Architect、Coder、Debugger、Tester、Reviewer、Writer
- **智能路由**: 自动识别任务类型，分配给最合适的智能体
- **并行执行**: 独立任务并行处理，提升效率
- **意图分类**: 基于语义分析的任务依赖识别

### 🛠️ 丰富的工具集
- **文件操作**: 读取、写入、编辑文件（支持大文件、行号范围）
- **代码搜索**: glob 模式匹配、grep 正则搜索
- **Shell 命令**: 安全执行 bash 命令（支持沙箱隔离）
- **Git 集成**: status、log、diff 等常用操作
- **Web 工具**: 网页搜索（DuckDuckGo）、网页抓取（Markdown 转换）
- **LSP 支持**: 代码补全、定义跳转、引用查找、悬停信息

### 🔒 安全与权限
- **沙箱隔离**: 资源限制（CPU、内存、文件大小）、网络访问控制
- **权限系统**: 三级权限模式（confirm/auto/deny）
- **敏感操作审计**: 完整的操作日志和确认流程
- **环境变量保护**: 自动隐藏 API Key 等敏感信息

### 🔌 扩展能力
- **MCP 协议**: 支持 Model Context Protocol 服务器
- **插件系统**: 可加载插件扩展功能，支持热重载
- **Skill 系统**: 可复用的 AI 能力模板
- **LSP 集成**: 支持多种语言服务器

### 🎨 现代化 TUI
- **流式响应**: 实时显示 AI 思考过程和回复
- **代码高亮**: 支持 Markdown 和代码语法高亮
- **Diff 视图**: 直观的文件变更对比
- **交互式确认**: 用户友好的操作确认界面

## 📦 安装

### 前置要求
- Go 1.25.0+
- Git

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/Young-us/ycode.git
cd ycode

# 构建
go build -o ycode ./cmd/ycode

# 运行
./ycode chat
```

### 配置 API Key

```bash
# 设置环境变量
export ANTHROPIC_API_KEY=your-api-key-here

# 或者在配置文件中设置
mkdir -p ~/.ycode
cp configs/example.yaml ~/.ycode/config.yaml
# 编辑 config.yaml，设置 api_key
```

## 🚀 快速开始

```bash
# 启动交互式会话
./ycode chat

# 指定模型
./ycode chat --model claude-sonnet-4-20250514

# 自动批准所有操作（谨慎使用）
./ycode chat --yolo

# 限制最大步骤数
./ycode chat --max-steps 5

# 无 UI 模式（纯命令行）
./ycode chat --no-ui
```

## 📖 使用示例

### 代码探索
```
你: 查找项目中所有的 Go 测试文件

AI: 我来帮你查找所有的 Go 测试文件...
使用 glob 工具搜索 *_test.go 文件...

找到了 15 个测试文件：
- internal/agent/orchestrator_test.go
- internal/tools/bash_test.go
...
```

### 代码修改
```
你: 在 internal/tools 目录下添加一个新的工具 logger.go，用于记录操作日志

AI: 我来创建 logger.go 工具...
使用 write_file 工具创建文件...

✓ 已创建 internal/tools/logger.go
  - 实现了 Logger 工具
  - 支持日志级别配置
  - 集成到工具管理器
```

### Git 操作
```
你: 查看 master 分支最近的提交历史

AI: 我来查看 git 提交历史...

最近的 10 条提交：
1. 506d6b7 - Major architecture improvements (2 minutes ago)
2. 7e951ca - Initial commit (1 hour ago)
...
```

### Web 搜索
```
你: 搜索 Go 1.25 的新特性

AI: 我来搜索 Go 1.25 的新特性...

搜索结果：
- Go 1.25 Release Notes
- New features in Go 1.25
- Performance improvements...
```

## 🏗️ 架构概览

```
cmd/ycode/main.go          # 应用入口
internal/
├── agent/                 # AI 智能体系统
│   ├── loop.go           # TAOR 循环（Think-Act-Observe-Repeat）
│   ├── orchestrator.go   # 多智能体编排
│   ├── classifier.go     # 意图分类器
│   ├── compactor.go      # 历史压缩
│   ├── workflow.go       # Plan mode 工作流
│   └── permission.go     # 权限管理
├── app/                   # 主应用和 TUI
├── config/                # 配置管理
├── llm/                   # LLM 客户端抽象
│   ├── anthropic.go      # Anthropic API 实现
│   └── mock.go           # Mock 客户端（测试用）
├── lsp/                   # LSP 客户端
├── mcp/                   # MCP 协议支持
│   ├── client.go         # MCP 客户端
│   └── oauth.go          # OAuth 2.0 认证
├── plugin/                # 插件系统
├── sandbox/               # 沙箱环境
├── tools/                 # AI 工具集
│   ├── read.go           # 文件读取
│   ├── write.go          # 文件写入
│   ├── edit.go           # 文件编辑
│   ├── bash.go           # Shell 命令
│   ├── glob.go           # 模式匹配
│   ├── grep.go           # 内容搜索
│   ├── git.go            # Git 操作
│   ├── lsp.go            # LSP 工具
│   ├── web_search.go     # Web 搜索
│   ├── web_fetch.go      # Web 抓取
│   └── diff.go           # Diff 计算
├── audit/                 # 审计日志
├── session/               # 会话管理
├── shell/                 # Shell 执行器
├── skill/                 # Skill 系统
└── ui/                    # UI 组件
    ├── tui_modern.go     # 主 TUI 界面
    ├── action_bar.go     # 操作栏
    ├── diff_viewer.go    # Diff 视图
    └── autocomplete.go   # 自动补全
```

## 🔧 配置

### 配置文件位置
配置按优先级加载（后者覆盖前者）：
1. **嵌入默认值**: 代码中的默认配置
2. `~/.ycode/config.yaml` - 全局配置
3. `.ycode/project.yaml` - 项目配置
4. 环境变量 (`YCODE_*`)

### 配置示例

```yaml
# LLM 配置
llm:
  provider: anthropic
  api_key: ${ANTHROPIC_API_KEY}
  model: claude-sonnet-4-20250514
  max_tokens: "128k"  # 支持 4k, 128k, 1M 等格式

# UI 配置
ui:
  theme: auto  # auto, light, dark
  streaming: true
  show_line_numbers: true

# 权限配置
permissions:
  mode: confirm  # confirm, auto, deny

# 沙箱配置
sandbox:
  enabled: true
  timeout: 30s
  max_memory_mb: 512
  allow_network: false

# 多智能体配置
agent:
  max_steps: 10
  multi_agent:
    enabled: true
    max_agents: 5

# MCP 服务器
mcp:
  servers:
    - name: example
      command: mcp-server
      args: []
      enabled: false

# LSP 服务器
lsp:
  servers:
    - name: gopls
      command: gopls
      args: ["serve"]
      enabled: false
```

## 🛠️ 开发

### 构建
```bash
go build ./...
go build -o ycode ./cmd/ycode
```

### 测试
```bash
# 运行所有测试
go test ./...

# 带覆盖率
go test -cover ./...

# 测试特定包
go test ./internal/tools/...

# 基准测试
go test -bench=. ./internal/agent/...
```

### 代码检查
```bash
# 格式化
gofmt -w .

# Lint
golangci-lint run
```

## 🎯 核心概念

### TAOR 模式
Ycode 采用 **Think-Act-Observe-Repeat** 模式：
1. **Think**: 分析用户请求，制定执行计划
2. **Act**: 调用工具执行操作
3. **Observe**: 观察执行结果
4. **Repeat**: 根据结果决定下一步行动

### 多智能体协作
```
用户请求
    ↓
意图分类器
    ↓
┌───────────────────┐
│   Orchestrator   │
└───────────────────┘
    ↓
    ├─→ Explorer  (代码探索)
    ├─→ Planner   (任务规划)
    ├─→ Architect (架构设计)
    ├─→ Coder     (代码实现)
    ├─→ Debugger  (问题调试)
    ├─→ Tester    (测试编写)
    ├─→ Reviewer  (代码审查)
    └─→ Writer    (文档编写)
```

### 工具分类
- **CategoryBasic**: 基础工具（始终可用）- read_file, glob, grep, web_search, web_fetch
- **CategoryWrite**: 写操作工具 - write_file, edit_file, bash
- **CategoryLSP**: LSP 工具 - hover, definition, references, completion
- **CategoryGit**: Git 工具 - git_status, git_log, git_diff

## 🔌 扩展

### 添加新工具

```go
package tools

type MyTool struct {
    workDir string
}

func (t *MyTool) Name() string { 
    return "my_tool" 
}

func (t *MyTool) Description() string { 
    return "Description of my tool" 
}

func (t *MyTool) Category() ToolCategory { 
    return CategoryBasic 
}

func (t *MyTool) Parameters() []Parameter {
    return []Parameter{
        {Name: "input", Type: "string", Description: "Input parameter", Required: true},
    }
}

func (t *MyTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
    // 实现工具逻辑
    return &ToolResult{Content: "result"}, nil
}
```

### 添加 Skill

在 `skills/my-skill/SKILL.md` 创建：

```markdown
---
triggers:
  - pattern: "创建.*工具"
  - pattern: "添加.*功能"
commands:
  - /my-skill
---

# My Skill

Skill description and instructions...
```

## 📚 文档

- [配置指南](configs/example.yaml)
- [沙箱安全指南](docs/sandbox-guide.md)
- [开发文档](CLAUDE.md)

## 🤝 贡献

欢迎贡献！请查看 [贡献指南](CONTRIBUTING.md)。

## 📄 许可证

[MIT License](LICENSE)

## 🙏 致谢

- [Anthropic](https://www.anthropic.com/) - Claude AI
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI 框架
- [Cobra](https://github.com/spf13/cobra) - CLI 框架
- [Viper](https://github.com/spf13/viper) - 配置管理

---

**Made with ❤️ by Young-us**