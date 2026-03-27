package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditLevel represents the severity level of an audit event
type AuditLevel string

const (
	AuditLevelInfo    AuditLevel = "info"
	AuditLevelWarning AuditLevel = "warning"
	AuditLevelDanger  AuditLevel = "danger"
	AuditLevelBlocked AuditLevel = "blocked"
)

// AuditEvent represents a single audit event
type AuditEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	Level       AuditLevel             `json:"level"`
	EventType   string                 `json:"event_type"`
	ToolName    string                 `json:"tool_name,omitempty"`
	Path        string                 `json:"path,omitempty"`
	Command     string                 `json:"command,omitempty"`
	User        string                 `json:"user,omitempty"`
	Decision    string                 `json:"decision"` // "allowed", "denied", "confirmed", "blocked"
	Reason      string                 `json:"reason,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	AgentType   string                 `json:"agent_type,omitempty"`
}

// AuditLogger handles audit logging
type AuditLogger struct {
	logPath    string
	events     []AuditEvent
	mu         sync.RWMutex
	maxEvents  int
	enabled    bool
	sessionID  string
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(workDir string) *AuditLogger {
	logDir := filepath.Join(workDir, ".ycode", "audit")
	os.MkdirAll(logDir, 0755)

	// Create log file with date
	today := time.Now().Format("2006-01-02")
	logPath := filepath.Join(logDir, fmt.Sprintf("audit-%s.jsonl", today))

	return &AuditLogger{
		logPath:   logPath,
		events:    make([]AuditEvent, 0),
		maxEvents: 10000,
		enabled:   true,
		sessionID: generateSessionID(),
	}
}

// SetEnabled enables or disables audit logging
func (l *AuditLogger) SetEnabled(enabled bool) {
	l.enabled = enabled
}

// Log records an audit event
func (l *AuditLogger) Log(event AuditEvent) {
	if !l.enabled {
		return
	}

	event.Timestamp = time.Now()
	event.SessionID = l.sessionID

	l.mu.Lock()
	l.events = append(l.events, event)

	// Trim if too many events
	if len(l.events) > l.maxEvents {
		l.events = l.events[len(l.events)-l.maxEvents:]
	}
	l.mu.Unlock()

	// Write to file
	l.writeEvent(event)
}

// LogToolExecution logs a tool execution event
func (l *AuditLogger) LogToolExecution(toolName string, args map[string]interface{}, decision, reason string, level AuditLevel) {
	event := AuditEvent{
		Level:     level,
		EventType: "tool_execution",
		ToolName:  toolName,
		Decision:  decision,
		Reason:    reason,
		Details:   args,
	}

	// Extract path if present
	if path, ok := args["path"].(string); ok {
		event.Path = path
	}
	if cmd, ok := args["command"].(string); ok {
		event.Command = cmd
	}

	l.Log(event)
}

// LogFileOperation logs a file operation event
func (l *AuditLogger) LogFileOperation(operation, path string, decision, reason string, level AuditLevel) {
	l.Log(AuditEvent{
		Level:     level,
		EventType: "file_operation",
		ToolName:  operation,
		Path:      path,
		Decision:  decision,
		Reason:    reason,
	})
}

// LogSecurityEvent logs a security-related event
func (l *AuditLogger) LogSecurityEvent(eventType, reason string, details map[string]interface{}) {
	l.Log(AuditEvent{
		Level:     AuditLevelWarning,
		EventType: eventType,
		Decision:  "blocked",
		Reason:    reason,
		Details:   details,
	})
}

// writeEvent writes an event to the log file
func (l *AuditLogger) writeEvent(event AuditEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	// Append to log file
	f, err := os.OpenFile(l.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	f.Write(data)
	f.Write([]byte("\n"))
}

// GetRecentEvents returns the most recent events
func (l *AuditLogger) GetRecentEvents(count int) []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if count > len(l.events) {
		count = len(l.events)
	}

	result := make([]AuditEvent, count)
	start := len(l.events) - count
	copy(result, l.events[start:])
	return result
}

// GetEventsByTool returns events for a specific tool
func (l *AuditLogger) GetEventsByTool(toolName string) []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []AuditEvent
	for _, event := range l.events {
		if event.ToolName == toolName {
			result = append(result, event)
		}
	}
	return result
}

// GetDangerousEvents returns all dangerous or blocked events
func (l *AuditLogger) GetDangerousEvents() []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []AuditEvent
	for _, event := range l.events {
		if event.Level == AuditLevelDanger || event.Level == AuditLevelBlocked {
			result = append(result, event)
		}
	}
	return result
}

// Export exports the audit log to a file
func (l *AuditLogger) Export(path string) error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	data, err := json.MarshalIndent(l.events, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GenerateReport generates a summary report
func (l *AuditLogger) GenerateReport() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var allowed, denied, blocked int
	toolCounts := make(map[string]int)

	for _, event := range l.events {
		switch event.Decision {
		case "allowed", "confirmed":
			allowed++
		case "denied":
			denied++
		case "blocked":
			blocked++
		}

		if event.ToolName != "" {
			toolCounts[event.ToolName]++
		}
	}

	report := fmt.Sprintf(`
=== Audit Report ===
Session ID: %s
Time Range: %s to %s

Summary:
  Total Events: %d
  Allowed: %d
  Denied: %d
  Blocked: %d

Tool Usage:
`, l.sessionID,
		l.events[0].Timestamp.Format("15:04:05"),
		l.events[len(l.events)-1].Timestamp.Format("15:04:05"),
		len(l.events), allowed, denied, blocked)

	for tool, count := range toolCounts {
		report += fmt.Sprintf("  %s: %d\n", tool, count)
	}

	return report
}

func generateSessionID() string {
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}