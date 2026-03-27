package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/Young-us/ycode/internal/lsp"
)

// LSPTool provides LSP features to the agent
type LSPTool struct {
	WorkingDir string
	Clients    map[string]*lsp.Client
}

// NewLSPTool creates a new LSP tool
func NewLSPTool(workingDir string, clients map[string]*lsp.Client) *LSPTool {
	return &LSPTool{
		WorkingDir: workingDir,
		Clients:    clients,
	}
}

// Name returns the tool name
func (t *LSPTool) Name() string {
	return "lsp"
}

// Description returns the tool description
func (t *LSPTool) Description() string {
	return "Query LSP servers for code intelligence features like hover, definition, references, completion, and diagnostics"
}

// Category returns the tool category
func (t *LSPTool) Category() ToolCategory {
	return CategoryLSP
}

// Parameters returns the tool parameters
func (t *LSPTool) Parameters() []Parameter {
	return []Parameter{
		{
			Name:        "action",
			Description: "The LSP action to perform: hover, definition, references, completion, diagnostics",
			Required:    true,
			Type:        "string",
		},
		{
			Name:        "file",
			Description: "The file path to query",
			Required:    true,
			Type:        "string",
		},
		{
			Name:        "line",
			Description: "Line number (0-indexed)",
			Required:    false,
			Type:        "integer",
		},
		{
			Name:        "character",
			Description: "Character position (0-indexed)",
			Required:    false,
			Type:        "integer",
		},
		{
			Name:        "server",
			Description: "LSP server name (optional, uses first available if not specified)",
			Required:    false,
			Type:        "string",
		},
	}
}

// Execute executes the LSP tool
func (t *LSPTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	action, ok := args["action"].(string)
	if !ok {
		return &ToolResult{
			Content: "Error: 'action' parameter is required",
			IsError: true,
		}, nil
	}

	file, ok := args["file"].(string)
	if !ok {
		return &ToolResult{
			Content: "Error: 'file' parameter is required",
			IsError: true,
		}, nil
	}

	// Get line and character (optional)
	line := 0
	character := 0
	if l, ok := args["line"].(float64); ok {
		line = int(l)
	}
	if c, ok := args["character"].(float64); ok {
		character = int(c)
	}

	// Get server (optional)
	serverName := ""
	if s, ok := args["server"].(string); ok {
		serverName = s
	}

	// Find the LSP client
	client, err := t.getClient(serverName)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error: %v", err),
			IsError: true,
		}, nil
	}

	// Convert file path to URI
	uri := lsp.DocumentURI("file://" + file)

	// Execute the action
	switch action {
	case "hover":
		return t.doHover(ctx, client, uri, line, character)
	case "definition":
		return t.doDefinition(ctx, client, uri, line, character)
	case "references":
		return t.doReferences(ctx, client, uri, line, character)
	case "completion":
		return t.doCompletion(ctx, client, uri, line, character)
	case "diagnostics":
		return t.doDiagnostics(client, uri)
	default:
		return &ToolResult{
			Content: fmt.Sprintf("Error: unknown action '%s'. Available actions: hover, definition, references, completion, diagnostics", action),
			IsError: true,
		}, nil
	}
}

// getClient returns an LSP client by name or the first available
func (t *LSPTool) getClient(name string) (*lsp.Client, error) {
	if len(t.Clients) == 0 {
		return nil, fmt.Errorf("no LSP servers available")
	}

	if name != "" {
		client, ok := t.Clients[name]
		if !ok {
			return nil, fmt.Errorf("LSP server '%s' not found", name)
		}
		return client, nil
	}

	// Return first available client
	for _, client := range t.Clients {
		return client, nil
	}

	return nil, fmt.Errorf("no LSP servers available")
}

// doHover performs a hover request
func (t *LSPTool) doHover(ctx context.Context, client *lsp.Client, uri lsp.DocumentURI, line, character int) (*ToolResult, error) {
	hover, err := client.Hover(ctx, uri, line, character)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error getting hover: %v", err),
			IsError: true,
		}, nil
	}

	if hover == nil {
		return &ToolResult{
			Content: "No hover information available at this position",
			IsError: false,
		}, nil
	}

	return &ToolResult{
		Content: hover.Contents.Value,
		IsError: false,
	}, nil
}

// doDefinition performs a go-to-definition request
func (t *LSPTool) doDefinition(ctx context.Context, client *lsp.Client, uri lsp.DocumentURI, line, character int) (*ToolResult, error) {
	locations, err := client.Definition(ctx, uri, line, character)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error getting definition: %v", err),
			IsError: true,
		}, nil
	}

	if len(locations) == 0 {
		return &ToolResult{
			Content: "No definition found at this position",
			IsError: false,
		}, nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d definition(s):\n\n", len(locations)))
	for i, loc := range locations {
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, loc.URI))
		result.WriteString(fmt.Sprintf("   Line %d, Column %d\n", loc.Range.Start.Line, loc.Range.Start.Character))
	}

	return &ToolResult{
		Content: result.String(),
		IsError: false,
	}, nil
}

// doReferences performs a find-references request
func (t *LSPTool) doReferences(ctx context.Context, client *lsp.Client, uri lsp.DocumentURI, line, character int) (*ToolResult, error) {
	locations, err := client.References(ctx, uri, line, character, true)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error getting references: %v", err),
			IsError: true,
		}, nil
	}

	if len(locations) == 0 {
		return &ToolResult{
			Content: "No references found at this position",
			IsError: false,
		}, nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d reference(s):\n\n", len(locations)))
	for i, loc := range locations {
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, loc.URI))
		result.WriteString(fmt.Sprintf("   Line %d, Column %d\n", loc.Range.Start.Line, loc.Range.Start.Character))
	}

	return &ToolResult{
		Content: result.String(),
		IsError: false,
	}, nil
}

// doCompletion performs a completion request
func (t *LSPTool) doCompletion(ctx context.Context, client *lsp.Client, uri lsp.DocumentURI, line, character int) (*ToolResult, error) {
	completionList, err := client.Completion(ctx, uri, line, character)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error getting completions: %v", err),
			IsError: true,
		}, nil
	}

	if completionList == nil || len(completionList.Items) == 0 {
		return &ToolResult{
			Content: "No completions available at this position",
			IsError: false,
		}, nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d completion(s):\n\n", len(completionList.Items)))
	for i, item := range completionList.Items {
		result.WriteString(fmt.Sprintf("%d. %s", i+1, item.Label))
		if item.Detail != "" {
			result.WriteString(fmt.Sprintf(" - %s", item.Detail))
		}
		result.WriteString("\n")
	}

	return &ToolResult{
		Content: result.String(),
		IsError: false,
	}, nil
}

// doDiagnostics gets diagnostics for a file
func (t *LSPTool) doDiagnostics(client *lsp.Client, uri lsp.DocumentURI) (*ToolResult, error) {
	diagnostics := client.GetDiagnostics(uri)

	if len(diagnostics) == 0 {
		return &ToolResult{
			Content: "No diagnostics found for this file",
			IsError: false,
		}, nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d diagnostic(s):\n\n", len(diagnostics)))
	for i, diag := range diagnostics {
		severity := "Info"
		switch diag.Severity {
		case 1:
			severity = "Error"
		case 2:
			severity = "Warning"
		case 3:
			severity = "Information"
		case 4:
			severity = "Hint"
		}

		result.WriteString(fmt.Sprintf("%d. [%s] Line %d, Column %d\n", i+1, severity, diag.Range.Start.Line, diag.Range.Start.Character))
		result.WriteString(fmt.Sprintf("   %s\n", diag.Message))
	}

	return &ToolResult{
		Content: result.String(),
		IsError: false,
	}, nil
}
