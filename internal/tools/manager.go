package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/Young-us/ycode/internal/logger"
)

// PluginHookTrigger is an interface for triggering plugin hooks
// This interface avoids circular imports between tools and plugin packages
type PluginHookTrigger interface {
	Trigger(ctx context.Context, hookName string, args map[string]interface{}) (map[string]interface{}, error)
	TriggerAsync(ctx context.Context, hookName string, args map[string]interface{})
	Enabled() bool
}

// Manager manages all available tools
type Manager struct {
	tools      map[string]Tool
	mu         sync.RWMutex
	permission PermissionChecker
	plugins    PluginHookTrigger
}

// PermissionChecker is called before executing tools
type PermissionChecker interface {
	CheckPermission(toolName string, args map[string]interface{}) (bool, error)
}

// NewManager creates a new tool manager
func NewManager() *Manager {
	return &Manager{
		tools: make(map[string]Tool),
	}
}

// SetPermissionChecker sets the permission checker
func (m *Manager) SetPermissionChecker(checker PermissionChecker) {
	m.permission = checker
}

// SetPluginManager sets the plugin manager for hook triggering
func (m *Manager) SetPluginManager(plugins PluginHookTrigger) {
	m.plugins = plugins
}

// Register adds a tool to the manager
func (m *Manager) Register(tool Tool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools[tool.Name()] = tool
}

// Get retrieves a tool by name
func (m *Manager) Get(name string) (Tool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tool, ok := m.tools[name]
	return tool, ok
}

// List returns all registered tools
func (m *Manager) List() []Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tools := make([]Tool, 0, len(m.tools))
	for _, tool := range m.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Definitions returns tool definitions for all registered tools
func (m *Manager) Definitions() []ToolDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	definitions := make([]ToolDefinition, 0, len(m.tools))
	for _, tool := range m.tools {
		definitions = append(definitions, ToDefinition(tool))
	}
	return definitions
}

// DefinitionsByCategory returns tool definitions for tools in the specified categories
func (m *Manager) DefinitionsByCategory(categories ...ToolCategory) []ToolDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	categorySet := make(map[ToolCategory]bool)
	for _, cat := range categories {
		categorySet[cat] = true
	}

	definitions := make([]ToolDefinition, 0)
	for _, tool := range m.tools {
		if categorySet[tool.Category()] {
			definitions = append(definitions, ToDefinition(tool))
		}
	}
	return definitions
}

// ToolsByCategory returns tools in the specified categories
func (m *Manager) ToolsByCategory(categories ...ToolCategory) []Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	categorySet := make(map[ToolCategory]bool)
	for _, cat := range categories {
		categorySet[cat] = true
	}

	tools := make([]Tool, 0)
	for _, tool := range m.tools {
		if categorySet[tool.Category()] {
			tools = append(tools, tool)
		}
	}
	return tools
}

// Categories returns all unique categories currently registered
func (m *Manager) Categories() []ToolCategory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	categorySet := make(map[ToolCategory]bool)
	for _, tool := range m.tools {
		categorySet[tool.Category()] = true
	}

	categories := make([]ToolCategory, 0, len(categorySet))
	for cat := range categorySet {
		categories = append(categories, cat)
	}
	return categories
}

// Execute runs a tool with the given name and arguments
func (m *Manager) Execute(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
	logger.Debug("tool", "Executing tool: %s with %d args", name, len(args))

	// Trigger on_tool_execute hook before execution
	if m.plugins != nil && m.plugins.Enabled() {
		hookArgs := map[string]interface{}{
			"tool_name": name,
			"args":      args,
		}
		resultArgs, err := m.plugins.Trigger(ctx, "on_tool_execute", hookArgs)
		// Plugin errors should NOT affect main program - just log and continue
		// Don't return error to avoid retry loops
		if err == nil && resultArgs != nil {
			// Check if plugin wants to skip execution
			if skip, ok := resultArgs["skip"].(bool); ok && skip {
				skipReason := "Skipped by plugin"
				if reason, ok := resultArgs["reason"].(string); ok {
					skipReason = reason
				}
				logger.Info("tool", "Tool %s skipped by plugin: %s", name, skipReason)
				return &ToolResult{Content: skipReason, IsError: false}, nil
			}

			// Update args if modified by plugin
			if modifiedArgs, ok := resultArgs["args"].(map[string]interface{}); ok {
				args = modifiedArgs
			}
		}
	}

	// Get tool
	tool, ok := m.Get(name)
	if !ok {
		logger.Error("tool", "Unknown tool: %s", name)
		return &ToolResult{
			Content: fmt.Sprintf("Error: unknown tool '%s'", name),
			IsError: true,
		}, nil
	}

	// Check permission if checker is set
	if m.permission != nil {
		allowed, err := m.permission.CheckPermission(name, args)
		if err != nil {
			logger.Error("tool", "Permission check failed for %s: %v", name, err)
			return &ToolResult{
				Content: fmt.Sprintf("Error checking permission: %v", err),
				IsError: true,
			}, nil
		}
		if !allowed {
			logger.Warn("tool", "Permission denied for tool: %s", name)
			return &ToolResult{
				Content: "Permission denied by user",
				IsError: true,
			}, nil
		}
	}

	// Execute tool
	result, err := tool.Execute(ctx, args)
	if err != nil {
		logger.Error("tool", "Tool %s execution failed: %v", name, err)
	} else {
		logger.Info("tool", "Tool %s completed successfully (result_len=%d)", name, len(result.Content))
	}

	// Trigger on_tool_complete hook after execution (async for performance)
	if m.plugins != nil && m.plugins.Enabled() {
		completeArgs := map[string]interface{}{
			"tool_name": name,
			"result":    result,
			"error":     err,
		}
		// Use async trigger - we don't need to wait for the result
		m.plugins.TriggerAsync(ctx, "on_tool_complete", completeArgs)
	}

	return result, err
}
