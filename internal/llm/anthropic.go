package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Young-us/ycode/internal/tools"
)

const (
	defaultBaseURL   = "https://api.anthropic.com"
	defaultModel     = "claude-sonnet-4-20250514"
	defaultMaxTokens = 4096
	anthropicVersion = "2023-06-01"

	// Retry configuration
	maxRetries     = 3
	initialDelay   = 1 * time.Second
	maxDelay       = 30 * time.Second
	retryMultiplier = 2.0
)

// AnthropicClient implements the Client interface for Anthropic API
type AnthropicClient struct {
	APIKey     string
	BaseURL    string
	Model      string
	MaxTokens  int
	HTTPClient *http.Client
	// RetryStatus holds the current retry status for UI display
	RetryStatus *RetryStatus
}

// NewAnthropicClient creates a new Anthropic client
func NewAnthropicClient(apiKey string) *AnthropicClient {
	return &AnthropicClient{
		APIKey:      apiKey,
		BaseURL:     defaultBaseURL,
		Model:       defaultModel,
		MaxTokens:   defaultMaxTokens,
		RetryStatus: NewRetryStatus(),
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Message types for Anthropic API
type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type anthropicToolUse struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

type anthropicToolResult struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
}

type anthropicTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicToolDef `json:"tools,omitempty"`
	Stream    bool               `json:"stream"`
	Thinking  *anthropicThinking `json:"thinking,omitempty"`
}

type anthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

type anthropicResponse struct {
	ID      string        `json:"id"`
	Type    string        `json:"type"`
	Role    string        `json:"role"`
	Content []interface{} `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`
	Delta struct {
		Type       string                 `json:"type,omitempty"`
		Text       string                 `json:"text,omitempty"`
		ToolUseID  string                 `json:"tool_use_id,omitempty"`
		Name       string                 `json:"name,omitempty"`
		Input      map[string]interface{} `json:"input,omitempty"`
		StopReason string                 `json:"stop_reason,omitempty"`
	} `json:"delta,omitempty"`
	ContentBlock *struct {
		Type  string                 `json:"type"`
		ID    string                 `json:"id,omitempty"`
		Name  string                 `json:"name,omitempty"`
		Input map[string]interface{} `json:"input,omitempty"`
	} `json:"content_block,omitempty"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"` // Token usage in message_start and message_delta
}

// Chat sends messages to Anthropic API and returns a response
func (c *AnthropicClient) Chat(ctx context.Context, messages []Message, toolDefs []tools.ToolDefinition) (*Response, error) {
	// Convert messages and extract system prompt
	systemPrompt, anthropicMessages := c.convertMessages(messages)

	// Convert tools
	anthropicTools := c.convertTools(toolDefs)

	// Build request with extended thinking enabled
	reqBody := anthropicRequest{
		Model:     c.Model,
		MaxTokens: c.MaxTokens,
		System:    systemPrompt,
		Messages:  anthropicMessages,
		Tools:     anthropicTools,
		Stream:    false,
		Thinking: &anthropicThinking{
			Type:         "enabled",
			BudgetTokens: 1024,
		},
	}

	// Send request
	respBody, err := c.sendRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	// Parse response
	var resp anthropicResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to our response format
	return c.convertResponse(&resp)
}

// Stream sends messages to Anthropic API and streams the response
func (c *AnthropicClient) Stream(ctx context.Context, messages []Message, toolDefs []tools.ToolDefinition) (<-chan StreamEvent, error) {
	// Convert messages and extract system prompt
	systemPrompt, anthropicMessages := c.convertMessages(messages)

	// Convert tools
	anthropicTools := c.convertTools(toolDefs)

	// Build request with extended thinking enabled
	reqBody := anthropicRequest{
		Model:     c.Model,
		MaxTokens: c.MaxTokens,
		System:    systemPrompt,
		Messages:  anthropicMessages,
		Tools:     anthropicTools,
		Stream:    true,
		Thinking: &anthropicThinking{
			Type:         "enabled",
			BudgetTokens: 1024, // Budget for thinking tokens
		},
	}

	// Send streaming request
	resp, err := c.sendStreamingRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	// Create event channel
	ch := make(chan StreamEvent, 100)

	// Process stream in background
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		// Use a done channel to signal when processing is complete
		done := make(chan struct{})
		defer close(done)

		// Start a goroutine to watch for context cancellation
		go func() {
			select {
			case <-ctx.Done():
				// Context cancelled, close the response body to unblock scanner
				resp.Body.Close()
			case <-done:
				// Processing finished normally
			}
		}()

		scanner := bufio.NewScanner(resp.Body)
		var currentToolCall *ToolCall
		var toolCallBuffer strings.Builder
		var currentEventType string // Track event type from "event:" line

		for scanner.Scan() {
			// Check for context cancellation
			select {
			case <-ctx.Done():
				// Context cancelled, stop streaming
				return
			default:
			}

			line := scanner.Text()

			// Skip empty lines
			if line == "" {
				continue
			}

			// Parse SSE event type line (e.g., "event:content_block_delta")
			if strings.HasPrefix(line, "event:") {
				currentEventType = strings.TrimPrefix(line, "event:")
				continue
			}

			// Skip comments
			if strings.HasPrefix(line, ":") {
				continue
			}

			// Parse SSE data line
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimPrefix(line, "data:")

			// Check for stream end
			if data == "[DONE]" {
				break
			}

			// Parse event data as generic map to handle both formats
			var rawData map[string]interface{}
			if err := json.Unmarshal([]byte(data), &rawData); err != nil {
				continue
			}

			// Get event type - prefer "event:" line, fallback to JSON "type" field
			eventType := currentEventType
			if eventType == "" {
				if t, ok := rawData["type"].(string); ok {
					eventType = t
				}
			}

			// Handle different event types
			switch eventType {
			case "content_block_start":
				if cb, ok := rawData["content_block"].(map[string]interface{}); ok {
					if cbType, ok := cb["type"].(string); ok && cbType == "tool_use" {
						// Start of tool call
						currentToolCall = &ToolCall{
							Arguments: make(map[string]interface{}),
						}
						if id, ok := cb["id"].(string); ok {
							currentToolCall.ID = id
						}
						if name, ok := cb["name"].(string); ok {
							currentToolCall.Name = name
						}
						toolCallBuffer.Reset()
					}
				}

			case "content_block_delta":
				// Parse delta from rawData
				if delta, ok := rawData["delta"].(map[string]interface{}); ok {
					if deltaType, ok := delta["type"].(string); ok {
						switch deltaType {
						case "text_delta":
							// Text content
							if text, ok := delta["text"].(string); ok {
								ch <- StreamEvent{
									Type:    "content",
									Content: text,
								}
							}
						case "thinking_delta":
							// Extended thinking content - forward to UI
							if thinking, ok := delta["thinking"].(string); ok {
								ch <- StreamEvent{
									Type:    "thinking",
									Content: thinking,
								}
							}
						case "input_json_delta":
							// Tool call input delta
							if currentToolCall != nil {
								if partial, ok := delta["partial_json"].(string); ok {
									toolCallBuffer.WriteString(partial)
								}
							}
						}
					}
				}

			case "content_block_stop":
				// End of content block
				if currentToolCall != nil {
					// Parse accumulated tool input
					if toolCallBuffer.Len() > 0 {
						json.Unmarshal([]byte(toolCallBuffer.String()), &currentToolCall.Arguments)
					}
					ch <- StreamEvent{
						Type:     "tool_call",
						ToolCall: currentToolCall,
					}
					currentToolCall = nil
				}

			case "message_stop":
				// End of message
				ch <- StreamEvent{Type: "done"}

			case "message_start":
				// Message start - contains initial usage
				// Note: input_tokens here includes the entire conversation history
				// We don't accumulate it, just store for this request
				if usage, ok := rawData["message"].(map[string]interface{})["usage"].(map[string]interface{}); ok {
					inputTokens, _ := usage["input_tokens"].(float64)
					outputTokens, _ := usage["output_tokens"].(float64)
					ch <- StreamEvent{
						Type: "usage",
						Usage: &Usage{
							InputTokens:  int(inputTokens),
							OutputTokens: int(outputTokens),
						},
					}
				}

			case "message_delta":
				// Message delta - contains final usage for this request
				// This is the accurate count for this specific API call
				if usage, ok := rawData["usage"].(map[string]interface{}); ok {
					outputTokens, _ := usage["output_tokens"].(float64)
					// Only output_tokens is in message_delta, input_tokens was in message_start
					ch <- StreamEvent{
						Type: "usage",
						Usage: &Usage{
							InputTokens:  0, // Already counted in message_start
							OutputTokens: int(outputTokens),
						},
					}
				}

			case "ping":
				// Ignore ping events
			}

			// Reset event type for next iteration
			currentEventType = ""
		}

		if err := scanner.Err(); err != nil {
			// Stream error - channel will close
		}
	}()

	return ch, nil
}

// convertMessages converts messages and extracts system prompt separately
// Returns (systemPrompt, anthropicMessages)
func (c *AnthropicClient) convertMessages(messages []Message) (string, []anthropicMessage) {
	var systemPrompt string
	var result []anthropicMessage

	for _, msg := range messages {
		// Extract system prompt separately (Anthropic standard uses 'system' field)
		if msg.Role == "system" {
			systemPrompt = msg.Content
			continue
		}

		anthropicMsg := anthropicMessage{
			Role: msg.Role,
		}

		// Check if message contains tool results
		if strings.HasPrefix(msg.Content, "[Tool:") {
			// Parse tool name and result
			lines := strings.SplitN(msg.Content, "\n", 2)
			toolName := strings.TrimPrefix(lines[0], "[Tool: ")
			toolName = strings.TrimSuffix(toolName, "]")

			resultContent := ""
			if len(lines) > 1 {
				resultContent = lines[1]
			}

			// Create tool result content
			anthropicMsg.Content = []anthropicToolResult{
				{
					Type:      "tool_result",
					ToolUseID: toolName, // Simplified - in real impl, track IDs
					Content:   resultContent,
				},
			}
		} else {
			// Regular text message
			anthropicMsg.Content = []anthropicTextBlock{
				{
					Type: "text",
					Text: msg.Content,
				},
			}
		}

		result = append(result, anthropicMsg)
	}

	return systemPrompt, result
}

func (c *AnthropicClient) convertTools(toolDefs []tools.ToolDefinition) []anthropicToolDef {
	var result []anthropicToolDef

	for _, td := range toolDefs {
		// Build input schema
		properties := make(map[string]interface{})
		var required []string

		for _, param := range td.Parameters {
			prop := map[string]interface{}{
				"type":        param.Type,
				"description": param.Description,
			}
			properties[param.Name] = prop

			if param.Required {
				required = append(required, param.Name)
			}
		}

		inputSchema := map[string]interface{}{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			inputSchema["required"] = required
		}

		result = append(result, anthropicToolDef{
			Name:        td.Name,
			Description: td.Description,
			InputSchema: inputSchema,
		})
	}

	return result
}

func (c *AnthropicClient) convertResponse(resp *anthropicResponse) (*Response, error) {
	result := &Response{}

	for _, content := range resp.Content {
		// Parse content block
		contentMap, ok := content.(map[string]interface{})
		if !ok {
			continue
		}

		contentType, _ := contentMap["type"].(string)

		switch contentType {
		case "text":
			text, _ := contentMap["text"].(string)
			if result.Content != "" {
				result.Content += "\n"
			}
			result.Content += text

		case "tool_use":
			toolCall := ToolCall{
				ID:   contentMap["id"].(string),
				Name: contentMap["name"].(string),
			}

			// Parse input
			if input, ok := contentMap["input"].(map[string]interface{}); ok {
				toolCall.Arguments = input
			}

			result.ToolCalls = append(result.ToolCalls, toolCall)
		}
	}

	return result, nil
}

func (c *AnthropicClient) sendRequest(ctx context.Context, reqBody anthropicRequest) ([]byte, error) {
	// Marshal request
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Execute with retry
	resp, _, err := c.doRequestWithRetry(ctx, reqJSON)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return body, nil
}

func (c *AnthropicClient) sendStreamingRequest(ctx context.Context, reqBody anthropicRequest) (*http.Response, error) {
	// Marshal request
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Execute with retry
	resp, _, err := c.doRequestWithRetry(ctx, reqJSON)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error, statusCode int) bool {
	// Network errors (timeout, connection refused, etc.)
	if err != nil {
		// Don't retry on context cancellation (user interrupt)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		// Don't retry on URL errors caused by context cancellation
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			if errors.Is(urlErr.Err, context.Canceled) || errors.Is(urlErr.Err, context.DeadlineExceeded) {
				return false
			}
		}
		return true
	}

	// Rate limit
	if statusCode == http.StatusTooManyRequests {
		return true
	}

	// Server errors (5xx)
	if statusCode >= 500 && statusCode < 600 {
		return true
	}

	return false
}

// calculateRetryDelay calculates the delay for a retry attempt
func calculateRetryDelay(attempt int) time.Duration {
	delay := initialDelay
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * retryMultiplier)
		if delay > maxDelay {
			delay = maxDelay
			break
		}
	}
	return delay
}

// notifyRetry updates the retry status for UI display
func (c *AnthropicClient) notifyRetry(attempt int, reason string, delay time.Duration) {
	if c.RetryStatus != nil {
		c.RetryStatus.Set(attempt, reason, delay)
	}
}

// GetRetryStatus returns the retry status for UI display
func (c *AnthropicClient) GetRetryStatus() *RetryStatus {
	return c.RetryStatus
}

// doRequestWithRetry executes an HTTP request with retry logic
func (c *AnthropicClient) doRequestWithRetry(ctx context.Context, reqJSON []byte) (*http.Response, []byte, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Check context cancellation
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}

		// Create new request for each attempt (body needs to be recreated)
		req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/messages", bytes.NewReader(reqJSON))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.APIKey)
		req.Header.Set("anthropic-version", anthropicVersion)

		// Send request
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = err

			if isRetryableError(err, 0) && attempt < maxRetries {
				delay := calculateRetryDelay(attempt)
				c.notifyRetry(attempt, err.Error(), delay)

				select {
				case <-time.After(delay):
					continue
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				}
			}
			continue
		}

		// Check status code
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))

			if isRetryableError(nil, resp.StatusCode) && attempt < maxRetries {
				delay := calculateRetryDelay(attempt)
				c.notifyRetry(attempt, fmt.Sprintf("HTTP %d", resp.StatusCode), delay)

				select {
				case <-time.After(delay):
					continue
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				}
			}
			continue
		}

		// Success
		if c.RetryStatus != nil {
			c.RetryStatus.Clear()
		}
		return resp, nil, nil
	}

	// All retries exhausted - clear retry status
	if c.RetryStatus != nil {
		c.RetryStatus.Clear()
	}
	return nil, nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
}
