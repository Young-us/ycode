package tools

import "context"

// Parameter represents a tool parameter definition
type Parameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ToolDefinition represents the schema sent to the LLM
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  []Parameter `json:"parameters"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Content  string      `json:"content"`
	IsError  bool        `json:"is_error"`
	Diff     *DiffResult `json:"diff,omitempty"`      // Optional diff info for file edits
	FilePath string      `json:"file_path,omitempty"` // File path for diff display
}

// ToolCategory represents the category of a tool
type ToolCategory int

const (
	CategoryBasic    ToolCategory = iota // Basic tools always available (read, glob, grep)
	CategoryWrite                        // Write operations (write_file, edit_file, bash)
	CategoryLSP                          // LSP tools (hover, definition, references)
	CategoryGit                          // Git tools (git_status, git_log, etc.)
)

// CategoryDefault returns the default category (CategoryBasic)
// Tools can embed this struct to get default category behavior
type CategoryDefault struct{}

func (c CategoryDefault) Category() ToolCategory { return CategoryBasic }

// Tool is the interface that all tools must implement
type Tool interface {
	// Name returns the tool name
	Name() string

	// Description returns a description of what the tool does
	Description() string

	// Parameters returns the tool's parameter definitions
	Parameters() []Parameter

	// Execute runs the tool with the given arguments
	Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error)

	// Category returns the tool category (optional, defaults to CategoryBasic)
	Category() ToolCategory
}

// ToDefinition converts a Tool to its definition schema
func ToDefinition(t Tool) ToolDefinition {
	return ToolDefinition{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters:  t.Parameters(),
	}
}
