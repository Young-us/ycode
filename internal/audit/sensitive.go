package audit

import (
	"fmt"
	"strings"
	"sync"
)

// SensitivityLevel represents how sensitive an operation is
type SensitivityLevel int

const (
	SensitivityLow      SensitivityLevel = iota // Normal operations
	SensitivityMedium                            // File writes, some bash commands
	SensitivityHigh                              // File deletion, sudo commands
	SensitivityCritical                          // rm -rf, dangerous patterns
)

// SensitiveOperation represents an operation that requires confirmation
type SensitiveOperation struct {
	ToolName       string
	Operation      string
	Target         string // File path or command
	Level          SensitivityLevel
	Reason         string
	SuggestedRisk  string // Description of potential risks
	AutoConfirm    bool   // If true, can be auto-confirmed in YOLO mode
}

// ConfirmationResult represents the result of a confirmation request
type ConfirmationResult struct {
	Confirmed bool
	Remember  bool // Remember this decision for future
	Reason    string
}

// ConfirmationCallback is called to request user confirmation
type ConfirmationCallback func(op SensitiveOperation) ConfirmationResult

// SensitiveOperationManager manages detection and confirmation of sensitive operations
type SensitiveOperationManager struct {
	auditLogger         *AuditLogger
	confirmationCb      ConfirmationCallback
	yoloMode            bool // Auto-confirm everything
	rememberedDecisions map[string]bool
	mu                  sync.RWMutex

	// Dangerous patterns
	dangerousCommands  []string
	sensitivePatterns  []string
	criticalPatterns   []string
}

// NewSensitiveOperationManager creates a new manager
func NewSensitiveOperationManager(auditLogger *AuditLogger) *SensitiveOperationManager {
	m := &SensitiveOperationManager{
		auditLogger:         auditLogger,
		rememberedDecisions: make(map[string]bool),
		yoloMode:           false,
	}

	m.initPatterns()
	return m
}

// SetYOLOMode enables or disables YOLO mode (auto-confirm everything)
func (m *SensitiveOperationManager) SetYOLOMode(enabled bool) {
	m.yoloMode = enabled
}

// SetConfirmationCallback sets the callback for requesting confirmation
func (m *SensitiveOperationManager) SetConfirmationCallback(cb ConfirmationCallback) {
	m.confirmationCb = cb
}

// initPatterns initializes dangerous and sensitive patterns
func (m *SensitiveOperationManager) initPatterns() {
	// Critical patterns - always require confirmation
	m.criticalPatterns = []string{
		"rm -rf /",
		"rm -rf /*",
		"rm -rf ~",
		"rm -rf ~/*",
		"sudo rm",
		"mkfs",
		"dd if=",
		":(){:|:&};:",  // Fork bomb
		"chmod -R 777 /",
		"chown -R",
		"wget | sh",
		"curl | sh",
		"> /dev/sda",
	}

	// Dangerous commands - require confirmation
	m.dangerousCommands = []string{
		"rm -rf",
		"rm -r",
		"sudo",
		"chmod 777",
		"chown",
		"kill -9",
		"pkill",
		"killall",
		"shutdown",
		"reboot",
		"halt",
		"poweroff",
		"fdisk",
		"parted",
		"apt-get purge",
		"apt-get remove",
		"yum remove",
		"dnf remove",
		"pacman -R",
		"brew uninstall",
		"npm uninstall",
		"pip uninstall",
		"git push --force",
		"git reset --hard",
		"git clean -fdx",
		"drop table",
		"drop database",
		"truncate",
		"delete from",
	}

	// Sensitive file patterns
	m.sensitivePatterns = []string{
		".env",
		".secret",
		".key",
		".pem",
		".p12",
		".pfx",
		"id_rsa",
		"id_ed25519",
		".ssh/",
		".gnupg/",
		".pgp/",
		"credentials",
		"secrets",
		"password",
		"token",
		"api_key",
		"apikey",
		".aws/credentials",
		".docker/config.json",
		"~/.netrc",
	}
}

// CheckOperation checks if an operation is sensitive and needs confirmation
func (m *SensitiveOperationManager) CheckOperation(toolName string, args map[string]interface{}) (SensitiveOperation, bool) {
	op := SensitiveOperation{
		ToolName: toolName,
	}

	// Check based on tool type
	switch toolName {
	case "bash":
		return m.checkBashCommand(args)
	case "write_file", "write":
		return m.checkFileWrite(args)
	case "edit_file", "edit":
		return m.checkFileEdit(args)
	case "git":
		return m.checkGitCommand(args)
	}

	return op, false
}

// checkBashCommand checks if a bash command is sensitive
func (m *SensitiveOperationManager) checkBashCommand(args map[string]interface{}) (SensitiveOperation, bool) {
	cmd, ok := args["command"].(string)
	if !ok {
		return SensitiveOperation{}, false
	}

	cmdLower := strings.ToLower(cmd)

	// Check critical patterns
	for _, pattern := range m.criticalPatterns {
		if strings.Contains(cmdLower, strings.ToLower(pattern)) {
			return SensitiveOperation{
				ToolName:      "bash",
				Operation:     "execute",
				Target:        cmd,
				Level:         SensitivityCritical,
				Reason:        fmt.Sprintf("Critical pattern detected: %s", pattern),
				SuggestedRisk: "This command can cause irreversible system damage",
				AutoConfirm:   false,
			}, true
		}
	}

	// Check dangerous commands
	for _, dangerous := range m.dangerousCommands {
		if strings.Contains(cmdLower, strings.ToLower(dangerous)) {
			return SensitiveOperation{
				ToolName:      "bash",
				Operation:     "execute",
				Target:        cmd,
				Level:         SensitivityHigh,
				Reason:        fmt.Sprintf("Dangerous command detected: %s", dangerous),
				SuggestedRisk: "This command can modify or delete important data",
				AutoConfirm:   false,
			}, true
		}
	}

	// Check for file deletion patterns
	if strings.Contains(cmdLower, "rm ") {
		return SensitiveOperation{
			ToolName:      "bash",
			Operation:     "delete",
			Target:        cmd,
			Level:         SensitivityHigh,
			Reason:        "File deletion command",
			SuggestedRisk: "This command will delete files",
			AutoConfirm:   false,
		}, true
	}

	return SensitiveOperation{}, false
}

// checkFileWrite checks if a file write is sensitive
func (m *SensitiveOperationManager) checkFileWrite(args map[string]interface{}) (SensitiveOperation, bool) {
	path, ok := args["path"].(string)
	if !ok {
		return SensitiveOperation{}, false
	}

	pathLower := strings.ToLower(path)

	// Check sensitive file patterns
	for _, pattern := range m.sensitivePatterns {
		if strings.Contains(pathLower, strings.ToLower(pattern)) {
			return SensitiveOperation{
				ToolName:      "write",
				Operation:     "write",
				Target:        path,
				Level:         SensitivityHigh,
				Reason:        fmt.Sprintf("Sensitive file pattern: %s", pattern),
				SuggestedRisk: "This file may contain sensitive credentials or secrets",
				AutoConfirm:   false,
			}, true
		}
	}

	// All writes are medium sensitivity
	return SensitiveOperation{
		ToolName:      "write",
		Operation:     "write",
		Target:        path,
		Level:         SensitivityMedium,
		Reason:        "File modification",
		SuggestedRisk: "This will modify the file content",
		AutoConfirm:   true,
	}, true
}

// checkFileEdit checks if a file edit is sensitive
func (m *SensitiveOperationManager) checkFileEdit(args map[string]interface{}) (SensitiveOperation, bool) {
	path, ok := args["path"].(string)
	if !ok {
		return SensitiveOperation{}, false
	}

	pathLower := strings.ToLower(path)

	// Check sensitive file patterns
	for _, pattern := range m.sensitivePatterns {
		if strings.Contains(pathLower, strings.ToLower(pattern)) {
			return SensitiveOperation{
				ToolName:      "edit",
				Operation:     "edit",
				Target:        path,
				Level:         SensitivityHigh,
				Reason:        fmt.Sprintf("Sensitive file pattern: %s", pattern),
				SuggestedRisk: "This file may contain sensitive credentials or secrets",
				AutoConfirm:   false,
			}, true
		}
	}

	return SensitiveOperation{
		ToolName:      "edit",
		Operation:     "edit",
		Target:        path,
		Level:         SensitivityMedium,
		Reason:        "File edit",
		SuggestedRisk: "This will modify the file content",
		AutoConfirm:   true,
	}, true
}

// checkGitCommand checks if a git command is sensitive
func (m *SensitiveOperationManager) checkGitCommand(args map[string]interface{}) (SensitiveOperation, bool) {
	// Check git command arguments
	for _, val := range args {
		if strVal, ok := val.(string); ok {
			strLower := strings.ToLower(strVal)

			// Check for force push
			if strings.Contains(strLower, "push --force") || strings.Contains(strLower, "push -f") {
				return SensitiveOperation{
					ToolName:      "git",
					Operation:     "push",
					Target:        strVal,
					Level:         SensitivityHigh,
					Reason:        "Force push detected",
					SuggestedRisk: "Force push can overwrite remote history and lose commits",
					AutoConfirm:   false,
				}, true
			}

			// Check for reset --hard
			if strings.Contains(strLower, "reset --hard") {
				return SensitiveOperation{
					ToolName:      "git",
					Operation:     "reset",
					Target:        strVal,
					Level:         SensitivityHigh,
					Reason:        "Hard reset detected",
					SuggestedRisk: "Hard reset will lose uncommitted changes",
					AutoConfirm:   false,
				}, true
			}
		}
	}

	return SensitiveOperation{}, false
}

// RequestConfirmation requests confirmation for a sensitive operation
func (m *SensitiveOperationManager) RequestConfirmation(op SensitiveOperation) ConfirmationResult {
	// Check YOLO mode
	if m.yoloMode && op.AutoConfirm {
		m.auditLogger.LogToolExecution(op.ToolName, map[string]interface{}{
			"target":  op.Target,
			"reason":  op.Reason,
		}, "confirmed", "YOLO mode auto-confirm", SensitivityLevelToAuditLevel(op.Level))
		return ConfirmationResult{Confirmed: true, Reason: "YOLO mode"}
	}

	// Check remembered decisions
	m.mu.RLock()
	key := fmt.Sprintf("%s:%s", op.ToolName, op.Target)
	if remembered, exists := m.rememberedDecisions[key]; exists {
		m.mu.RUnlock()
		m.auditLogger.LogToolExecution(op.ToolName, map[string]interface{}{
			"target": op.Target,
		}, "confirmed", "Remembered decision", SensitivityLevelToAuditLevel(op.Level))
		return ConfirmationResult{Confirmed: remembered, Reason: "Remembered decision"}
	}
	m.mu.RUnlock()

	// Request confirmation via callback
	if m.confirmationCb != nil {
		result := m.confirmationCb(op)

		// Log the decision
		decision := "denied"
		if result.Confirmed {
			decision = "confirmed"
		}
		m.auditLogger.LogToolExecution(op.ToolName, map[string]interface{}{
			"target": op.Target,
			"reason": op.Reason,
		}, decision, result.Reason, SensitivityLevelToAuditLevel(op.Level))

		// Remember decision if requested
		if result.Remember {
			m.mu.Lock()
			m.rememberedDecisions[key] = result.Confirmed
			m.mu.Unlock()
		}

		return result
	}

	// No callback - deny by default for safety
	return ConfirmationResult{Confirmed: false, Reason: "No confirmation callback configured"}
}

// SensitivityLevelToString converts sensitivity level to string
func SensitivityLevelToString(level SensitivityLevel) string {
	switch level {
	case SensitivityLow:
		return "low"
	case SensitivityMedium:
		return "medium"
	case SensitivityHigh:
		return "high"
	case SensitivityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// SensitivityLevelToAuditLevel converts sensitivity level to audit level
func SensitivityLevelToAuditLevel(level SensitivityLevel) AuditLevel {
	switch level {
	case SensitivityLow:
		return AuditLevelInfo
	case SensitivityMedium:
		return AuditLevelWarning
	case SensitivityHigh:
		return AuditLevelDanger
	case SensitivityCritical:
		return AuditLevelBlocked
	default:
		return AuditLevelInfo
	}
}