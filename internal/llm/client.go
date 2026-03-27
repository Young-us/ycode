package llm

import (
	"context"
	"sync"
	"time"

	"github.com/Young-us/ycode/internal/tools"
)

// RetryStatus holds the current retry status in a thread-safe manner
type RetryStatus struct {
	mu      sync.RWMutex
	active  bool
	attempt int
	reason  string
	delay   time.Duration
}

// NewRetryStatus creates a new RetryStatus
func NewRetryStatus() *RetryStatus {
	return &RetryStatus{}
}

// Set updates the retry status
func (r *RetryStatus) Set(attempt int, reason string, delay time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = true
	r.attempt = attempt
	r.reason = reason
	r.delay = delay
}

// Clear clears the retry status
func (r *RetryStatus) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = false
	r.attempt = 0
	r.reason = ""
	r.delay = 0
}

// Get returns the current retry status
func (r *RetryStatus) Get() (active bool, attempt int, reason string, delay time.Duration) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active, r.attempt, r.reason, r.delay
}

// Message represents a chat message
type Message struct {
	Role     string `json:"role"` // "user", "assistant", "system"
	Content  string `json:"content"`
	Thinking string `json:"thinking,omitempty"` // Extended thinking content
}

// ToolCall represents a tool call from the LLM
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// Response represents the LLM response
type Response struct {
	Content   string     `json:"content"`
	Thinking  string     `json:"thinking,omitempty"` // Extended thinking content
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Usage     *Usage     `json:"usage,omitempty"` // Token usage from API
}

// Usage represents token usage information
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamEvent represents a streaming event
type StreamEvent struct {
	Type     string    `json:"type"` // "content", "thinking", "tool_call", "done", "usage", "retry"
	Content  string    `json:"content,omitempty"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
	Usage    *Usage    `json:"usage,omitempty"` // Token usage (sent at end of stream)
	// Retry information
	RetryAttempt int    `json:"retry_attempt,omitempty"` // Current retry attempt (1-indexed)
	RetryReason  string `json:"retry_reason,omitempty"`  // Reason for retry
}

// Client is the interface for LLM clients
type Client interface {
	// Chat sends messages to the LLM and returns a response
	Chat(ctx context.Context, messages []Message, toolDefs []tools.ToolDefinition) (*Response, error)

	// Stream sends messages to the LLM and streams the response
	Stream(ctx context.Context, messages []Message, toolDefs []tools.ToolDefinition) (<-chan StreamEvent, error)
}
