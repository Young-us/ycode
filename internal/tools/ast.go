package tools

import (
	"context"
	"fmt"

	"github.com/Young-us/ycode/internal/ast"
)

// ASTTool uses AST parser to analyze code
type ASTTool struct {
	Parser *ast.Parser
}

// NewASTTool creates a new AST tool
func NewASTTool(workingDir string) *ASTTool {
	return &ASTTool{
		Parser: ast.NewParser(workingDir),
	}
}

func (t *ASTTool) Name() string {
	return "ast"
}

func (t *ASTTool) Description() string {
	return "Analyze code structure using AST parsing. Use this to find functions, classes, and other code elements."
}

func (t *ASTTool) Category() ToolCategory {
	return CategoryAST
}

func (t *ASTTool) Parameters() []Parameter {
	return []Parameter{
		{
			Name:        "path",
			Type:        "string",
			Description: "The path to the file to analyze",
			Required:    true,
		},
		{
			Name:        "action",
			Type:        "string",
			Description: "The action to perform: 'functions' to list functions, 'imports' to list imports, 'find' to find a specific function",
			Required:    true,
		},
		{
			Name:        "name",
			Type:        "string",
			Description: "The function name to find (only for 'find' action)",
			Required:    false,
		},
	}
}

func (t *ASTTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &ToolResult{
			Content: "Error: 'path' parameter is required and must be a string",
			IsError: true,
		}, nil
	}

	action, ok := args["action"].(string)
	if !ok {
		return &ToolResult{
			Content: "Error: 'action' parameter is required and must be a string",
			IsError: true,
		}, nil
	}

	// Parse the file
	node, err := t.Parser.Parse(ctx, path)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error parsing file: %v", err),
			IsError: true,
		}, nil
	}

	switch action {
	case "functions":
		functions := t.Parser.GetFunctions(node)
		if len(functions) == 0 {
			return &ToolResult{
				Content: "No functions found",
				IsError: false,
			}, nil
		}

		result := fmt.Sprintf("Found %d function(s):\n\n", len(functions))
		for _, fn := range functions {
			result += fmt.Sprintf("  - %s\n", fn.Content)
		}

		return &ToolResult{
			Content: result,
			IsError: false,
		}, nil

	case "imports":
		imports := t.Parser.GetImports(node)
		if len(imports) == 0 {
			return &ToolResult{
				Content: "No imports found",
				IsError: false,
			}, nil
		}

		result := fmt.Sprintf("Found %d import(s):\n\n", len(imports))
		for _, imp := range imports {
			result += fmt.Sprintf("  - %s\n", imp.Content)
		}

		return &ToolResult{
			Content: result,
			IsError: false,
		}, nil

	case "find":
		name, ok := args["name"].(string)
		if !ok || name == "" {
			return &ToolResult{
				Content: "Error: 'name' parameter is required for 'find' action",
				IsError: true,
			}, nil
		}

		fn := t.Parser.FindFunction(node, name)
		if fn == nil {
			return &ToolResult{
				Content: fmt.Sprintf("Function '%s' not found", name),
				IsError: false,
			}, nil
		}

		return &ToolResult{
			Content: fmt.Sprintf("Found function '%s':\n\n%s", name, fn.Content),
			IsError: false,
		}, nil

	default:
		return &ToolResult{
			Content: fmt.Sprintf("Unknown action: %s. Use 'functions', 'imports', or 'find'", action),
			IsError: true,
		}, nil
	}
}
