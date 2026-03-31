package agent

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PermissionAction represents the action to take when a permission rule matches
type PermissionAction string

const (
	// PermissionAllow automatically allows the operation
	PermissionAllow PermissionAction = "allow"
	// PermissionDeny automatically denies the operation
	PermissionDeny PermissionAction = "deny"
	// PermissionAsk prompts the user for confirmation
	PermissionAsk PermissionAction = "ask"
)

// PermissionRule defines a single permission rule with tool, action, and pattern matching
type PermissionRule struct {
	// Tool is the tool name to match (e.g., "read", "bash", "write")
	// Use "*" to match all tools
	Tool string

	// Action is the permission action to take when this rule matches
	Action PermissionAction

	// Pattern is a glob pattern for file path matching
	// Examples: "*.env", "*.secret", "config/*"
	// Use "*" to match all paths
	Pattern string
}

// AgentPermissions defines the permission configuration for an agent
type AgentPermissions struct {
	// Mode is the permission mode: "strict", "normal", or "permissive"
	// - strict: All operations require explicit permission
	// - normal: Safe operations are allowed, dangerous ones require confirmation
	// - permissive: Most operations are allowed automatically
	Mode string

	// Rules is the ordered list of permission rules
	// Rules are evaluated in order, first match wins
	Rules []PermissionRule
}

// PermissionResult represents the result of a permission check
type PermissionResult struct {
	// Allowed indicates whether the operation is allowed
	Allowed bool

	// Action is the permission action that was applied
	Action PermissionAction

	// Rule is the rule that matched (nil if default action was used)
	Rule *PermissionRule

	// Message provides additional context about the decision
	Message string
}

// NewPermissionRule creates a new permission rule
func NewPermissionRule(tool, action, pattern string) PermissionRule {
	return PermissionRule{
		Tool:    tool,
		Action:  PermissionAction(action),
		Pattern: pattern,
	}
}

// Match checks if this rule matches the given tool name and file path
func (r PermissionRule) Match(toolName, filePath string) bool {
	// Check tool match
	if r.Tool != "*" && r.Tool != toolName {
		return false
	}

	// If no pattern specified, match all paths
	if r.Pattern == "" || r.Pattern == "*" {
		return true
	}

	// Normalize file path for matching
	normalizedPath := filepath.ToSlash(filePath)

	// Try glob pattern matching
	matched, err := filepath.Match(r.Pattern, normalizedPath)
	if err != nil {
		// Invalid pattern, no match
		return false
	}

	if matched {
		return true
	}

	// Also try matching against just the filename
	baseName := filepath.Base(normalizedPath)
	matched, err = filepath.Match(r.Pattern, baseName)
	if err != nil {
		return false
	}

	return matched
}

// String returns a string representation of the rule
func (r PermissionRule) String() string {
	if r.Pattern == "" || r.Pattern == "*" {
		return fmt.Sprintf("%s: %s", r.Tool, r.Action)
	}
	return fmt.Sprintf("%s [%s]: %s", r.Tool, r.Pattern, r.Action)
}

// NewAgentPermissions creates a new AgentPermissions with the given mode
func NewAgentPermissions(mode string) *AgentPermissions {
	return &AgentPermissions{
		Mode:  mode,
		Rules: []PermissionRule{},
	}
}

// AddRule adds a permission rule to the agent permissions
func (p *AgentPermissions) AddRule(tool, action, pattern string) {
	p.Rules = append(p.Rules, NewPermissionRule(tool, action, pattern))
}

// AddRules adds multiple permission rules at once
func (p *AgentPermissions) AddRules(rules ...PermissionRule) {
	p.Rules = append(p.Rules, rules...)
}

// Check evaluates whether a tool operation is allowed for the given path
// Returns a PermissionResult with the decision and context
func (p *AgentPermissions) Check(toolName, filePath string) PermissionResult {
	// Normalize inputs
	toolName = strings.ToLower(toolName)
	filePath = filepath.Clean(filePath)

	// Evaluate rules in order (first match wins)
	for i := range p.Rules {
		rule := &p.Rules[i]
		if rule.Match(toolName, filePath) {
			result := PermissionResult{
				Action: rule.Action,
				Rule:   rule,
			}

			switch rule.Action {
			case PermissionAllow:
				result.Allowed = true
				result.Message = fmt.Sprintf("Allowed by rule: %s", rule)
			case PermissionDeny:
				result.Allowed = false
				result.Message = fmt.Sprintf("Denied by rule: %s", rule)
			case PermissionAsk:
				result.Allowed = false // Ask defaults to not allowed until user confirms
				result.Message = fmt.Sprintf("Requires confirmation: %s", rule)
			}

			return result
		}
	}

	// No rule matched, use default based on mode
	return p.defaultResult(toolName, filePath)
}

// defaultResult returns the default permission result based on the mode
func (p *AgentPermissions) defaultResult(toolName, filePath string) PermissionResult {
	switch p.Mode {
	case "strict":
		return PermissionResult{
			Allowed: false,
			Action:  PermissionAsk,
			Message: fmt.Sprintf("No rule matched for %s [%s] in strict mode, requires confirmation", toolName, filePath),
		}
	case "permissive":
		return PermissionResult{
			Allowed: true,
			Action:  PermissionAllow,
			Message: fmt.Sprintf("No rule matched for %s [%s] in permissive mode, allowed by default", toolName, filePath),
		}
	default: // "normal" mode
		// In normal mode, read operations are allowed, write operations require confirmation
		if isReadOnlyTool(toolName) {
			return PermissionResult{
				Allowed: true,
				Action:  PermissionAllow,
				Message: fmt.Sprintf("Read-only tool %s allowed by default in normal mode", toolName),
			}
		}
		return PermissionResult{
			Allowed: false,
			Action:  PermissionAsk,
			Message: fmt.Sprintf("Write tool %s requires confirmation in normal mode", toolName),
		}
	}
}

// isReadOnlyTool checks if a tool only performs read operations
func isReadOnlyTool(toolName string) bool {
	readOnlyTools := map[string]bool{
		"read":  true,
		"glob":  true,
		"grep":  true,
		"ast":   true,
		"git":   false, // git can modify state
		"bash":  false, // bash can modify state
		"write": false,
		"edit":  false,
	}
	return readOnlyTools[toolName]
}

// Merge combines another AgentPermissions into this one
// Rules from other are appended, with other's rules taking precedence
// The mode from other is used if it's more restrictive
func (p *AgentPermissions) Merge(other *AgentPermissions) {
	if other == nil {
		return
	}

	// Merge mode: use the more restrictive mode
	p.Mode = mergeModes(p.Mode, other.Mode)

	// Append rules from other (they take precedence)
	p.Rules = append(p.Rules, other.Rules...)
}

// mergeModes returns the more restrictive of two permission modes
func mergeModes(mode1, mode2 string) string {
	modeRestriction := map[string]int{
		"strict":     3,
		"normal":     2,
		"permissive": 1,
	}

	r1 := modeRestriction[mode1]
	r2 := modeRestriction[mode2]

	if r1 >= r2 {
		return mode1
	}
	return mode2
}

// Clone creates a deep copy of the AgentPermissions
func (p *AgentPermissions) Clone() *AgentPermissions {
	clone := &AgentPermissions{
		Mode:  p.Mode,
		Rules: make([]PermissionRule, len(p.Rules)),
	}
	copy(clone.Rules, p.Rules)
	return clone
}

// String returns a string representation of the permissions
func (p *AgentPermissions) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "AgentPermissions (mode: %s)\n", p.Mode)
	fmt.Fprintf(&sb, "Rules (%d):\n", len(p.Rules))
	for i, rule := range p.Rules {
		fmt.Fprintf(&sb, "  %d. %s\n", i+1, rule)
	}
	return sb.String()
}

// DefaultPermissions returns default permissions for different agent types
func DefaultPermissions(agentType string) *AgentPermissions {
	switch agentType {
	case "explorer":
		return explorerPermissions()
	case "planner":
		return plannerPermissions()
	case "architect":
		return architectPermissions()
	case "coder":
		return coderPermissions()
	case "debugger":
		return debuggerPermissions()
	case "tester":
		return testerPermissions()
	case "reviewer":
		return reviewerPermissions()
	case "writer":
		return writerPermissions()
	default:
		return normalPermissions()
	}
}

// explorerPermissions returns permissions for exploration/analysis agents
// These agents primarily read and search code
func explorerPermissions() *AgentPermissions {
	p := NewAgentPermissions("normal")
	p.AddRules(
		// Allow all read operations
		NewPermissionRule("read", "allow", "*"),
		NewPermissionRule("glob", "allow", "*"),
		NewPermissionRule("grep", "allow", "*"),
		NewPermissionRule("ast", "allow", "*"),
		// Allow git read operations
		NewPermissionRule("git", "allow", "*"),
		// Deny write operations to sensitive files
		NewPermissionRule("write", "deny", "*.env"),
		NewPermissionRule("write", "deny", "*.secret"),
		NewPermissionRule("write", "deny", "*.key"),
		NewPermissionRule("write", "deny", "*.pem"),
		// Ask for other write operations
		NewPermissionRule("write", "ask", "*"),
		NewPermissionRule("edit", "ask", "*"),
		// Deny dangerous bash commands
		NewPermissionRule("bash", "deny", "rm -rf /*"),
		NewPermissionRule("bash", "ask", "*"),
	)
	return p
}

// coderPermissions returns permissions for code writing agents
// These agents can read and write code files
func coderPermissions() *AgentPermissions {
	p := NewAgentPermissions("normal")
	p.AddRules(
		// Allow all read operations
		NewPermissionRule("read", "allow", "*"),
		NewPermissionRule("glob", "allow", "*"),
		NewPermissionRule("grep", "allow", "*"),
		NewPermissionRule("ast", "allow", "*"),
		// Allow git operations
		NewPermissionRule("git", "allow", "*"),
		// Allow write/edit for common code files
		NewPermissionRule("write", "allow", "*.go"),
		NewPermissionRule("write", "allow", "*.js"),
		NewPermissionRule("write", "allow", "*.ts"),
		NewPermissionRule("write", "allow", "*.py"),
		NewPermissionRule("write", "allow", "*.java"),
		NewPermissionRule("write", "allow", "*.rs"),
		NewPermissionRule("write", "allow", "*.c"),
		NewPermissionRule("write", "allow", "*.cpp"),
		NewPermissionRule("write", "allow", "*.h"),
		NewPermissionRule("write", "allow", "*.md"),
		NewPermissionRule("write", "allow", "*.txt"),
		NewPermissionRule("write", "allow", "*.json"),
		NewPermissionRule("write", "allow", "*.yaml"),
		NewPermissionRule("write", "allow", "*.yml"),
		NewPermissionRule("write", "allow", "*.toml"),
		NewPermissionRule("write", "allow", "*.xml"),
		NewPermissionRule("write", "allow", "*.html"),
		NewPermissionRule("write", "allow", "*.css"),
		NewPermissionRule("write", "allow", "*.scss"),
		// Deny write operations to sensitive files
		NewPermissionRule("write", "deny", "*.env"),
		NewPermissionRule("write", "deny", "*.secret"),
		NewPermissionRule("write", "deny", "*.key"),
		NewPermissionRule("write", "deny", "*.pem"),
		NewPermissionRule("write", "deny", "*.p12"),
		NewPermissionRule("write", "deny", "*.pfx"),
		// Ask for other write operations
		NewPermissionRule("write", "ask", "*"),
		NewPermissionRule("edit", "ask", "*"),
		// Deny dangerous bash commands
		NewPermissionRule("bash", "deny", "rm -rf /*"),
		NewPermissionRule("bash", "ask", "*"),
	)
	return p
}

// reviewerPermissions returns permissions for code review agents
// These agents can only read code, no modifications
func reviewerPermissions() *AgentPermissions {
	p := NewAgentPermissions("strict")
	p.AddRules(
		// Allow all read operations
		NewPermissionRule("read", "allow", "*"),
		NewPermissionRule("glob", "allow", "*"),
		NewPermissionRule("grep", "allow", "*"),
		NewPermissionRule("ast", "allow", "*"),
		// Allow git read operations
		NewPermissionRule("git", "allow", "*"),
		// Deny all write operations
		NewPermissionRule("write", "deny", "*"),
		NewPermissionRule("edit", "deny", "*"),
		// Deny bash execution
		NewPermissionRule("bash", "deny", "*"),
	)
	return p
}

// normalPermissions returns default normal permissions
func normalPermissions() *AgentPermissions {
	p := NewAgentPermissions("normal")
	p.AddRules(
		// Allow all read operations
		NewPermissionRule("read", "allow", "*"),
		NewPermissionRule("glob", "allow", "*"),
		NewPermissionRule("grep", "allow", "*"),
		NewPermissionRule("ast", "allow", "*"),
		// Allow git operations
		NewPermissionRule("git", "allow", "*"),
		// Deny write operations to sensitive files
		NewPermissionRule("write", "deny", "*.env"),
		NewPermissionRule("write", "deny", "*.secret"),
		NewPermissionRule("write", "deny", "*.key"),
		NewPermissionRule("write", "deny", "*.pem"),
		// Deny dangerous bash commands
		NewPermissionRule("bash", "deny", "rm -rf /*"),
		// Ask for write operations
		NewPermissionRule("write", "ask", "*"),
		NewPermissionRule("edit", "ask", "*"),
		NewPermissionRule("bash", "ask", "*"),
	)
	return p
}

// plannerPermissions returns permissions for planning agents
// These agents analyze and plan but cannot modify code
func plannerPermissions() *AgentPermissions {
	p := NewAgentPermissions("strict")
	p.AddRules(
		// Allow all read operations
		NewPermissionRule("read", "allow", "*"),
		NewPermissionRule("glob", "allow", "*"),
		NewPermissionRule("grep", "allow", "*"),
		NewPermissionRule("ast", "allow", "*"),
		// Allow git operations for analysis
		NewPermissionRule("git", "allow", "*"),
		// Allow writing documentation files only
		NewPermissionRule("write", "allow", "*.md"),
		NewPermissionRule("write", "allow", "*.txt"),
		// Deny all code modifications
		NewPermissionRule("write", "deny", "*.go"),
		NewPermissionRule("write", "deny", "*.js"),
		NewPermissionRule("write", "deny", "*.ts"),
		NewPermissionRule("write", "deny", "*.py"),
		NewPermissionRule("write", "deny", "*.java"),
		NewPermissionRule("write", "deny", "*.rs"),
		// Deny bash execution
		NewPermissionRule("bash", "deny", "*"),
	)
	return p
}

// architectPermissions returns permissions for architecture design agents
// These agents can design interfaces and write documentation
func architectPermissions() *AgentPermissions {
	p := NewAgentPermissions("strict")
	p.AddRules(
		// Allow all read operations
		NewPermissionRule("read", "allow", "*"),
		NewPermissionRule("glob", "allow", "*"),
		NewPermissionRule("grep", "allow", "*"),
		NewPermissionRule("ast", "allow", "*"),
		// Allow git operations
		NewPermissionRule("git", "allow", "*"),
		// Allow writing documentation and interface files
		NewPermissionRule("write", "allow", "*.md"),
		NewPermissionRule("write", "allow", "*.yaml"),
		NewPermissionRule("write", "allow", "*.yml"),
		NewPermissionRule("write", "allow", "*.json"),
		NewPermissionRule("write", "allow", "interface*.go"),
		NewPermissionRule("write", "allow", "*_interface.go"),
		// Deny sensitive files
		NewPermissionRule("write", "deny", "*.env"),
		NewPermissionRule("write", "deny", "*.secret"),
		NewPermissionRule("write", "deny", "*.key"),
		// Ask for other operations
		NewPermissionRule("write", "ask", "*"),
		NewPermissionRule("edit", "ask", "*"),
		// Deny bash execution
		NewPermissionRule("bash", "deny", "*"),
	)
	return p
}

// debuggerPermissions returns permissions for debugging agents
// These agents can run diagnostic commands and analyze issues
func debuggerPermissions() *AgentPermissions {
	p := NewAgentPermissions("normal")
	p.AddRules(
		// Allow all read operations
		NewPermissionRule("read", "allow", "*"),
		NewPermissionRule("glob", "allow", "*"),
		NewPermissionRule("grep", "allow", "*"),
		NewPermissionRule("ast", "allow", "*"),
		// Allow git operations
		NewPermissionRule("git", "allow", "*"),
		// Allow diagnostic bash commands
		NewPermissionRule("bash", "allow", "go test *"),
		NewPermissionRule("bash", "allow", "go build *"),
		NewPermissionRule("bash", "allow", "go vet *"),
		NewPermissionRule("bash", "allow", "npm test"),
		NewPermissionRule("bash", "allow", "npm run build"),
		NewPermissionRule("bash", "allow", "pytest *"),
		NewPermissionRule("bash", "allow", "python *"),
		// Deny dangerous commands
		NewPermissionRule("bash", "deny", "rm -rf /*"),
		NewPermissionRule("bash", "deny", "sudo *"),
		// Deny sensitive files
		NewPermissionRule("write", "deny", "*.env"),
		NewPermissionRule("write", "deny", "*.secret"),
		NewPermissionRule("write", "deny", "*.key"),
		// Ask for other operations
		NewPermissionRule("write", "ask", "*"),
		NewPermissionRule("edit", "ask", "*"),
		NewPermissionRule("bash", "ask", "*"),
	)
	return p
}

// testerPermissions returns permissions for testing agents
// These agents can write test files and run tests
func testerPermissions() *AgentPermissions {
	p := NewAgentPermissions("normal")
	p.AddRules(
		// Allow all read operations
		NewPermissionRule("read", "allow", "*"),
		NewPermissionRule("glob", "allow", "*"),
		NewPermissionRule("grep", "allow", "*"),
		NewPermissionRule("ast", "allow", "*"),
		// Allow git operations
		NewPermissionRule("git", "allow", "*"),
		// Allow writing test files
		NewPermissionRule("write", "allow", "*_test.go"),
		NewPermissionRule("write", "allow", "*_test.py"),
		NewPermissionRule("write", "allow", "*_test.js"),
		NewPermissionRule("write", "allow", "*_test.ts"),
		NewPermissionRule("write", "allow", "*spec.js"),
		NewPermissionRule("write", "allow", "*spec.ts"),
		NewPermissionRule("write", "allow", "test_*.py"),
		NewPermissionRule("write", "allow", "*.mock.go"),
		// Allow running tests
		NewPermissionRule("bash", "allow", "go test *"),
		NewPermissionRule("bash", "allow", "npm test"),
		NewPermissionRule("bash", "allow", "pytest *"),
		// Deny sensitive files
		NewPermissionRule("write", "deny", "*.env"),
		NewPermissionRule("write", "deny", "*.secret"),
		NewPermissionRule("write", "deny", "*.key"),
		// Deny dangerous commands
		NewPermissionRule("bash", "deny", "rm -rf /*"),
		NewPermissionRule("bash", "deny", "sudo *"),
		// Ask for other operations
		NewPermissionRule("write", "ask", "*"),
		NewPermissionRule("edit", "ask", "*"),
		NewPermissionRule("bash", "ask", "*"),
	)
	return p
}

// writerPermissions returns permissions for documentation agents
// These agents can write documentation files only
func writerPermissions() *AgentPermissions {
	p := NewAgentPermissions("strict")
	p.AddRules(
		// Allow all read operations
		NewPermissionRule("read", "allow", "*"),
		NewPermissionRule("glob", "allow", "*"),
		NewPermissionRule("grep", "allow", "*"),
		// Allow git operations
		NewPermissionRule("git", "allow", "*"),
		// Allow writing documentation files
		NewPermissionRule("write", "allow", "*.md"),
		NewPermissionRule("write", "allow", "*.txt"),
		NewPermissionRule("write", "allow", "*.rst"),
		NewPermissionRule("write", "allow", "*.adoc"),
		NewPermissionRule("write", "allow", "README*"),
		NewPermissionRule("write", "allow", "CHANGELOG*"),
		NewPermissionRule("write", "allow", "CONTRIBUTING*"),
		NewPermissionRule("write", "allow", "LICENSE*"),
		// Deny all code modifications
		NewPermissionRule("write", "deny", "*.go"),
		NewPermissionRule("write", "deny", "*.js"),
		NewPermissionRule("write", "deny", "*.ts"),
		NewPermissionRule("write", "deny", "*.py"),
		NewPermissionRule("write", "deny", "*.java"),
		NewPermissionRule("write", "deny", "*.rs"),
		// Deny bash execution
		NewPermissionRule("bash", "deny", "*"),
	)
	return p
}

// ReadOnlyPermissions returns permissions for plan mode
// Allows only read-only tools for research and exploration
func ReadOnlyPermissions() *AgentPermissions {
	p := NewAgentPermissions("strict")
	p.AddRules(
		// Allow all read operations
		NewPermissionRule("read", "allow", "*"),
		NewPermissionRule("glob", "allow", "*"),
		NewPermissionRule("grep", "allow", "*"),
		NewPermissionRule("ast", "allow", "*"),
		// Allow LSP tools (read-only)
		NewPermissionRule("lsp_hover", "allow", "*"),
		NewPermissionRule("lsp_definition", "allow", "*"),
		NewPermissionRule("lsp_references", "allow", "*"),
		NewPermissionRule("lsp_symbols", "allow", "*"),
		// Allow web tools for research
		NewPermissionRule("web_search", "allow", "*"),
		NewPermissionRule("web_fetch", "allow", "*"),
		// Allow git read operations
		NewPermissionRule("git_status", "allow", "*"),
		NewPermissionRule("git_log", "allow", "*"),
		NewPermissionRule("git_diff", "allow", "*"),
		NewPermissionRule("git_show", "allow", "*"),
		NewPermissionRule("git_branch", "allow", "*"),
		// Deny all write operations
		NewPermissionRule("write", "deny", "*"),
		NewPermissionRule("edit", "deny", "*"),
		NewPermissionRule("bash", "deny", "*"),
	)
	return p
}
