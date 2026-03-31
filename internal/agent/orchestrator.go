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
	AgentExplorer  AgentType = "explorer"
	AgentPlanner   AgentType = "planner"
	AgentArchitect AgentType = "architect"
	AgentCoder     AgentType = "coder"
	AgentDebugger  AgentType = "debugger"
	AgentTester    AgentType = "tester"
	AgentReviewer  AgentType = "reviewer"
	AgentWriter    AgentType = "writer"
	AgentPlanMode  AgentType = "plan_mode" // Plan mode agent with read-only tools
	AgentDefault   AgentType = "default"
)

// AgentEventType represents the type of agent event
type AgentEventType string

const (
	AgentEventSwitch   AgentEventType = "switch"   // Agent switched
	AgentEventStart    AgentEventType = "start"    // Agent started task
	AgentEventComplete AgentEventType = "complete" // Agent completed task
	AgentEventParallel AgentEventType = "parallel" // Parallel execution started
	AgentEventProgress AgentEventType = "progress" // Progress update for parallel tasks
)

// AgentEvent represents an event from the agent system
type AgentEvent struct {
	Type        AgentEventType
	AgentType   AgentType
	AgentName   string
	Description string
	TaskID      string // For parallel task tracking
	Progress    int    // Progress percentage (0-100)
	TotalTasks  int    // Total tasks in parallel execution
	TasksDone   int    // Completed tasks
	Error       error  // Error if any
}

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

// Task represents a task to be executed by an agent
type Task struct {
	ID          string
	Description string
	AgentType   AgentType // Which agent type should handle this task
	Priority    int
	Status      string // "pending", "running", "completed", "failed"
	Result      string
	Error       error
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

	// Callback for agent events (for UI notifications)
	agentCallback func(event AgentEvent)

	// Compactor for history compression
	compactor *Compactor

	// Plugin manager for hooks
	plugins PluginHookTrigger

	// Shared history (all agents use the same history)
	sharedHistory []llm.Message

	// Intent classifier for semantic analysis
	classifier *IntentClassifier

	// Last classification result
	lastClassifyResult *ClassifyResult
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(client llm.Client, toolManager *tools.Manager, maxSteps int) *Orchestrator {
	o := &Orchestrator{
		LLMClient:     client,
		ToolManager:   toolManager,
		agents:        make(map[AgentType]*AgentInfo),
		currentAgent:  AgentDefault,
		tasks:         make([]*Task, 0),
		maxSteps:      maxSteps,
		compactor:     NewCompactor(client),
		classifier:    NewIntentClassifier(client),
		sharedHistory: make([]llm.Message, 0),
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
			Keywords:    []string{"find", "search", "locate", "explore", "where", "lookup", "查找", "搜索", "寻找", "定位", "在哪", "找到"},
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
			Keywords:    []string{"plan", "design", "breakdown", "outline", "strategy", "计划", "规划", "设计", "分解"},
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
			Keywords:    []string{"architecture", "structure", "interface", "refactor", "redesign", "架构", "结构", "接口", "重构"},
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
			Keywords:    []string{"implement", "write", "create", "add", "build", "code", "实现", "编写", "创建", "添加", "代码"},
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
			Keywords:    []string{"debug", "fix", "bug", "error", "issue", "problem", "investigate", "调试", "修复", "问题", "错误", "bug"},
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
			Keywords:    []string{"test", "spec", "coverage", "verify", "assert", "测试", "验证", "覆盖率"},
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
			Keywords:    []string{"review", "check", "audit", "inspect", "审查", "检查", "审核"},
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
			Keywords:    []string{"document", "readme", "docs", "documentation", "comment", "文档", "说明", "注释"},
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

	// Plan mode agent (read-only tools for research and planning)
	planModePrompt := `You are a helpful assistant in plan mode. Your task is to analyze the user's request and provide helpful guidance.

WORKFLOW:
1. RESEARCH: Use read-only tools (read_file, glob, grep, web_search) to explore and understand the codebase
2. ANALYZE: Analyze the problem, understand existing code structure, identify what needs to change
3. RESPOND: Provide clear, actionable guidance in natural language

RESPONSE FORMAT:
- For research tasks (finding, searching, exploring): Provide findings directly with file paths and relevant information
- For implementation tasks: Explain what needs to be done, which files to modify, and provide code snippets if helpful
- For debugging tasks: Explain the root cause and suggest fixes

GUIDELINES:
- Be concise but thorough
- Include file paths with line numbers when referencing code (file.go:123)
- Use code blocks for code snippets
- Suggest concrete next steps when appropriate
- Always respond in the SAME LANGUAGE as the user's request

You have read-only access - use read_file, glob, grep, web_search to explore. Do not modify any files.`

	planModeLoop := NewLoopWithPermissions(o.LLMClient, o.ToolManager, o.maxSteps, ReadOnlyPermissions())
	planModeLoop.SetSystemPrompt(planModePrompt)
	o.agents[AgentPlanMode] = &AgentInfo{
		Type:         AgentPlanMode,
		Name:         "Plan Mode",
		Description:  "Research and propose solutions",
		Keywords:     []string{},
		Commands:     []string{},
		SystemPrompt: planModePrompt,
		Permissions:  ReadOnlyPermissions(),
		Loop:         planModeLoop,
	}
}

// SetCallback sets the streaming callback function
func (o *Orchestrator) SetCallback(callback func(event llm.StreamEvent)) {
	o.callback = callback
}

// SetAgentCallback sets the agent event callback for UI notifications
func (o *Orchestrator) SetAgentCallback(callback func(event AgentEvent)) {
	o.agentCallback = callback
}

// emitAgentEvent sends an agent event to the callback if set
func (o *Orchestrator) emitAgentEvent(event AgentEvent) {
	logger.Debug("agent", "[DEBUG] emitAgentEvent: type=%s, agentType=%s, name=%s, callback=%v", event.Type, event.AgentType, event.AgentName, o.agentCallback != nil)
	if o.agentCallback != nil {
		o.agentCallback(event)
	}
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

	oldAgent := o.currentAgent
	o.currentAgent = agentType
	logger.Info("agent", "Switched to agent: %s", agentType)

	// Emit agent switch event
	if oldAgent != agentType {
		info := o.agents[agentType]
		go o.emitAgentEvent(AgentEvent{
			Type:        AgentEventSwitch,
			AgentType:   agentType,
			AgentName:   info.Name,
			Description: info.Description,
		})
	}

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

// DetectAgentFromInput analyzes user input to determine the best agent using keywords
// This is now a fallback method when semantic analysis is not available
func (o *Orchestrator) DetectAgentFromInput(input string) AgentType {
	o.mu.RLock()
	defer o.mu.RUnlock()

	inputLower := strings.ToLower(input)

	// Score each agent based on keyword matches
	bestAgent := AgentDefault
	bestScore := 0

	for agentType, info := range o.agents {
		if agentType == AgentDefault {
			continue
		}

		score := 0
		for _, keyword := range info.Keywords {
			if strings.Contains(inputLower, keyword) {
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
	// 1. Semantic analysis (background execution, using Chat API)
	var classifyResult *ClassifyResult
	if explicitAgent == AgentDefault || explicitAgent == "" {
		// Use classifier for semantic analysis
		classifyResult = o.classifier.AnalyzeIntent(ctx, input)
		o.lastClassifyResult = classifyResult
		logger.Debug("agent", "Semantic analysis result: agent=%s, tasks=%d, parallel=%v",
			classifyResult.Agent, len(classifyResult.Tasks), classifyResult.ShouldParallel)
	}

	// 2. Determine which agent to use
	var detectedAgent AgentType
	if explicitAgent != AgentDefault && explicitAgent != "" {
		detectedAgent = explicitAgent
		logger.Info("agent", "Using explicit agent: %s", explicitAgent)
	} else if classifyResult != nil && len(classifyResult.Tasks) == 0 {
		detectedAgent = classifyResult.Agent
		// Validate agent type exists
		if _, exists := o.agents[detectedAgent]; !exists {
			logger.Debug("agent", "Invalid agent from classifier: %s, falling back to keyword detection", detectedAgent)
			detectedAgent = o.DetectAgentFromInput(input)
		}
	} else if classifyResult != nil && len(classifyResult.Tasks) > 0 {
		detectedAgent = AgentType("orchestrator")
	} else {
		// Fallback to keyword-based detection
		detectedAgent = o.DetectAgentFromInput(input)
	}

	// 3. Final validation - ensure detectedAgent is valid
	if _, exists := o.agents[detectedAgent]; !exists {
		logger.Warn("agent", "Invalid agent detected: %s, using default", detectedAgent)
		detectedAgent = AgentDefault
	}

	// 3. Check if multi-agent coordination is needed
	if classifyResult != nil && len(classifyResult.Tasks) > 0 {
		o.emitAgentEvent(AgentEvent{
			Type:       AgentEventParallel,
			AgentName:  "Orchestrator",
			TotalTasks: len(classifyResult.Tasks),
		})

		return o.runCoordinatedTasks(ctx, input, classifyResult)
	}

	// 4. Single agent execution
	o.mu.Lock()
	agent, exists := o.agents[detectedAgent]
	if !exists {
		o.mu.Unlock()
		return fmt.Errorf("unknown agent type: %s", detectedAgent)
	}
	oldAgent := o.currentAgent
	o.currentAgent = detectedAgent
	o.mu.Unlock()

	// Emit agent switch event if agent changed
	if oldAgent != detectedAgent {
		o.emitAgentEvent(AgentEvent{
			Type:        AgentEventSwitch,
			AgentType:   detectedAgent,
			AgentName:   agent.Name,
			Description: agent.Description,
		})
	}

	// Emit agent start event
	o.emitAgentEvent(AgentEvent{
		Type:        AgentEventStart,
		AgentType:   detectedAgent,
		AgentName:   agent.Name,
		Description: "Starting task execution",
	})

	// Execute with shared history
	err := o.runWithSharedHistory(ctx, agent, input)

	// Emit agent complete event
	o.emitAgentEvent(AgentEvent{
		Type:      AgentEventComplete,
		AgentType: detectedAgent,
		AgentName: agent.Name,
		Error:     err,
	})

	return err
}

// runWithSharedHistory executes a task using shared history
func (o *Orchestrator) runWithSharedHistory(ctx context.Context, agent *AgentInfo, input string) error {
	// Add user message to shared history
	o.mu.Lock()
	o.sharedHistory = append(o.sharedHistory, llm.Message{
		Role:    "user",
		Content: input,
	})
	historyCopy := make([]llm.Message, len(o.sharedHistory))
	copy(historyCopy, o.sharedHistory)
	o.mu.Unlock()

	// Set agent's history to shared history
	agent.Loop.SetHistory(historyCopy)

	// Execute task
	var responseContent strings.Builder
	err := agent.Loop.Run(ctx, input, func(event llm.StreamEvent) {
		// Forward stream events
		if o.callback != nil {
			o.callback(event)
		}

		// Collect response content
		if event.Type == "content" {
			responseContent.WriteString(event.Content)
		}
	})

	// Update shared history (add assistant response)
	if responseContent.Len() > 0 {
		o.mu.Lock()
		o.sharedHistory = append(o.sharedHistory, llm.Message{
			Role:    "assistant",
			Content: responseContent.String(),
		})
		o.mu.Unlock()
	}

	return err
}

// runCoordinatedTasks executes multiple tasks with dependency handling
func (o *Orchestrator) runCoordinatedTasks(ctx context.Context, originalInput string, classifyResult *ClassifyResult) error {
	tasks := classifyResult.Tasks
	results := make(map[string]*TaskResult)

	// Build task dependency map
	taskMap := make(map[string]TaskInfo)
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	// Execution order: topological sort based on dependencies
	// Tasks without dependencies execute in parallel, tasks with dependencies execute in order
	completed := make(map[string]bool)

	// Add original user input to shared history
	o.mu.Lock()
	o.sharedHistory = append(o.sharedHistory, llm.Message{
		Role:    "user",
		Content: originalInput,
	})
	o.mu.Unlock()

	for len(completed) < len(tasks) {
		// Find all tasks whose dependencies are satisfied
		var readyTasks []TaskInfo
		for _, t := range tasks {
			if completed[t.ID] {
				continue
			}

			allDepsDone := true
			for _, dep := range t.DependsOn {
				if !completed[dep] {
					allDepsDone = false
					break
				}
			}

			if allDepsDone {
				readyTasks = append(readyTasks, t)
			}
		}

		if len(readyTasks) == 0 {
			// No executable tasks, possible circular dependency
			return fmt.Errorf("circular dependency detected in tasks")
		}

		// Execute all ready tasks in parallel
		var wg sync.WaitGroup
		var mu sync.Mutex
		sem := make(chan struct{}, 3) // Max 3 parallel

		for _, task := range readyTasks {
			wg.Add(1)
			go func(t TaskInfo) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				// Get agent
				o.mu.RLock()
				agentInfo := o.agents[t.Agent]
				o.mu.RUnlock()

				if agentInfo == nil {
					agentInfo = o.agents[AgentDefault]
				}

				// Emit start event
				o.emitAgentEvent(AgentEvent{
					Type:        AgentEventStart,
					AgentType:   t.Agent,
					AgentName:   agentInfo.Name,
					TaskID:      t.ID,
					Description: t.Description,
				})

				// Build input (including previous task results)
				o.mu.RLock()
				historyCopy := make([]llm.Message, len(o.sharedHistory))
				copy(historyCopy, o.sharedHistory)
				o.mu.RUnlock()

				// Add previous task results as context
				var taskInput strings.Builder
				taskInput.WriteString(t.Description)
				for _, depID := range t.DependsOn {
					if r, ok := results[depID]; ok {
						taskInput.WriteString(fmt.Sprintf("\n\n[前置任务 %s 结果]:\n%s", depID, r.Content))
					}
				}

				agentInfo.Loop.SetHistory(historyCopy)

				var result strings.Builder
				err := agentInfo.Loop.Run(ctx, taskInput.String(), func(event llm.StreamEvent) {
					if event.Type == "content" {
						result.WriteString(event.Content)
					}
					// Forward stream events to main callback for UI updates
					if o.callback != nil {
						o.callback(event)
					}
				})

				// Save result
				mu.Lock()
				results[t.ID] = &TaskResult{
					ID:          t.ID,
					Description: t.Description,
					Content:     result.String(),
					Error:       err,
				}
				completed[t.ID] = true
				mu.Unlock()

				// Emit progress event
				o.emitAgentEvent(AgentEvent{
					Type:       AgentEventProgress,
					AgentType:  t.Agent,
					AgentName:  agentInfo.Name,
					TaskID:     t.ID,
					Progress:   len(completed) * 100 / len(tasks),
					TotalTasks: len(tasks),
					TasksDone:  len(completed),
				})

				// Emit complete event
				o.emitAgentEvent(AgentEvent{
					Type:      AgentEventComplete,
					AgentType: t.Agent,
					AgentName: agentInfo.Name,
					TaskID:    t.ID,
					Error:     err,
				})
			}(task)
		}

		wg.Wait()
	}

	// Aggregate all results and stream final response
	var allResults strings.Builder
	allResults.WriteString("## 任务执行完成\n\n")
	for _, t := range tasks {
		if r, ok := results[t.ID]; ok {
			allResults.WriteString(fmt.Sprintf("### %s\n%s\n\n", t.Description, r.Content))
		}
	}

	// Update shared history
	o.mu.Lock()
	o.sharedHistory = append(o.sharedHistory, llm.Message{
		Role:    "assistant",
		Content: allResults.String(),
	})
	o.mu.Unlock()

	// Stream aggregated result to user
	if o.callback != nil {
		o.callback(llm.StreamEvent{
			Type:    "content",
			Content: allResults.String(),
		})
	}

	return nil
}

// RunWithAgent executes a task with a specific agent type
func (o *Orchestrator) RunWithAgent(ctx context.Context, agentType AgentType, input string) error {
	return o.Run(ctx, input, agentType)
}

// GetHistory returns the shared conversation history
func (o *Orchestrator) GetHistory() []llm.Message {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.sharedHistory
}

// GetSharedHistory returns the shared conversation history
func (o *Orchestrator) GetSharedHistory() []llm.Message {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.sharedHistory
}

// ClearSharedHistory clears the shared conversation history
func (o *Orchestrator) ClearSharedHistory() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sharedHistory = make([]llm.Message, 0)
}

// SetSharedHistory sets the shared conversation history (for session restoration)
func (o *Orchestrator) SetSharedHistory(history []llm.Message) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sharedHistory = history
	logger.Debug("agent", "Shared history restored with %d messages", len(history))
}

// ClearHistory clears the conversation history of all agents
func (o *Orchestrator) ClearHistory() {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Clear shared history
	o.sharedHistory = make([]llm.Message, 0)

	// Clear each agent's history
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

	// Get agent loop tokens
	if agent, exists := o.agents[o.currentAgent]; exists {
		inputTokens = agent.Loop.TotalInputTokens
		outputTokens = agent.Loop.TotalOutputTokens
	}

	// Add plan mode tokens if active
	if planState != nil && planState.IsActive {
		inputTokens += planState.InputTokens
		outputTokens += planState.OutputTokens
	}

	return inputTokens, outputTokens
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

// RunParallel executes multiple independent tasks in parallel using multiple agents
func (o *Orchestrator) RunParallel(ctx context.Context, tasks []string) map[string]*Task {
	if len(tasks) == 0 {
		return nil
	}

	// Emit parallel execution start event
	o.emitAgentEvent(AgentEvent{
		Type:       AgentEventParallel,
		AgentName:  "Orchestrator",
		TotalTasks: len(tasks),
	})

	results := make(map[string]*Task)
	var resultsMu sync.Mutex

	// Create task objects
	taskObjs := make([]*Task, len(tasks))
	for i, desc := range tasks {
		taskObjs[i] = &Task{
			ID:          fmt.Sprintf("task-%d", i),
			Description: desc,
			Status:      "pending",
		}
	}

	// Worker function
	worker := func(task *Task) {
		// Detect agent type for this task
		agentType := o.DetectAgentFromInput(task.Description)
		agentInfo, err := o.GetAgentInfo(agentType)
		if err != nil {
			task.Status = "failed"
			task.Error = err
			resultsMu.Lock()
			results[task.ID] = task
			resultsMu.Unlock()
			return
		}

		// Emit agent start event
		o.emitAgentEvent(AgentEvent{
			Type:        AgentEventStart,
			AgentType:   agentType,
			AgentName:   agentInfo.Name,
			TaskID:      task.ID,
			Description: task.Description,
		})

		task.Status = "running"

		// Execute task
		var result strings.Builder
		err = agentInfo.Loop.Run(ctx, task.Description, func(event llm.StreamEvent) {
			if event.Type == "content" {
				result.WriteString(event.Content)
			}
			// Forward stream events to main callback for UI updates
			if o.callback != nil {
				o.callback(event)
			}
		})

		// Update result
		if err != nil {
			task.Status = "failed"
			task.Error = err
		} else {
			task.Status = "completed"
			task.Result = result.String()
		}

		// Store result
		resultsMu.Lock()
		results[task.ID] = task
		done := len(results)
		resultsMu.Unlock()

		// Emit progress event
		o.emitAgentEvent(AgentEvent{
			Type:       AgentEventProgress,
			AgentType:  agentType,
			AgentName:  agentInfo.Name,
			TaskID:     task.ID,
			Progress:   (done * 100) / len(tasks),
			TotalTasks: len(tasks),
			TasksDone:  done,
		})

		// Clear history for next task
		agentInfo.Loop.ClearHistory()
	}

	// Execute tasks in parallel (limit concurrency)
	maxConcurrency := 3
	if len(tasks) < maxConcurrency {
		maxConcurrency = len(tasks)
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	for _, task := range taskObjs {
		wg.Add(1)
		go func(t *Task) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			worker(t)
		}(task)
	}

	wg.Wait()

	return results
}

// AnalyzeComplexity analyzes the complexity of a task to determine if parallel execution is beneficial
func (o *Orchestrator) AnalyzeComplexity(input string) (complexity string, shouldParallelize bool) {
	input = strings.ToLower(input)

	// Indicators for complex tasks that could benefit from parallelization
	parallelIndicators := []string{
		"multiple", "several", "all", "each", "every",
		"parallel", "simultaneous", "concurrent",
		"and", "also", "additionally", "furthermore",
	}

	// Indicators for simple tasks
	simpleIndicators := []string{
		"show", "display", "list", "print",
		"what", "how", "why", "explain",
		"read", "get", "fetch",
	}

	parallelScore := 0
	simpleScore := 0

	for _, indicator := range parallelIndicators {
		if strings.Contains(input, indicator) {
			parallelScore++
		}
	}

	for _, indicator := range simpleIndicators {
		if strings.Contains(input, indicator) {
			simpleScore++
		}
	}

	// Determine complexity
	if parallelScore >= 2 {
		return "high", true
	} else if parallelScore >= 1 {
		return "medium", simpleScore < 2
	} else if simpleScore >= 2 {
		return "low", false
	}

	return "medium", false
}

// GetCurrentAgentInfo returns the current agent info for UI display
func (o *Orchestrator) GetCurrentAgentInfo() *AgentInfo {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if agent, exists := o.agents[o.currentAgent]; exists {
		return agent
	}
	return nil
}

// Plan mode methods

// planState holds the current plan mode state
var planState *PlanModeState

// UpdatePlan updates the plan based on user feedback with streaming
func (o *Orchestrator) UpdatePlan(ctx context.Context, plan *PlanResult, feedback string, callback func(event llm.StreamEvent)) (*PlanResult, error) {
	updatePrompt := `Update your guidance based on user feedback.

Previous guidance:
%s

User feedback: %s

Please provide updated guidance that addresses the user's concerns.`

	messages := []llm.Message{
		{Role: "system", Content: "You are a helpful assistant. Update your guidance based on user feedback. IMPORTANT: Always respond in the SAME LANGUAGE as the user's request."},
		{Role: "user", Content: fmt.Sprintf(updatePrompt, plan.Content, feedback)},
	}

	// Use streaming API for real-time feedback
	stream, err := o.LLMClient.Stream(ctx, messages, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to update plan: %w", err)
	}

	var fullContent strings.Builder
	var thinkingContent strings.Builder
	var usage *llm.Usage

	for event := range stream {
		switch event.Type {
		case "content":
			fullContent.WriteString(event.Content)
			if callback != nil {
				callback(llm.StreamEvent{Type: "content", Content: event.Content})
			}
		case "thinking":
			thinkingContent.WriteString(event.Content)
			if callback != nil {
				callback(llm.StreamEvent{Type: "thinking", Content: event.Content})
			}
		case "usage":
			usage = event.Usage
		case "done":
			break
		}
	}

	// Track token usage for plan update
	if usage != nil && planState != nil {
		planState.InputTokens += usage.InputTokens
		planState.OutputTokens += usage.OutputTokens
	}

	updatedPlan := parsePlanResult(fullContent.String())
	updatedPlan.UserInput = plan.UserInput
	updatedPlan.ID = plan.ID
	updatedPlan.Iteration = plan.Iteration + 1

	return updatedPlan, nil
}

// RunPlanMode runs the plan mode workflow
func (o *Orchestrator) RunPlanMode(ctx context.Context, input string, callback func(event llm.StreamEvent)) error {
	// Use PlanMode agent to research and respond
	agent, exists := o.agents[AgentPlanMode]
	if !exists {
		return fmt.Errorf("plan mode agent not found")
	}

	// Run the agent loop with callback - AI responds freely
	err := agent.Loop.Run(ctx, input, callback)
	if err != nil {
		return err
	}

	// Extract plan from agent's last response
	history := agent.Loop.GetHistory()
	if len(history) == 0 {
		return fmt.Errorf("no response from plan agent")
	}

	// Find the last assistant message
	var lastAssistantMsg string
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" {
			lastAssistantMsg = history[i].Content
			break
		}
	}

	// Create plan from the response
	plan := parsePlanResult(lastAssistantMsg)
	plan.UserInput = input
	plan.ID = generatePlanID()
	plan.Iteration = 1

	planState = &PlanModeState{
		CurrentPlan: plan,
		IsActive:    true,
		FeedbackCh:  make(chan PlanInteraction, 10),
	}

	// Show confirmation prompt
	if callback != nil {
		callback(llm.StreamEvent{
			Type:    "content",
			Content: "\n\n---\n按 'y' 确认执行, 'n' 取消, 'm' 修改方案.\n",
		})
		callback(llm.StreamEvent{
			Type: "plan_generated",
		})
	}

	// Also emit via emitPlanEvent for internal tracking
	o.emitPlanEvent(PlanInteraction{
		Type:    "plan_generated",
		Plan:    plan,
		Message: "Please review the execution plan",
	})

	// 3. Wait for user feedback loop
	for planState.IsActive {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case interaction := <-planState.FeedbackCh:
			switch interaction.Action {
			case "confirm":
				// User confirmed, execute plan
				planState.IsConfirmed = true

				// Notify UI that plan is being executed
				if callback != nil {
					callback(llm.StreamEvent{
						Type:    "content",
						Content: "\n✅ Plan confirmed. Executing...\n\n",
					})
				}

				o.emitPlanEvent(PlanInteraction{
					Type:    "plan_confirmed",
					Message: "Executing the plan...",
				})

				// Execute with selected agent based on semantic analysis
				return o.executePlanWithAgent(ctx, plan, callback)

			case "modify":
				// User requested modification - update plan with streaming
				updatedPlan, err := o.UpdatePlan(ctx, plan, interaction.Message, callback)
				if err != nil {
					// Notify UI of error
					if callback != nil {
						callback(llm.StreamEvent{
							Type:    "content",
							Content: fmt.Sprintf("\n❌ Failed to update: %v\n\n", err),
						})
					}
					o.emitPlanEvent(PlanInteraction{
						Type:    "plan_error",
						Message: fmt.Sprintf("Failed to update: %v", err),
					})
					continue
				}
				plan = updatedPlan
				planState.CurrentPlan = plan

				// Show confirmation prompt
				if callback != nil {
					callback(llm.StreamEvent{
						Type:    "content",
						Content: "\n\n---\n按 'y' 确认执行, 'n' 取消, 'm' 修改方案.\n",
					})
					callback(llm.StreamEvent{
						Type: "plan_generated",
					})
				}

				o.emitPlanEvent(PlanInteraction{
					Type:    "plan_updated",
					Plan:    plan,
					Message: "Plan updated. Please review.",
				})

			case "cancel":
				// User cancelled
				planState.IsActive = false

				// Notify UI
				if callback != nil {
					callback(llm.StreamEvent{
						Type:    "content",
						Content: "\n❌ Plan cancelled.\n\n",
					})
				}

				o.emitPlanEvent(PlanInteraction{
					Type:    "plan_cancelled",
					Message: "Plan cancelled by user",
				})
				return nil
			}
		}
	}

	return nil
}

// executePlanWithAgent executes the plan with the appropriate agent
func (o *Orchestrator) executePlanWithAgent(ctx context.Context, plan *PlanResult, callback func(event llm.StreamEvent)) error {
	// Perform semantic analysis to select execution agent
	classifyResult := o.classifier.AnalyzeIntent(ctx, plan.UserInput)
	o.lastClassifyResult = classifyResult

	// Get the agent
	var agentType AgentType
	if classifyResult != nil && classifyResult.Agent != AgentDefault {
		agentType = classifyResult.Agent
	} else {
		agentType = AgentCoder // Default to Coder for execution
	}

	o.mu.Lock()
	agent, exists := o.agents[agentType]
	o.currentAgent = agentType
	o.mu.Unlock()

	if !exists {
		agent = o.agents[AgentDefault]
	}

	// Emit agent switch event
	o.emitAgentEvent(AgentEvent{
		Type:        AgentEventSwitch,
		AgentType:   agentType,
		AgentName:   agent.Name,
		Description: agent.Description,
	})

	// Build execution context from plan
	var execContext strings.Builder
	execContext.WriteString(fmt.Sprintf("Original user request: %s\n\n", plan.UserInput))
	execContext.WriteString("Execute the following guidance:\n\n")
	execContext.WriteString(plan.Content)
	execContext.WriteString("\n\nIMPORTANT: Respond in the same language as the original user request.\n")

	// Execute using the agent
	return o.runWithSharedHistory(ctx, agent, execContext.String())
}

// SubmitPlanFeedback submits feedback for the current plan
func (o *Orchestrator) SubmitPlanFeedback(action, message string) {
	if planState != nil && planState.IsActive {
		planState.FeedbackCh <- PlanInteraction{
			Action:  action,
			Message: message,
		}
	}
}

// GetPlanState returns the current plan state
func (o *Orchestrator) GetPlanState() *PlanModeState {
	return planState
}

// emitPlanEvent emits a plan interaction event
func (o *Orchestrator) emitPlanEvent(interaction PlanInteraction) {
	if o.callback != nil {
		// Send plan event with the interaction type (plan_updated, plan_error, etc.)
		o.callback(llm.StreamEvent{
			Type:    interaction.Type, // Use the actual interaction type
			Content: interaction.Message,
		})
		// Also send the full plan if available
		if interaction.Plan != nil {
			o.callback(llm.StreamEvent{
				Type: "plan_generated", // Re-use plan_generated to show new plan
			})
		}
	}
}

// parsePlanResult converts AI response content into a PlanResult
func parsePlanResult(content string) *PlanResult {
	return &PlanResult{
		Content: strings.TrimSpace(content),
	}
}
