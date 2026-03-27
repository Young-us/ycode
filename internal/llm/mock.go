package llm

import (
	"context"

	"github.com/Young-us/ycode/internal/tools"
)

// MockClient is a mock LLM client for testing
type MockClient struct {
	Responses []Response
	CallIndex int
	Calls     []MockCall
}

// MockCall records a call to the mock client
type MockCall struct {
	Messages []Message
	ToolDefs []tools.ToolDefinition
}

// NewMockClient creates a new mock client with predefined responses
func NewMockClient(responses ...Response) *MockClient {
	return &MockClient{
		Responses: responses,
	}
}

// Chat implements the Client interface
func (m *MockClient) Chat(ctx context.Context, messages []Message, toolDefs []tools.ToolDefinition) (*Response, error) {
	// Record the call
	m.Calls = append(m.Calls, MockCall{
		Messages: messages,
		ToolDefs: toolDefs,
	})

	// Return predefined response or default
	if m.CallIndex < len(m.Responses) {
		resp := m.Responses[m.CallIndex]
		m.CallIndex++
		return &resp, nil
	}

	// Default response if no more predefined responses
	return &Response{
		Content: "Mock response: no more predefined responses",
	}, nil
}

// Stream implements the Client interface
func (m *MockClient) Stream(ctx context.Context, messages []Message, toolDefs []tools.ToolDefinition) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 1)

	go func() {
		defer close(ch)

		resp, err := m.Chat(ctx, messages, toolDefs)
		if err != nil {
			return
		}

		// Send content as single event
		if resp.Content != "" {
			ch <- StreamEvent{
				Type:    "content",
				Content: resp.Content,
			}
		}

		// Send tool calls
		for _, tc := range resp.ToolCalls {
			ch <- StreamEvent{
				Type:     "tool_call",
				ToolCall: &tc,
			}
		}

		// Send done
		ch <- StreamEvent{Type: "done"}
	}()

	return ch, nil
}
