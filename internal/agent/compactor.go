package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Young-us/ycode/internal/llm"
	"github.com/Young-us/ycode/internal/logger"
)

// CompactionResult holds the result of a compaction operation
type CompactionResult struct {
	Summary       string        // Generated summary
	KeptMessages  []llm.Message // Messages kept after summary
	RemovedCount  int           // Number of messages removed
	SavedTokens   int           // Estimated tokens saved
	SummaryTokens int           // Estimated tokens in summary
}

// CompactionConfig holds configuration for auto-compaction
type CompactionConfig struct {
	Threshold     float64 // Threshold ratio (0.0-1.0) to trigger compaction
	KeepRecent    int     // Number of recent messages to keep
	MaxTokens     int     // Maximum token budget
	AutoCompact   bool    // Enable auto-compaction
	ShowIndicator bool    // Show compaction indicator in UI
}

// DefaultCompactionConfig returns default compaction configuration
func DefaultCompactionConfig() *CompactionConfig {
	return &CompactionConfig{
		Threshold:     0.75,  // Compact at 75% of max tokens
		KeepRecent:    6,     // Keep last 6 messages
		MaxTokens:     128000, // Default context window
		AutoCompact:   true,
		ShowIndicator: true,
	}
}

// Compactor handles conversation compaction/summarization
type Compactor struct {
	client   llm.Client
	config   *CompactionConfig
	// Real token tracking from API
	realInputTokens  int
	realOutputTokens int
}

// NewCompactor creates a new compactor
func NewCompactor(client llm.Client) *Compactor {
	return &Compactor{
		client: client,
		config: DefaultCompactionConfig(),
	}
}

// SetConfig sets the compaction configuration
func (c *Compactor) SetConfig(config *CompactionConfig) {
	c.config = config
}

// UpdateTokenUsage updates real token usage from API
func (c *Compactor) UpdateTokenUsage(inputTokens, outputTokens int) {
	c.realInputTokens = inputTokens
	c.realOutputTokens = outputTokens
}

// ShouldCompact checks if the conversation should be compacted
func (c *Compactor) ShouldCompact(messages []llm.Message, threshold float64, maxTokens int) bool {
	estimatedTokens := c.EstimateTokens(messages)
	usageRatio := float64(estimatedTokens) / float64(maxTokens)
	logger.Debug("compactor", "ShouldCompact: tokens=%d, ratio=%.2f, threshold=%.2f", estimatedTokens, usageRatio, threshold)
	return usageRatio >= threshold
}

// GetRealTokenUsage returns the real token usage from API (if available)
func (c *Compactor) GetRealTokenUsage() (input, output int) {
	return c.realInputTokens, c.realOutputTokens
}

// ShouldCompactWithRealTokens checks using real API token counts
func (c *Compactor) ShouldCompactWithRealTokens(maxTokens int) bool {
	if c.realInputTokens == 0 {
		return false // No real token data yet
	}
	usageRatio := float64(c.realInputTokens) / float64(maxTokens)
	logger.Debug("compactor", "ShouldCompactWithRealTokens: input=%d, ratio=%.2f", c.realInputTokens, usageRatio)
	return usageRatio >= c.config.Threshold
}

// EstimateTokens estimates the token count for messages
func (c *Compactor) EstimateTokens(messages []llm.Message) int {
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)
	}
	return totalChars / 4 // Rough approximation: 1 token ≈ 4 characters
}

// Compact creates a summary of the conversation, keeping recent messages
// keepRecent: number of recent messages to keep (not summarized)
func (c *Compactor) Compact(ctx context.Context, messages []llm.Message, keepRecent int) (*CompactionResult, error) {
	if len(messages) <= keepRecent {
		return &CompactionResult{
			KeptMessages: messages,
			RemovedCount: 0,
			SavedTokens:  0,
		}, nil
	}

	// Split messages to summarize vs keep
	toSummarize := messages[:len(messages)-keepRecent]
	keepMessages := messages[len(messages)-keepRecent:]

	// Build summary prompt
	summaryPrompt := c.buildSummaryPrompt(toSummarize)

	// Call LLM to generate summary
	response, err := c.client.Chat(ctx, []llm.Message{
		{Role: "user", Content: summaryPrompt},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	// Calculate token savings
	oldTokens := c.EstimateTokens(toSummarize)
	summaryTokens := len(response.Content) / 4

	result := &CompactionResult{
		Summary:       response.Content,
		KeptMessages:  keepMessages,
		RemovedCount:  len(toSummarize),
		SavedTokens:   oldTokens - summaryTokens,
		SummaryTokens: summaryTokens,
	}

	return result, nil
}

// CompactToHistory creates a compacted history with summary + recent messages
func (c *Compactor) CompactToHistory(ctx context.Context, messages []llm.Message, keepRecent int) ([]llm.Message, error) {
	result, err := c.Compact(ctx, messages, keepRecent)
	if err != nil {
		return messages, err
	}

	// Build new history: summary message + recent messages
	var newHistory []llm.Message

	// Add summary as system message
	if result.Summary != "" {
		newHistory = append(newHistory, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("[对话摘要]\n%s", result.Summary),
		})
	}

	// Add kept recent messages
	newHistory = append(newHistory, result.KeptMessages...)

	return newHistory, nil
}

func (c *Compactor) buildSummaryPrompt(messages []llm.Message) string {
	var conversation strings.Builder
	conversation.WriteString("请简洁地总结以下对话内容，保留关键信息、决策和上下文，以便继续讨论。使用中文回复。\n\n")
	conversation.WriteString("格式要求:\n")
	conversation.WriteString("1. 主要话题和目标\n")
	conversation.WriteString("2. 已完成的工作\n")
	conversation.WriteString("3. 重要的决策和结论\n")
	conversation.WriteString("4. 待处理的事项\n\n")
	conversation.WriteString("--- 对话内容 ---\n\n")

	for _, msg := range messages {
		role := msg.Role
		if role == "user" {
			role = "用户"
		} else if role == "assistant" {
			role = "助手"
		} else if role == "system" {
			continue // Skip system messages in summary
		}

		// Truncate long messages
		content := msg.Content
		if len(content) > 800 {
			content = content[:800] + "..."
		}

		conversation.WriteString(fmt.Sprintf("%s: %s\n\n", role, content))
	}

	return conversation.String()
}

// CompactIfNeeded automatically compacts if needed
func (c *Compactor) CompactIfNeeded(ctx context.Context, messages []llm.Message, threshold float64, maxTokens int, keepRecent int) ([]llm.Message, error) {
	if !c.ShouldCompact(messages, threshold, maxTokens) {
		return messages, nil
	}

	return c.CompactToHistory(ctx, messages, keepRecent)
}

// QuickSummary generates a quick summary without LLM call (for fast compaction)
func (c *Compactor) QuickSummary(messages []llm.Message) string {
	if len(messages) == 0 {
		return ""
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("对话包含 %d 条消息。", len(messages)))

	// Count by role
	userCount := 0
	assistantCount := 0
	for _, msg := range messages {
		if msg.Role == "user" {
			userCount++
		} else if msg.Role == "assistant" {
			assistantCount++
		}
	}

	summary.WriteString(fmt.Sprintf("用户 %d 次，助手 %d 次。", userCount, assistantCount))

	// Extract first user message as topic hint
	for _, msg := range messages {
		if msg.Role == "user" {
			content := msg.Content
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			summary.WriteString(fmt.Sprintf(" 首次话题: %s", content))
			break
		}
	}

	return summary.String()
}

// SmartCompact intelligently compacts history based on token budget
func (c *Compactor) SmartCompact(ctx context.Context, messages []llm.Message, maxTokens int, keepRecent int) ([]llm.Message, error) {
	currentTokens := c.EstimateTokens(messages)

	// If under budget, no compaction needed
	if currentTokens <= maxTokens {
		return messages, nil
	}

	// Calculate how many messages we need to compact
	// Target: get under maxTokens * 0.7 to leave room for new messages
	targetTokens := int(float64(maxTokens) * 0.7)

	// Try compacting with different keepRecent values if needed
	for keep := keepRecent; keep >= 2; keep-- {
		result, err := c.Compact(ctx, messages, keep)
		if err != nil {
			continue
		}

		newTokens := result.SummaryTokens + c.EstimateTokens(result.KeptMessages)
		if newTokens <= targetTokens {
			// Build and return compacted history
			var newHistory []llm.Message
			if result.Summary != "" {
				newHistory = append(newHistory, llm.Message{
					Role:    "system",
					Content: fmt.Sprintf("[对话摘要]\n%s", result.Summary),
				})
			}
			newHistory = append(newHistory, result.KeptMessages...)
			return newHistory, nil
		}
	}

	// Fallback: just keep recent messages
	if len(messages) > keepRecent {
		return messages[len(messages)-keepRecent:], nil
	}

	return messages, nil
}

// GetCompactionStats returns statistics about potential compaction
func (c *Compactor) GetCompactionStats(messages []llm.Message, keepRecent int) map[string]interface{} {
	totalTokens := c.EstimateTokens(messages)

	var toSummarizeTokens int
	var keptTokens int

	if len(messages) > keepRecent {
		toSummarizeTokens = c.EstimateTokens(messages[:len(messages)-keepRecent])
		keptTokens = c.EstimateTokens(messages[len(messages)-keepRecent:])
	} else {
		keptTokens = totalTokens
	}

	return map[string]interface{}{
		"total_messages":   len(messages),
		"total_tokens":     totalTokens,
		"to_summarize":     len(messages) - keepRecent,
		"to_summarize_tokens": toSummarizeTokens,
		"kept_messages":    min(keepRecent, len(messages)),
		"kept_tokens":      keptTokens,
		"estimated_savings": toSummarizeTokens - (toSummarizeTokens / 10), // Rough estimate: summary is ~10% of original
	}
}

// SessionSummary represents a stored summary for a session
type SessionSummary struct {
	CreatedAt   time.Time `json:"created_at"`
	MessageCount int      `json:"message_count"`
	TokenCount  int      `json:"token_count"`
	Summary     string   `json:"summary"`
}
