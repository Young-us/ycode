package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Young-us/ycode/internal/llm"
	"github.com/Young-us/ycode/internal/logger"
	"github.com/Young-us/ycode/internal/tools"
)

// PluginHookTrigger is an interface for triggering plugin hooks
type PluginHookTrigger interface {
	Trigger(ctx context.Context, hookName string, args map[string]interface{}) (map[string]interface{}, error)
	Enabled() bool
}

// Loop represents the TAOR (Think-Act-Observe-Repeat) agent loop
type Loop struct {
	LLMClient   llm.Client
	ToolManager *tools.Manager
	MaxSteps    int
	History     []llm.Message
	Permissions *AgentPermissions
	plugins     PluginHookTrigger
	// Agent-specific system prompt
	systemPrompt string
	// Token tracking
	TotalInputTokens  int
	TotalOutputTokens int
	// Partial content for interruption recovery
	PartialContent  string
	PartialThinking string
}

// NewLoop creates a new agent loop
func NewLoop(client llm.Client, toolManager *tools.Manager, maxSteps int) *Loop {
	return &Loop{
		LLMClient:   client,
		ToolManager: toolManager,
		MaxSteps:    maxSteps,
		History:     []llm.Message{},
		Permissions: DefaultPermissions("normal"),
	}
}

// NewLoopWithPermissions creates a new agent loop with specific permissions
func NewLoopWithPermissions(client llm.Client, toolManager *tools.Manager, maxSteps int, permissions *AgentPermissions) *Loop {
	return &Loop{
		LLMClient:   client,
		ToolManager: toolManager,
		MaxSteps:    maxSteps,
		History:     []llm.Message{},
		Permissions: permissions,
	}
}

// SetPluginManager sets the plugin manager for hook triggering
func (l *Loop) SetPluginManager(plugins PluginHookTrigger) {
	l.plugins = plugins
}

// SetSystemPrompt sets the agent-specific system prompt
func (l *Loop) SetSystemPrompt(prompt string) {
	l.systemPrompt = prompt
}

// GetSystemPrompt returns the current system prompt
func (l *Loop) GetSystemPrompt() string {
	return l.systemPrompt
}

// getToolDefinitions returns all tool definitions
func (l *Loop) getToolDefinitions() []tools.ToolDefinition {
	return l.ToolManager.Definitions()
}

// buildMessagesWithSystemPrompt builds the message list with system prompt prepended
func (l *Loop) buildMessagesWithSystemPrompt() []llm.Message {
	if l.systemPrompt == "" {
		return l.History
	}

	// Check if we already have a system message at the start
	if len(l.History) > 0 && l.History[0].Role == "system" {
		return l.History
	}

	// Prepend system prompt as a system message
	messages := make([]llm.Message, 0, len(l.History)+1)
	messages = append(messages, llm.Message{
		Role:    "system",
		Content: l.systemPrompt,
	})
	messages = append(messages, l.History...)
	return messages
}

// Run executes the agent loop for a user input with streaming support
func (l *Loop) Run(ctx context.Context, userInput string, callback func(event llm.StreamEvent)) error {
	logger.Info("agent", "Starting agent loop with input (length=%d)", len(userInput))

	// Trigger on_chat_start hook
	if l.plugins != nil && l.plugins.Enabled() {
		l.plugins.Trigger(ctx, "on_chat_start", map[string]interface{}{
			"input":   userInput,
			"history": len(l.History),
		})
	}

	// Add user message to history
	l.History = append(l.History, llm.Message{
		Role:    "user",
		Content: userInput,
	})

	// Get tool definitions
	toolDefs := l.getToolDefinitions()
	logger.Debug("agent", "Loaded %d tool definitions", len(toolDefs))

	// TAOR loop - no artificial step limit
	// LLM decides when to complete, user can interrupt with ESC
	step := 0
	for {
		step++
		logger.Info("agent", "Starting step %d", step)

		// Check context cancellation (ESC interrupt)
		if ctx.Err() != nil {
			logger.Warn("agent", "Context cancelled at step %d", step)
			return ctx.Err()
		}

		// Build messages with system prompt prepended if set
		messages := l.buildMessagesWithSystemPrompt()

		// Think: Send to LLM with streaming
		var response *llm.Response
		var err error

		// Try streaming first
		if streamer, ok := l.LLMClient.(interface {
			Stream(ctx context.Context, messages []llm.Message, toolDefs []tools.ToolDefinition) (<-chan llm.StreamEvent, error)
		}); ok && callback != nil {
			logger.Debug("agent", "Starting LLM stream request")
			streamCh, streamErr := streamer.Stream(ctx, messages, toolDefs)
			if streamErr == nil {
				// Collect streamed content
				var content strings.Builder
				var thinking strings.Builder
				var toolCalls []llm.ToolCall
				var streamError error

				for event := range streamCh {
					if ctx.Err() != nil {
						// Save partial content for recovery
						l.PartialContent = content.String()
						l.PartialThinking = thinking.String()
						logger.Warn("agent", "Stream interrupted by context cancellation, saved partial content (len=%d)", len(l.PartialContent))
						return ctx.Err()
					}

					switch event.Type {
					case "content":
						content.WriteString(event.Content)
						// Forward to callback for UI updates
						callback(event)
					case "thinking":
						// Extended thinking - accumulate and forward to UI
						thinking.WriteString(event.Content)
						callback(event)
					case "tool_call":
						if event.ToolCall != nil {
							toolCalls = append(toolCalls, *event.ToolCall)
							logger.Info("agent", "Tool call received: %s", event.ToolCall.Name)
							// Forward tool call event
							callback(event)
						}
					case "usage":
						// Track token usage from API
						// Note: input_tokens includes the entire conversation history
						// We track the last request's usage, not accumulate
						if event.Usage != nil {
							// Only update if there's actual data (message_start has input, message_delta has output)
							if event.Usage.InputTokens > 0 {
								l.TotalInputTokens = event.Usage.InputTokens
							}
							if event.Usage.OutputTokens > 0 {
								l.TotalOutputTokens += event.Usage.OutputTokens
							}
							logger.Debug("agent", "Token usage: input=%d, output=%d, total_input=%d, total_output=%d",
								event.Usage.InputTokens, event.Usage.OutputTokens,
								l.TotalInputTokens, l.TotalOutputTokens)
						}
					case "done":
						// Stream complete
						logger.Debug("agent", "Stream completed successfully")
					case "error":
						streamError = fmt.Errorf("stream error: %s", event.Content)
						logger.Error("agent", "Stream error: %s", event.Content)
					}
				}

				if streamError != nil {
					// Trigger on_error hook
					if l.plugins != nil && l.plugins.Enabled() {
						l.plugins.Trigger(ctx, "on_error", map[string]interface{}{
							"error":    streamError.Error(),
							"context":  "stream",
							"severity": "error",
						})
					}
					return streamError
				}

				response = &llm.Response{
					Content:   content.String(),
					Thinking:  thinking.String(),
					ToolCalls: toolCalls,
				}
				logger.Debug("agent", "Response: content_len=%d, thinking_len=%d, tool_calls=%d",
					len(response.Content), len(response.Thinking), len(response.ToolCalls))
			} else {
				// Fall back to non-streaming
				logger.Warn("agent", "Stream failed, falling back to non-streaming: %v", streamErr)
				response, err = l.LLMClient.Chat(ctx, messages, toolDefs)
				if err != nil {
					logger.Error("agent", "LLM chat failed: %v", err)
					// Trigger on_error hook
					if l.plugins != nil && l.plugins.Enabled() {
						l.plugins.Trigger(ctx, "on_error", map[string]interface{}{
							"error":    err.Error(),
							"context":  "llm_chat",
							"severity": "error",
						})
					}
					return fmt.Errorf("LLM error: %w", err)
				}
			}
		} else {
			// Non-streaming fallback
			logger.Debug("agent", "Using non-streaming LLM request")
			response, err = l.LLMClient.Chat(ctx, messages, toolDefs)
			if err != nil {
				logger.Error("agent", "LLM chat failed: %v", err)
				// Trigger on_error hook
				if l.plugins != nil && l.plugins.Enabled() {
					l.plugins.Trigger(ctx, "on_error", map[string]interface{}{
						"error":    err.Error(),
						"context":  "llm_chat",
						"severity": "error",
					})
				}
				return fmt.Errorf("LLM error: %w", err)
			}

			// Callback for non-streaming response
			if callback != nil && response.Content != "" {
				callback(llm.StreamEvent{
					Type:    "content",
					Content: response.Content,
				})
			}
			// Callback for thinking if present
			if callback != nil && response.Thinking != "" {
				callback(llm.StreamEvent{
					Type:    "thinking",
					Content: response.Thinking,
				})
			}
		}

		// Add assistant message to history
		l.History = append(l.History, llm.Message{
			Role:     "assistant",
			Content:  response.Content,
			Thinking: response.Thinking,
		})

		// If no tool calls, we're done
		if len(response.ToolCalls) == 0 {
			logger.Info("agent", "Agent loop completed successfully in %d steps", step+1)
			// Trigger on_chat_complete hook
			if l.plugins != nil && l.plugins.Enabled() {
				l.plugins.Trigger(ctx, "on_chat_complete", map[string]interface{}{
					"response": response.Content,
					"steps":    step + 1,
				})
			}
			return nil
		}

		// Act: Execute tool calls
		// Check if we can run tools in parallel (all read-only tools)
		canParallelize := true
		readOnlyTools := map[string]bool{
			"read_file":   true,
			"glob":        true,
			"grep":        true,
			"git_status":  true,
			"git_log":     true,
			"git_diff":    true,
			"git_show":    true,
			"git_branch":  true,
			"lsp_hover":   true,
			"lsp_definition": true,
			"lsp_references": true,
			"lsp_symbols": true,
		}

		for _, toolCall := range response.ToolCalls {
			if !readOnlyTools[toolCall.Name] {
				canParallelize = false
				break
			}
		}

		if canParallelize && len(response.ToolCalls) > 1 {
			// Execute tools in parallel
			logger.Info("agent", "Executing %d tools in parallel", len(response.ToolCalls))

			var wg sync.WaitGroup
			var mu sync.Mutex
			results := make([]struct {
				toolCall llm.ToolCall
				result   *tools.ToolResult
				err      error
			}, len(response.ToolCalls))

			for i, toolCall := range response.ToolCalls {
				wg.Add(1)
				go func(idx int, tc llm.ToolCall) {
					defer wg.Done()

					logger.Info("agent", "Executing tool: %s with %d arguments", tc.Name, len(tc.Arguments))

					// Callback for tool call
					if callback != nil {
						callback(llm.StreamEvent{
							Type:     "tool_call",
							ToolCall: &tc,
						})
					}

					result, err := l.ToolManager.Execute(ctx, tc.Name, tc.Arguments)
					if err != nil {
						logger.Error("agent", "Tool execution failed: %s - %v", tc.Name, err)
					} else {
						logger.Info("agent", "Tool %s completed: result_len=%d, is_error=%v", tc.Name, len(result.Content))
					}

					mu.Lock()
					results[idx] = struct {
						toolCall llm.ToolCall
						result   *tools.ToolResult
						err      error
					}{toolCall: tc, result: result, err: err}
					mu.Unlock()
				}(i, toolCall)
			}

			wg.Wait()

			// Process results
			for _, r := range results {
				if r.err != nil {
					// Trigger on_error hook
					if l.plugins != nil && l.plugins.Enabled() {
						l.plugins.Trigger(ctx, "on_error", map[string]interface{}{
							"error":    r.err.Error(),
							"context":  "tool_execute",
							"tool":     r.toolCall.Name,
							"severity": "error",
						})
					}
					return fmt.Errorf("tool execution error: %w", r.err)
				}

				// Observe: Add tool result to history
				resultContent := r.result.Content
				if r.result.IsError {
					resultContent = "Error: " + resultContent
				}

				l.History = append(l.History, llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("[Tool: %s]\n%s", r.toolCall.Name, resultContent),
				})
			}
		} else {
			// Execute tools sequentially
			for _, toolCall := range response.ToolCalls {
				logger.Info("agent", "Executing tool: %s with %d arguments", toolCall.Name, len(toolCall.Arguments))

				// Callback for tool call
				if callback != nil {
					callback(llm.StreamEvent{
						Type:     "tool_call",
						ToolCall: &toolCall,
					})
				}

				// Execute tool
				result, err := l.ToolManager.Execute(ctx, toolCall.Name, toolCall.Arguments)
				if err != nil {
					logger.Error("agent", "Tool execution failed: %s - %v", toolCall.Name, err)
					// Trigger on_error hook
					if l.plugins != nil && l.plugins.Enabled() {
						l.plugins.Trigger(ctx, "on_error", map[string]interface{}{
							"error":    err.Error(),
							"context":  "tool_execute",
							"tool":     toolCall.Name,
							"severity": "error",
						})
					}
					return fmt.Errorf("tool execution error: %w", err)
				}

				logger.Info("agent", "Tool %s completed: result_len=%d, is_error=%v", toolCall.Name, len(result.Content))

				// Observe: Add tool result to history
				resultContent := result.Content
				if result.IsError {
					resultContent = "Error: " + resultContent
				}

				l.History = append(l.History, llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("[Tool: %s]\n%s", toolCall.Name, resultContent),
				})
			}
		}

		// Repeat: Continue loop
	}

	// This should never be reached - loop exits via:
	// 1. LLM returns response without tool calls (normal completion)
	// 2. Context cancellation (user ESC interrupt)
	// 3. Error during execution
	return nil
}

// ClearHistory clears the conversation history
func (l *Loop) ClearHistory() {
	l.History = []llm.Message{}
	logger.Debug("agent", "History cleared")
}

// GetHistory returns the conversation history
func (l *Loop) GetHistory() []llm.Message {
	return l.History
}

// SetHistory sets the conversation history (for session restoration)
func (l *Loop) SetHistory(history []llm.Message) {
	l.History = history
	logger.Debug("agent", "History restored with %d messages", len(history))
}

// GetPartialContent returns the partial content from an interrupted stream
func (l *Loop) GetPartialContent() (content string, thinking string) {
	return l.PartialContent, l.PartialThinking
}

// ClearPartialContent clears the partial content
func (l *Loop) ClearPartialContent() {
	l.PartialContent = ""
	l.PartialThinking = ""
}
