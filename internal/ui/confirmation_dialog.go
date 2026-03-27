package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/Young-us/ycode/internal/audit"
)

// ConfirmationDialogModel displays a confirmation dialog for sensitive operations
type ConfirmationDialogModel struct {
	width      int
	height     int
	operation  audit.SensitiveOperation
	visible    bool
	remember   bool
	selected   int // 0 = confirm, 1 = deny, 2 = remember checkbox
}

// NewConfirmationDialogModel creates a new confirmation dialog
func NewConfirmationDialogModel() *ConfirmationDialogModel {
	return &ConfirmationDialogModel{}
}

// Show shows the dialog for an operation
func (m *ConfirmationDialogModel) Show(op audit.SensitiveOperation) {
	m.operation = op
	m.visible = true
	m.remember = false
	m.selected = 0
}

// Hide hides the dialog
func (m *ConfirmationDialogModel) Hide() {
	m.visible = false
}

// IsVisible returns true if the dialog is visible
func (m *ConfirmationDialogModel) IsVisible() bool {
	return m.visible
}

// GetResult returns the confirmation result
func (m *ConfirmationDialogModel) GetResult() audit.ConfirmationResult {
	return audit.ConfirmationResult{
		Confirmed: m.selected == 0,
		Remember:  m.remember,
		Reason:    "User confirmation",
	}
}

// MoveSelection moves the selection
func (m *ConfirmationDialogModel) MoveSelection(delta int) {
	m.selected += delta
	if m.selected < 0 {
		m.selected = 2
	}
	if m.selected > 2 {
		m.selected = 0
	}
}

// ToggleRemember toggles the remember checkbox
func (m *ConfirmationDialogModel) ToggleRemember() {
	m.remember = !m.remember
}

// Select selects the current option
func (m *ConfirmationDialogModel) Select() {
	if m.selected == 2 {
		m.ToggleRemember()
	}
}

// View renders the confirmation dialog
func (m *ConfirmationDialogModel) View() string {
	if !m.visible {
		return ""
	}

	// Styles
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("1")). // Red border for warning
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("1")) // Red

	levelStyle := lipgloss.NewStyle().
		Bold(true)

	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("3")) // Yellow

	// Determine severity text and color
	var severityText string
	var severityColor lipgloss.Color
	switch m.operation.Level {
	case audit.SensitivityCritical:
		severityText = "⚠️  CRITICAL"
		severityColor = lipgloss.Color("1") // Red
	case audit.SensitivityHigh:
		severityText = "⚡ HIGH RISK"
		severityColor = lipgloss.Color("3") // Yellow
	case audit.SensitivityMedium:
		severityText = "⚠️  MEDIUM"
		severityColor = lipgloss.Color("6") // Cyan
	default:
		severityText = "ℹ️ LOW"
		severityColor = lipgloss.Color("8") // Gray
	}

	levelStyle = levelStyle.Foreground(severityColor)

	// Build content
	var content strings.Builder

	// Title
	content.WriteString(titleStyle.Render("🔒 Confirmation Required"))
	content.WriteString("\n\n")

	// Severity level
	content.WriteString(levelStyle.Render(severityText))
	content.WriteString("\n\n")

	// Operation details
	content.WriteString(infoStyle.Render("Operation: "))
	content.WriteString(fmt.Sprintf("%s %s\n", m.operation.ToolName, m.operation.Operation))

	if m.operation.Target != "" {
		content.WriteString(infoStyle.Render("Target: "))
		// Truncate long targets
		target := m.operation.Target
		if len(target) > 50 {
			target = target[:50] + "..."
		}
		content.WriteString(target + "\n")
	}

	content.WriteString("\n")

	// Reason
	if m.operation.Reason != "" {
		content.WriteString(infoStyle.Render("Reason: "))
		content.WriteString(m.operation.Reason + "\n")
	}

	// Risk warning
	if m.operation.SuggestedRisk != "" {
		content.WriteString("\n")
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			Render("⚠️ Risk: " + m.operation.SuggestedRisk))
	}

	content.WriteString("\n\n")

	// Options
	confirmStyle := lipgloss.NewStyle()
	denyStyle := lipgloss.NewStyle()
	rememberStyle := lipgloss.NewStyle()

	if m.selected == 0 {
		confirmStyle = confirmStyle.Foreground(lipgloss.Color("2")).Bold(true) // Green
	} else {
		confirmStyle = confirmStyle.Foreground(lipgloss.Color("8")) // Gray
	}

	if m.selected == 1 {
		denyStyle = denyStyle.Foreground(lipgloss.Color("1")).Bold(true) // Red
	} else {
		denyStyle = denyStyle.Foreground(lipgloss.Color("8")) // Gray
	}

	confirmIcon := "  "
	denyIcon := "  "
	if m.selected == 0 {
		confirmIcon = "▶ "
	}
	if m.selected == 1 {
		denyIcon = "▶ "
	}

	content.WriteString(confirmStyle.Render(confirmIcon + "[Y] Confirm"))
	content.WriteString("  ")
	content.WriteString(denyStyle.Render(denyIcon + "[N] Deny"))
	content.WriteString("\n\n")

	// Remember checkbox
	rememberText := "☐ Remember this decision"
	if m.remember {
		rememberText = "☑ Remember this decision"
	}
	if m.selected == 2 {
		rememberStyle = rememberStyle.Foreground(lipgloss.Color("6")).Bold(true)
	} else {
		rememberStyle = rememberStyle.Foreground(lipgloss.Color("8"))
	}
	content.WriteString(rememberStyle.Render("  " + rememberText))

	content.WriteString("\n\n")

	// Controls hint
	content.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Render("↑/↓ or Tab: Navigate | Enter: Select | ESC: Cancel"))

	return borderStyle.
		Width(m.width - 4).
		Render(content.String())
}

// FormatOperationForMessage formats an operation for display in a message
func FormatOperationForMessage(op audit.SensitiveOperation) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("**⚠️ Sensitive Operation Detected**\n\n"))
	sb.WriteString(fmt.Sprintf("**Tool:** %s\n", op.ToolName))
	sb.WriteString(fmt.Sprintf("**Operation:** %s\n", op.Operation))

	if op.Target != "" {
		sb.WriteString(fmt.Sprintf("**Target:** `%s`\n", op.Target))
	}

	sb.WriteString(fmt.Sprintf("**Risk Level:** %s\n", audit.SensitivityLevelToString(op.Level)))
	sb.WriteString(fmt.Sprintf("**Reason:** %s\n", op.Reason))

	if op.SuggestedRisk != "" {
		sb.WriteString(fmt.Sprintf("\n⚠️ **Warning:** %s\n", op.SuggestedRisk))
	}

	return sb.String()
}