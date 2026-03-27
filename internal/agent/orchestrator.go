package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Young-us/ycode/internal/llm"
	"github.com/Young-us/ycode/internal/logger"
	"github.com/Young-us/ycode/internal/tools"
)

// AgentType represents the type of agent
type AgentType string

const (
	AgentExplorer   AgentType = "explorer"
	AgentPlanner    AgentType = "planner"
	AgentArchitect  AgentType = "architect"
	AgentCoder      AgentType = "coder"
	AgentDebugger   AgentType = "debugger"
	AgentTester     AgentType = "tester"
	AgentReviewer   AgentType = "reviewer"
	AgentWriter     AgentType = "writer"
	AgentDefault    AgentType = "default"
)

// AgentInfo contains information about an agent
type AgentInfo struct {
	Type         AgentType
	Name         string
	Description  string
	Keywords     []string // Keywords that trigger this agent
	Commands     []string // Slash commands to invoke this agent
	SystemPrompt string   // System prompt that guides the agent's behavior
	Permissions  *AgentPermissions
	Loop         *Loop
}

// Orchestrator manages multiple agents and routes tasks to appropriate agents
type Orchestrator struct {
	LLMClient   llm.Client
	ToolManager *tools.Manager

	mu           sync.RWMutex
	agents       map[AgentType]*AgentInfo
	currentAgent AgentType
	tasks        []*Task
	maxSteps     int

	// Callback for streaming events
	callback func(event llm.StreamEvent)

	// Compactor for history compression
	compactor *Compactor

	// Plugin manager for hooks
	plugins PluginHookTrigger
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(client llm.Client, toolManager *tools.Manager, maxSteps int) *Orchestrator {
	o := &Orchestrator{
		LLMClient:   client,
		ToolManager: toolManager,
		agents:      make(map[AgentType]*AgentInfo),
		currentAgent: AgentDefault,
		tasks:       make([]*Task, 0),
		maxSteps:    maxSteps,
		compactor:   NewCompactor(client),
	}

	// Register all agent types
	o.registerAgents()

	return o
}

// SetPluginManager sets the plugin manager for all agents
func (o *Orchestrator) SetPluginManager(plugins PluginHookTrigger) {
	o.plugins = plugins
	// Propagate to all agent loops
	for _, agent := range o.agents {
		agent.Loop.SetPluginManager(plugins)
	}
}

// SetProjectContext sets the project context (from CLAUDE.md) for all agents
// This context is prepended to each agent's system prompt
func (o *Orchestrator) SetProjectContext(context string) {
	if context == "" {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	// Update each agent's system prompt to include project context
	for _, agent := range o.agents {
		currentPrompt := agent.Loop.GetSystemPrompt()
		// Prepend project context if not already present
		if !strings.HasPrefix(currentPrompt, "# Project Context") {
			enhancedPrompt := fmt.Sprintf("# Project Context\n\n%s\n\n---\n\n%s", context, currentPrompt)
			agent.Loop.SetSystemPrompt(enhancedPrompt)
		}
	}
}

// RetryCallback is the callback function type for retry events
type RetryCallback func(attempt int, reason string, delay time.Duration)

// SetRetryCallback sets the retry callback on the LLM client
// The callback is called when a retry occurs, allowing UI notification
func (o *Orchestrator) SetRetryCallback(callback RetryCallback) {
	// The LLMClient interface doesn't have SetRetryCallback, but we can
	// check if the underlying client implements it via type assertion
	// This is handled in app.go where we have access to the concrete type
}

// GetRetryStatus returns the retry status from the LLM client
// Returns nil if the client doesn't support retry status
func (o *Orchestrator) GetRetryStatus() *llm.RetryStatus {
	// Type assert to check if client has RetryStatus
	if rs, ok := o.LLMClient.(interface{ GetRetryStatus() *llm.RetryStatus }); ok {
		return rs.GetRetryStatus()
	}
	return nil
}

// registerAgents registers all available agent types
func (o *Orchestrator) registerAgents() {
	agentDefinitions := []struct {
		Type         AgentType
		Name         string
		Description  string
		Keywords     []string
		Commands     []string
		SystemPrompt string
	}{
		{
			Type:        AgentExplorer,
			Name:        "Explorer",
			Description: "Explores and searches the codebase",
			Keywords:    []string{"find", "search", "locate", "explore", "where", "lookup"},
			Commands:    []string{"/explorer", "/explore"},
			SystemPrompt: `You are an Explorer agent specialized in codebase navigation and discovery.

YOUR PRIMARY MISSION:
- Search and locate files, functions, classes, variables, and patterns
- Navigate and understand project structure
- Find relevant code snippets and dependencies
- Report findings clearly with file paths and line numbers

YOUR APPROACH:
1. Start with broad searches, then narrow down
2. Use glob to find files by pattern, grep for content search
3. Read relevant files to understand context
4. Provide clear, organized summaries of what you find

CONSTRAINTS:
- You are READ-ONLY: do NOT modify any code or files
- Focus on discovery and analysis, not implementation
- When you find something, report it precisely (file:line format)
- If you cannot find something, say so clearly

OUTPUT STYLE:
- Use clear headings and bullet points
- Include file paths with line numbers
- Summarize findings before diving into details`,
		},
		{
			Type:        AgentPlanner,
			Name:        "Planner",
			Description: "Plans and breaks down tasks into steps",
			Keywords:    []string{"plan", "design", "breakdown", "outline", "strategy"},
			Commands:    []string{"/planner", "/plan"},
			SystemPrompt: `You are a Planner agent specialized in task breakdown and strategic thinking.

YOUR PRIMARY MISSION:
- Analyze complex tasks and break them into manageable steps
- Create clear implementation plans with dependencies
- Identify potential risks and edge cases
- Define success criteria for each step

YOUR APPROACH:
1. Understand the full scope of the request
2. Identify prerequisites and dependencies
3. Break down into logical, sequential steps
4. Estimate complexity for each step
5. Consider alternative approaches

CONSTRAINTS:
- You are READ-ONLY: analyze and plan, do NOT implement
- You may write documentation files (*.md, *.txt) for plans
- Focus on clarity and completeness
- Do NOT write or modify code files

OUTPUT STYLE:
- Use numbered steps with clear dependencies
- Include acceptance criteria for each step
- Identify risks and mitigation strategies
- Provide estimated effort (small/medium/large)`,
		},
		{
			Type:        AgentArchitect,
			Name:        "Architect",
			Description: "Designs architecture and interfaces",
			Keywords:    []string{"architecture", "structure", "interface", "refactor", "redesign"},
			Commands:    []string{"/architect", "/arch"},
			SystemPrompt: `You are an Architect agent specialized in system design and structure.

YOUR PRIMARY MISSION:
- Design clean, maintainable system architecture
- Define interfaces, modules, and their relationships
- Propose refactoring strategies for improved structure
- Balance trade-offs between flexibility, performance, and simplicity

YOUR APPROACH:
1. Understand current architecture and constraints
2. Identify pain points and improvement opportunities
3. Design with SOLID principles in mind
4. Consider scalability, testability, and maintainability
5. Document decisions and trade-offs

CONSTRAINTS:
- You may write interface definitions and documentation
- You may write *.md, *.yaml, *.json for architecture docs
- You may write interface*.go or *_interface.go files
- Do NOT implement concrete business logic

OUTPUT STYLE:
- Use diagrams (ASCII or mermaid) for structure
- Define clear interfaces with method signatures
- Explain design decisions and trade-offs
- Provide migration paths for refactoring`,
		},
		{
			Type:        AgentCoder,
			Name:        "Coder",
			Description: "Writes and modifies code",
			Keywords:    []string{"implement", "write", "create", "add", "build", "code"},
			Commands:    []string{"/coder", "/code"},
			SystemPrompt: `You are a Coder agent specialized in writing clean, working code.

YOUR PRIMARY MISSION:
- Implement features and functionality
- Write clean, idiomatic, well-documented code
- Follow project conventions and patterns
- Ensure code is testable and maintainable

YOUR APPROACH:
1. Understand requirements fully before coding
2. Check existing patterns in the codebase
3. Write code incrementally with clear commits
4. Handle edge cases and errors properly
5. Add appropriate logging and comments

CONSTRAINTS:
- You CAN read, write, and edit code files
- You CAN run bash commands for building/testing
- Follow existing code style in the project
- Do NOT modify sensitive files (*.env, *.key, *.secret)

OUTPUT STYLE:
- Write complete, working code (not snippets)
- Include error handling and edge cases
- Add comments for complex logic
- Follow the project's naming conventions`,
		},
		{
			Type:        AgentDebugger,
			Name:        "Debugger",
			Description: "Analyzes and fixes bugs",
			Keywords:    []string{"debug", "fix", "bug", "error", "issue", "problem", "investigate"},
			Commands:    []string{"/debugger", "/debug"},
			SystemPrompt: `You are a Debugger agent specialized in finding and fixing issues.

YOUR PRIMARY MISSION:
- Analyze errors, bugs, and unexpected behavior
- Find root causes through systematic investigation
- Propose and implement fixes
- Prevent similar issues in the future

YOUR APPROACH:
1. Reproduce and understand the issue
2. Gather relevant logs, stack traces, and context
3. Formulate hypotheses about root cause
4. Test hypotheses systematically
5. Implement minimal, targeted fixes
6. Verify the fix resolves the issue

CONSTRAINTS:
- You CAN read code and run diagnostic commands
- You CAN run tests, build commands, and diagnostic tools
- You CAN modify code to fix issues
- Focus on root cause, not just symptoms

DEBUGGING CHECKLIST:
- [ ] Can I reproduce the issue?
- [ ] What changed recently?
- [ ] Are there error messages or logs?
- [ ] What are the assumptions?
- [ ] Can I isolate the problematic component?

OUTPUT STYLE:
- State the problem clearly
- Show your investigation process
- Explain the root cause
- Provide the fix with explanation
- Suggest preventive measures`,
		},
		{
			Type:        AgentTester,
			Name:        "Tester",
			Description: "Writes and runs tests",
			Keywords:    []string{"test", "spec", "coverage", "verify", "assert"},
			Commands:    []string{"/tester", "/test"},
			SystemPrompt: `You are a Tester agent specialized in quality assurance and testing.

YOUR PRIMARY MISSION:
- Write comprehensive tests (unit, integration, e2e)
- Run tests and analyze results
- Improve test coverage
- Ensure code correctness and reliability

YOUR APPROACH:
1. Understand what needs to be tested
2. Identify test cases (happy path, edge cases, error cases)
3. Write clear, focused tests
4. Run tests and verify results
5. Report failures with clear information

CONSTRAINTS:
- You CAN write test files (*_test.go, *_test.py, *_test.js, etc.)
- You CAN run test commands (go test, npm test, pytest, etc.)
- You CAN write mock files for testing
- Do NOT modify production code (only test files)

TESTING PRINCIPLES:
- Test behavior, not implementation
- One assertion per test when possible
- Use descriptive test names
- Arrange-Act-Assert pattern
- Test edge cases and error conditions

OUTPUT STYLE:
- Clear test descriptions
- Comprehensive coverage report
- Specific failure information
- Suggestions for improving tests`,
		},
		{
			Type:        AgentReviewer,
			Name:        "Reviewer",
			Description: "Reviews code for quality and issues",
			Keywords:    []string{"review", "check", "audit", "inspect"},
			Commands:    []string{"/reviewer", "/review"},
			SystemPrompt: `You are a Reviewer agent specialized in code quality and best practices.

YOUR PRIMARY MISSION:
- Review code for correctness, quality, and maintainability
- Identify potential bugs, security issues, and anti-patterns
- Suggest improvements following best practices
- Ensure code follows project conventions

YOUR APPROACH:
1. Understand the purpose of the code
2. Check for correctness and edge cases
3. Evaluate code quality and readability
4. Identify security and performance concerns
5. Provide actionable feedback

CONSTRAINTS:
- You are READ-ONLY: do NOT modify any code
- You may only read files for review
- You may NOT run any bash commands
- Focus on providing feedback, not implementing changes

REVIEW CHECKLIST:
- [ ] Does the code do what it's supposed to?
- [ ] Are there potential bugs or edge cases?
- [ ] Is the code readable and well-organized?
- [ ] Are there security concerns?
- [ ] Does it follow project conventions?
- [ ] Are errors handled properly?
- [ ] Is there adequate test coverage?

OUTPUT STYLE:
- Use severity levels (Critical/High/Medium/Low)
- Be specific with file:line references
- Explain WHY something is an issue
- Suggest concrete improvements
- Acknowledge good practices found`,
		},
		{
			Type:        AgentWriter,
			Name:        "Writer",
			Description: "Writes documentation",
			Keywords:    []string{"document", "readme", "docs", "documentation", "comment"},
			Commands:    []string{"/writer", "/doc"},
			SystemPrompt: `You are a Writer agent specialized in documentation and communication.

YOUR PRIMARY MISSION:
- Write clear, comprehensive documentation
- Create and update README files
- Document APIs, functions, and modules
- Write user guides and tutorials

YOUR APPROACH:
1. Understand the audience (developers, users, etc.)
2. Organize information logically
3. Use clear, concise language
4. Include examples and code snippets
5. Keep documentation up-to-date

CONSTRAINTS:
- You CAN write documentation files (*.md, *.txt, *.rst, etc.)
- You CAN write README, CHANGELOG, CONTRIBUTING files
- You are READ-ONLY for code files (do NOT modify)
- You may NOT run bash commands

DOCUMENTATION TYPES:
- README: Project overview and quick start
- API docs: Function/method documentation
- Guides: Step-by-step tutorials
- CHANGELOG: Version history
- CONTRIBUTING: Contribution guidelines

OUTPUT STYLE:
- Use proper markdown formatting
- Include code examples with syntax highlighting
- Use headings for structure
- Add table of contents for long documents
- Keep paragraphs focused and concise`,
		},
	}

	for _, def := range agentDefinitions {
		permissions := DefaultPermissions(string(def.Type))
		loop := NewLoopWithPermissions(o.LLMClient, o.ToolManager, o.maxSteps, permissions)
		loop.SetSystemPrompt(def.SystemPrompt)
		o.agents[def.Type] = &AgentInfo{
			Type:         def.Type,
			Name:         def.Name,
			Description:  def.Description,
			Keywords:     def.Keywords,
			Commands:     def.Commands,
			SystemPrompt: def.SystemPrompt,
			Permissions:  permissions,
			Loop:         loop,
		}
	}

	// Default agent (general purpose, no specific system prompt)
	o.agents[AgentDefault] = &AgentInfo{
		Type:         AgentDefault,
		Name:         "Default",
		Description:  "General purpose agent",
		Keywords:     []string{},
		Commands:     []string{},
		SystemPrompt: "",
		Permissions:  DefaultPermissions("normal"),
		Loop:         NewLoop(o.LLMClient, o.ToolManager, o.maxSteps),
	}
}

// SetCallback sets the streaming callback function
func (o *Orchestrator) SetCallback(callback func(event llm.StreamEvent)) {
	o.callback = callback
}

// GetCurrentAgent returns the current agent type
func (o *Orchestrator) GetCurrentAgent() AgentType {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.currentAgent
}

// SetCurrentAgent sets the current agent type
func (o *Orchestrator) SetCurrentAgent(agentType AgentType) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.agents[agentType]; !exists {
		logger.Error("agent", "Unknown agent type: %s", agentType)
		return fmt.Errorf("unknown agent type: %s", agentType)
	}

	o.currentAgent = agentType
	logger.Info("agent", "Switched to agent: %s", agentType)
	return nil
}

// GetAgentInfo returns information about an agent type
func (o *Orchestrator) GetAgentInfo(agentType AgentType) (*AgentInfo, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	info, exists := o.agents[agentType]
	if !exists {
		return nil, fmt.Errorf("unknown agent type: %s", agentType)
	}

	return info, nil
}

// ListAgents returns all available agent types
func (o *Orchestrator) ListAgents() []*AgentInfo {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make([]*AgentInfo, 0, len(o.agents))
	for _, info := range o.agents {
		result = append(result, info)
	}

	return result
}

// DetectAgentFromCommand detects agent type from a slash command
func (o *Orchestrator) DetectAgentFromCommand(command string) (AgentType, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	command = strings.ToLower(command)
	for agentType, info := range o.agents {
		for _, cmd := range info.Commands {
			if strings.ToLower(cmd) == command {
				return agentType, true
			}
		}
	}

	return AgentDefault, false
}

// DetectAgentFromInput analyzes user input to determine the best agent
func (o *Orchestrator) DetectAgentFromInput(input string) AgentType {
	o.mu.RLock()
	defer o.mu.RUnlock()

	input = strings.ToLower(input)

	// Score each agent based on keyword matches
	bestAgent := AgentDefault
	bestScore := 0

	for agentType, info := range o.agents {
		if agentType == AgentDefault {
			continue
		}

		score := 0
		for _, keyword := range info.Keywords {
			if strings.Contains(input, keyword) {
				score++
			}
		}

		if score > bestScore {
			bestScore = score
			bestAgent = agentType
		}
	}

	if bestAgent != AgentDefault {
		logger.Debug("agent", "Detected agent %s from input (score=%d)", bestAgent, bestScore)
	}

	return bestAgent
}

// Run executes a task using the appropriate agent
func (o *Orchestrator) Run(ctx context.Context, input string, explicitAgent AgentType) error {
	// Detect agent BEFORE acquiring lock to avoid deadlock with DetectAgentFromInput
	var detectedAgent AgentType
	if explicitAgent != AgentDefault && explicitAgent != "" {
		detectedAgent = explicitAgent
		logger.Info("agent", "Using explicit agent: %s", explicitAgent)
	} else {
		detectedAgent = o.DetectAgentFromInput(input)
	}

	o.mu.Lock()
	var agent *AgentInfo
	var exists bool
	agent, exists = o.agents[detectedAgent]
	if !exists {
		o.mu.Unlock()
		return fmt.Errorf("unknown agent type: %s", detectedAgent)
	}
	o.currentAgent = detectedAgent
	o.mu.Unlock()

	// Run the agent loop
	return agent.Loop.Run(ctx, input, o.callback)
}

// RunWithAgent executes a task with a specific agent type
func (o *Orchestrator) RunWithAgent(ctx context.Context, agentType AgentType, input string) error {
	return o.Run(ctx, input, agentType)
}

// GetHistory returns the conversation history of the current agent
func (o *Orchestrator) GetHistory() []llm.Message {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if agent, exists := o.agents[o.currentAgent]; exists {
		return agent.Loop.GetHistory()
	}
	return nil
}

// ClearHistory clears the conversation history of all agents
func (o *Orchestrator) ClearHistory() {
	o.mu.Lock()
	defer o.mu.Unlock()

	for _, agent := range o.agents {
		agent.Loop.ClearHistory()
	}
}

// CompactHistory compacts the conversation history to save tokens
func (o *Orchestrator) CompactHistory(ctx context.Context, keepRecent int) (*CompactionResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if agent, exists := o.agents[o.currentAgent]; exists {
		history := agent.Loop.GetHistory()
		result, err := o.compactor.Compact(ctx, history, keepRecent)
		if err != nil {
			return nil, err
		}

		// Build new history with summary
		var newHistory []llm.Message
		if result.Summary != "" {
			newHistory = append(newHistory, llm.Message{
				Role:    "system",
				Content: fmt.Sprintf("[对话摘要]\n%s", result.Summary),
			})
		}
		newHistory = append(newHistory, result.KeptMessages...)

		// Set compacted history
		agent.Loop.SetHistory(newHistory)

		return result, nil
	}

	return nil, fmt.Errorf("no current agent")
}

// ShouldCompact checks if history should be compacted
func (o *Orchestrator) ShouldCompact(threshold float64, maxTokens int) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if agent, exists := o.agents[o.currentAgent]; exists {
		history := agent.Loop.GetHistory()
		return o.compactor.ShouldCompact(history, threshold, maxTokens)
	}
	return false
}

// GetHistoryStats returns statistics about current history
func (o *Orchestrator) GetHistoryStats(keepRecent int) map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if agent, exists := o.agents[o.currentAgent]; exists {
		history := agent.Loop.GetHistory()
		return o.compactor.GetCompactionStats(history, keepRecent)
	}
	return nil
}

// GetCompactor returns the compactor instance
func (o *Orchestrator) GetCompactor() *Compactor {
	return o.compactor
}

// GetAgentDescription returns a description of what each agent does
func GetAgentDescription(agentType AgentType) string {
	descriptions := map[AgentType]string{
		AgentExplorer:  "Searches and explores the codebase to find files, functions, or patterns",
		AgentPlanner:   "Breaks down complex tasks into manageable steps and creates implementation plans",
		AgentArchitect: "Designs system architecture, interfaces, and module structures",
		AgentCoder:     "Implements features, writes code, and makes modifications",
		AgentDebugger:  "Investigates issues, analyzes bugs, and finds root causes",
		AgentTester:    "Writes tests, verifies functionality, and ensures code coverage",
		AgentReviewer:  "Reviews code for quality, security, and best practices",
		AgentWriter:    "Creates and updates documentation, READMEs, and comments",
		AgentDefault:   "General purpose agent that can handle various tasks",
	}

	if desc, ok := descriptions[agentType]; ok {
		return desc
	}
	return "Unknown agent type"
}

// GetTokenUsage returns the total token usage from the current agent's loop
func (o *Orchestrator) GetTokenUsage() (inputTokens, outputTokens int) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if agent, exists := o.agents[o.currentAgent]; exists {
		return agent.Loop.TotalInputTokens, agent.Loop.TotalOutputTokens
	}
	return 0, 0
}

// GetPartialContent returns the partial content from an interrupted stream
func (o *Orchestrator) GetPartialContent() (content string, thinking string) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if agent, exists := o.agents[o.currentAgent]; exists {
		return agent.Loop.GetPartialContent()
	}
	return "", ""
}

// ClearPartialContent clears the partial content
func (o *Orchestrator) ClearPartialContent() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if agent, exists := o.agents[o.currentAgent]; exists {
		agent.Loop.ClearPartialContent()
	}
}