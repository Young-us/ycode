package agent

import (
	"testing"
)

func TestPermissionRule_Match(t *testing.T) {
	tests := []struct {
		name     string
		rule     PermissionRule
		toolName string
		filePath string
		want     bool
	}{
		{
			name:     "wildcard tool matches any tool",
			rule:     PermissionRule{Tool: "*", Action: PermissionAllow, Pattern: "*"},
			toolName: "read",
			filePath: "test.go",
			want:     true,
		},
		{
			name:     "exact tool match",
			rule:     PermissionRule{Tool: "read", Action: PermissionAllow, Pattern: "*"},
			toolName: "read",
			filePath: "test.go",
			want:     true,
		},
		{
			name:     "tool mismatch",
			rule:     PermissionRule{Tool: "read", Action: PermissionAllow, Pattern: "*"},
			toolName: "write",
			filePath: "test.go",
			want:     false,
		},
		{
			name:     "glob pattern match",
			rule:     PermissionRule{Tool: "*", Action: PermissionDeny, Pattern: "*.env"},
			toolName: "write",
			filePath: ".env",
			want:     true,
		},
		{
			name:     "glob pattern no match",
			rule:     PermissionRule{Tool: "*", Action: PermissionDeny, Pattern: "*.env"},
			toolName: "write",
			filePath: "test.go",
			want:     false,
		},
		{
			name:     "filename only match",
			rule:     PermissionRule{Tool: "*", Action: PermissionDeny, Pattern: "*.key"},
			toolName: "write",
			filePath: "/path/to/secret.key",
			want:     true,
		},
		{
			name:     "empty pattern matches all",
			rule:     PermissionRule{Tool: "read", Action: PermissionAllow, Pattern: ""},
			toolName: "read",
			filePath: "any/file.txt",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rule.Match(tt.toolName, tt.filePath)
			if got != tt.want {
				t.Errorf("PermissionRule.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAgentPermissions_Check(t *testing.T) {
	tests := []struct {
		name        string
		permissions *AgentPermissions
		toolName    string
		filePath    string
		wantAllowed bool
		wantAction  PermissionAction
	}{
		{
			name:        "allow read in normal mode",
			permissions: normalPermissions(),
			toolName:    "read",
			filePath:    "test.go",
			wantAllowed: true,
			wantAction:  PermissionAllow,
		},
		{
			name:        "deny write to .env file",
			permissions: normalPermissions(),
			toolName:    "write",
			filePath:    ".env",
			wantAllowed: false,
			wantAction:  PermissionDeny,
		},
		{
			name:        "ask for write to regular file",
			permissions: normalPermissions(),
			toolName:    "write",
			filePath:    "test.go",
			wantAllowed: false,
			wantAction:  PermissionAsk,
		},
		{
			name:        "deny bash rm -rf",
			permissions: normalPermissions(),
			toolName:    "bash",
			filePath:    "rm -rf /*",
			wantAllowed: false,
			wantAction:  PermissionDeny,
		},
		{
			name:        "ask for bash in normal mode",
			permissions: normalPermissions(),
			toolName:    "bash",
			filePath:    "ls -la",
			wantAllowed: false,
			wantAction:  PermissionAsk,
		},
		{
			name:        "strict mode requires confirmation",
			permissions: reviewerPermissions(),
			toolName:    "write",
			filePath:    "test.go",
			wantAllowed: false,
			wantAction:  PermissionDeny,
		},
		{
			name:        "strict mode allows read",
			permissions: reviewerPermissions(),
			toolName:    "read",
			filePath:    "test.go",
			wantAllowed: true,
			wantAction:  PermissionAllow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.permissions.Check(tt.toolName, tt.filePath)
			if result.Allowed != tt.wantAllowed {
				t.Errorf("Check().Allowed = %v, want %v", result.Allowed, tt.wantAllowed)
			}
			if result.Action != tt.wantAction {
				t.Errorf("Check().Action = %v, want %v", result.Action, tt.wantAction)
			}
		})
	}
}

func TestAgentPermissions_Merge(t *testing.T) {
	base := NewAgentPermissions("normal")
	base.AddRule("read", "allow", "*")
	base.AddRule("write", "ask", "*.go")

	override := NewAgentPermissions("strict")
	override.AddRule("write", "deny", "*.env")

	base.Merge(override)

	// Check that mode became more restrictive
	if base.Mode != "strict" {
		t.Errorf("Merge() mode = %v, want %v", base.Mode, "strict")
	}

	// Check that rules were combined
	if len(base.Rules) != 3 {
		t.Errorf("Merge() rules count = %v, want %v", len(base.Rules), 3)
	}

	// Check that override rules take precedence
	result := base.Check("write", ".env")
	if result.Allowed {
		t.Errorf("Merge() should deny write to .env, got allowed")
	}
	if result.Action != PermissionDeny {
		t.Errorf("Merge() action = %v, want %v", result.Action, PermissionDeny)
	}
}

func TestAgentPermissions_Clone(t *testing.T) {
	original := normalPermissions()
	clone := original.Clone()

	// Verify clone has same mode
	if clone.Mode != original.Mode {
		t.Errorf("Clone().Mode = %v, want %v", clone.Mode, original.Mode)
	}

	// Verify clone has same number of rules
	if len(clone.Rules) != len(original.Rules) {
		t.Errorf("Clone().Rules count = %v, want %v", len(clone.Rules), len(original.Rules))
	}

	// Verify modifying clone doesn't affect original
	clone.AddRule("test", "allow", "*")
	if len(original.Rules) == len(clone.Rules) {
		t.Errorf("Clone() should be independent, but modifying clone affected original")
	}
}

func TestDefaultPermissions(t *testing.T) {
	tests := []struct {
		name      string
		agentType string
		wantMode  string
	}{
		{
			name:      "explorer permissions",
			agentType: "explorer",
			wantMode:  "normal",
		},
		{
			name:      "coder permissions",
			agentType: "coder",
			wantMode:  "normal",
		},
		{
			name:      "reviewer permissions",
			agentType: "reviewer",
			wantMode:  "strict",
		},
		{
			name:      "executor permissions",
			agentType: "executor",
			wantMode:  "normal",
		},
		{
			name:      "unknown agent type",
			agentType: "unknown",
			wantMode:  "normal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perms := DefaultPermissions(tt.agentType)
			if perms.Mode != tt.wantMode {
				t.Errorf("DefaultPermissions(%v).Mode = %v, want %v", tt.agentType, perms.Mode, tt.wantMode)
			}
			if len(perms.Rules) == 0 {
				t.Errorf("DefaultPermissions(%v) should have rules", tt.agentType)
			}
		})
	}
}

func TestPermissionRule_String(t *testing.T) {
	tests := []struct {
		name string
		rule PermissionRule
		want string
	}{
		{
			name: "rule without pattern",
			rule: PermissionRule{Tool: "read", Action: PermissionAllow, Pattern: ""},
			want: "read: allow",
		},
		{
			name: "rule with wildcard pattern",
			rule: PermissionRule{Tool: "read", Action: PermissionAllow, Pattern: "*"},
			want: "read: allow",
		},
		{
			name: "rule with specific pattern",
			rule: PermissionRule{Tool: "write", Action: PermissionDeny, Pattern: "*.env"},
			want: "write [*.env]: deny",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rule.String()
			if got != tt.want {
				t.Errorf("PermissionRule.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAgentPermissions_String(t *testing.T) {
	perms := NewAgentPermissions("normal")
	perms.AddRule("read", "allow", "*")
	perms.AddRule("write", "deny", "*.env")

	str := perms.String()
	if str == "" {
		t.Errorf("AgentPermissions.String() should not be empty")
	}
	// Just verify it doesn't panic and returns something
}

func TestExplorerPermissions(t *testing.T) {
	perms := explorerPermissions()

	// Explorer should be able to read
	result := perms.Check("read", "test.go")
	if !result.Allowed {
		t.Errorf("Explorer should be able to read files")
	}

	// Explorer should be able to glob
	result = perms.Check("glob", "*.go")
	if !result.Allowed {
		t.Errorf("Explorer should be able to use glob")
	}

	// Explorer should be able to grep
	result = perms.Check("grep", "pattern")
	if !result.Allowed {
		t.Errorf("Explorer should be able to use grep")
	}

	// Explorer should not be able to write to .env
	result = perms.Check("write", ".env")
	if result.Allowed {
		t.Errorf("Explorer should not be able to write to .env")
	}
}

func TestCoderPermissions(t *testing.T) {
	perms := coderPermissions()

	// Coder should be able to read
	result := perms.Check("read", "test.go")
	if !result.Allowed {
		t.Errorf("Coder should be able to read files")
	}

	// Coder should be able to write Go files
	result = perms.Check("write", "main.go")
	if !result.Allowed {
		t.Errorf("Coder should be able to write Go files")
	}

	// Coder should not be able to write to .env
	result = perms.Check("write", ".env")
	if result.Allowed {
		t.Errorf("Coder should not be able to write to .env")
	}

	// Coder should not be able to write to .key files
	result = perms.Check("write", "secret.key")
	if result.Allowed {
		t.Errorf("Coder should not be able to write to .key files")
	}
}

func TestReviewerPermissions(t *testing.T) {
	perms := reviewerPermissions()

	// Reviewer should be able to read
	result := perms.Check("read", "test.go")
	if !result.Allowed {
		t.Errorf("Reviewer should be able to read files")
	}

	// Reviewer should not be able to write
	result = perms.Check("write", "test.go")
	if result.Allowed {
		t.Errorf("Reviewer should not be able to write files")
	}

	// Reviewer should not be able to use bash
	result = perms.Check("bash", "ls")
	if result.Allowed {
		t.Errorf("Reviewer should not be able to use bash")
	}
}

func TestExecutorPermissions(t *testing.T) {
	perms := executorPermissions()

	// Executor should be able to read
	result := perms.Check("read", "test.go")
	if !result.Allowed {
		t.Errorf("Executor should be able to read files")
	}

	// Executor should be able to write Go files
	result = perms.Check("write", "main.go")
	if !result.Allowed {
		t.Errorf("Executor should be able to write Go files")
	}

	// Executor should not be able to use sudo
	result = perms.Check("bash", "sudo apt-get update")
	if result.Allowed {
		t.Errorf("Executor should not be able to use sudo")
	}

	// Executor should not be able to chmod 777
	result = perms.Check("bash", "chmod 777 /tmp")
	if result.Allowed {
		t.Errorf("Executor should not be able to chmod 777")
	}
}
