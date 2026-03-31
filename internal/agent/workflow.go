package agent

import (
	"crypto/rand"
	"encoding/hex"
)

// PlanResult stores the AI response in plan mode
type PlanResult struct {
	ID        string `json:"id"`
	Content   string `json:"content"`    // AI response content
	UserInput string `json:"user_input"` // Original user request
	Iteration int    `json:"iteration"`  // Modification count
}

// PlanInteraction plan interaction message
type PlanInteraction struct {
	Type    string      `json:"type"`
	Plan    *PlanResult `json:"plan,omitempty"`
	Message string      `json:"message,omitempty"`
	Action  string      `json:"action,omitempty"` // "confirm", "modify", "cancel"
}

// PlanModeState plan mode state
type PlanModeState struct {
	CurrentPlan  *PlanResult
	IsActive     bool
	IsConfirmed  bool
	FeedbackCh   chan PlanInteraction
	InputTokens  int
	OutputTokens int
}

// NewPlanModeState creates a new plan mode state
func NewPlanModeState() *PlanModeState {
	return &PlanModeState{
		FeedbackCh: make(chan PlanInteraction, 10),
	}
}

// generatePlanID generates a unique plan ID
func generatePlanID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}