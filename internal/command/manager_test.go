package command

import (
	"log"
	"os"
	"testing"
)

func TestExtractHints(t *testing.T) {
	tests := []struct {
		name     string
		template string
		expected []string
	}{
		{
			name:     "empty template",
			template: "",
			expected: nil,
		},
		{
			name:     "no placeholders",
			template: "Hello world",
			expected: nil,
		},
		{
			name:     "single argument placeholder",
			template: "Review $1",
			expected: []string{"$1"},
		},
		{
			name:     "multiple argument placeholders",
			template: "Compare $1 with $2",
			expected: []string{"$1", "$2"},
		},
		{
			name:     "arguments placeholder",
			template: "Run command: $ARGUMENTS",
			expected: []string{"$ARGUMENTS"},
		},
		{
			name:     "mixed placeholders",
			template: "Process $1 with $ARGUMENTS and $2",
			expected: []string{"$ARGUMENTS", "$1", "$2"},
		},
		{
			name:     "duplicate placeholders",
			template: "Use $1 and $1 again",
			expected: []string{"$1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractHints(tt.template)
			if len(result) != len(tt.expected) {
				t.Errorf("ExtractHints() returned %d hints, expected %d", len(result), len(tt.expected))
				return
			}
			for i, hint := range result {
				if hint != tt.expected[i] {
					t.Errorf("ExtractHints()[%d] = %s, expected %s", i, hint, tt.expected[i])
				}
			}
		})
	}
}

func TestProcessTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		argsStr  string
		args     []string
		expected string
	}{
		{
			name:     "no placeholders",
			template: "Hello world",
			argsStr:  "foo bar",
			args:     []string{"foo", "bar"},
			expected: "Hello world",
		},
		{
			name:     "arguments placeholder",
			template: "Run: $ARGUMENTS",
			argsStr:  "foo bar",
			args:     []string{"foo", "bar"},
			expected: "Run: foo bar",
		},
		{
			name:     "numbered placeholders",
			template: "Compare $1 with $2",
			argsStr:  "file1.txt file2.txt",
			args:     []string{"file1.txt", "file2.txt"},
			expected: "Compare file1.txt with file2.txt",
		},
		{
			name:     "mixed placeholders",
			template: "Process $1 with $ARGUMENTS",
			argsStr:  "input.txt --verbose",
			args:     []string{"input.txt", "--verbose"},
			expected: "Process input.txt with input.txt --verbose",
		},
		{
			name:     "missing arguments",
			template: "Use $1 and $2 and $3",
			argsStr:  "only two",
			args:     []string{"only", "two"},
			expected: "Use only and two and $3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessTemplate(tt.template, tt.argsStr, tt.args)
			if result != tt.expected {
				t.Errorf("ProcessTemplate() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestCommandManager_Register(t *testing.T) {
	m := &CommandManager{
		commands: make(map[string]*Command),
		logger:   log.New(os.Stderr, "", 0),
	}

	cmd := &Command{
		Name:        "test",
		Description: "Test command",
		Template:    "Test $1",
	}

	m.Register(cmd)

	if !m.HasCommand("test") {
		t.Error("Command 'test' should be registered")
	}

	retrieved, exists := m.Get("test")
	if !exists {
		t.Error("Get() should return the command")
	}
	if retrieved.Name != "test" {
		t.Errorf("Get().Name = %s, expected 'test'", retrieved.Name)
	}
}

func TestCommandManager_FindMatching(t *testing.T) {
	m := &CommandManager{
		commands: make(map[string]*Command),
		logger:   log.New(os.Stderr, "", 0),
	}

	commands := []*Command{
		{Name: "help", Description: "Show help"},
		{Name: "history", Description: "Show history"},
		{Name: "health", Description: "Check health"},
		{Name: "config", Description: "Show config"},
	}

	for _, cmd := range commands {
		m.Register(cmd)
	}

	tests := []struct {
		prefix   string
		expected []string
	}{
		{"h", []string{"health", "help", "history"}},
		{"he", []string{"health", "help"}},
		{"hel", []string{"help"}},
		{"c", []string{"config"}},
		{"z", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			matched := m.FindMatching(tt.prefix)
			if len(matched) != len(tt.expected) {
				t.Errorf("FindMatching(%q) returned %d commands, expected %d", tt.prefix, len(matched), len(tt.expected))
				return
			}
			for i, cmd := range matched {
				if cmd.Name != tt.expected[i] {
					t.Errorf("FindMatching(%q)[%d] = %s, expected %s", tt.prefix, i, cmd.Name, tt.expected[i])
				}
			}
		})
	}
}

func TestCommandManager_ListBySource(t *testing.T) {
	m := &CommandManager{
		commands: make(map[string]*Command),
		logger:   log.New(os.Stderr, "", 0),
	}

	commands := []*Command{
		{Name: "builtin1", Source: SourceCommand},
		{Name: "builtin2", Source: SourceCommand},
		{Name: "mcp1", Source: SourceMCP},
		{Name: "skill1", Source: SourceSkill},
	}

	for _, cmd := range commands {
		m.Register(cmd)
	}

	builtin := m.ListBySource(SourceCommand)
	if len(builtin) != 2 {
		t.Errorf("ListBySource(SourceCommand) returned %d commands, expected 2", len(builtin))
	}

	mcp := m.ListBySource(SourceMCP)
	if len(mcp) != 1 {
		t.Errorf("ListBySource(SourceMCP) returned %d commands, expected 1", len(mcp))
	}

	skill := m.ListBySource(SourceSkill)
	if len(skill) != 1 {
		t.Errorf("ListBySource(SourceSkill) returned %d commands, expected 1", len(skill))
	}
}

func TestCommandManager_Execute(t *testing.T) {
	m := &CommandManager{
		commands: make(map[string]*Command),
		logger:   log.New(os.Stderr, "", 0),
	}

	// Register a command with handler
	handlerCalled := false
	m.Register(&Command{
		Name: "test",
		Handler: func(args []string) (string, error) {
			handlerCalled = true
			return "test output", nil
		},
	})

	// Register a command with template
	m.Register(&Command{
		Name:     "greet",
		Template: "Hello $1!",
	})

	tests := []struct {
		name      string
		input     string
		handled   bool
		result    string
		expectErr bool
	}{
		{
			name:    "not a command",
			input:   "hello world",
			handled: false,
		},
		{
			name:    "command with handler",
			input:   "/test",
			handled: true,
			result:  "test output",
		},
		{
			name:    "command with template",
			input:   "/greet World",
			handled: true,
			result:  "Hello World!",
		},
		{
			name:      "unknown command",
			input:     "/unknown",
			handled:   true,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled = false
			handled, result, err := m.Execute(tt.input)

			if handled != tt.handled {
				t.Errorf("Execute(%q) handled = %v, expected %v", tt.input, handled, tt.handled)
			}

			if tt.expectErr && err == nil {
				t.Errorf("Execute(%q) expected error, got nil", tt.input)
			}

			if !tt.expectErr && result != tt.result {
				t.Errorf("Execute(%q) result = %q, expected %q", tt.input, result, tt.result)
			}

			if tt.name == "command with handler" && !handlerCalled {
				t.Error("Handler should have been called")
			}
		})
	}
}

func TestCommandManager_RegisterFromConfig(t *testing.T) {
	m := &CommandManager{
		commands: make(map[string]*Command),
		logger:   log.New(os.Stderr, "", 0),
	}

	configCommands := []ConfigCommand{
		{
			Name:        "custom1",
			Description: "Custom command 1",
			Template:    "Run $1",
			Agent:       "coder",
			Model:       "claude-sonnet",
			Subtask:     true,
		},
		{
			Name:        "custom2",
			Description: "Custom command 2",
			Template:    "Process $ARGUMENTS",
		},
	}

	m.RegisterFromConfig(configCommands)

	if m.Count() != 2 {
		t.Errorf("RegisterFromConfig() should register 2 commands, got %d", m.Count())
	}

	cmd, exists := m.Get("custom1")
	if !exists {
		t.Error("Command 'custom1' should be registered")
	}
	if cmd.Agent != "coder" {
		t.Errorf("Command.Agent = %s, expected 'coder'", cmd.Agent)
	}
	if cmd.Model != "claude-sonnet" {
		t.Errorf("Command.Model = %s, expected 'claude-sonnet'", cmd.Model)
	}
	if !cmd.Subtask {
		t.Error("Command.Subtask should be true")
	}
	if len(cmd.Hints) != 1 || cmd.Hints[0] != "$1" {
		t.Errorf("Command.Hints = %v, expected ['$1']", cmd.Hints)
	}
}

func TestFormatCommands(t *testing.T) {
	commands := []*Command{
		{
			Name:        "help",
			Description: "Show help",
			Source:      SourceCommand,
		},
		{
			Name:        "review",
			Description: "Review code",
			Source:      SourceSkill,
			Agent:       "reviewer",
			Subtask:     true,
			Hints:       []string{"$1"},
		},
	}

	// Test non-verbose format
	result := FormatCommands(commands, false)
	if result == "" {
		t.Error("FormatCommands() should return non-empty string")
	}

	// Test verbose format
	verboseResult := FormatCommands(commands, true)
	if verboseResult == "" {
		t.Error("FormatCommands(verbose=true) should return non-empty string")
	}
}
