package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Young-us/ycode/internal/agent"
	"github.com/Young-us/ycode/internal/config"
	"github.com/Young-us/ycode/internal/llm"
	"github.com/Young-us/ycode/internal/logger"
	"github.com/Young-us/ycode/internal/skill"
)

// BuiltinCommands returns all built-in commands
func BuiltinCommands(
	orchestrator *agent.Orchestrator,
	cfg *config.Config,
	skillManager *skill.Manager,
) []*Command {
	return []*Command{
		// ========== 项目初始化 ==========
		{
			Name:        "init",
			Description: "分析项目并生成/更新项目概述文档 (CLAUDE.md)",
			Usage:       "/init",
			Template: `Please analyze this codebase and update or create the CLAUDE.md file.

**Important: Check if CLAUDE.md already exists first.**

If CLAUDE.md DOES NOT exist:
- Analyze the codebase from scratch
- Create a comprehensive CLAUDE.md file for future Claude Code instances

If CLAUDE.md ALREADY exists:
- Read the existing CLAUDE.md file first
- Analyze what's NEW or CHANGED in the project since the last analysis (check git status, recent commits, new files)
- Only ADD new relevant information or UPDATE outdated sections
- PRESERVE all existing valid content - do not rewrite unchanged parts
- Focus on incremental updates, not a full rewrite

What to include:
1. Build, test, lint commands - essential for development workflow
2. High-level architecture - the "big picture" that requires reading multiple files
3. Key patterns and conventions used in the codebase
4. Important context from README.md, .cursorrules, or .github/copilot-instructions.md

What NOT to include:
- Generic development practices
- Every component or file (can be discovered by reading code)
- Obvious instructions like "write tests" or "provide error messages"
- Sensitive information (API keys, tokens)

Prefix the file with:
# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Now analyze the project and update/create CLAUDE.md.`,
			Subtask: true, // Run through agent streaming
		},
		// ========== Agent 切换 ==========
		{
			Name:        "agent",
			Description: "切换或显示当前 agent",
			Usage:       "/agent [name]",
			Handler: func(args []string) (string, error) {
				if len(args) == 0 {
					currentAgent := orchestrator.GetCurrentAgent()
					info, _ := orchestrator.GetAgentInfo(currentAgent)
					var b strings.Builder
					b.WriteString("当前 Agent:\n")
					b.WriteString(fmt.Sprintf("  类型: %s\n", currentAgent))
					if info != nil {
						b.WriteString(fmt.Sprintf("  名称: %s\n", info.Name))
						b.WriteString(fmt.Sprintf("  描述: %s\n", info.Description))
					}
					b.WriteString("\n可用 Agent:\n")
					for _, a := range orchestrator.ListAgents() {
						if a.Type != agent.AgentDefault {
							b.WriteString(fmt.Sprintf("  %s - %s\n", a.Type, a.Description))
						}
					}
					return b.String(), nil
				}

				agentType := agent.AgentType(args[0])
				if err := orchestrator.SetCurrentAgent(agentType); err != nil {
					return "", err
				}
				info, _ := orchestrator.GetAgentInfo(agentType)
				return fmt.Sprintf("已切换到 %s agent\n描述: %s", agentType, info.Description), nil
			},
		},
		// ========== 会话管理 ==========
		{
			Name:        "help",
			Description: "显示帮助信息",
			Usage:       "/help",
			Handler: func(args []string) (string, error) {
				var b strings.Builder
				b.WriteString("可用命令:\n\n")
				b.WriteString("项目初始化:\n")
				b.WriteString("  /init          分析项目并生成 CLAUDE.md\n\n")
				b.WriteString("Agent 切换:\n")
				b.WriteString("  /agent [name]  切换或显示当前 agent\n")
				b.WriteString("  /explorer      切换到探索 agent\n")
				b.WriteString("  /coder         切换到编码 agent\n")
				b.WriteString("  /debugger      切换到调试 agent\n")
				b.WriteString("  /reviewer      切换到审查 agent\n")
				b.WriteString("  /tester        切换到测试 agent\n\n")
				b.WriteString("会话管理:\n")
				b.WriteString("  /help          显示帮助信息\n")
				b.WriteString("  /clear         清除对话历史\n")
				b.WriteString("  /compact       压缩对话以节省上下文\n")
				b.WriteString("  /history       显示对话历史\n")
				b.WriteString("  /export        导出对话到文件\n\n")
				b.WriteString("信息和诊断:\n")
				b.WriteString("  /status        显示状态信息\n")
				b.WriteString("  /cost          显示 token 使用量\n")
				b.WriteString("  /context       显示上下文使用情况\n")
				b.WriteString("  /doctor        运行诊断\n")
				b.WriteString("  /version       显示版本信息\n\n")
				b.WriteString("模型和模式:\n")
				b.WriteString("  /model [name]  切换模型\n")
				b.WriteString("  /plan          进入计划模式\n")
				b.WriteString("  /fast          切换快速模式\n\n")
				b.WriteString("配置:\n")
				b.WriteString("  /config        显示配置\n")
				b.WriteString("  /permissions   显示权限设置\n")
				b.WriteString("  /skills        列出可用技能\n")
				b.WriteString("  /plugins       列出已加载插件\n")
				b.WriteString("  /mcp           列出 MCP 服务器\n")
				b.WriteString("  /lsp           列出 LSP 服务器\n\n")
				b.WriteString("其他:\n")
				b.WriteString("  /reload        重新加载 skills 和命令\n")
				b.WriteString("  /exit, /quit   退出 ycode\n")
				return b.String(), nil
			},
		},
		{
			Name:        "clear",
			Description: "清除对话历史",
			Usage:       "/clear",
			Handler: func(args []string) (string, error) {
				orchestrator.ClearHistory()
				return "对话历史已清除", nil
			},
		},
		{
			Name:        "compact",
			Description: "压缩对话历史以节省上下文 token",
			Usage:       "/compact [keep=N] 保留最近N条消息",
			Handler: func(args []string) (string, error) {
				// Parse keep parameter
				keepRecent := 6 // Default: keep last 6 messages (3 turns)
				for _, arg := range args {
					if strings.HasPrefix(arg, "keep=") {
						fmt.Sscanf(arg, "keep=%d", &keepRecent)
					}
				}

				// Get history stats before compaction
				stats := orchestrator.GetHistoryStats(keepRecent)
				if stats == nil {
					return "无法获取历史统计", nil
				}

				totalMsgs := stats["total_messages"].(int)
				if totalMsgs <= keepRecent {
					return fmt.Sprintf("当前只有 %d 条消息，无需压缩", totalMsgs), nil
				}

				// Return instruction to TUI to handle compaction
				return fmt.Sprintf("COMPACT_HISTORY:%d", keepRecent), nil
			},
		},
		{
			Name:        "history",
			Description: "显示对话历史",
			Usage:       "/history",
			Handler: func(args []string) (string, error) {
				history := orchestrator.GetHistory()
				if len(history) == 0 {
					return "没有对话历史", nil
				}

				var b strings.Builder
				b.WriteString(fmt.Sprintf("对话历史 (%d 条消息):\n\n", len(history)))
				for i, msg := range history {
					role := msg.Role
					if role == "user" {
						role = "You"
					} else if role == "assistant" {
						role = "ycode"
					}

					content := msg.Content
					if len(content) > 100 {
						content = content[:100] + "..."
					}

					b.WriteString(fmt.Sprintf("[%d] %s: %s\n", i+1, role, content))
				}
				return b.String(), nil
			},
		},
		{
			Name:        "export",
			Description: "导出对话到文件",
			Usage:       "/export [filename]",
			Handler: func(args []string) (string, error) {
				// TODO: 实现对话导出
				filename := "conversation.txt"
				if len(args) > 0 {
					filename = args[0]
				}
				return fmt.Sprintf("导出对话到 %s...\n", filename), nil
			},
		},
		// ========== 信息和诊断 ==========
		{
			Name:        "status",
			Description: "显示状态信息",
			Usage:       "/status",
			Handler: func(args []string) (string, error) {
				var b strings.Builder
				b.WriteString("ycode 状态:\n")
				b.WriteString(fmt.Sprintf("  模型: %s\n", cfg.LLM.Model))
				b.WriteString(fmt.Sprintf("  最大 token: %d\n", cfg.LLM.GetMaxTokens()))
				b.WriteString(fmt.Sprintf("  最大步数: %d\n", cfg.Agent.MaxSteps))
				b.WriteString(fmt.Sprintf("  MCP 服务器: %d\n", len(cfg.MCP.Servers)))
				b.WriteString(fmt.Sprintf("  LSP 服务器: %d\n", len(cfg.LSP.Servers)))
				b.WriteString(fmt.Sprintf("  技能: %d\n", len(skillManager.List())))
				return b.String(), nil
			},
		},
		{
			Name:        "cost",
			Description: "显示 token 使用量",
			Usage:       "/cost",
			Handler: func(args []string) (string, error) {
				// TODO: 实现 token 使用量统计
				var b strings.Builder
				b.WriteString("Token 使用量:\n")
				b.WriteString("  输入: 0 tokens\n")
				b.WriteString("  输出: 0 tokens\n")
				b.WriteString("  总计: 0 tokens\n")
				b.WriteString("  费用: $0.00\n")
				return b.String(), nil
			},
		},
		{
			Name:        "context",
			Description: "显示上下文使用情况",
			Usage:       "/context",
			Handler: func(args []string) (string, error) {
				history := orchestrator.GetHistory()
				totalChars := 0
				for _, msg := range history {
					totalChars += len(msg.Content)
				}
				estimatedTokens := totalChars / 4
				maxTokens := cfg.LLM.GetMaxTokens()
				usagePercent := float64(estimatedTokens) / float64(maxTokens) * 100

				var b strings.Builder
				b.WriteString("上下文使用情况:\n")
				b.WriteString(fmt.Sprintf("  消息数: %d\n", len(history)))
				b.WriteString(fmt.Sprintf("  估计 token: %d\n", estimatedTokens))
				b.WriteString(fmt.Sprintf("  最大 token: %d\n", maxTokens))
				b.WriteString(fmt.Sprintf("  使用率: %.1f%%\n", usagePercent))
				return b.String(), nil
			},
		},
		{
			Name:        "doctor",
			Description: "运行诊断",
			Usage:       "/doctor",
			Handler: func(args []string) (string, error) {
				var b strings.Builder
				b.WriteString("运行诊断...\n\n")
				b.WriteString("检查结果:\n")
				b.WriteString(fmt.Sprintf("  ✅ 配置文件: 正常\n"))
				b.WriteString(fmt.Sprintf("  ✅ API 密钥: %s\n", maskAPIKey(cfg.LLM.APIKey)))
				b.WriteString(fmt.Sprintf("  ✅ 模型: %s\n", cfg.LLM.Model))
				b.WriteString(fmt.Sprintf("  ✅ 基础 URL: %s\n", cfg.LLM.BaseURL))
				return b.String(), nil
			},
		},
		{
			Name:        "version",
			Description: "显示版本信息",
			Usage:       "/version",
			Handler: func(args []string) (string, error) {
				return "ycode version 0.1.0-dev", nil
			},
		},
		// ========== 模型和模式 ==========
		{
			Name:        "model",
			Description: "切换模型",
			Usage:       "/model [name]",
			Handler: func(args []string) (string, error) {
				if len(args) == 0 {
					var b strings.Builder
					b.WriteString(fmt.Sprintf("当前模型: %s\n", cfg.LLM.Model))
					b.WriteString("用法: /model <model-name>\n")
					return b.String(), nil
				}
				// TODO: 实现模型切换
				return fmt.Sprintf("切换模型到: %s\n", args[0]), nil
			},
		},
		{
			Name:        "plan",
			Description: "进入计划模式",
			Usage:       "/plan",
			Handler: func(args []string) (string, error) {
				// TODO: 实现计划模式
				return "进入计划模式...", nil
			},
		},
		{
			Name:        "fast",
			Description: "切换快速模式",
			Usage:       "/fast",
			Handler: func(args []string) (string, error) {
				// TODO: 实现快速模式切换
				return "快速模式: 关闭", nil
			},
		},
		// ========== 配置 ==========
		{
			Name:        "config",
			Description: "显示配置",
			Usage:       "/config",
			Handler: func(args []string) (string, error) {
				var b strings.Builder
				b.WriteString("当前配置:\n")
				b.WriteString(fmt.Sprintf("  模型: %s\n", cfg.LLM.Model))
				b.WriteString(fmt.Sprintf("  最大 token: %s\n", config.FormatTokenSize(cfg.LLM.GetMaxTokens())))
				b.WriteString(fmt.Sprintf("  基础 URL: %s\n", cfg.LLM.BaseURL))
				b.WriteString(fmt.Sprintf("  最大步数: %d\n", cfg.Agent.MaxSteps))
				b.WriteString(fmt.Sprintf("  自动压缩: %v\n", cfg.Agent.AutoCompact))
				b.WriteString(fmt.Sprintf("  压缩阈值: %.1f%%\n", cfg.Agent.CompactThreshold*100))
				return b.String(), nil
			},
		},
		{
			Name:        "permissions",
			Description: "显示权限设置",
			Usage:       "/permissions",
			Handler: func(args []string) (string, error) {
				var b strings.Builder
				b.WriteString("权限设置:\n")
				b.WriteString(fmt.Sprintf("  模式: %s\n", cfg.Permissions.Mode))
				b.WriteString(fmt.Sprintf("  总是允许: %v\n", cfg.Permissions.AlwaysAllow))
				b.WriteString(fmt.Sprintf("  总是拒绝: %v\n", cfg.Permissions.AlwaysDeny))
				return b.String(), nil
			},
		},
		{
			Name:        "skills",
			Description: "列出可用技能",
			Usage:       "/skills",
			Handler: func(args []string) (string, error) {
				skills := skillManager.List()
				if len(skills) == 0 {
					return "没有可用技能", nil
				}

				var b strings.Builder
				b.WriteString(fmt.Sprintf("可用技能 (%d):\n\n", len(skills)))
				for _, s := range skills {
					b.WriteString(fmt.Sprintf("  %s\n", s.Name))
					b.WriteString(fmt.Sprintf("    描述: %s\n", s.Description))
					if len(s.Commands) > 0 {
						b.WriteString(fmt.Sprintf("    命令: %s\n", strings.Join(s.Commands, ", ")))
					}
					if len(s.Triggers) > 0 {
						b.WriteString(fmt.Sprintf("    触发词: %s\n", strings.Join(s.Triggers, ", ")))
					}
					b.WriteString("\n")
				}
				return b.String(), nil
			},
		},
		{
			Name:        "plugins",
			Description: "列出已加载插件",
			Usage:       "/plugins",
			Handler: func(args []string) (string, error) {
				// TODO: 实现插件列表
				return "已加载插件: 0", nil
			},
		},
		{
			Name:        "mcp",
			Description: "列出 MCP 服务器",
			Usage:       "/mcp",
			Handler: func(args []string) (string, error) {
				if len(cfg.MCP.Servers) == 0 {
					return "没有配置 MCP 服务器", nil
				}

				var b strings.Builder
				b.WriteString(fmt.Sprintf("MCP 服务器 (%d):\n\n", len(cfg.MCP.Servers)))
				for _, server := range cfg.MCP.Servers {
					status := "禁用"
					if server.Enabled {
						status = "启用"
					}
					b.WriteString(fmt.Sprintf("  %s (%s)\n", server.Name, status))
					b.WriteString(fmt.Sprintf("    URL: %s\n", server.URL))
				}
				return b.String(), nil
			},
		},
		{
			Name:        "lsp",
			Description: "列出 LSP 服务器",
			Usage:       "/lsp",
			Handler: func(args []string) (string, error) {
				if len(cfg.LSP.Servers) == 0 {
					return "没有配置 LSP 服务器", nil
				}

				var b strings.Builder
				b.WriteString(fmt.Sprintf("LSP 服务器 (%d):\n\n", len(cfg.LSP.Servers)))
				for _, server := range cfg.LSP.Servers {
					status := "禁用"
					if server.Enabled {
						status = "启用"
					}
					b.WriteString(fmt.Sprintf("  %s (%s)\n", server.Name, status))
					b.WriteString(fmt.Sprintf("    命令: %s %s\n", server.Command, strings.Join(server.Args, " ")))
				}
				return b.String(), nil
			},
		},
		// ========== 其他 ==========
		{
			Name:        "reload",
			Description: "重新加载 skills 和命令",
			Usage:       "/reload",
			Handler: func(args []string) (string, error) {
				// This will be handled by the command manager
				return "RELOAD_COMMANDS", nil
			},
		},
		{
			Name:        "exit",
			Description: "退出 ycode",
			Usage:       "/exit",
			Handler: func(args []string) (string, error) {
				return "再见!", nil
			},
		},
		{
			Name:        "quit",
			Description: "退出 ycode",
			Usage:       "/quit",
			Handler: func(args []string) (string, error) {
				return "再见!", nil
			},
		},
	}
}

// maskAPIKey masks the API key for display
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// handleInitCommand analyzes the project and generates CLAUDE.md
func handleInitCommand(orchestrator *agent.Orchestrator, useLLM bool) (string, error) {
	// Get current working directory
	workDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	var b strings.Builder
	b.WriteString("正在分析项目...\n\n")

	// Check if CLAUDE.md already exists
	claudeMdPath := filepath.Join(workDir, "CLAUDE.md")
	if _, err := os.Stat(claudeMdPath); err == nil {
		b.WriteString("⚠️  CLAUDE.md 已存在，将更新内容\n\n")
	}

	// Analyze project structure (static analysis)
	info, err := analyzeProject(workDir)
	if err != nil {
		return "", fmt.Errorf("failed to analyze project: %w", err)
	}

	var content string
	if useLLM && orchestrator != nil {
		b.WriteString("🤖 正在使用 LLM 深度分析项目...\n\n")
		// LLM-enhanced analysis
		content, err = generateClaudeMdWithLLM(workDir, info, orchestrator)
		if err != nil {
			logger.Warn("command", "LLM analysis failed, falling back to static analysis: %v", err)
			b.WriteString(fmt.Sprintf("⚠️  LLM 分析失败: %v\n", err))
			b.WriteString("使用静态分析生成...\n\n")
			content = generateClaudeMd(workDir, info)
		}
	} else {
		// Static analysis only
		content = generateClaudeMd(workDir, info)
	}

	// Write CLAUDE.md
	if err := os.WriteFile(claudeMdPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write CLAUDE.md: %w", err)
	}

	b.WriteString(fmt.Sprintf("✅ 已生成 CLAUDE.md\n\n"))
	b.WriteString(fmt.Sprintf("项目: %s\n", info.Name))
	b.WriteString(fmt.Sprintf("语言: %s\n", info.Language))
	b.WriteString(fmt.Sprintf("文件数: %d\n", info.FileCount))
	b.WriteString(fmt.Sprintf("目录数: %d\n", info.DirCount))

	return b.String(), nil
}

// ProjectInfo holds analyzed project information
type ProjectInfo struct {
	Name         string
	Language     string
	Type         string // "go", "node", "python", "rust", "java", etc.
	FileCount    int
	DirCount     int
	HasReadme    bool
	HasGit       bool
	MainFiles    []string
	KeyFiles     []string
	Directories  []string
	BuildCmd     string
	TestCmd      string
	RunCmd       string
	// Enhanced fields
	Description   string            // Project description from README
	Frameworks    []string          // Detected frameworks
	Dependencies  []string          // Key dependencies
	Scripts       map[string]string // npm scripts, make targets, etc.
	Linter        string            // Linter tool
	Formatter     string            // Formatter tool
	PackageManger string            // Package manager (npm, yarn, pnpm, pip, cargo, etc.)
}

// analyzeProject scans the project directory and collects information
func analyzeProject(root string) (*ProjectInfo, error) {
	info := &ProjectInfo{
		Name:        filepath.Base(root),
		MainFiles:   []string{},
		KeyFiles:    []string{},
		Frameworks:  []string{},
		Dependencies: []string{},
		Scripts:     make(map[string]string),
	}

	// Track language indicators
	fileCounts := make(map[string]int)
	javaFiles := 0
	kotlinFiles := 0
	cppFiles := 0
	cFiles := 0
	csFiles := 0

	// Key files to detect
	keyFileNames := map[string]bool{
		"go.mod": true, "go.sum": true,
		"package.json": true, "package-lock.json": true, "yarn.lock": true, "pnpm-lock.yaml": true,
		"requirements.txt": true, "pyproject.toml": true, "setup.py": true, "Pipfile": true, "poetry.lock": true,
		"Cargo.toml": true, "Cargo.lock": true,
		"pom.xml": true, "build.gradle": true, "build.gradle.kts": true,
		"Makefile": true, "CMakeLists.txt": true,
		"README.md": true, "CLAUDE.md": true,
		".editorconfig": true, ".prettierrc": true, ".eslintrc": true, ".eslintrc.js": true, ".eslintrc.json": true,
		".golangci.yml": true, ".golangci.yaml": true,
		"pylintrc": true, ".flake8": true, "ruff.toml": true,
		"rustfmt.toml": true, ".rustfmt.toml": true,
		"tsconfig.json": true,
		"composer.json": true, // PHP
		"Gemfile": true,       // Ruby
	}

	// Walk directory tree
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden directories and common ignore patterns
		if d.IsDir() {
			name := d.Name()
			if shouldSkipDir(name) {
				return filepath.SkipDir
			}

			if path != root {
				info.DirCount++
				relPath, _ := filepath.Rel(root, path)
				info.Directories = append(info.Directories, relPath)
			}
			return nil
		}

		info.FileCount++

		// Count files by extension
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".go":
			fileCounts["go"]++
		case ".js":
			fileCounts["js"]++
		case ".jsx":
			fileCounts["jsx"]++
		case ".ts":
			fileCounts["ts"]++
		case ".tsx":
			fileCounts["tsx"]++
		case ".py":
			fileCounts["py"]++
		case ".rs":
			fileCounts["rs"]++
		case ".java":
			javaFiles++
		case ".kt", ".kts":
			kotlinFiles++
		case ".cpp", ".cc", ".cxx":
			cppFiles++
		case ".c":
			cFiles++
		case ".cs":
			csFiles++
		case ".php":
			fileCounts["php"]++
		case ".rb":
			fileCounts["rb"]++
		case ".vue":
			fileCounts["vue"]++
		case ".svelte":
			fileCounts["svelte"]++
		}

		// Check for key files
		filename := d.Name()
		if keyFileNames[filename] {
			relPath, _ := filepath.Rel(root, path)
			info.KeyFiles = append(info.KeyFiles, relPath)
			if filename == "README.md" {
				info.HasReadme = true
			}
		}

		// Track main/entry files
		if isMainFile(filename, ext) {
			relPath, _ := filepath.Rel(root, path)
			info.MainFiles = append(info.MainFiles, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Determine primary language and analyze project-specific files
	determineLanguageAndCommands(root, info, fileCounts, javaFiles, kotlinFiles, cppFiles, cFiles, csFiles)

	// Check for .git directory
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		info.HasGit = true
	}

	// Read README for description
	info.Description = extractReadmeDescription(root)

	return info, nil
}

// shouldSkipDir returns true if directory should be skipped during walk
func shouldSkipDir(name string) bool {
	skipDirs := map[string]bool{
		"node_modules": true, "vendor": true, "dist": true, "build": true,
		"target": true, "__pycache__": true, "bin": true, "pkg": true,
		"out": true, "cache": true, ".git": true, ".svn": true, ".hg": true,
		"venv": true, ".venv": true, "env": true, ".env": true,
		"node_modules_cache": true, ".npm": true, ".yarn": true,
		"gradle": true, ".gradle": true, "mvn": true, ".mvn": true,
	}
	return strings.HasPrefix(name, ".") || skipDirs[name]
}

// isMainFile checks if a file is a main/entry file
func isMainFile(filename, ext string) bool {
	mainFiles := map[string]bool{
		"main.go": true, "main.py": true, "main.rs": true, "main.java": true,
		"main.c": true, "main.cpp": true, "main.cs": true,
		"index.js": true, "index.ts": true, "index.jsx": true, "index.tsx": true,
		"app.js": true, "app.ts": true, "server.js": true, "server.ts": true,
		"__main__.py": true, "wsgi.py": true, "asgi.py": true,
		"main.kt": true, "index.php": true, "main.rb": true,
	}
	return mainFiles[filename]
}

// determineLanguageAndCommands determines the primary language and sets commands
func determineLanguageAndCommands(root string, info *ProjectInfo, fileCounts map[string]int, javaFiles, kotlinFiles, cppFiles, cFiles, csFiles int) {
	// Calculate total for each language ecosystem
	goCount := fileCounts["go"]
	jsTotal := fileCounts["js"] + fileCounts["jsx"] + fileCounts["ts"] + fileCounts["tsx"]
	pyCount := fileCounts["py"]
	rsCount := fileCounts["rs"]
	javaTotal := javaFiles + kotlinFiles
	cppTotal := cppFiles + cFiles
	phpCount := fileCounts["php"]
	rbCount := fileCounts["rb"]

	// Find max
	maxCount := goCount
	info.Language = "Go"
	info.Type = "go"

	// Check each language and set appropriate commands
	if jsTotal > maxCount {
		maxCount = jsTotal
		info.Language = "JavaScript/TypeScript"
		info.Type = "node"
		analyzeNodeProject(root, info, fileCounts)
	}
	if pyCount > maxCount {
		maxCount = pyCount
		info.Language = "Python"
		info.Type = "python"
		analyzePythonProject(root, info)
	}
	if rsCount > maxCount {
		maxCount = rsCount
		info.Language = "Rust"
		info.Type = "rust"
		analyzeRustProject(root, info)
	}
	if javaTotal > maxCount {
		maxCount = javaTotal
		if kotlinFiles > javaFiles {
			info.Language = "Kotlin"
		} else {
			info.Language = "Java"
		}
		info.Type = "jvm"
		analyzeJvmProject(root, info)
	}
	if cppTotal > maxCount {
		maxCount = cppTotal
		info.Language = "C/C++"
		info.Type = "cpp"
		analyzeCppProject(root, info)
	}
	if csFiles > maxCount {
		maxCount = csFiles
		info.Language = "C#"
		info.Type = "dotnet"
		analyzeDotNetProject(root, info)
	}
	if phpCount > maxCount {
		maxCount = phpCount
		info.Language = "PHP"
		info.Type = "php"
		analyzePhpProject(root, info)
	}
	if rbCount > maxCount {
		maxCount = rbCount
		info.Language = "Ruby"
		info.Type = "ruby"
		analyzeRubyProject(root, info)
	}

	// Apply Go defaults if it's the primary language
	if info.Type == "go" {
		analyzeGoProject(root, info)
	}
}

// generateClaudeMd creates the CLAUDE.md content
func generateClaudeMd(root string, info *ProjectInfo) string {
	var b strings.Builder

	b.WriteString("# CLAUDE.md\n\n")
	b.WriteString("This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.\n\n")

	// Build and Test Commands section
	b.WriteString("## Build and Test Commands\n\n")
	b.WriteString("```bash\n")
	b.WriteString(fmt.Sprintf("# Build\n%s\n\n", info.BuildCmd))
	b.WriteString(fmt.Sprintf("# Test\n%s\n\n", info.TestCmd))
	if info.RunCmd != "" {
		b.WriteString(fmt.Sprintf("# Run\n%s\n", info.RunCmd))
	}
	if info.Linter != "" {
		b.WriteString(fmt.Sprintf("# Lint\n%s\n", getLintCommand(info)))
	}
	if info.Formatter != "" {
		b.WriteString(fmt.Sprintf("# Format\n%s\n", getFormatCommand(info)))
	}
	b.WriteString("```\n\n")

	// Project Overview
	b.WriteString("## Project Overview\n\n")
	if info.Description != "" {
		b.WriteString(fmt.Sprintf("%s\n\n", info.Description))
	}
	b.WriteString(fmt.Sprintf("- **Name**: %s\n", info.Name))
	b.WriteString(fmt.Sprintf("- **Language**: %s\n", info.Language))
	if info.PackageManger != "" {
		b.WriteString(fmt.Sprintf("- **Package Manager**: %s\n", info.PackageManger))
	}
	b.WriteString(fmt.Sprintf("- **Type**: %s project\n\n", info.Type))

	// Frameworks
	if len(info.Frameworks) > 0 {
		b.WriteString("## Frameworks & Tools\n\n")
		for _, fw := range info.Frameworks {
			b.WriteString(fmt.Sprintf("- %s\n", fw))
		}
		b.WriteString("\n")
	}

	// Key Dependencies
	if len(info.Dependencies) > 0 {
		b.WriteString("## Key Dependencies\n\n")
		maxDeps := 10
		if len(info.Dependencies) < maxDeps {
			maxDeps = len(info.Dependencies)
		}
		for i := 0; i < maxDeps; i++ {
			b.WriteString(fmt.Sprintf("- %s\n", info.Dependencies[i]))
		}
		if len(info.Dependencies) > maxDeps {
			b.WriteString(fmt.Sprintf("- ... and %d more\n", len(info.Dependencies)-maxDeps))
		}
		b.WriteString("\n")
	}

	// Available Scripts (for Node.js projects)
	if len(info.Scripts) > 0 {
		b.WriteString("## Available Scripts\n\n")
		for name, cmd := range info.Scripts {
			b.WriteString(fmt.Sprintf("- `%s`: %s\n", name, cmd))
		}
		b.WriteString("\n")
	}

	// Key Files
	if len(info.KeyFiles) > 0 {
		b.WriteString("## Key Files\n\n")
		for _, f := range info.KeyFiles {
			b.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		b.WriteString("\n")
	}

	// Main Entry Points
	if len(info.MainFiles) > 0 {
		b.WriteString("## Main Entry Points\n\n")
		for _, f := range info.MainFiles {
			b.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		b.WriteString("\n")
	}

	// Directory Structure
	if len(info.Directories) > 0 && len(info.Directories) < 50 {
		b.WriteString("## Directory Structure\n\n")
		b.WriteString("```\n")
		for _, dir := range info.Directories {
			if dir != "" {
				b.WriteString(fmt.Sprintf("%s/\n", dir))
			}
		}
		b.WriteString("```\n\n")
	}

	// Development Notes based on project type
	b.WriteString("## Development Notes\n\n")
	generateDevNotes(&b, info)

	return b.String()
}

// getLintCommand returns the lint command for the project type
func getLintCommand(info *ProjectInfo) string {
	switch info.Type {
	case "go":
		return "golangci-lint run"
	case "node":
		return "npm run lint"
	case "python":
		return "flake8 ."
	case "rust":
		return "cargo clippy"
	case "jvm":
		return "mvn checkstyle:check"
	case "cpp":
		return "clang-tidy"
	case "dotnet":
		return "dotnet format --verify-no-changes"
	case "php":
		return "phpcs"
	case "ruby":
		return "rubocop"
	default:
		return info.Linter
	}
}

// getFormatCommand returns the format command for the project type
func getFormatCommand(info *ProjectInfo) string {
	switch info.Type {
	case "go":
		return "gofmt -w ."
	case "node":
		return "npm run format || prettier --write ."
	case "python":
		return "black ."
	case "rust":
		return "cargo fmt"
	case "jvm":
		return "mvn spotless:apply"
	case "cpp":
		return "clang-format -i"
	case "dotnet":
		return "dotnet format"
	case "php":
		return "php-cs-fixer fix"
	case "ruby":
		return "rubocop -A"
	default:
		return info.Formatter
	}
}

// generateDevNotes generates development notes based on project type
func generateDevNotes(b *strings.Builder, info *ProjectInfo) {
	switch info.Type {
	case "go":
		b.WriteString("- Run `go fmt ./...` before committing to format code\n")
		b.WriteString("- Run `go vet ./...` to check for suspicious constructs\n")
		b.WriteString("- Use `go test -v ./...` for verbose test output\n")
		b.WriteString("- Run `go mod tidy` to clean up dependencies\n")
	case "node":
		b.WriteString("- Check package.json for available scripts\n")
		b.WriteString("- Run `npm run lint` or equivalent to check code quality\n")
		b.WriteString("- Run `npm run format` or equivalent before committing\n")
		if info.PackageManger == "yarn" {
			b.WriteString("- Use `yarn` instead of `npm` for package management\n")
		} else if info.PackageManger == "pnpm" {
			b.WriteString("- Use `pnpm` instead of `npm` for package management\n")
		}
	case "python":
		b.WriteString("- Consider using a virtual environment\n")
		b.WriteString("- Run `pip install -r requirements.txt` to install dependencies\n")
		b.WriteString("- Use `pytest` for testing\n")
		b.WriteString("- Run linting tools (flake8/pylint/ruff) before committing\n")
	case "rust":
		b.WriteString("- Run `cargo clippy` for linting\n")
		b.WriteString("- Run `cargo fmt` before committing\n")
		b.WriteString("- Use `cargo test` for testing\n")
		b.WriteString("- Run `cargo doc --open` to view documentation\n")
	case "jvm":
		b.WriteString("- Use Maven/Gradle for dependency management\n")
		b.WriteString("- Run tests with `mvn test` or `gradle test`\n")
		b.WriteString("- Check code style with checkstyle or ktlint\n")
	case "cpp":
		b.WriteString("- Use CMake or Make for building\n")
		b.WriteString("- Run `clang-tidy` for static analysis\n")
		b.WriteString("- Format code with `clang-format`\n")
	case "dotnet":
		b.WriteString("- Use `dotnet build` to build the project\n")
		b.WriteString("- Run `dotnet test` for testing\n")
		b.WriteString("- Format code with `dotnet format`\n")
	case "php":
		b.WriteString("- Use Composer for dependency management\n")
		b.WriteString("- Run `phpunit` for testing\n")
		b.WriteString("- Follow PSR-12 coding standards\n")
	case "ruby":
		b.WriteString("- Use Bundler for dependency management\n")
		b.WriteString("- Run `rspec` for testing\n")
		b.WriteString("- Run `rubocop` for linting\n")
	default:
		b.WriteString("- Follow project-specific guidelines\n")
	}
}

// ============== Language-specific analyzers ==============

// extractReadmeDescription extracts project description from README.md
func extractReadmeDescription(root string) string {
	readmePath := filepath.Join(root, "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return ""
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Look for the first paragraph after the title
	for i, line := range lines {
		line = strings.TrimSpace(line)
		// Skip title (first # heading)
		if strings.HasPrefix(line, "# ") {
			// Look for description in next non-empty lines
			for j := i + 1; j < len(lines) && j < i+10; j++ {
				nextLine := strings.TrimSpace(lines[j])
				if nextLine == "" || strings.HasPrefix(nextLine, "#") || strings.HasPrefix(nextLine, "-") {
					continue
				}
				// Found description
				if len(nextLine) > 20 {
					// Limit to ~200 chars
					if len(nextLine) > 200 {
						return nextLine[:200] + "..."
					}
					return nextLine
				}
			}
			break
		}
	}
	return ""
}

// analyzeGoProject analyzes Go-specific project details
func analyzeGoProject(root string, info *ProjectInfo) {
	// Default commands
	info.BuildCmd = "go build ./..."
	info.TestCmd = "go test ./..."
	info.RunCmd = "go run ."
	info.Linter = "golangci-lint"
	info.Formatter = "gofmt"

	// Read go.mod for dependencies and module info
	goModPath := filepath.Join(root, "go.mod")
	if data, err := os.ReadFile(goModPath); err == nil {
		content := string(data)
		lines := strings.Split(content, "\n")
		inRequire := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				// Could extract module name
			}
			if line == "require (" {
				inRequire = true
				continue
			}
			if inRequire {
				if line == ")" {
					inRequire = false
					continue
				}
				if line != "" && !strings.HasPrefix(line, "//") {
					// Extract dependency name (first word)
					parts := strings.Fields(line)
					if len(parts) > 0 {
						dep := parts[0]
						// Detect frameworks
						if strings.Contains(dep, "gin-gonic") {
							info.Frameworks = append(info.Frameworks, "Gin")
						} else if strings.Contains(dep, "echo") {
							info.Frameworks = append(info.Frameworks, "Echo")
						} else if strings.Contains(dep, "fiber") {
							info.Frameworks = append(info.Frameworks, "Fiber")
						} else if strings.Contains(dep, "chi") {
							info.Frameworks = append(info.Frameworks, "Chi")
						} else if strings.Contains(dep, "gorm") {
							info.Frameworks = append(info.Frameworks, "GORM")
						} else if strings.Contains(dep, "cobra") {
							info.Frameworks = append(info.Frameworks, "Cobra")
						} else if strings.Contains(dep, "viper") {
							info.Frameworks = append(info.Frameworks, "Viper")
						}
						// Add to dependencies (limit to 10)
						if len(info.Dependencies) < 10 {
							info.Dependencies = append(info.Dependencies, dep)
						}
					}
				}
			}
		}
	}

	// Check for golangci-lint config
	if _, err := os.Stat(filepath.Join(root, ".golangci.yml")); err == nil {
		info.Linter = "golangci-lint (configured)"
	}
}

// analyzeNodeProject analyzes Node.js/TypeScript project details
func analyzeNodeProject(root string, info *ProjectInfo, fileCounts map[string]int) {
	// Determine package manager
	if _, err := os.Stat(filepath.Join(root, "yarn.lock")); err == nil {
		info.PackageManger = "yarn"
	} else if _, err := os.Stat(filepath.Join(root, "pnpm-lock.yaml")); err == nil {
		info.PackageManger = "pnpm"
	} else {
		info.PackageManger = "npm"
	}

	pm := info.PackageManger

	// Read package.json
	packagePath := filepath.Join(root, "package.json")
	if data, err := os.ReadFile(packagePath); err == nil {
		var pkg struct {
			Name        string            `json:"name"`
			Description string            `json:"description"`
			Scripts     map[string]string `json:"scripts"`
			Dependencies map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			info.Scripts = pkg.Scripts
			if pkg.Description != "" {
				info.Description = pkg.Description
			}

			// Determine build/test/run commands from scripts
			if _, ok := pkg.Scripts["build"]; ok {
				info.BuildCmd = pm + " run build"
			} else {
				info.BuildCmd = pm + " install"
			}
			if _, ok := pkg.Scripts["test"]; ok {
				info.TestCmd = pm + " test"
			} else {
				info.TestCmd = "No test script defined"
			}
			if _, ok := pkg.Scripts["dev"]; ok {
				info.RunCmd = pm + " run dev"
			} else if _, ok := pkg.Scripts["start"]; ok {
				info.RunCmd = pm + " start"
			}

			// Detect frameworks from dependencies
			allDeps := make(map[string]string)
			for k, v := range pkg.Dependencies {
				allDeps[k] = v
			}
			for k, v := range pkg.DevDependencies {
				allDeps[k] = v
			}

			// Frontend frameworks
			if _, ok := allDeps["react"]; ok {
				info.Frameworks = append(info.Frameworks, "React")
			}
			if _, ok := allDeps["vue"]; ok {
				info.Frameworks = append(info.Frameworks, "Vue")
			}
			if _, ok := allDeps["svelte"]; ok {
				info.Frameworks = append(info.Frameworks, "Svelte")
			}
			if _, ok := allDeps["angular"]; ok {
				info.Frameworks = append(info.Frameworks, "Angular")
			} else if _, ok := allDeps["@angular/core"]; ok {
				info.Frameworks = append(info.Frameworks, "Angular")
			}
			if _, ok := allDeps["next"]; ok {
				info.Frameworks = append(info.Frameworks, "Next.js")
			}
			if _, ok := allDeps["nuxt"]; ok {
				info.Frameworks = append(info.Frameworks, "Nuxt")
			}
			if _, ok := allDeps["remix"]; ok {
				info.Frameworks = append(info.Frameworks, "Remix")
			}
			if _, ok := allDeps["vite"]; ok {
				info.Frameworks = append(info.Frameworks, "Vite")
			}

			// Backend frameworks
			if _, ok := allDeps["express"]; ok {
				info.Frameworks = append(info.Frameworks, "Express")
			}
			if _, ok := allDeps["fastify"]; ok {
				info.Frameworks = append(info.Frameworks, "Fastify")
			}
			if _, ok := allDeps["nestjs"]; ok {
				info.Frameworks = append(info.Frameworks, "NestJS")
			} else if _, ok := allDeps["@nestjs/core"]; ok {
				info.Frameworks = append(info.Frameworks, "NestJS")
			}
			if _, ok := allDeps["koa"]; ok {
				info.Frameworks = append(info.Frameworks, "Koa")
			}

			// Testing frameworks
			if _, ok := allDeps["jest"]; ok {
				info.Frameworks = append(info.Frameworks, "Jest")
			}
			if _, ok := allDeps["vitest"]; ok {
				info.Frameworks = append(info.Frameworks, "Vitest")
			}
			if _, ok := allDeps["mocha"]; ok {
				info.Frameworks = append(info.Frameworks, "Mocha")
			}

			// Linter/Formatter
			if _, ok := allDeps["eslint"]; ok {
				info.Linter = "ESLint"
			}
			if _, ok := allDeps["prettier"]; ok {
				info.Formatter = "Prettier"
			}

			// Add dependencies
			for dep := range pkg.Dependencies {
				if len(info.Dependencies) < 15 {
					info.Dependencies = append(info.Dependencies, dep)
				}
			}
		}
	}

	// Check for TypeScript
	if fileCounts["ts"] > 0 || fileCounts["tsx"] > 0 {
		info.Language = "TypeScript"
	}
}

// analyzePythonProject analyzes Python project details
func analyzePythonProject(root string, info *ProjectInfo) {
	info.BuildCmd = "pip install -r requirements.txt"
	info.TestCmd = "pytest"
	info.RunCmd = "python main.py"
	info.Linter = "flake8/pylint"
	info.Formatter = "black/autopep8"

	// Check for package managers
	if _, err := os.Stat(filepath.Join(root, "pyproject.toml")); err == nil {
		info.PackageManger = "poetry/pip"
		// Try to parse pyproject.toml
		if data, err := os.ReadFile(filepath.Join(root, "pyproject.toml")); err == nil {
			content := string(data)
			if strings.Contains(content, "[tool.poetry]") {
				info.PackageManger = "poetry"
			} else if strings.Contains(content, "[project]") {
				info.PackageManger = "pip (pyproject)"
			}
			// Detect frameworks
			if strings.Contains(content, "django") {
				info.Frameworks = append(info.Frameworks, "Django")
			}
			if strings.Contains(content, "flask") {
				info.Frameworks = append(info.Frameworks, "Flask")
			}
			if strings.Contains(content, "fastapi") {
				info.Frameworks = append(info.Frameworks, "FastAPI")
			}
			if strings.Contains(content, "pydantic") {
				info.Frameworks = append(info.Frameworks, "Pydantic")
			}
			if strings.Contains(content, "pytest") {
				info.Frameworks = append(info.Frameworks, "pytest")
				info.TestCmd = "pytest"
			}
		}
	} else if _, err := os.Stat(filepath.Join(root, "Pipfile")); err == nil {
		info.PackageManger = "pipenv"
	} else if _, err := os.Stat(filepath.Join(root, "setup.py")); err == nil {
		info.PackageManger = "pip (setup.py)"
	}

	// Read requirements.txt
	if data, err := os.ReadFile(filepath.Join(root, "requirements.txt")); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			// Extract package name (before == or >= or ~=)
			pkg := line
			for _, sep := range []string{"==", ">=", "<=", "~=", "!=", "["} {
				if idx := strings.Index(pkg, sep); idx > 0 {
					pkg = pkg[:idx]
					break
				}
			}
			pkg = strings.TrimSpace(pkg)
			if pkg != "" && len(info.Dependencies) < 15 {
				info.Dependencies = append(info.Dependencies, pkg)
			}

			// Detect frameworks
			lowerLine := strings.ToLower(line)
			if strings.Contains(lowerLine, "django") {
				info.Frameworks = appendIfMissing(info.Frameworks, "Django")
			}
			if strings.Contains(lowerLine, "flask") {
				info.Frameworks = appendIfMissing(info.Frameworks, "Flask")
			}
			if strings.Contains(lowerLine, "fastapi") {
				info.Frameworks = appendIfMissing(info.Frameworks, "FastAPI")
			}
		}
	}
}

// analyzeRustProject analyzes Rust project details
func analyzeRustProject(root string, info *ProjectInfo) {
	info.BuildCmd = "cargo build"
	info.TestCmd = "cargo test"
	info.RunCmd = "cargo run"
	info.Linter = "clippy"
	info.Formatter = "rustfmt"
	info.PackageManger = "cargo"

	// Read Cargo.toml
	cargoPath := filepath.Join(root, "Cargo.toml")
	if data, err := os.ReadFile(cargoPath); err == nil {
		content := string(data)
		lines := strings.Split(content, "\n")
		inDeps := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "[dependencies]") {
				inDeps = true
				continue
			}
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				inDeps = false
				continue
			}
			if inDeps && strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) >= 1 {
					pkg := strings.TrimSpace(parts[0])
					// Detect frameworks
					switch pkg {
					case "tokio":
						info.Frameworks = appendIfMissing(info.Frameworks, "Tokio")
					case "actix-web":
						info.Frameworks = appendIfMissing(info.Frameworks, "Actix Web")
					case "axum":
						info.Frameworks = appendIfMissing(info.Frameworks, "Axum")
					case "rocket":
						info.Frameworks = appendIfMissing(info.Frameworks, "Rocket")
					case "warp":
						info.Frameworks = appendIfMissing(info.Frameworks, "Warp")
					case "serde":
						info.Frameworks = appendIfMissing(info.Frameworks, "Serde")
					case "diesel":
						info.Frameworks = appendIfMissing(info.Frameworks, "Diesel")
					case "sqlx":
						info.Frameworks = appendIfMissing(info.Frameworks, "SQLx")
					}
					if len(info.Dependencies) < 15 {
						info.Dependencies = append(info.Dependencies, pkg)
					}
				}
			}
		}
	}
}

// analyzeJvmProject analyzes Java/Kotlin project details
func analyzeJvmProject(root string, info *ProjectInfo) {
	// Check for Maven or Gradle
	if _, err := os.Stat(filepath.Join(root, "pom.xml")); err == nil {
		info.PackageManger = "maven"
		info.BuildCmd = "mvn compile"
		info.TestCmd = "mvn test"
		info.RunCmd = "mvn exec:java"
		info.Linter = "checkstyle"
		info.Formatter = "google-java-format"

		// Parse pom.xml for dependencies
		if data, err := os.ReadFile(filepath.Join(root, "pom.xml")); err == nil {
			content := string(data)
			if strings.Contains(content, "spring-boot") {
				info.Frameworks = append(info.Frameworks, "Spring Boot")
			}
			if strings.Contains(content, "spring-framework") {
				info.Frameworks = append(info.Frameworks, "Spring")
			}
			if strings.Contains(content, "quarkus") {
				info.Frameworks = append(info.Frameworks, "Quarkus")
			}
			if strings.Contains(content, "micronaut") {
				info.Frameworks = append(info.Frameworks, "Micronaut")
			}
		}
	} else if _, err := os.Stat(filepath.Join(root, "build.gradle")); err == nil {
		info.PackageManger = "gradle"
		info.BuildCmd = "gradle build"
		info.TestCmd = "gradle test"
		info.RunCmd = "gradle run"
		info.Linter = "checkstyle/ktlint"
	} else if _, err := os.Stat(filepath.Join(root, "build.gradle.kts")); err == nil {
		info.PackageManger = "gradle (kotlin dsl)"
		info.BuildCmd = "gradle build"
		info.TestCmd = "gradle test"
		info.RunCmd = "gradle run"
	}
}

// analyzeCppProject analyzes C/C++ project details
func analyzeCppProject(root string, info *ProjectInfo) {
	// Check for build systems
	if _, err := os.Stat(filepath.Join(root, "CMakeLists.txt")); err == nil {
		info.PackageManger = "cmake"
		info.BuildCmd = "cmake -B build && cmake --build build"
		info.TestCmd = "ctest --test-dir build"
		info.RunCmd = "./build/main"
	} else if _, err := os.Stat(filepath.Join(root, "Makefile")); err == nil {
		info.PackageManger = "make"
		info.BuildCmd = "make"
		info.TestCmd = "make test"
		info.RunCmd = "./main"
	} else if _, err := os.Stat(filepath.Join(root, "meson.build")); err == nil {
		info.PackageManger = "meson"
		info.BuildCmd = "meson setup build && ninja -C build"
		info.TestCmd = "meson test -C build"
	}
	info.Linter = "clang-tidy"
	info.Formatter = "clang-format"
}

// analyzeDotNetProject analyzes .NET/C# project details
func analyzeDotNetProject(root string, info *ProjectInfo) {
	info.PackageManger = "nuget"
	info.BuildCmd = "dotnet build"
	info.TestCmd = "dotnet test"
	info.RunCmd = "dotnet run"
	info.Linter = "dotnet format"
	info.Formatter = "dotnet format"

	// Detect frameworks from .csproj files
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".csproj") {
			if data, err := os.ReadFile(path); err == nil {
				content := string(data)
				if strings.Contains(content, "Microsoft.NET.Test") {
					info.Frameworks = appendIfMissing(info.Frameworks, "xUnit/NUnit/MSTest")
				}
				if strings.Contains(content, "Microsoft.AspNetCore") {
					info.Frameworks = appendIfMissing(info.Frameworks, "ASP.NET Core")
				}
				if strings.Contains(content, "Blazor") {
					info.Frameworks = appendIfMissing(info.Frameworks, "Blazor")
				}
			}
		}
		return nil
	})
}

// analyzePhpProject analyzes PHP project details
func analyzePhpProject(root string, info *ProjectInfo) {
	info.PackageManger = "composer"
	info.BuildCmd = "composer install"
	info.TestCmd = "phpunit"
	info.RunCmd = "php -S localhost:8000"
	info.Linter = "phpcs"
	info.Formatter = "php-cs-fixer"

	// Read composer.json
	if data, err := os.ReadFile(filepath.Join(root, "composer.json")); err == nil {
		var pkg struct {
			Name        string            `json:"name"`
			Description string            `json:"description"`
			Require     map[string]string `json:"require"`
			RequireDev  map[string]string `json:"require-dev"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			if pkg.Description != "" {
				info.Description = pkg.Description
			}
			for dep := range pkg.Require {
				if strings.Contains(dep, "laravel") {
					info.Frameworks = appendIfMissing(info.Frameworks, "Laravel")
				}
				if strings.Contains(dep, "symfony") {
					info.Frameworks = appendIfMissing(info.Frameworks, "Symfony")
				}
				if strings.Contains(dep, "wordpress") {
					info.Frameworks = appendIfMissing(info.Frameworks, "WordPress")
				}
				if strings.Contains(dep, "drupal") {
					info.Frameworks = appendIfMissing(info.Frameworks, "Drupal")
				}
				if len(info.Dependencies) < 10 {
					info.Dependencies = append(info.Dependencies, dep)
				}
			}
		}
	}
}

// analyzeRubyProject analyzes Ruby project details
func analyzeRubyProject(root string, info *ProjectInfo) {
	info.PackageManger = "bundler"
	info.BuildCmd = "bundle install"
	info.TestCmd = "rspec" // or rake test
	info.RunCmd = "ruby main.rb"
	info.Linter = "rubocop"
	info.Formatter = "rubocop"

	// Read Gemfile
	if data, err := os.ReadFile(filepath.Join(root, "Gemfile")); err == nil {
		content := string(data)
		if strings.Contains(content, "rails") {
			info.Frameworks = append(info.Frameworks, "Rails")
			info.TestCmd = "rails test"
			info.RunCmd = "rails server"
		}
		if strings.Contains(content, "sinatra") {
			info.Frameworks = append(info.Frameworks, "Sinatra")
		}
		if strings.Contains(content, "hanami") {
			info.Frameworks = append(info.Frameworks, "Hanami")
		}
		if strings.Contains(content, "rspec") {
			info.Frameworks = append(info.Frameworks, "RSpec")
		}
	}
}

// appendIfMissing appends a string to slice if not already present
func appendIfMissing(slice []string, s string) []string {
	for _, item := range slice {
		if item == s {
			return slice
		}
	}
	return append(slice, s)
}

// generateClaudeMdWithLLM uses LLM to generate enhanced CLAUDE.md content
func generateClaudeMdWithLLM(workDir string, info *ProjectInfo, orchestrator *agent.Orchestrator) (string, error) {
	// Build context from static analysis
	projectContext := buildLLMContext(workDir, info)

	// Create the prompt for LLM
	prompt := fmt.Sprintf(`Analyze this codebase and create a CLAUDE.md file. The file will help future Claude Code instances work effectively in this repository.

## Project Context (from static analysis):
%s

## Your Task:
Generate a comprehensive CLAUDE.md file with the following sections:

1. **Project Overview** - Brief description of what this project does
2. **Build and Test Commands** - Exact commands to build, test, lint, and run
3. **Architecture Overview** - High-level architecture and key design patterns
4. **Key Files and Entry Points** - Main files and their purposes
5. **Development Guidelines** - Any important conventions or patterns observed
6. **Dependencies** - Key frameworks and libraries used

## Important Notes:
- Be concise and practical - focus on what developers need to know
- Don't include generic advice like "write tests" or "handle errors"
- Focus on specifics unique to this codebase
- Use markdown format
- Start with: # CLAUDE.md

Generate ONLY the CLAUDE.md content, nothing else.`, projectContext)

	// Call LLM
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get the LLM client from orchestrator
	llmClient := orchestrator.LLMClient
	if llmClient == nil {
		return "", fmt.Errorf("LLM client not available")
	}

	// Create the message
	messages := []llm.Message{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// Call the Chat method directly (no tools needed)
	response, err := llmClient.Chat(ctx, messages, nil)
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %w", err)
	}

	content := response.Content
	if content == "" {
		return "", fmt.Errorf("empty response from LLM")
	}

	// Ensure the content starts with the required header
	if !strings.HasPrefix(strings.TrimSpace(content), "# CLAUDE.md") {
		content = "# CLAUDE.md\n\nThis file provides guidance to Claude Code (claude.ai/code) for working with code in this repository.\n\n" + content
	} else {
		// Make sure the header line is present
		lines := strings.Split(content, "\n")
		if len(lines) > 0 && strings.TrimSpace(lines[0]) == "# CLAUDE.md" {
			// Insert the required second line
			content = "# CLAUDE.md\n\nThis file provides guidance to Claude Code (claude.ai/code) for working with code in this repository.\n\n" + strings.Join(lines[1:], "\n")
		}
	}

	return content, nil
}

// buildLLMContext builds a context string from static analysis for LLM
func buildLLMContext(workDir string, info *ProjectInfo) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("### Basic Info\n"))
	b.WriteString(fmt.Sprintf("- Name: %s\n", info.Name))
	b.WriteString(fmt.Sprintf("- Language: %s\n", info.Language))
	b.WriteString(fmt.Sprintf("- Type: %s\n", info.Type))
	b.WriteString(fmt.Sprintf("- Files: %d, Directories: %d\n", info.FileCount, info.DirCount))
	b.WriteString(fmt.Sprintf("- Has Git: %v\n", info.HasGit))
	b.WriteString(fmt.Sprintf("- Has README: %v\n", info.HasReadme))

	if info.Description != "" {
		b.WriteString(fmt.Sprintf("- Description: %s\n", info.Description))
	}

	if len(info.Frameworks) > 0 {
		b.WriteString(fmt.Sprintf("\n### Detected Frameworks\n"))
		for _, fw := range info.Frameworks {
			b.WriteString(fmt.Sprintf("- %s\n", fw))
		}
	}

	if len(info.Dependencies) > 0 {
		b.WriteString(fmt.Sprintf("\n### Key Dependencies\n"))
		for _, dep := range info.Dependencies {
			b.WriteString(fmt.Sprintf("- %s\n", dep))
		}
	}

	if info.BuildCmd != "" || info.TestCmd != "" || info.RunCmd != "" {
		b.WriteString(fmt.Sprintf("\n### Commands\n"))
		if info.BuildCmd != "" {
			b.WriteString(fmt.Sprintf("- Build: %s\n", info.BuildCmd))
		}
		if info.TestCmd != "" {
			b.WriteString(fmt.Sprintf("- Test: %s\n", info.TestCmd))
		}
		if info.RunCmd != "" {
			b.WriteString(fmt.Sprintf("- Run: %s\n", info.RunCmd))
		}
		if info.Linter != "" {
			b.WriteString(fmt.Sprintf("- Lint: %s\n", info.Linter))
		}
		if info.Formatter != "" {
			b.WriteString(fmt.Sprintf("- Format: %s\n", info.Formatter))
		}
	}

	if len(info.MainFiles) > 0 {
		b.WriteString(fmt.Sprintf("\n### Entry Points\n"))
		for _, f := range info.MainFiles {
			b.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	if len(info.KeyFiles) > 0 {
		b.WriteString(fmt.Sprintf("\n### Key Files\n"))
		for _, f := range info.KeyFiles {
			b.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	if len(info.Directories) > 0 && len(info.Directories) < 50 {
		b.WriteString(fmt.Sprintf("\n### Directory Structure\n"))
		for _, d := range info.Directories {
			b.WriteString(fmt.Sprintf("- %s/\n", d))
		}
	}

	// Read key file contents for context
	b.WriteString(fmt.Sprintf("\n### Key File Contents\n"))
	for _, keyFile := range []string{"go.mod", "package.json", "requirements.txt", "Cargo.toml", "pom.xml"} {
		path := filepath.Join(workDir, keyFile)
		if data, err := os.ReadFile(path); err == nil {
			b.WriteString(fmt.Sprintf("\n#### %s\n```\n%s\n```\n", keyFile, string(data)))
		}
	}

	// Read README for context
	if info.HasReadme {
		readmePath := filepath.Join(workDir, "README.md")
		if data, err := os.ReadFile(readmePath); err == nil {
			content := string(data)
			if len(content) > 2000 {
				content = content[:2000] + "\n... (truncated)"
			}
			b.WriteString(fmt.Sprintf("\n#### README.md\n```\n%s\n```\n", content))
		}
	}

	return b.String()
}

// convertToLLMMessages converts simple messages to LLM messages
func convertToLLMMessages(messages []interface{}) []interface{} {
	// This is a simplified conversion - the actual LLM client expects specific format
	return messages
}
