# Multi-Agent System

ycode uses a multi-agent system with specialized agents for different tasks.

## Agent Types

| Agent | Role | Capabilities |
|-------|------|--------------|
| **Explorer** | Codebase exploration | Read-only tools, search, navigation |
| **Planner** | Task planning | Read-only, plan generation |
| **Architect** | System design | Interface design, architecture |
| **Coder** | Code implementation | Full write capabilities |
| **Debugger** | Bug fixing | Analysis, debugging, fixing |
| **Tester** | Test writing | Test creation and verification |
| **Reviewer** | Code review | Read-only, quality analysis |
| **Writer** | Documentation | Markdown, documentation |

## Intent Classification

The `IntentClassifier` (`internal/agent/classifier.go`) analyzes user requests to:

1. Determine if a request needs single or multiple agents
2. Identify task dependencies for coordinated execution
3. Support both independent parallel tasks and dependent sequential tasks
4. Cache classification results for efficiency

## Orchestration

The orchestrator (`internal/agent/orchestrator.go`) manages:

- Agent selection based on task type
- Task delegation and coordination
- Result aggregation
- Error handling and recovery

## Workflow

```
User Request
    ↓
Intent Classification
    ↓
Agent Selection
    ↓
Task Execution
    ↓
Result Aggregation
    ↓
Response
```

## Plan Mode

The orchestrator supports a plan mode workflow (`internal/agent/workflow.go`):

1. **GeneratePlan**: LLM creates execution plan with:
   - Task steps
   - Required files
   - Commands to run
   - Risk assessment

2. **User Review**: User can:
   - Approve plan
   - Modify steps
   - Reject plan

3. **ExecutePlan**: Executes approved steps with appropriate agent

## Configuration

Enable multi-agent in configuration:

```yaml
agent:
  max_steps: 10
  multi_agent:
    enabled: true
    max_agents: 5
```

## Examples

### Single Agent Task
```
User: "Read the main.go file and explain it"
→ Agent: Explorer
```

### Multi-Agent Task
```
User: "Fix the bug in auth.go and add tests"
→ Agents: Debugger (fix) → Tester (tests)
```

### Complex Task
```
User: "Design a new API endpoint and implement it"
→ Agents: Architect (design) → Coder (implement) → Tester (tests)
```

## Related

- [Architecture](architecture.md) - System design
- [Tools](tools.md) - Available tools
- [Development](development.md) - Contributing