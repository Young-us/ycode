---
name: git-master
description: "Complete Git workflow management with intelligent handling of all scenarios. Use for ANY git operation: init, commit, push, pull, branch, merge, conflict resolution, and more."
triggers:
  - git
  - commit
  - push
  - pull
  - branch
  - merge
  - stash
  - rebase
  - clone
  - checkout
  - 版本控制
  - 提交代码
  - 推送代码
  - 拉取代码
  - 分支管理
commands:
  - /git
---

# Git Master - Complete Git Workflow Management

## User Request

The user wants to: **$ARGUMENTS**

Please help them accomplish this Git operation following the workflows below.

---

## Overview

This skill provides **intelligent Git workflow management** that handles all common scenarios automatically, including:

- Auto-initialization when no git repo exists
- Smart commit message generation based on changes
- Interactive remote setup when needed
- Conflict detection and resolution guidance
- Branch management and switching
- Stash operations for work-in-progress

**Core Principle**: Be proactive, ask when uncertain, handle errors gracefully.

---

## Pre-flight Checks

Before ANY git operation, ALWAYS perform these checks in order:

### 1. Check Git Installation
```bash
git --version
```
If git is not installed, inform the user and stop.

### 2. Check Repository State
```bash
git rev-parse --git-dir
```
- **If succeeds**: Repository exists, continue
- **If fails**: Repository does NOT exist → Go to "Repository Initialization" workflow

### 3. Check Working Directory State
```bash
git status --porcelain
```
Understand the current state before proceeding.

---

## Workflow: Repository Initialization

**Trigger**: When git operations are requested but no `.git` directory exists

### Step 1: Ask User Confirmation
```
当前目录不是 Git 仓库。需要先初始化才能执行 Git 操作。

选项：
1. 初始化新仓库 (git init)
2. 克隆远程仓库 (git clone <url>)
3. 取消操作

请选择 (1/2/3):
```

### Step 2: Execute Based on Choice

**Choice 1 - Initialize New Repository:**
```bash
git init
git add .
git commit -m "Initial commit"
```

**Choice 2 - Clone Repository:**
```
请输入仓库 URL:
```
Then execute:
```bash
git clone <user-provided-url> .
```

**Choice 3 - Cancel:**
Stop all operations.

### Step 3: Remote Setup (Optional)
After initialization, ask:
```
是否需要添加远程仓库? (y/n)
```
If yes:
```
请输入远程仓库 URL (例如: git@github.com:user/repo.git):
```
Then:
```bash
git remote add origin <user-provided-url>
git push -u origin main
```

---

## Workflow: Commit

**Triggers**: commit, 提交, 提交代码

### Step 1: Check Repository State
```bash
git status --porcelain
```

### Step 2: Handle Different States

**If no changes:**
```
没有需要提交的更改。
```
Stop here.

**If untracked files exist:**
```
发现未跟踪的文件:
- file1.go
- file2.md

是否添加这些文件? (y/n/all/部分)
```
- `y` or `all`: `git add .`
- `n`: Skip untracked
- `部分`: Ask for specific files

**If modified files exist:**
```
发现已修改的文件:
- file1.go (modified)
- file2.go (deleted)

是否暂存所有更改? (y/n/选择)
```

### Step 3: Generate Commit Message

**Option A: Auto-generate (Default for simple changes)**

Analyze the diff to generate commit message:
```bash
git diff --cached --stat
git diff --cached
```

Generate message based on:
- Files changed (feat/fix/refactor/docs/test)
- Nature of changes
- Scope of changes

Format: `<type>(<scope>): <description>`

Examples:
- `feat(mcp): add MCP server management`
- `fix(git): handle repository initialization`
- `refactor(tools): simplify git tool implementation`
- `docs(readme): update installation guide`

**Option B: Ask user for message**
```
请输入提交信息 (或按 Enter 自动生成):
```

### Step 4: Execute Commit
```bash
git add .
git commit -m "<message>"
```

### Step 5: Ask About Push
```
提交成功! 是否推送到远程仓库? (y/n)
```
If yes → Execute Push workflow

---

## Workflow: Push

**Triggers**: push, 推送, 推送代码

### Step 1: Check Remote Configuration
```bash
git remote -v
```

**If no remote configured:**
```
未配置远程仓库。请提供远程仓库 URL:
```
Then:
```bash
git remote add origin <url>
```

### Step 2: Check Current Branch
```bash
git branch --show-current
```

### Step 3: Check for Unpushed Commits
```bash
git log @{u}.. 2>/dev/null || echo "No upstream"
```

### Step 4: Handle Different Scenarios

**Scenario 1: First push to new branch**
```bash
git push -u origin <branch-name>
```

**Scenario 2: Normal push**
```bash
git push
```

**Scenario 3: Remote has new commits (need pull first)**
```
远程仓库有新的提交，需要先拉取:
```
Execute Pull workflow, then retry push.

**Scenario 4: Force push needed (diverged history)**
```
本地和远程历史已分叉。选项:
1. 拉取并合并 (git pull --rebase)
2. 强制推送 (git push --force) ⚠️ 危险
3. 取消

请选择 (1/2/3):
```

### Step 5: Verify Push
```bash
git status
```

---

## Workflow: Pull

**Triggers**: pull, 拉取, 拉取代码, fetch

### Step 1: Check Working Directory
```bash
git status --porcelain
```

**If uncommitted changes exist:**
```
有未提交的更改:
- file1.go (modified)
- file2.go (staged)

选项:
1. 暂存更改 (git stash)
2. 提交更改 (git commit)
3. 丢弃更改 (git checkout -- .) ⚠️ 危险
4. 取消拉取

请选择 (1/2/3/4):
```

### Step 2: Execute Pull
```bash
git pull --rebase
```

### Step 3: Handle Merge Conflicts

**If conflicts detected:**
```
检测到合并冲突!

冲突文件:
- file1.go
- file2.conf

请手动解决冲突后告诉我。解决后输入 "resolved" 继续。
```

**Wait for user confirmation**, then:
```bash
git add .
git rebase --continue
```

**If rebase abort needed:**
```
如果需要放弃合并，输入 "abort"
```
Then:
```bash
git rebase --abort
```

### Step 4: Restore Stashed Changes (if any)
```bash
git stash pop
```

---

## Workflow: Branch Management

**Triggers**: branch, 分支, 创建分支, 切换分支

### Step 1: List Branches
```bash
git branch -a
```

### Step 2: Handle Different Requests

**Create new branch:**
```
请输入新分支名称:
```
```bash
git checkout -b <branch-name>
```

**Switch branch:**
```
当前分支: main

可用分支:
- feature/mcp
- bugfix/git
- main

请输入要切换的分支名称:
```
```bash
git checkout <branch-name>
```

**Delete branch:**
```
要删除哪个分支?
```
```bash
git branch -d <branch-name>
```

**Merge branch:**
```
要将哪个分支合并到当前分支?
```
Then execute Merge workflow.

---

## Workflow: Merge

**Triggers**: merge, 合并

### Step 1: Check Current Branch
```bash
git branch --show-current
```

### Step 2: Ask for Source Branch
```
当前分支: main

要将哪个分支合并到当前分支?
```

### Step 3: Execute Merge
```bash
git merge <source-branch>
```

### Step 4: Handle Conflicts

**If no conflicts:**
```
合并成功!
```

**If conflicts:**
```
合并冲突!

冲突文件:
- file1.go
- file2.conf

请解决冲突后告诉我。输入 "resolved" 继续，或 "abort" 放弃合并。
```

Wait for user, then:
```bash
git add .
git commit -m "Merge <source-branch> into <current-branch>"
```

---

## Workflow: Stash

**Triggers**: stash, 暂存

### Step 1: Check Stash
```bash
git stash list
```

### Step 2: Handle Request

**Save stash:**
```bash
git stash push -m "WIP: <description>"
```

**Apply stash:**
```
可用的 stash:
0: WIP on main: abc1234 Fix bug
1: WIP on feature: def5678 Add feature

请输入要应用的 stash 编号:
```
```bash
git stash pop stash@{<number>}
```

**Drop stash:**
```bash
git stash drop stash@{<number>}
```

---

## Workflow: Log & History

**Triggers**: log, 历史, 提交记录

### Show Recent Commits
```bash
git log --oneline -20
```

### Show Detailed Log
```bash
git log --stat -10
```

### Show File History
```bash
git log --follow -- <file-path>
```

### Show Branch Graph
```bash
git log --graph --oneline --all
```

---

## Workflow: Diff

**Triggers**: diff, 差异, 修改了什么

### Show Unstaged Changes
```bash
git diff
```

### Show Staged Changes
```bash
git diff --cached
```

### Show Changes Between Branches
```bash
git diff <branch1>..<branch2>
```

### Show Changes in Commit
```bash
git show <commit-hash>
```

---

## Workflow: Tag

**Triggers**: tag, 版本, release

### List Tags
```bash
git tag -l
```

### Create Tag
```
请输入版本号 (例如: v1.0.0):
```
```bash
git tag -a <version> -m "Release <version>"
git push origin <version>
```

### Delete Tag
```bash
git tag -d <tag-name>
git push origin --delete <tag-name>
```

---

## Workflow: Reset & Revert

**Triggers**: reset, revert, 回退, 撤销

### Soft Reset (Keep changes staged)
```bash
git reset --soft HEAD~1
```

### Mixed Reset (Keep changes unstaged)
```bash
git reset HEAD~1
```

### Hard Reset (Discard changes) ⚠️
```
⚠️ 警告: 这将丢弃所有未提交的更改!

确认执行硬重置? (yes/no):
```
```bash
git reset --hard HEAD~1
```

### Revert Commit (Create new commit)
```bash
git revert <commit-hash>
```

---

## Workflow: Remote Management

**Triggers**: remote, 远程

### List Remotes
```bash
git remote -v
```

### Add Remote
```
请输入远程名称 (默认: origin):
请输入远程 URL:
```
```bash
git remote add <name> <url>
```

### Remove Remote
```bash
git remote remove <name>
```

### Change Remote URL
```bash
git remote set-url <name> <new-url>
```

---

## Smart Commit Message Generation

When auto-generating commit messages, follow these rules:

### 1. Determine Change Type

| Pattern | Type | Example |
|---------|------|---------|
| New file with content | `feat` | `feat(auth): add login endpoint` |
| Bug fix | `fix` | `fix(api): handle null response` |
| Code improvement | `refactor` | `refactor(utils): simplify validation` |
| Documentation | `docs` | `docs(readme): update installation` |
| Tests | `test` | `test(auth): add unit tests` |
| Config/build | `chore` | `chore(deps): update dependencies` |

### 2. Determine Scope

Analyze changed files to determine scope:
- If all changes in one module → use module name
- If changes across modules → use general scope like `core`, `api`, `ui`

### 3. Generate Description

- Start with verb in imperative mood
- Keep under 50 characters
- Be specific but concise

### 4. Add Body (if complex changes)

```
feat(mcp): add MCP server management

- Implement MCP client connection
- Add tool registration from MCP servers
- Support multiple MCP server configurations
```

---

## Error Handling

### Common Errors & Solutions

**Error: "Please tell me who you are"**
```bash
git config --global user.name "Your Name"
git config --global user.email "your@email.com"
```

**Error: "Permission denied (publickey)"**
```
SSH 密钥未配置。选项:
1. 使用 HTTPS URL (https://github.com/user/repo.git)
2. 配置 SSH 密钥

请选择 (1/2):
```

**Error: "Remote origin already exists"**
```bash
git remote set-url origin <new-url>
```

**Error: "Your branch is ahead of 'origin/main'"**
```bash
git push
```

**Error: "Your branch is behind 'origin/main'"**
```bash
git pull --rebase
```

**Error: "You have unstaged changes"**
```
有未暂存的更改。选项:
1. 暂存更改 (git add .)
2. 暂存到 stash (git stash)
3. 丢弃更改 (git checkout -- .)

请选择 (1/2/3):
```

**Error: "CONFLICT (content)"**
```
文件冲突: <filename>

请手动编辑文件解决冲突 markers (<<<<<<, ======, >>>>>>), 然后输入 "resolved"。
```

---

## Quick Reference

| 操作 | 命令 | 说明 |
|------|------|------|
| 初始化 | `git init` | 创建新仓库 |
| 克隆 | `git clone <url>` | 克隆远程仓库 |
| 状态 | `git status` | 查看状态 |
| 添加 | `git add <file>` | 暂存文件 |
| 提交 | `git commit -m "msg"` | 提交更改 |
| 推送 | `git push` | 推送到远程 |
| 拉取 | `git pull --rebase` | 拉取并变基 |
| 分支 | `git branch <name>` | 创建分支 |
| 切换 | `git checkout <branch>` | 切换分支 |
| 合并 | `git merge <branch>` | 合并分支 |
| 日志 | `git log --oneline` | 查看提交历史 |
| 差异 | `git diff` | 查看更改 |
| 暂存 | `git stash` | 临时保存更改 |
| 标签 | `git tag <version>` | 创建版本标签 |

---

## Interactive Examples

### Example 1: First-time setup
```
用户: 提交代码

AI: 当前目录不是 Git 仓库。需要先初始化才能提交代码。

选项:
1. 初始化新仓库
2. 克隆远程仓库
3. 取消

请选择 (1/2/3): 1

AI: 已初始化 Git 仓库。
添加了 15 个文件到暂存区。

检测到这是首次提交，自动生成提交信息:
"Initial commit: project setup"

是否推送到远程仓库? (y/n): y

AI: 未配置远程仓库。请输入远程仓库 URL:
用户: git@github.com:user/project.git

AI: 已添加远程仓库并推送成功!
```

### Example 2: Conflict resolution
```
用户: 拉取最新代码

AI: 检测到未提交的更改:
- main.go (modified)
- config.yaml (staged)

如何处理这些更改?
1. 暂存到 stash
2. 提交更改
3. 丢弃更改
4. 取消拉取

请选择 (1/2/3/4): 1

AI: 已暂存更改到 stash。
正在拉取最新代码...

检测到合并冲突!
冲突文件:
- main.go

请手动解决冲突后告诉我。输入 "resolved" 继续。
用户: resolved

AI: 冲突已解决。
已恢复之前暂存的更改。
拉取完成!
```

### Example 3: Smart commit
```
用户: 提交今天的修改

AI: 检测到以下更改:

已修改:
- internal/mcp/client.go (修复 ID 匹配问题)
- internal/app/app.go (添加 connectedMCPServers)

未跟踪:
- skills/manage-mcp/SKILL.md (新 skill)

自动分析并生成提交信息:
"fix(mcp): resolve JSON-RPC ID type mismatch

- Add normalizeID() to handle float64/int conversion
- Track connected MCP servers for UI status display"

是否使用此提交信息? (y/n/edit): y

AI: 提交成功!
是否推送到远程? (y/n):
```

---

## Best Practices

1. **Always check status before operations** - Know the current state
2. **Ask before destructive operations** - Hard reset, force push, etc.
3. **Generate meaningful commit messages** - Follow conventional commits
4. **Handle conflicts gracefully** - Guide user through resolution
5. **Provide clear options** - Numbered choices when decisions needed
6. **Confirm successful operations** - Show result summary
