# Contributing to Ycode

感谢你考虑为 Ycode 做贡献！

## 🤝 如何贡献

### 报告 Bug

如果你发现了 bug，请通过 [GitHub Issues](https://github.com/Young-us/ycode/issues) 提交报告。

提交 bug 报告时，请包含：

1. **清晰标题**: 简要描述问题
2. **复现步骤**: 详细说明如何复现问题
3. **预期行为**: 描述你期望发生什么
4. **实际行为**: 描述实际发生了什么
5. **环境信息**: 
   - 操作系统 (Linux/macOS/Windows)
   - Go 版本
   - Ycode 版本
6. **日志**: 如果可能，提供相关日志输出

### 提出新功能

如果你有新功能的想法：

1. 先检查 [Issues](https://github.com/Young-us/ycode/issues) 确保没有重复
2. 创建一个 Issue，详细描述功能需求
3. 说明该功能如何改进项目
4. 提供使用示例（如果可能）

### 提交代码

#### 开发环境设置

```bash
# 1. Fork 仓库
# 2. Clone 你的 fork
git clone https://github.com/YOUR_USERNAME/ycode.git
cd ycode

# 3. 添加上游仓库
git remote add upstream https://github.com/Young-us/ycode.git

# 4. 安装依赖
go mod download

# 5. 构建项目
go build -o ycode ./cmd/ycode

# 6. 运行测试
go test ./...
```

#### 开发流程

1. **创建分支**
   ```bash
   git checkout -b feature/your-feature-name
   # 或
   git checkout -b fix/your-bug-fix
   ```

2. **编写代码**
   - 遵循代码风格指南（见下文）
   - 添加必要的测试
   - 更新相关文档

3. **运行测试和检查**
   ```bash
   # 运行测试
   go test ./...

   # 代码格式化
   gofmt -w .

   # 代码检查
   golangci-lint run
   ```

4. **提交更改**
   ```bash
   git add .
   git commit -m "type: brief description"
   ```

   提交消息格式：
   - `feat: 添加新功能`
   - `fix: 修复 bug`
   - `docs: 更新文档`
   - `refactor: 重构代码`
   - `test: 添加测试`
   - `chore: 其他更改`

5. **推送并创建 Pull Request**
   ```bash
   git push origin feature/your-feature-name
   ```
   然后在 GitHub 上创建 Pull Request

#### Pull Request 检查清单

- [ ] 代码通过所有测试
- [ ] 代码风格一致
- [ ] 添加了必要的注释
- [ ] 更新了相关文档
- [ ] 提交消息清晰明了
- [ ] PR 描述详细说明了变更内容

## 📝 代码风格指南

### 1. 导入组织

标准库在前，然后是第三方库，最后是内部包（用空行分隔）：

```go
import (
    "context"
    "fmt"

    "github.com/spf13/viper"

    "github.com/Young-us/ycode/internal/config"
)
```

### 2. 命名规范

| 元素 | 规范 | 示例 |
|------|------|------|
| 包 | 小写，无下划线 | `internal/tools` |
| 函数 | 导出用 PascalCase，内部用 camelCase | `NewBashTool()`, `execute()` |
| 变量 | camelCase | `workingDir`, `maxLines` |
| 常量 | 导出用 PascalCase，内部用 camelCase | `ErrCodeConfigLoad` |
| 接口 | PascalCase，优先 -er 后缀 | `Client`, `Tool` |
| 结构体 | PascalCase | `AnthropicClient` |

### 3. 错误处理

使用 `internal/errors` 包中的结构化错误：

```go
// 创建新错误
return nil, errors.New(errors.ErrCodeToolExecution, "failed to execute")

// 包装错误
return nil, errors.Wrap(errors.ErrCodeFileRead, "cannot read config", err)

// 检查错误类型
if errors.IsErrorCode(err, errors.ErrCodeToolTimeout) { ... }
```

**规则**：
- 始终显式处理错误
- 使用 `fmt.Errorf("context: %w", err)` 包装错误
- 工具函数返回 `(*ToolResult, error)`

### 4. Context 使用

始终将 `context.Context` 作为第一个参数传递：

```go
func (t *MyTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
    // 使用 WithTimeout 进行限时操作
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    // ...
}
```

### 5. 测试规范

- 使用表驱动测试
- 使用 `t.TempDir()` 创建临时文件
- 处理跨平台差异

```go
func TestExecutor_Execute(t *testing.T) {
    tests := []struct {
        name    string
        cmd     string
        wantErr bool
    }{
        {"simple echo", "echo hello", false},
        {"invalid command", "nonexistent", true},
    }
    
    executor := NewExecutor(t.TempDir())
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := executor.Execute(context.Background(), tt.cmd)
            if (err != nil) != tt.wantErr {
                t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## 🏗️ 添加新工具

要添加新工具，请遵循以下步骤：

1. **创建工具文件**

在 `internal/tools/` 目录下创建新文件：

```go
// internal/tools/my_tool.go
package tools

import "context"

type MyTool struct {
    workDir string
}

func NewMyTool(workDir string) *MyTool {
    return &MyTool{workDir: workDir}
}

func (t *MyTool) Name() string {
    return "my_tool"
}

func (t *MyTool) Description() string {
    return "Description of what this tool does"
}

func (t *MyTool) Category() ToolCategory {
    return CategoryBasic // 或 CategoryWrite, CategoryLSP, CategoryGit
}

func (t *MyTool) Parameters() []Parameter {
    return []Parameter{
        {
            Name:        "input",
            Type:        "string",
            Description: "Input parameter description",
            Required:    true,
        },
    }
}

func (t *MyTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
    input, ok := args["input"].(string)
    if !ok {
        return nil, errors.New(errors.ErrCodeInvalidParam, "input must be a string")
    }
    
    // 实现工具逻辑
    
    return &ToolResult{
        Content: "result",
        IsError: false,
    }, nil
}
```

2. **注册工具**

在 `internal/app/app.go` 中注册：

```go
toolManager.Register(tools.NewMyTool(workDir))
```

3. **添加测试**

创建 `internal/tools/my_tool_test.go`：

```go
package tools

import (
    "context"
    "testing"
)

func TestMyTool_Execute(t *testing.T) {
    tool := NewMyTool(t.TempDir())
    
    result, err := tool.Execute(context.Background(), map[string]interface{}{
        "input": "test",
    })
    
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    if result.IsError {
        t.Error("expected success, got error")
    }
}
```

## 📚 文档贡献

文档和代码一样重要！你可以：

- 改进 README.md
- 更新 CLAUDE.md
- 添加使用示例
- 编写教程或指南
- 翻译文档

## 🔍 代码审查

所有 PR 都需要审查。审查时会关注：

- **功能正确性**: 代码是否实现了预期功能？
- **代码质量**: 是否遵循风格指南？是否清晰易读？
- **测试覆盖**: 是否添加了足够的测试？
- **文档完整性**: 是否更新了相关文档？
- **性能影响**: 是否引入性能问题？
- **安全性**: 是否存在安全风险？

## 📜 行为准则

- 尊重所有贡献者
- 接受建设性批评
- 专注于对项目最有利的事情
- 对新手保持耐心和友好

## 🙋 获取帮助

如果你有任何问题：

- 查看 [文档](README.md)
- 搜索 [Issues](https://github.com/Young-us/ycode/issues)
- 在 [Discussions](https://github.com/Young-us/ycode/discussions) 提问

---

再次感谢你的贡献！❤️