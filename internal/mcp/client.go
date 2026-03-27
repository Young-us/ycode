package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/Young-us/ycode/internal/logger"
	"github.com/Young-us/ycode/internal/tools"
)

// MCPStatus represents the connection status of an MCP server
type MCPStatus string

const (
	// StatusConnected indicates the server is connected and ready
	StatusConnected MCPStatus = "connected"
	// StatusDisabled indicates the server is disabled by configuration
	StatusDisabled MCPStatus = "disabled"
	// StatusFailed indicates the server connection failed
	StatusFailed MCPStatus = "failed"
	// StatusNeedsAuth indicates the server requires authentication
	StatusNeedsAuth MCPStatus = "needs_auth"
	// StatusConnecting indicates the server is connecting
	StatusConnecting MCPStatus = "connecting"
)

// String returns the string representation of MCPStatus
func (s MCPStatus) String() string {
	return string(s)
}

// JSON-RPC 2.0 message types
type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

// UnmarshalResult unmarshals the result field into the target
func (r *jsonRPCResponse) UnmarshalResult(target interface{}) error {
	if r.Result == nil {
		return fmt.Errorf("result is nil")
	}
	data, err := json.Marshal(r.Result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}
	return json.Unmarshal(data, target)
}

type jsonRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP protocol types
type initializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    clientCapabilities `json:"capabilities"`
	ClientInfo      clientInfo         `json:"clientInfo"`
}

type clientCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    serverCapabilities `json:"capabilities"`
	ServerInfo      serverInfo         `json:"serverInfo"`
}

type serverCapabilities struct {
	Tools     *toolsCapability     `json:"tools,omitempty"`
	Prompts   *promptsCapability   `json:"prompts,omitempty"`
	Resources *resourcesCapability `json:"resources,omitempty"`
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type promptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type resourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool represents an MCP tool
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// CallToolParams represents parameters for calling a tool
type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolResult represents the result of an MCP tool call
type ToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content represents content in a tool result
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Prompt represents an MCP prompt
type Prompt struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Arguments   []PromptArg `json:"arguments,omitempty"`
}

// PromptArg represents a prompt argument
type PromptArg struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// Resource represents an MCP resource
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceContent represents the content of a resource
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

// Client represents an MCP client using JSON-RPC 2.0 over stdio
type Client struct {
	Name        string
	Command     string
	Args        []string
	Status      MCPStatus
	ServerError error

	// Process management
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	// JSON-RPC state
	requestID int
	mu        sync.RWMutex
	pending   map[interface{}]chan jsonRPCResponse

	// Server capabilities
	capabilities *serverCapabilities
	serverInfo   *serverInfo

	// Notification handlers
	onToolsChanged  func()
	onStatusChanged func(status MCPStatus)

	// Context for lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// NewClient creates a new MCP client
func NewClient(name, command string, args ...string) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		Name:    name,
		Command: command,
		Args:    args,
		Status:  StatusConnecting,
		pending: make(map[interface{}]chan jsonRPCResponse),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// NewClientFromURL creates a new MCP client (legacy compatibility)
func NewClientFromURL(name, baseURL string) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		Name:    name,
		Command: baseURL,
		Args:    []string{},
		Status:  StatusConnecting,
		pending: make(map[interface{}]chan jsonRPCResponse),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// StartServer starts the MCP server process
func (c *Client) StartServer() error {
	logger.Info("mcp", "Starting MCP server: %s (command: %s %v)", c.Name, c.Command, c.Args)

	c.cmd = exec.CommandContext(c.ctx, c.Command, c.Args...)

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		logger.Error("mcp", "Failed to create stdin pipe for %s: %v", c.Name, err)
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		logger.Error("mcp", "Failed to create stdout pipe for %s: %v", c.Name, err)
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		logger.Error("mcp", "Failed to create stderr pipe for %s: %v", c.Name, err)
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		c.Status = StatusFailed
		c.ServerError = fmt.Errorf("failed to start MCP server: %w", err)
		logger.Error("mcp", "Failed to start MCP server %s: %v", c.Name, err)
		return c.ServerError
	}

	logger.Info("mcp", "MCP server %s process started successfully", c.Name)

	// Start reading responses
	go c.readResponses()

	// Log stderr
	go c.logStderr()

	return nil
}

// Initialize performs the MCP initialization handshake
func (c *Client) Initialize(ctx context.Context) error {
	logger.Debug("mcp", "Initializing MCP server: %s", c.Name)

	params := initializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities: clientCapabilities{
			Experimental: map[string]interface{}{},
		},
		ClientInfo: clientInfo{
			Name:    "ycode",
			Version: "0.1.0",
		},
	}

	result, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		c.Status = StatusFailed
		c.ServerError = fmt.Errorf("initialize failed: %w", err)
		logger.Error("mcp", "MCP server %s initialization failed: %v", c.Name, err)
		return c.ServerError
	}

	var initResult initializeResult
	if err := result.UnmarshalResult(&initResult); err != nil {
		c.Status = StatusFailed
		c.ServerError = fmt.Errorf("failed to parse initialize result: %w", err)
		logger.Error("mcp", "Failed to parse initialize result for %s: %v", c.Name, err)
		return c.ServerError
	}

	c.capabilities = &initResult.Capabilities
	c.serverInfo = &initResult.ServerInfo

	// Send initialized notification
	if err := c.sendNotification("initialized", nil); err != nil {
		c.Status = StatusFailed
		c.ServerError = fmt.Errorf("failed to send initialized: %w", err)
		logger.Error("mcp", "Failed to send initialized notification for %s: %v", c.Name, err)
		return c.ServerError
	}

	c.Status = StatusConnected
	logger.Info("mcp", "MCP server %s initialized successfully (server: %s v%s)", c.Name, c.serverInfo.Name, c.serverInfo.Version)
	return nil
}

// StartServerAndInitialize starts the server and performs initialization
func (c *Client) StartServerAndInitialize(ctx context.Context) error {
	if err := c.StartServer(); err != nil {
		return err
	}

	// Give the server time to start (npx may need time to download/cache)
	// Also wait for any startup messages to be written
	time.Sleep(3 * time.Second)

	return c.Initialize(ctx)
}

// readResponses reads JSON-RPC responses from stdout
func (c *Client) readResponses() {
	scanner := bufio.NewScanner(c.stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg jsonRPCResponse
		if err := json.Unmarshal(line, &msg); err != nil {
			// Check if it's a notification (no ID)
			var notification jsonRPCNotification
			if err2 := json.Unmarshal(line, &notification); err2 == nil && notification.Method != "" {
				c.handleNotification(notification)
			}
			continue
		}

		// Normalize ID to int (JSON unmarshals numbers as float64)
		msgID := normalizeID(msg.ID)

		c.mu.Lock()
		ch, ok := c.pending[msgID]
		if ok {
			delete(c.pending, msgID)
		}
		c.mu.Unlock()

		if ok {
			ch <- msg
		}
	}

	c.mu.Lock()
	c.Status = StatusFailed
	c.ServerError = fmt.Errorf("MCP server disconnected")
	c.mu.Unlock()
}

// normalizeID converts an interface{} ID to int for consistent map lookups
func normalizeID(id interface{}) int {
	switch v := id.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		// Try to parse string ID as int
		var result int
		if _, err := fmt.Sscanf(v, "%d", &result); err == nil {
			return result
		}
	}
	return 0
}

// logStderr logs stderr output
func (c *Client) logStderr() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		// Log stderr if needed
	}
}

// handleNotification handles server-initiated notifications
func (c *Client) handleNotification(notification jsonRPCNotification) {
	switch notification.Method {
	case "notifications/tools/list_changed":
		c.mu.RLock()
		handler := c.onToolsChanged
		c.mu.RUnlock()
		if handler != nil {
			handler()
		}
	}
}

// sendRequest sends a JSON-RPC request and waits for response
func (c *Client) sendRequest(ctx context.Context, method string, params interface{}) (*jsonRPCResponse, error) {
	c.mu.Lock()
	c.requestID++
	id := c.requestID
	ch := make(chan jsonRPCResponse, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	data = append(data, '\n')

	c.mu.Lock()
	_, err = c.stdin.Write(data)
	c.mu.Unlock()

	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return &resp, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("request timeout for method %s", method)
	}
}

// sendNotification sends a JSON-RPC notification
func (c *Client) sendNotification(method string, params interface{}) error {
	notif := jsonRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	data = append(data, '\n')

	c.mu.Lock()
	_, err = c.stdin.Write(data)
	c.mu.Unlock()

	return err
}

// ListTools lists available tools from the MCP server
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	if c.Status == StatusDisabled {
		return nil, fmt.Errorf("MCP server %s is disabled", c.Name)
	}
	if c.Status != StatusConnected {
		return nil, fmt.Errorf("MCP server %s is not connected (status: %s)", c.Name, c.Status)
	}

	logger.Debug("mcp", "Listing tools from MCP server: %s", c.Name)

	result, err := c.sendRequest(ctx, "tools/list", map[string]interface{}{})
	if err != nil {
		logger.Error("mcp", "Failed to list tools from %s: %v", c.Name, err)
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	var toolsResult struct {
		Tools []Tool `json:"tools"`
	}
	if err := result.UnmarshalResult(&toolsResult); err != nil {
		logger.Error("mcp", "Failed to parse tools response from %s: %v", c.Name, err)
		return nil, fmt.Errorf("failed to parse tools response: %w", err)
	}

	logger.Info("mcp", "MCP server %s returned %d tools", c.Name, len(toolsResult.Tools))
	return toolsResult.Tools, nil
}

// CallTool calls a tool on the MCP server
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
	if c.Status == StatusDisabled {
		return nil, fmt.Errorf("MCP server %s is disabled", c.Name)
	}
	if c.Status != StatusConnected {
		return nil, fmt.Errorf("MCP server %s is not connected (status: %s)", c.Name, c.Status)
	}

	logger.Debug("mcp", "Calling tool %s on MCP server %s with %d args", name, c.Name, len(args))

	params := CallToolParams{
		Name:      name,
		Arguments: args,
	}

	result, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		logger.Error("mcp", "Failed to call tool %s on %s: %v", name, c.Name, err)
		return nil, fmt.Errorf("failed to call tool %s: %w", name, err)
	}

	var toolResult ToolResult
	if err := result.UnmarshalResult(&toolResult); err != nil {
		logger.Error("mcp", "Failed to parse tool result from %s: %v", c.Name, err)
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}

	logger.Info("mcp", "MCP tool %s/%s executed successfully (isError=%v)", c.Name, name, toolResult.IsError)
	return &toolResult, nil
}

// ListPrompts lists available prompts from the MCP server
func (c *Client) ListPrompts(ctx context.Context) ([]Prompt, error) {
	if c.Status == StatusDisabled {
		return nil, fmt.Errorf("MCP server %s is disabled", c.Name)
	}
	if c.Status != StatusConnected {
		return nil, fmt.Errorf("MCP server %s is not connected (status: %s)", c.Name, c.Status)
	}

	result, err := c.sendRequest(ctx, "prompts/list", map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("failed to list prompts: %w", err)
	}

	var promptsResult struct {
		Prompts []Prompt `json:"prompts"`
	}
	if err := result.UnmarshalResult(&promptsResult); err != nil {
		return nil, fmt.Errorf("failed to parse prompts response: %w", err)
	}

	return promptsResult.Prompts, nil
}

// GetPrompt gets a specific prompt with arguments
func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]string) (string, error) {
	if c.Status == StatusDisabled {
		return "", fmt.Errorf("MCP server %s is disabled", c.Name)
	}
	if c.Status != StatusConnected {
		return "", fmt.Errorf("MCP server %s is not connected (status: %s)", c.Name, c.Status)
	}

	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}

	result, err := c.sendRequest(ctx, "prompts/get", params)
	if err != nil {
		return "", fmt.Errorf("failed to get prompt %s: %w", name, err)
	}

	var promptResult struct {
		Description string `json:"description"`
		Messages    []struct {
			Role    string `json:"role"`
			Content struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := result.UnmarshalResult(&promptResult); err != nil {
		return "", fmt.Errorf("failed to parse prompt result: %w", err)
	}

	content := ""
	for _, msg := range promptResult.Messages {
		if msg.Content.Type == "text" {
			content += msg.Content.Text + "\n"
		}
	}

	return content, nil
}

// ListResources lists available resources from the MCP server
func (c *Client) ListResources(ctx context.Context) ([]Resource, error) {
	if c.Status == StatusDisabled {
		return nil, fmt.Errorf("MCP server %s is disabled", c.Name)
	}
	if c.Status != StatusConnected {
		return nil, fmt.Errorf("MCP server %s is not connected (status: %s)", c.Name, c.Status)
	}

	result, err := c.sendRequest(ctx, "resources/list", map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	var resourcesResult struct {
		Resources []Resource `json:"resources"`
	}
	if err := result.UnmarshalResult(&resourcesResult); err != nil {
		return nil, fmt.Errorf("failed to parse resources response: %w", err)
	}

	return resourcesResult.Resources, nil
}

// ReadResource reads the content of a resource
func (c *Client) ReadResource(ctx context.Context, uri string) (*ResourceContent, error) {
	if c.Status == StatusDisabled {
		return nil, fmt.Errorf("MCP server %s is disabled", c.Name)
	}
	if c.Status != StatusConnected {
		return nil, fmt.Errorf("MCP server %s is not connected (status: %s)", c.Name, c.Status)
	}

	params := map[string]interface{}{
		"uri": uri,
	}

	result, err := c.sendRequest(ctx, "resources/read", params)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource %s: %w", uri, err)
	}

	var resourceResult struct {
		Contents []ResourceContent `json:"contents"`
	}
	if err := result.UnmarshalResult(&resourceResult); err != nil {
		return nil, fmt.Errorf("failed to parse resource content: %w", err)
	}

	if len(resourceResult.Contents) == 0 {
		return nil, fmt.Errorf("no content returned for resource %s", uri)
	}

	return &resourceResult.Contents[0], nil
}

// CheckHealth checks if the MCP server is healthy
func (c *Client) CheckHealth(ctx context.Context) error {
	if c.Status == StatusDisabled {
		return fmt.Errorf("MCP server %s is disabled", c.Name)
	}
	_, err := c.ListTools(ctx)
	return err
}

// Disconnect gracefully disconnects from the MCP server
func (c *Client) Disconnect(ctx context.Context) error {
	c.cancel()

	if c.Status == StatusConnected {
		c.sendRequest(ctx, "shutdown", nil)
		c.sendNotification("exit", nil)
	}

	if c.stdin != nil {
		c.stdin.Close()
	}

	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Wait()
	}

	c.Status = StatusDisabled
	return nil
}

// IsConnected returns true if the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Status == StatusConnected
}

// GetStatus returns the current status
func (c *Client) GetStatus() MCPStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Status
}

// GetLastError returns the last error that occurred
func (c *Client) GetLastError() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ServerError
}

// SetOnToolsChanged sets the callback for when the tool list changes
func (c *Client) SetOnToolsChanged(handler func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onToolsChanged = handler
}

// SetOnStatusChanged sets the callback for when the status changes
func (c *Client) SetOnStatusChanged(handler func(status MCPStatus)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onStatusChanged = handler
}

// updateStatus updates the client status and notifies listeners
func (c *Client) updateStatus(status MCPStatus) {
	c.mu.Lock()
	oldStatus := c.Status
	c.Status = status
	handler := c.onStatusChanged
	c.mu.Unlock()

	if oldStatus != status && handler != nil {
		handler(status)
	}
}

// setStatusFailed sets the status to failed with an error
func (c *Client) setStatusFailed(err error) {
	c.mu.Lock()
	c.ServerError = err
	c.mu.Unlock()
	c.updateStatus(StatusFailed)
}

// convertMcpTool converts an MCP Tool to a ycode Tool
func convertMcpTool(mcpTool Tool, client *Client) tools.Tool {
	return &mcpToolAdapter{
		name:        mcpTool.Name,
		description: mcpTool.Description,
		inputSchema: mcpTool.InputSchema,
		client:      client,
	}
}

// mcpToolAdapter adapts an MCP Tool to the ycode Tool interface
type mcpToolAdapter struct {
	name        string
	description string
	inputSchema interface{}
	client      *Client
}

func (t *mcpToolAdapter) Name() string {
	return fmt.Sprintf("mcp_%s_%s", t.client.Name, t.name)
}

func (t *mcpToolAdapter) Description() string {
	return fmt.Sprintf("[MCP:%s] %s", t.client.Name, t.description)
}

func (t *mcpToolAdapter) Parameters() []tools.Parameter {
	params := []tools.Parameter{}

	if schema, ok := t.inputSchema.(map[string]interface{}); ok {
		if properties, ok := schema["properties"].(map[string]interface{}); ok {
			requiredFields := make(map[string]bool)
			if req, ok := schema["required"].([]interface{}); ok {
				for _, r := range req {
					if s, ok := r.(string); ok {
						requiredFields[s] = true
					}
				}
			}

			for name, prop := range properties {
				if propMap, ok := prop.(map[string]interface{}); ok {
					param := tools.Parameter{
						Name:     name,
						Required: requiredFields[name],
					}

					if desc, ok := propMap["description"].(string); ok {
						param.Description = desc
					}
					if typ, ok := propMap["type"].(string); ok {
						param.Type = typ
					}

					params = append(params, param)
				}
			}
		}
	}

	return params
}

func (t *mcpToolAdapter) Execute(ctx context.Context, args map[string]interface{}) (*tools.ToolResult, error) {
	result, err := t.client.CallTool(ctx, t.name, args)
	if err != nil {
		return &tools.ToolResult{
			Content: fmt.Sprintf("Error calling MCP tool: %v", err),
			IsError: true,
		}, nil
	}

	content := ""
	for _, c := range result.Content {
		if c.Type == "text" {
			content += c.Text + "\n"
		}
	}

	if result.IsError {
		return &tools.ToolResult{
			Content: content,
			IsError: true,
		}, nil
	}

	return &tools.ToolResult{
		Content: content,
		IsError: false,
	}, nil
}

func (t *mcpToolAdapter) Category() tools.ToolCategory {
	// MCP tools are treated as basic tools by default
	return tools.CategoryBasic
}

// ConvertTools converts all MCP tools to ycode tools
func (c *Client) ConvertTools(mcpTools []Tool) []tools.Tool {
	result := make([]tools.Tool, 0, len(mcpTools))
	for _, t := range mcpTools {
		result = append(result, convertMcpTool(t, c))
	}
	return result
}

// NotifyToolsChanged triggers the tools changed notification
func (c *Client) NotifyToolsChanged() {
	c.mu.RLock()
	handler := c.onToolsChanged
	c.mu.RUnlock()

	if handler != nil {
		handler()
	}
}
