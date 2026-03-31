package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Young-us/ycode/internal/llm"
	"github.com/Young-us/ycode/internal/logger"
)

// analyzeTaskPrompt is the prompt for semantic analysis
const analyzeTaskPrompt = `Analyze the user's request and determine:
1. Whether this needs multiple agents to complete
2. If multiple agents needed, identify task dependencies
3. Assign appropriate agent to each task

Available agents:
- explorer: Finding, searching, locating code/files
- planner: Planning, breaking down tasks
- architect: Architecture design, refactoring
- coder: Writing/implementing code
- debugger: Fixing bugs, debugging
- tester: Writing tests
- reviewer: Code review
- writer: Documentation
- default: General purpose, use when unsure

User request: %s

Respond ONLY with valid JSON format:

For single agent tasks:
{
  "agent": "<agent_type>",
  "confidence": 0.9,
  "tasks": []
}

For independent parallel tasks:
{
  "agent": "orchestrator",
  "confidence": 0.9,
  "parallel": true,
  "tasks": [
    {"id": "1", "agent": "<agent_type>", "task": "<task_description>", "depends_on": []},
    {"id": "2", "agent": "<agent_type>", "task": "<task_description>", "depends_on": []}
  ]
}

For dependent sequential tasks:
{
  "agent": "orchestrator",
  "confidence": 0.9,
  "parallel": false,
  "tasks": [
    {"id": "1", "agent": "<agent_type>", "task": "<task_description>", "depends_on": []},
    {"id": "2", "agent": "<agent_type>", "task": "<task_description>", "depends_on": ["1"]}
  ]
}

IMPORTANT: Always return a valid agent type. Use "default" if unsure about the best agent.`

// IntentClassifier classifies user intent and determines which agent(s) to use
type IntentClassifier struct {
	llmClient llm.Client
	cache     map[string]*ClassifyResult
}

// ClassifyResult contains the result of intent classification
type ClassifyResult struct {
	Agent           AgentType
	Confidence      float64
	ShouldParallel  bool       // Whether tasks can be executed in parallel (no dependencies)
	HasDependencies bool       // Whether tasks have dependencies
	Tasks           []TaskInfo // Task list with dependency info
}

// TaskInfo contains information about a sub-task
type TaskInfo struct {
	ID          string     // Task ID
	Agent       AgentType  // Agent to execute this task
	Description string     // Task description
	DependsOn   []string   // IDs of tasks this depends on
}

// TaskResult contains the result of a task execution
type TaskResult struct {
	ID          string
	Description string
	Content     string
	Error       error
}

// NewIntentClassifier creates a new IntentClassifier
func NewIntentClassifier(client llm.Client) *IntentClassifier {
	return &IntentClassifier{
		llmClient: client,
		cache:     make(map[string]*ClassifyResult),
	}
}

// AnalyzeIntent performs semantic analysis to determine agent(s) and task structure
func (c *IntentClassifier) AnalyzeIntent(ctx context.Context, input string) *ClassifyResult {
	// Check cache
	if result, ok := c.cache[input]; ok {
		logger.Debug("classifier", "Using cached classification result")
		return result
	}

	// Build prompt
	prompt := fmt.Sprintf(analyzeTaskPrompt, input)
	messages := []llm.Message{{Role: "user", Content: prompt}}

	// Use Chat API (background execution, fast return)
	// Increased timeout to 30s for LLM API calls
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	response, err := c.llmClient.Chat(ctx, messages, nil)
	if err != nil {
		logger.Warn("classifier", "LLM classification failed: %v", err)
		// Return default result with fallback agent based on keyword detection
		return &ClassifyResult{Agent: AgentDefault, Confidence: 0}
	}

	// Parse result
	result := c.parseClassifyResult(response.Content)

	// Detect if there are dependencies
	if len(result.Tasks) > 0 {
		for _, task := range result.Tasks {
			if len(task.DependsOn) > 0 {
				result.HasDependencies = true
				result.ShouldParallel = false
				break
			}
		}
	}

	// Cache result
	c.cache[input] = result
	logger.Debug("classifier", "Classified intent: agent=%s, tasks=%d, parallel=%v, deps=%v",
		result.Agent, len(result.Tasks), result.ShouldParallel, result.HasDependencies)

	return result
}

// parseClassifyResult parses the LLM response into a ClassifyResult
func (c *IntentClassifier) parseClassifyResult(content string) *ClassifyResult {
	content = extractJSON(content)

	var data struct {
		Agent      string `json:"agent"`
		Confidence float64 `json:"confidence"`
		Parallel   bool   `json:"parallel"`
		Tasks      []struct {
			ID       string   `json:"id"`
			Agent    string   `json:"agent"`
			Task     string   `json:"task"`
			DependsOn []string `json:"depends_on"`
		} `json:"tasks"`
	}

	if err := json.Unmarshal([]byte(content), &data); err != nil {
		logger.Warn("classifier", "Failed to parse classification result: %v, content: %s", err, content)
		return &ClassifyResult{Agent: AgentDefault, Confidence: 0}
	}

	tasks := make([]TaskInfo, len(data.Tasks))
	for i, t := range data.Tasks {
		tasks[i] = TaskInfo{
			ID:          t.ID,
			Agent:       AgentType(t.Agent),
			Description: t.Task,
			DependsOn:   t.DependsOn,
		}
	}

	// Validate agent type - default to AgentDefault if invalid
	agent := AgentType(data.Agent)
	if agent == "" || agent == "none" {
		agent = AgentDefault
	}

	return &ClassifyResult{
		Agent:          agent,
		Confidence:     data.Confidence,
		ShouldParallel: data.Parallel,
		Tasks:          tasks,
	}
}

// extractJSON extracts JSON from text content
func extractJSON(content string) string {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start != -1 && end != -1 && end > start {
		return content[start : end+1]
	}
	return content
}