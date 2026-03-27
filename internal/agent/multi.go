package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Young-us/ycode/internal/llm"
	"github.com/Young-us/ycode/internal/tools"
)

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

// MultiAgent manages multiple agents working on tasks in parallel
type MultiAgent struct {
	Orchestrator *Orchestrator
	MaxAgents    int
	tasks        []*Task
	mu           sync.Mutex
}

// NewMultiAgent creates a new multi-agent manager
func NewMultiAgent(client llm.Client, toolManager *tools.Manager, maxAgents, maxSteps int) *MultiAgent {
	return &MultiAgent{
		Orchestrator: NewOrchestrator(client, toolManager, maxSteps),
		MaxAgents:    maxAgents,
		tasks:        make([]*Task, 0),
	}
}

// AddTask adds a task to the queue
func (m *MultiAgent) AddTask(task *Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks = append(m.tasks, task)
}

// AddTaskWithAgent adds a task with a specific agent type
func (m *MultiAgent) AddTaskWithAgent(id, description string, agentType AgentType, priority int) {
	m.AddTask(&Task{
		ID:          id,
		Description: description,
		AgentType:   agentType,
		Priority:    priority,
		Status:      "pending",
	})
}

// Run executes all tasks using multiple agents in parallel
func (m *MultiAgent) Run(ctx context.Context) error {
	m.mu.Lock()
	tasks := make([]*Task, len(m.tasks))
	copy(tasks, m.tasks)
	m.mu.Unlock()

	// Create channels for task distribution
	taskChan := make(chan *Task, len(tasks))
	resultChan := make(chan *Task, len(tasks))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < m.MaxAgents; i++ {
		wg.Add(1)
		go m.worker(ctx, i, taskChan, resultChan, &wg)
	}

	// Send tasks to workers (sort by priority)
	for _, task := range tasks {
		taskChan <- task
	}
	close(taskChan)

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results
	for task := range resultChan {
		m.mu.Lock()
		// Update task status
		for _, t := range m.tasks {
			if t.ID == task.ID {
				t.Status = task.Status
				t.Result = task.Result
				t.Error = task.Error
				break
			}
		}
		m.mu.Unlock()
	}

	return nil
}

func (m *MultiAgent) worker(ctx context.Context, id int, tasks <-chan *Task, results chan<- *Task, wg *sync.WaitGroup) {
	defer wg.Done()

	for task := range tasks {
		// Update task status
		task.Status = "running"

		// Determine agent type if not specified
		agentType := task.AgentType
		if agentType == "" || agentType == AgentDefault {
			agentType = m.Orchestrator.DetectAgentFromInput(task.Description)
		}

		// Get agent info
		agentInfo, err := m.Orchestrator.GetAgentInfo(agentType)
		if err != nil {
			task.Status = "failed"
			task.Error = err
			results <- task
			continue
		}

		// Execute task with the appropriate agent
		var result strings.Builder
		err = agentInfo.Loop.Run(ctx, task.Description, func(event llm.StreamEvent) {
			if event.Type == "content" {
				result.WriteString(event.Content)
			}
		})

		// Update task result
		if err != nil {
			task.Status = "failed"
			task.Error = err
		} else {
			task.Status = "completed"
			task.Result = result.String()
		}

		// Send result
		results <- task

		// Clear history for next task
		agentInfo.Loop.ClearHistory()
	}
}

// RunParallel executes multiple independent tasks in parallel
func (m *MultiAgent) RunParallel(ctx context.Context, taskDescriptions []string) map[string]*Task {
	m.mu.Lock()
	m.tasks = make([]*Task, 0)
	for i, desc := range taskDescriptions {
		m.tasks = append(m.tasks, &Task{
			ID:          fmt.Sprintf("task-%d", i),
			Description: desc,
			Status:      "pending",
		})
	}
	m.mu.Unlock()

	_ = m.Run(ctx)

	// Return results
	m.mu.Lock()
	defer m.mu.Unlock()
	results := make(map[string]*Task)
	for _, task := range m.tasks {
		results[task.ID] = task
	}
	return results
}

// RunSequence executes tasks in sequence, passing context from one to the next
func (m *MultiAgent) RunSequence(ctx context.Context, tasks []*Task) error {
	for _, task := range tasks {
		task.Status = "running"

		// Determine agent type
		agentType := task.AgentType
		if agentType == "" || agentType == AgentDefault {
			agentType = m.Orchestrator.DetectAgentFromInput(task.Description)
		}

		// Get agent and execute
		agentInfo, err := m.Orchestrator.GetAgentInfo(agentType)
		if err != nil {
			task.Status = "failed"
			task.Error = err
			return err
		}

		var result strings.Builder
		err = agentInfo.Loop.Run(ctx, task.Description, func(event llm.StreamEvent) {
			if event.Type == "content" {
				result.WriteString(event.Content)
			}
		})

		if err != nil {
			task.Status = "failed"
			task.Error = err
			return err
		}

		task.Status = "completed"
		task.Result = result.String()
	}

	return nil
}

// GetTaskStatus returns the status of a task
func (m *MultiAgent) GetTaskStatus(id string) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, task := range m.tasks {
		if task.ID == id {
			return task, nil
		}
	}

	return nil, fmt.Errorf("task not found: %s", id)
}

// GetCompletedTasks returns all completed tasks
func (m *MultiAgent) GetCompletedTasks() []*Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	var completed []*Task
	for _, task := range m.tasks {
		if task.Status == "completed" {
			completed = append(completed, task)
		}
	}

	return completed
}

// GetPendingTasks returns all pending tasks
func (m *MultiAgent) GetPendingTasks() []*Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	var pending []*Task
	for _, task := range m.tasks {
		if task.Status == "pending" {
			pending = append(pending, task)
		}
	}

	return pending
}

// ClearTasks clears all tasks
func (m *MultiAgent) ClearTasks() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks = make([]*Task, 0)
}