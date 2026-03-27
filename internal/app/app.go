package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Young-us/ycode/internal/agent"
	"github.com/Young-us/ycode/internal/audit"
	"github.com/Young-us/ycode/internal/command"
	"github.com/Young-us/ycode/internal/config"
	"github.com/Young-us/ycode/internal/llm"
	"github.com/Young-us/ycode/internal/logger"
	"github.com/Young-us/ycode/internal/lsp"
	"github.com/Young-us/ycode/internal/mcp"
	"github.com/Young-us/ycode/internal/plugin"
	"github.com/Young-us/ycode/internal/skill"
	"github.com/Young-us/ycode/internal/tools"
	"github.com/Young-us/ycode/internal/ui"
	"github.com/spf13/cobra"
)

// Run executes the ycode application
func Run() error {
	rootCmd := newRootCmd()
	return rootCmd.Execute()
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "ycode",
		Short: "AI-powered coding assistant",
		Long: `ycode is an AI-powered coding assistant that helps you code faster.
It understands your codebase and can execute tasks through natural language commands.`,
	}

	// Add subcommands
	rootCmd.AddCommand(newChatCmd())
	rootCmd.AddCommand(newVersionCmd())

	return rootCmd
}

func newChatCmd() *cobra.Command {
	var noUI bool
	var maxSteps int
	var yolo bool
	var model string

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Start a chat session with ycode",
		Long:  `Start an interactive chat session where you can give instructions to ycode.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Override model if specified
			if model != "" {
				cfg.LLM.Model = model
			}

			// Override max steps if specified
			if cmd.Flags().Changed("max-steps") {
				cfg.Agent.MaxSteps = maxSteps
			}

			// Get working directory
			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			// Initialize LLM client
			var llmClient llm.Client
			if cfg.LLM.APIKey == "" {
				// Use mock client if no API key
				logger.Warn("startup", "No API key found. Using mock client.")
				llmClient = llm.NewMockClient(llm.Response{
					Content: "Hello! I'm ycode, your AI coding assistant. How can I help you today?",
				})
			} else {
				// Use real Anthropic client
				anthropicClient := llm.NewAnthropicClient(cfg.LLM.APIKey)
				anthropicClient.Model = cfg.LLM.Model
				anthropicClient.MaxTokens = cfg.LLM.GetMaxTokens()
				if cfg.LLM.BaseURL != "" {
					anthropicClient.BaseURL = cfg.LLM.BaseURL
				}
				llmClient = anthropicClient
			}

			// Initialize tool manager
			toolManager := tools.NewManager()
			toolManager.Register(tools.NewReadFileTool(workDir))
			toolManager.Register(tools.NewWriteFileTool(workDir))
			toolManager.Register(tools.NewEditFileTool(workDir))
			toolManager.Register(tools.NewGlobTool(workDir))
			toolManager.Register(tools.NewGrepTool(workDir))
			toolManager.Register(tools.NewBashTool(workDir))
			toolManager.Register(tools.NewGitTool(workDir))
			toolManager.Register(tools.NewASTTool(workDir))

			// Initialize MCP clients and register tools
			mcpClients := make(map[string]*mcp.Client)
			connectedMCPServers := []string{}
			for _, server := range cfg.MCP.Servers {
				if !server.Enabled {
					continue
				}

				var client *mcp.Client
				ctx := context.Background()

				// Support both command-based (standard MCP) and URL-based (legacy) servers
				if server.Command != "" {
					// Stdio-based MCP server (standard JSON-RPC 2.0)
					client = mcp.NewClient(server.Name, server.Command, server.Args...)
					if err := client.StartServerAndInitialize(ctx); err != nil {
						logger.Error("mcp", "Failed to start MCP server %s: %v", server.Name, err)
						continue
					}
				} else if server.URL != "" {
					// Legacy URL-based MCP server
					client = mcp.NewClientFromURL(server.Name, server.URL)
					client.Status = mcp.StatusConnected
				} else {
					logger.Warn("startup", "MCP server %s has no command or URL configured", server.Name)
					continue
				}

				mcpClients[server.Name] = client
				connectedMCPServers = append(connectedMCPServers, server.Name)

				// List and register MCP tools
				mcpTools, err := client.ListTools(ctx)
				if err != nil {
					logger.Warn("startup", "failed to list tools from MCP server %s: %v", server.Name, err)
					continue
				}

				// Convert and register tools
				convertedTools := client.ConvertTools(mcpTools)
				for _, tool := range convertedTools {
					toolManager.Register(tool)
				}

				logger.Info("startup", "MCP server connected: %s (%d tools)", server.Name, len(convertedTools))
			}

			// Initialize LSP clients and start servers
			lspClients := make(map[string]*lsp.Client)
			connectedLSPServers := []string{}
			for _, server := range cfg.LSP.Servers {
				if server.Enabled {
					// Create LSP client
					client := lsp.NewClient()

					// Find the full path to the command
					commandPath, err := exec.LookPath(server.Command)
					if err != nil {
						logger.Warn("startup", "LSP server %s not found in PATH: %s", server.Name, server.Command)
						continue
					}

					// Start the LSP server process
					if err := client.StartServer(commandPath, server.Args...); err != nil {
						logger.Warn("startup", "failed to start LSP server %s: %v", server.Name, err)
						continue
					}

					// Initialize the LSP server with timeout
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					workDirURI := lsp.DocumentURI("file://" + workDir)
					_, err = client.Initialize(ctx, workDirURI)
					cancel() // Always cancel to release resources

					if err != nil {
						logger.Warn("startup", "failed to initialize LSP server %s: %v", server.Name, err)
						continue
					}

					lspClients[server.Name] = client
					connectedLSPServers = append(connectedLSPServers, server.Name)
					logger.Info("startup", "LSP server started: %s (command: %s)", server.Name, commandPath)
				}
			}

			// Register LSP tool if any servers are running
			if len(lspClients) > 0 {
				lspTool := tools.NewLSPTool(workDir, lspClients)
				toolManager.Register(lspTool)
				logger.Info("startup", "LSP tool registered with %d server(s)", len(lspClients))
			}

			// Initialize plugin manager
			var pluginManager *plugin.Manager
			if cfg.Plugins.Enabled {
				pluginManager = plugin.NewManager(cfg.Plugins)

				// Load plugins from multiple directories:
				// 1. plugins/ - System built-in plugins
				// 2. ~/.ycode/plugins/ - Global user plugins
				// 3. .ycode/plugins/ - Project-specific plugins
				ctx := context.Background()
				if err := pluginManager.LoadFromConfig(ctx); err != nil {
					logger.Warn("startup", "failed to load plugins: %v", err)
				} else {
					plugins := pluginManager.List()
					if len(plugins) > 0 {
						logger.Info("startup", "Loaded %d plugin(s)", len(plugins))
						// Connect plugin manager to tool manager
						toolManager.SetPluginManager(pluginManager)
					}
				}
			}

			// Initialize multi-agent if enabled
			var multiAgent *agent.MultiAgent
			if cfg.Agent.MultiAgent.Enabled {
				multiAgent = agent.NewMultiAgent(llmClient, toolManager, cfg.Agent.MultiAgent.MaxAgents, cfg.Agent.MaxSteps)
				logger.Info("startup", "Multi-agent enabled with %d agents", cfg.Agent.MultiAgent.MaxAgents)
			}

			// Initialize skill manager
			skillManager := skill.NewManager()

			// Add skill directories
			// 1. Built-in skills
			skillManager.AddSkillDir("skills")

			// 2. User skills
			homeDir, _ := os.UserHomeDir()
			userSkillDir := filepath.Join(homeDir, ".ycode", "skills")
			skillManager.AddSkillDir(userSkillDir)

			// 3. Project skills
			skillManager.AddSkillDir(".ycode/skills")

			// Load all skills
			if err := skillManager.LoadAll(); err != nil {
				logger.Warn("startup", "failed to load skills: %v", err)
			} else {
				skills := skillManager.List()
				if len(skills) > 0 {
					logger.Info("startup", "Loaded %d skill(s)", len(skills))
				}
			}

			// Create orchestrator for multi-agent system
			orchestrator := agent.NewOrchestrator(llmClient, toolManager, cfg.Agent.MaxSteps)

			// Load CLAUDE.md as project context if exists
			claudeMdPath := filepath.Join(workDir, "CLAUDE.md")
			if content, err := os.ReadFile(claudeMdPath); err == nil {
				claudeMdContent := string(content)
				if len(claudeMdContent) > 0 {
					orchestrator.SetProjectContext(claudeMdContent)
					logger.Info("startup", "Loaded CLAUDE.md (%d bytes) as project context", len(claudeMdContent))
				}
			}

			// Initialize audit logger
			auditLogger := audit.NewAuditLogger(workDir)

			// Initialize sensitive operation manager
			sensitiveOpManager := audit.NewSensitiveOperationManager(auditLogger)

			// Set YOLO mode if enabled
			if yolo {
				sensitiveOpManager.SetYOLOMode(true)
			}

			// Connect plugin manager to orchestrator (for agent hooks)
			if pluginManager != nil {
				orchestrator.SetPluginManager(pluginManager)
			}

			// Initialize compactor if enabled
			var compactor *agent.Compactor
			if cfg.Agent.AutoCompact {
				compactor = agent.NewCompactor(llmClient)
				// TODO: Integrate compactor with orchestrator
				_ = compactor
			}

			// Initialize command manager with orchestrator
			cmdManager := command.NewCommandManager(orchestrator, cfg, skillManager)

			// Register skill commands dynamically
			for _, s := range skillManager.List() {
				cmdManager.RegisterFromSkill(s)
			}

			// Print startup info
			logger.Info("startup", "ycode v0.1.0-dev")
			logger.Info("startup", "Working directory: %s", workDir)
			logger.Info("startup", "Model: %s", cfg.LLM.Model)
			logger.Info("startup", "Max tokens: %s (%d)", config.FormatTokenSize(cfg.LLM.GetMaxTokens()), cfg.LLM.GetMaxTokens())
			logger.Info("startup", "Base URL: %s", cfg.LLM.BaseURL)
			logger.Info("startup", "Max steps: %d", cfg.Agent.MaxSteps)
			logger.Info("startup", "MCP servers: %d", len(mcpClients))
			logger.Info("startup", "LSP servers: %d", len(lspClients))
			if pluginManager != nil {
				logger.Info("startup", "Plugins: %d", len(pluginManager.List()))
			}
			logger.Info("startup", "Skills: %d", len(skillManager.List()))
			logger.Info("startup", "Commands: %d", len(cmdManager.List()))
			if multiAgent != nil {
				logger.Info("startup", "Multi-agent: enabled")
			}
			if yolo {
				logger.Info("startup", "Mode: YOLO (auto-approve all)")
			}
			fmt.Println()

			// Start TUI or simple mode
			if noUI {
				return runSimpleMode(orchestrator, cmdManager, sensitiveOpManager)
			} else {
				return runTUIMode(orchestrator, cfg, cmdManager, connectedLSPServers, connectedMCPServers, sensitiveOpManager)
			}
		},
	}

	cmd.Flags().BoolVar(&noUI, "no-ui", false, "Run without TUI (simple text mode)")
	cmd.Flags().IntVar(&maxSteps, "max-steps", 10, "Maximum number of agent loop steps")
	cmd.Flags().BoolVar(&yolo, "yolo", false, "Auto-approve all tool executions")
	cmd.Flags().StringVar(&model, "model", "", "Override model (e.g., claude-sonnet-4-20250514)")

	return cmd
}

func runSimpleMode(orchestrator *agent.Orchestrator, cmdManager *command.CommandManager, sensitiveOpManager *audit.SensitiveOperationManager) error {
	_ = sensitiveOpManager // Will be used for confirmation in simple mode
	logger.Info("startup", "Running in simple mode...")
	logger.Info("startup", "Type 'exit' or press Ctrl+C to quit")
	logger.Info("startup", "Type /help to see available commands")
	fmt.Println()

	for {
		fmt.Print("You: ")
		var input string
		_, err := fmt.Scanln(&input)
		if err != nil {
			break
		}

		if input == "exit" || input == "quit" {
			break
		}

		// Check if it's a command
		if strings.HasPrefix(input, "/") {
			handled, result, err := cmdManager.Execute(input)
			if handled {
				if err != nil {
					logger.Info("startup", "Error: %v", err)
				} else if result != "" {
					fmt.Println(result)
				}
				fmt.Println()
				continue
			}
		}

		// Check if it's an agent shortcut command
		if agentType, ok := orchestrator.DetectAgentFromCommand(input); ok {
			orchestrator.SetCurrentAgent(agentType)
			info, _ := orchestrator.GetAgentInfo(agentType)
			logger.Info("startup", "Switched to %s agent", agentType)
			if info != nil {
				logger.Info("startup", "Description: %s", info.Description)
			}
			fmt.Println()
			continue
		}

		// Process with orchestrator
		ctx := context.Background()
		err = orchestrator.Run(ctx, input, "")
		if err != nil {
			logger.Info("startup", "Error: %v", err)
		}
		fmt.Println()
	}

	return nil
}

func runTUIMode(orchestrator *agent.Orchestrator, cfg *config.Config, cmdManager *command.CommandManager, connectedLSPServers []string, connectedMCPServers []string, sensitiveOpManager *audit.SensitiveOperationManager) error {
	// Create modern TUI model with improved layout
	model := ui.NewModernTUIModel(orchestrator, cfg, cmdManager, connectedLSPServers, connectedMCPServers, sensitiveOpManager)

	// Start TUI
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	return err
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			logger.Info("startup", "ycode version 0.1.0-dev")
		},
	}
}
