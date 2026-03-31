# Sandbox 功能使用指南

## 📖 概述

Sandbox（沙箱）功能为 ycode 提供了安全的命令执行环境，通过多层安全机制保护系统和敏感信息。

## 🔒 安全特性

### 1. 命令黑名单
阻止执行危险命令，包括：
- `rm -rf /` - 删除根目录
- `mkfs` - 格式化磁盘
- `dd if=/dev/zero` - 磁盘覆盖
- Fork 炸弹 `:(){ :|:& };:`
- `chmod -R 777 /` - 权限滥用
- 其他 sudo 相关的危险操作

### 2. 危险模式检测
自动检测和阻止危险操作模式：
- 磁盘覆盖操作
- Fork 炸弹
- 系统文件修改
- 网络攻击脚本

### 3. 环境变量保护
自动过滤敏感环境变量：
- `API_KEY` 系列
- `SECRET` 系列
- `PASSWORD` 系列
- `TOKEN` 系列
- `PRIVATE_KEY` 系列
- `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`
- `ANTHROPIC_API_KEY`

### 4. 路径验证
- 只允许在指定目录内写入文件
- 防止路径遍历攻击
- 默认限制在工作目录内

### 5. 资源限制
- **超时控制**: 默认 30 秒
- **内存限制**: 默认 512MB
- **文件大小限制**: 默认 100MB
- **网络访问**: 默认禁用

## ⚙️ 配置选项

### 配置文件位置

1. **全局配置**: `~/.ycode/config.yaml`
2. **项目配置**: `.ycode/project.yaml`

### 配置示例

```yaml
sandbox:
  enabled: true                    # 启用/禁用沙箱（默认: true）
  timeout: 30s                     # 命令超时时间
  max_memory_mb: 512               # 最大内存限制（MB）
  max_file_size_mb: 100            # 最大文件大小限制（MB）
  allow_network: false             # 允许网络访问（默认: false）
  allow_write: true                # 允许写文件操作（默认: true）
  allowed_paths:                   # 允许写入的目录列表
    - ~/projects                   # 示例：允许写入到 ~/projects
    - /tmp                         # 示例：允许写入到 /tmp
  blocked_commands:                # 自定义命令黑名单（会与默认合并）
    - rm -rf /
    - mkfs
    - dd if=/dev/zero
  restricted_env_vars:             # 自定义敏感环境变量（会与默认合并）
    - API_KEY
    - SECRET
    - PASSWORD
```

### 配置项说明

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enabled` | bool | `true` | 是否启用沙箱 |
| `timeout` | duration | `30s` | 命令执行超时时间 |
| `max_memory_mb` | int | `512` | 最大内存使用（MB，0=无限制） |
| `max_file_size_mb` | int | `100` | 最大文件大小（MB，0=无限制） |
| `allow_network` | bool | `false` | 是否允许网络访问 |
| `allow_write` | bool | `true` | 是否允许文件写入 |
| `allowed_paths` | []string | `[]` | 允许写入的目录列表 |
| `blocked_commands` | []string | 默认黑名单 | 自定义命令黑名单 |
| `restricted_env_vars` | []string | 默认列表 | 自定义敏感环境变量列表 |

## 🚀 使用场景

### 1. 完全启用（推荐）
```yaml
sandbox:
  enabled: true
  allow_network: false
```

**适用场景**:
- 日常开发工作
- 代码审查
- 文件操作
- 构建和测试

### 2. 高级用户模式
```yaml
sandbox:
  enabled: true
  allow_network: true
  allowed_paths:
    - ~/projects
    - /tmp
```

**适用场景**:
- 需要网络访问的开发（如下载依赖）
- 需要跨目录操作
- 有经验的开发者

### 3. 禁用沙箱（不推荐）
```yaml
sandbox:
  enabled: false
```

**适用场景**:
- 完全信任的环境
- 特殊需求的高级用户
- **⚠️ 仅在完全了解风险的情况下使用**

### 4. 临时禁用
通过环境变量临时禁用：
```bash
export YCODE_SANDBOX_ENABLED=false
ycode chat
```

## 🔧 与工具的集成

Sandbox 已集成到以下工具中：

### BashTool
- 自动应用所有安全检查
- 环境变量自动过滤
- 超时和内存限制

### 其他文件工具
- WriteTool/EditTool: 路径验证
- ReadTool: 文件大小检查

## 📊 安全级别对比

| 功能 | 无 Sandbox | 有 Sandbox | 提升 |
|------|-----------|-----------|------|
| 命令执行安全 | ❌ 无保护 | ✅ 多层保护 | ⭐⭐⭐⭐⭐ |
| 环境变量保护 | ❌ 完全暴露 | ✅ 自动过滤 | ⭐⭐⭐⭐⭐ |
| 路径安全 | ❌ 无限制 | ✅ 白名单控制 | ⭐⭐⭐⭐ |
| 资源限制 | ❌ 无限制 | ✅ 多维度限制 | ⭐⭐⭐⭐ |
| 网络控制 | ❌ 完全开放 | ✅ 可配置 | ⭐⭐⭐ |

## 🛡️ 最佳实践

### 1. 默认配置
- 保持 `enabled: true`
- 保持 `allow_network: false`
- 只在需要时添加 `allowed_paths`

### 2. 项目级配置
在 `.ycode/project.yaml` 中配置项目特定的安全策略：
```yaml
sandbox:
  allowed_paths:
    - ./src
    - ./tests
  allow_network: true  # 项目需要下载依赖
```

### 3. 敏感信息管理
- 不要在配置文件中存储真实密钥
- 使用环境变量传递敏感信息
- Sandbox 会自动过滤这些变量

### 4. 测试环境
测试时可以适当放宽限制：
```yaml
sandbox:
  allow_network: true  # 测试需要网络
  max_memory_mb: 1024  # 测试可能需要更多内存
```

## ⚠️ 注意事项

1. **禁用风险**: 禁用 Sandbox 会移除所有安全保护，仅推荐在完全信任的环境使用
2. **路径配置**: `allowed_paths` 为空时，只允许在工作目录内写入
3. **黑名单合并**: 自定义 `blocked_commands` 会与默认黑名单合并
4. **环境变量**: 自定义 `restricted_env_vars` 会与默认列表合并
5. **超时影响**: 超时设置影响所有命令执行，包括构建和测试

## 📝 示例配置

### 开发者推荐配置
```yaml
sandbox:
  enabled: true
  timeout: 60s               # 增加超时以适应构建
  max_memory_mb: 1024        # 增加内存以适应编译
  allow_network: true        # 允许下载依赖
  allow_write: true
  allowed_paths:
    - ./                     # 当前项目目录
```

### CI/CD 环境配置
```yaml
sandbox:
  enabled: true
  timeout: 120s              # CI 可能需要更长时间
  max_memory_mb: 2048        # CI 环境内存更充裕
  allow_network: true        # CI 需要网络访问
  allow_write: true
```

### 安全审查配置
```yaml
sandbox:
  enabled: true
  timeout: 30s
  allow_network: false       # 禁用网络
  allow_write: false         # 禁用写入，只读模式
  allowed_paths: []          # 空列表
```

## 🔍 故障排查

### 问题：命令被阻止
```
Error: command contains blocked pattern 'rm -rf /'
```

**解决方案**:
1. 检查命令是否真的危险
2. 如果是误判，可以修改配置移除该黑名单项
3. 如果确实需要，考虑使用替代命令

### 问题：环境变量缺失
```
Warning: environment variable filtered
```

**解决方案**:
1. 检查是否是敏感变量
2. 如果不是，从 `restricted_env_vars` 中移除
3. 使用非敏感的变量名

### 问题：路径访问被拒绝
```
Error: path outside working directory not allowed
```

**解决方案**:
1. 将路径添加到 `allowed_paths`
2. 或在允许的目录内工作
3. 检查路径是否正确

### 问题：命令超时
```
Error: command timed out after 30s
```

**解决方案**:
1. 增加 `timeout` 设置
2. 优化命令执行速度
3. 分步执行长时间任务

## 🎯 总结

Sandbox 功能为 ycode 提供了全面的安全保护：

- ✅ **默认启用**: 安全优先原则
- ✅ **可配置**: 灵活的配置选项
- ✅ **多层保护**: 命令、环境、路径、资源
- ✅ **向后兼容**: 可以禁用以适应特殊需求
- ✅ **透明集成**: 自动应用于所有工具

**推荐做法**: 保持 Sandbox 启用，只在必要时调整配置。

---

更多信息请参考：
- [配置文档](../CLAUDE.md)
- [示例配置](../configs/example.yaml)
- [安全最佳实践](../docs/security-best-practices.md)