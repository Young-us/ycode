package tools

import (
	"fmt"
	"sync"

	"github.com/Young-us/ycode/internal/audit"
	"github.com/Young-us/ycode/internal/logger"
)

// ExecutionMode represents the workflow execution mode
type ExecutionMode string

const (
	ModeConfirm ExecutionMode = "confirm" // Each operation requires confirmation
	ModeAuto    ExecutionMode = "auto"    // Automatic execution
	ModePlan    ExecutionMode = "plan"    // Plan first, then execute
)

// PermissionAction represents the action from permission check
type PermissionAction string

const (
	PermissionAllow PermissionAction = "allow"
	PermissionDeny  PermissionAction = "deny"
	PermissionAsk   PermissionAction = "ask"
)

// AgentPermissionChecker is an interface for checking agent permissions
// This avoids circular imports between tools and agent packages
type AgentPermissionChecker interface {
	Check(toolName, filePath string) PermissionResult
}

// PermissionResult represents the result of an agent permission check
type PermissionResult struct {
	Allowed bool
	Action  PermissionAction
	Message string
}

// UnifiedPermissionChecker implements PermissionChecker interface
// It combines AgentPermissions and SensitiveOperationManager checks
type UnifiedPermissionChecker struct {
	mu              sync.RWMutex
	mode            ExecutionMode
	agentPerms      AgentPermissionChecker
	sensitiveMgr    *audit.SensitiveOperationManager
	confirmCallback func(op audit.SensitiveOperation) audit.ConfirmationResult
}

// NewUnifiedPermissionChecker creates a new unified permission checker
func NewUnifiedPermissionChecker(
	mode ExecutionMode,
	agentPerms AgentPermissionChecker,
	sensitiveMgr *audit.SensitiveOperationManager,
) *UnifiedPermissionChecker {
	return &UnifiedPermissionChecker{
		mode:         mode,
		agentPerms:   agentPerms,
		sensitiveMgr: sensitiveMgr,
	}
}

// SetMode updates the execution mode
func (c *UnifiedPermissionChecker) SetMode(mode ExecutionMode) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mode = mode
	logger.Debug("permission", "Execution mode changed to: %s", mode)
}

// SetAgentPermissions updates the agent permissions
func (c *UnifiedPermissionChecker) SetAgentPermissions(perms AgentPermissionChecker) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.agentPerms = perms
}

// SetConfirmCallback sets the confirmation callback
func (c *UnifiedPermissionChecker) SetConfirmCallback(cb func(op audit.SensitiveOperation) audit.ConfirmationResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.confirmCallback = cb
}

// CheckPermission implements PermissionChecker interface
func (c *UnifiedPermissionChecker) CheckPermission(toolName string, args map[string]interface{}) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	filePath := extractFilePath(args)

	// 1. Check Agent permissions (deny takes precedence)
	if c.agentPerms != nil {
		result := c.agentPerms.Check(toolName, filePath)
		logger.Debug("permission", "Agent permission check: tool=%s, path=%s, action=%s, allowed=%v",
			toolName, filePath, result.Action, result.Allowed)

		// Deny is absolute - cannot be overridden
		if result.Action == PermissionDeny {
			return false, fmt.Errorf("denied by agent permission: %s", result.Message)
		}
	}

	// 2. Check sensitive operation level
	var op audit.SensitiveOperation
	var isSensitive bool
	if c.sensitiveMgr != nil {
		op, isSensitive = c.sensitiveMgr.CheckOperation(toolName, args)
	}

	// 3. Critical level always requires confirmation (cannot be overridden)
	if isSensitive && op.Level == audit.SensitivityCritical {
		logger.Info("permission", "Critical operation detected: tool=%s", toolName)
		return c.requestConfirmation(op)
	}

	// 4. Read operations are always allowed
	if isReadOperation(toolName) {
		return true, nil
	}

	// 5. Check agent permission ask (can be overridden by mode)
	if c.agentPerms != nil {
		result := c.agentPerms.Check(toolName, filePath)
		if result.Action == PermissionAsk {
			// Mode determines behavior for ask
			switch c.mode {
			case ModeAuto:
				// Auto mode overrides ask - allow automatically
				return true, nil
			case ModeConfirm:
				// Confirm mode - need to check with user
				if isSensitive {
					return c.requestConfirmation(op)
				}
				return c.requestConfirmation(op)
			case ModePlan:
				// Plan mode should not reach here
				return false, fmt.Errorf("plan mode: unexpected permission check")
			}
		}
	}

	// 6. Handle based on mode
	switch c.mode {
	case ModeAuto:
		// Auto mode: allow everything (Critical already checked above)
		return true, nil

	case ModeConfirm:
		// Confirm mode: sensitive operations need confirmation
		if isSensitive {
			return c.requestConfirmation(op)
		}
		return true, nil

	case ModePlan:
		// Plan mode should not be executing tools directly
		return false, fmt.Errorf("plan mode: unexpected permission check")

	default:
		return true, nil
	}
}

// requestConfirmation requests confirmation from the user
func (c *UnifiedPermissionChecker) requestConfirmation(op audit.SensitiveOperation) (bool, error) {
	// Use SensitiveOperationManager's RequestConfirmation if available
	// It handles YOLO mode, remembered decisions, and callback
	if c.sensitiveMgr != nil {
		result := c.sensitiveMgr.RequestConfirmation(op)
		if result.Confirmed {
			return true, nil
		}
		return false, fmt.Errorf("permission denied by user: %s", result.Reason)
	}

	// Fallback to direct callback
	if c.confirmCallback == nil {
		return false, fmt.Errorf("no confirmation callback set")
	}

	result := c.confirmCallback(op)
	if result.Confirmed {
		return true, nil
	}
	return false, fmt.Errorf("permission denied by user")
}

// extractFilePath extracts file path from tool arguments
func extractFilePath(args map[string]interface{}) string {
	if path, ok := args["path"].(string); ok {
		return path
	}
	if path, ok := args["file_path"].(string); ok {
		return path
	}
	if path, ok := args["target"].(string); ok {
		return path
	}
	return ""
}

// isReadOperation checks if a tool only performs read operations
func isReadOperation(toolName string) bool {
	readTools := map[string]bool{
		"read_file":        true,
		"glob":             true,
		"grep":             true,
		"git_status":       true,
		"git_log":          true,
		"git_diff":         true,
		"git_show":         true,
		"git_branch":       true,
		"web_search":       true,
		"web_fetch":        true,
		"lsp_hover":        true,
		"lsp_definition":   true,
		"lsp_references":   true,
		"lsp_symbols":      true,
		"lsp_diagnostics":  true,
	}
	return readTools[toolName]
}

// GetMode returns the current execution mode
func (c *UnifiedPermissionChecker) GetMode() ExecutionMode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mode
}

// AgentPermissionAdapter adapts an AgentPermissions-like object to AgentPermissionChecker
// This is used to connect the agent package's permissions to the tools package
type AgentPermissionAdapter struct {
	checkFunc func(toolName, filePath string) (bool, PermissionAction, string)
}

// NewAgentPermissionAdapter creates an adapter from a check function
func NewAgentPermissionAdapter(checkFunc func(toolName, filePath string) (bool, PermissionAction, string)) *AgentPermissionAdapter {
	return &AgentPermissionAdapter{checkFunc: checkFunc}
}

// Check implements AgentPermissionChecker
func (a *AgentPermissionAdapter) Check(toolName, filePath string) PermissionResult {
	if a.checkFunc == nil {
		return PermissionResult{
			Allowed: true,
			Action:  PermissionAllow,
			Message: "No permission checker configured",
		}
	}
	allowed, action, message := a.checkFunc(toolName, filePath)
	return PermissionResult{
		Allowed: allowed,
		Action:  action,
		Message: message,
	}
}