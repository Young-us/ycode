package command

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/Young-us/ycode/internal/agent"
	"github.com/Young-us/ycode/internal/config"
	"github.com/Young-us/ycode/internal/logger"
	"github.com/Young-us/ycode/internal/skill"
)

// CommandManager manages slash commands from multiple sources
type CommandManager struct {
	mu            sync.RWMutex
	commands      map[string]*Command
	orchestrator  *agent.Orchestrator
	config        *config.Config
	skillManager  *skill.Manager
}

// NewCommandManager creates a new command manager with dependencies
func NewCommandManager(
	orch *agent.Orchestrator,
	cfg *config.Config,
	skillMgr *skill.Manager,
) *CommandManager {
	m := &CommandManager{
		commands:     make(map[string]*Command),
		orchestrator: orch,
		config:       cfg,
		skillManager: skillMgr,
	}

	// Register builtin commands
	m.registerBuiltin()

	return m
}

// registerBuiltin registers all builtin commands
func (m *CommandManager) registerBuiltin() {
	builtins := BuiltinCommands(m.orchestrator, m.config, m.skillManager)
	for _, cmd := range builtins {
		cmd.Source = SourceCommand
		m.Register(cmd)
	}

	// Register agent shortcut commands
	agentShortcuts := []struct {
		name       string
		agentType  agent.AgentType
		desc       string
	}{
		{"explorer", agent.AgentExplorer, "切换到探索 agent"},
		{"planner", agent.AgentPlanner, "切换到规划 agent"},
		{"architect", agent.AgentArchitect, "切换到架构 agent"},
		{"coder", agent.AgentCoder, "切换到编码 agent"},
		{"debugger", agent.AgentDebugger, "切换到调试 agent"},
		{"tester", agent.AgentTester, "切换到测试 agent"},
		{"reviewer", agent.AgentReviewer, "切换到审查 agent"},
		{"writer", agent.AgentWriter, "切换到文档 agent"},
	}

	for _, s := range agentShortcuts {
		agentType := s.agentType
		m.Register(&Command{
			Name:        s.name,
			Description: s.desc,
			Usage:       "/" + s.name,
			Source:      SourceCommand,
			Handler: func(args []string) (string, error) {
				if err := m.orchestrator.SetCurrentAgent(agentType); err != nil {
					return "", err
				}
				info, _ := m.orchestrator.GetAgentInfo(agentType)
				return fmt.Sprintf("已切换到 %s agent\n描述: %s", agentType, info.Description), nil
			},
		})
	}
}

// Register registers a command
func (m *CommandManager) Register(cmd *Command) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cmd.Source == "" {
		cmd.Source = SourceCommand
	}

	// Extract hints from template if not provided
	if len(cmd.Hints) == 0 && cmd.Template != "" {
		cmd.Hints = ExtractHints(cmd.Template)
	}

	m.commands[cmd.Name] = cmd
	logger.Debug("command", "Registered command: /%s (source: %s)", cmd.Name, cmd.Source)
}

// RegisterFromMCP registers commands from MCP servers
func (m *CommandManager) RegisterFromMCP(serverName string, tools []MCPToolDefinition) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, tool := range tools {
		cmd := &Command{
			Name:        fmt.Sprintf("mcp.%s.%s", serverName, tool.Name),
			Description: tool.Description,
			Usage:       fmt.Sprintf("/mcp.%s.%s [args]", serverName, tool.Name),
			Template:    tool.Template,
			Source:      SourceMCP,
			Subtask:     true, // MCP commands run as subtasks
			Hints:       ExtractHints(tool.Template),
		}
		m.commands[cmd.Name] = cmd
		logger.Debug("command", "Registered MCP command: /%s (server: %s)", cmd.Name, serverName)
	}
}

// RegisterFromSkill registers commands from a skill
func (m *CommandManager) RegisterFromSkill(s *skill.Skill) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cmdName := range s.Commands {
		// Remove leading slash if present
		name := strings.TrimPrefix(cmdName, "/")

		cmd := &Command{
			Name:        name,
			Description: s.Description,
			Usage:       fmt.Sprintf("/%s [args]", name),
			Template:    s.Instructions,
			Source:      SourceSkill,
			Subtask:     true, // Skill commands run as subtasks
			Hints:       ExtractHints(s.Instructions),
		}
		m.commands[cmd.Name] = cmd
		logger.Debug("command", "Registered skill command: /%s (skill: %s)", cmd.Name, s.Name)
	}
}

// ReloadFromSkills reloads all commands from skills dynamically
// This allows commands to be updated when skills change without restarting
func (m *CommandManager) ReloadFromSkills() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove existing skill commands
	for name, cmd := range m.commands {
		if cmd.Source == SourceSkill {
			delete(m.commands, name)
			logger.Debug("command", "Unregistered skill command: /%s", name)
		}
	}

	// Reload from skill manager
	if m.skillManager == nil {
		return 0
	}

	// Force reload skills
	if err := m.skillManager.Reload(); err != nil {
		logger.Warn("command", "Failed to reload skills: %v", err)
	}

	// Re-register all skill commands
	count := 0
	for _, s := range m.skillManager.List() {
		for _, cmdName := range s.Commands {
			name := strings.TrimPrefix(cmdName, "/")

			cmd := &Command{
				Name:        name,
				Description: s.Description,
				Usage:       fmt.Sprintf("/%s [args]", name),
				Template:    s.Instructions,
				Source:      SourceSkill,
				Subtask:     true,
				Hints:       ExtractHints(s.Instructions),
			}
			m.commands[cmd.Name] = cmd
			logger.Debug("command", "Registered skill command: /%s (skill: %s)", cmd.Name, s.Name)
			count++
		}
	}

	return count
}

// RegisterFromConfig registers commands from configuration
func (m *CommandManager) RegisterFromConfig(commands []ConfigCommand) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cfg := range commands {
		cmd := &Command{
			Name:        cfg.Name,
			Description: cfg.Description,
			Usage:       cfg.Usage,
			Template:    cfg.Template,
			Agent:       cfg.Agent,
			Model:       cfg.Model,
			Source:      SourceCommand,
			Subtask:     cfg.Subtask,
			Hints:       ExtractHints(cfg.Template),
		}
		m.commands[cmd.Name] = cmd
		logger.Debug("command", "Registered config command: /%s", cmd.Name)
	}
}

// Get returns a command by name
func (m *CommandManager) Get(name string) (*Command, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cmd, exists := m.commands[name]
	return cmd, exists
}

// List returns all commands sorted by name
func (m *CommandManager) List() []*Command {
	m.mu.RLock()
	defer m.mu.RUnlock()

	commands := make([]*Command, 0, len(m.commands))
	for _, cmd := range m.commands {
		commands = append(commands, cmd)
	}

	// Sort by name for consistent output
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})

	return commands
}

// ListBySource returns commands filtered by source
func (m *CommandManager) ListBySource(source Source) []*Command {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var commands []*Command
	for _, cmd := range m.commands {
		if cmd.Source == source {
			commands = append(commands, cmd)
		}
	}

	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})

	return commands
}

// FindMatching finds commands matching a prefix (for command palette)
func (m *CommandManager) FindMatching(prefix string) []*Command {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matched []*Command
	prefix = strings.ToLower(prefix)

	for _, cmd := range m.commands {
		if strings.HasPrefix(strings.ToLower(cmd.Name), prefix) {
			matched = append(matched, cmd)
		}
	}

	// Sort by name for consistent ordering
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Name < matched[j].Name
	})

	return matched
}

// Execute executes a command from user input
func (m *CommandManager) Execute(input string) (bool, string, error) {
	// Check if input starts with /
	if !strings.HasPrefix(input, "/") {
		return false, "", nil
	}

	// Parse command and args
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false, "", nil
	}

	cmdName := strings.TrimPrefix(parts[0], "/")
	args := strings.Join(parts[1:], " ")

	logger.Debug("command", "Execute: looking for command '%s', total commands: %d", cmdName, len(m.commands))

	// Find command
	cmd, exists := m.Get(cmdName)
	if !exists {
		// Debug: list all available commands
		logger.Debug("command", "Execute: command '%s' not found. Available: %v", cmdName, m.listCommandNames())
		return true, "", fmt.Errorf("unknown command: /%s", cmdName)
	}

	// If command has a handler, use it
	if cmd.Handler != nil {
		output, err := cmd.Handler(parts[1:])
		return true, output, err
	}

	// Process template with arguments
	result := ProcessTemplate(cmd.Template, args, parts[1:])
	return true, result, nil
}

// Unregister removes a command
func (m *CommandManager) Unregister(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.commands, name)
	logger.Debug("command", "Unregistered command: /%s", name)
}

// Clear removes all commands
func (m *CommandManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.commands = make(map[string]*Command)
	logger.Debug("command", "Cleared all commands")
}

// Count returns the number of registered commands
func (m *CommandManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.commands)
}

// HasCommand checks if a command exists
func (m *CommandManager) HasCommand(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.commands[name]
	return exists
}

// listCommandNames returns all command names for debugging
func (m *CommandManager) listCommandNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.commands))
	for name := range m.commands {
		names = append(names, name)
	}
	return names
}

// MCPToolDefinition represents a tool definition from MCP server
type MCPToolDefinition struct {
	Name        string
	Description string
	Template    string
	InputSchema map[string]interface{}
}

// ConfigCommand represents a command from configuration
type ConfigCommand struct {
	Name        string `mapstructure:"name"`
	Description string `mapstructure:"description"`
	Usage       string `mapstructure:"usage"`
	Template    string `mapstructure:"template"`
	Agent       string `mapstructure:"agent"`
	Model       string `mapstructure:"model"`
	Subtask     bool   `mapstructure:"subtask"`
}

// ProcessTemplate processes a template string with arguments
// Supports: $ARGUMENTS, $1, $2, etc.
func ProcessTemplate(template string, argsStr string, args []string) string {
	result := template

	// Replace $ARGUMENTS with all arguments
	result = strings.ReplaceAll(result, "$ARGUMENTS", argsStr)

	// Replace numbered arguments ($1, $2, etc.)
	for i, arg := range args {
		placeholder := fmt.Sprintf("$%d", i+1)
		result = strings.ReplaceAll(result, placeholder, arg)
	}

	return result
}

// ExtractHints extracts template hints (placeholders) from a template string
// Returns list of placeholders like ["$1", "$2", "$ARGUMENTS"]
func ExtractHints(template string) []string {
	if template == "" {
		return nil
	}

	hints := make([]string, 0)
	seen := make(map[string]bool)

	// Find $ARGUMENTS
	if strings.Contains(template, "$ARGUMENTS") && !seen["$ARGUMENTS"] {
		hints = append(hints, "$ARGUMENTS")
		seen["$ARGUMENTS"] = true
	}

	// Find numbered placeholders ($1, $2, etc.)
	re := regexp.MustCompile(`\$\d+`)
	matches := re.FindAllString(template, -1)
	for _, match := range matches {
		if !seen[match] {
			hints = append(hints, match)
			seen[match] = true
		}
	}

	return hints
}

// FormatCommands formats commands for display
func FormatCommands(commands []*Command, verbose bool) string {
	if len(commands) == 0 {
		return "No commands available."
	}

	if verbose {
		var result strings.Builder
		for _, cmd := range commands {
			result.WriteString(fmt.Sprintf("  /%s\n", cmd.Name))
			result.WriteString(fmt.Sprintf("    Description: %s\n", cmd.Description))
			result.WriteString(fmt.Sprintf("    Source: %s\n", cmd.Source))
			if cmd.Agent != "" {
				result.WriteString(fmt.Sprintf("    Agent: %s\n", cmd.Agent))
			}
			if cmd.Model != "" {
				result.WriteString(fmt.Sprintf("    Model: %s\n", cmd.Model))
			}
			if cmd.Subtask {
				result.WriteString("    Subtask: true\n")
			}
			if len(cmd.Hints) > 0 {
				result.WriteString(fmt.Sprintf("    Hints: %s\n", strings.Join(cmd.Hints, ", ")))
			}
			result.WriteString("\n")
		}
		return result.String()
	}

	var result strings.Builder
	result.WriteString("## Available Commands\n\n")

	// Group by source
	bySource := make(map[Source][]*Command)
	for _, cmd := range commands {
		bySource[cmd.Source] = append(bySource[cmd.Source], cmd)
	}

	// Display builtin commands first
	if cmds, ok := bySource[SourceCommand]; ok {
		result.WriteString("### Built-in Commands\n\n")
		for _, cmd := range cmds {
			result.WriteString(fmt.Sprintf("- **/%s**: %s\n", cmd.Name, cmd.Description))
		}
		result.WriteString("\n")
	}

	// Display skill commands
	if cmds, ok := bySource[SourceSkill]; ok {
		result.WriteString("### Skill Commands\n\n")
		for _, cmd := range cmds {
			result.WriteString(fmt.Sprintf("- **/%s**: %s\n", cmd.Name, cmd.Description))
		}
		result.WriteString("\n")
	}

	// Display MCP commands
	if cmds, ok := bySource[SourceMCP]; ok {
		result.WriteString("### MCP Commands\n\n")
		for _, cmd := range cmds {
			result.WriteString(fmt.Sprintf("- **/%s**: %s\n", cmd.Name, cmd.Description))
		}
		result.WriteString("\n")
	}

	return result.String()
}
