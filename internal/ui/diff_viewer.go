package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/Young-us/ycode/internal/tools"
)

// DiffViewerModel displays a diff preview with confirmation
type DiffViewerModel struct {
	width       int
	height      int
	diff        *tools.DiffResult
	path        string
	scrollY     int
	visible     bool
	confirmMode bool // true = waiting for user confirmation
}

// NewDiffViewerModel creates a new diff viewer
func NewDiffViewerModel() *DiffViewerModel {
	return &DiffViewerModel{}
}

// SetDiff sets the diff to display
func (m *DiffViewerModel) SetDiff(path string, diff *tools.DiffResult) {
	m.path = path
	m.diff = diff
	m.scrollY = 0
	m.visible = true
	m.confirmMode = true
}

// Hide hides the diff viewer
func (m *DiffViewerModel) Hide() {
	m.visible = false
	m.confirmMode = false
}

// IsVisible returns true if the diff viewer is visible
func (m *DiffViewerModel) IsVisible() bool {
	return m.visible
}

// IsConfirmMode returns true if waiting for confirmation
func (m *DiffViewerModel) IsConfirmMode() bool {
	return m.confirmMode
}

// Scroll scrolls the diff content
func (m *DiffViewerModel) Scroll(delta int) {
	m.scrollY += delta
	if m.scrollY < 0 {
		m.scrollY = 0
	}
	maxScroll := m.getMaxScroll()
	if m.scrollY > maxScroll {
		m.scrollY = maxScroll
	}
}

func (m *DiffViewerModel) getMaxScroll() int {
	if m.diff == nil {
		return 0
	}
	contentLines := len(strings.Split(m.formatDiffContent(), "\n"))
	maxScroll := contentLines - m.height + 4 // Leave room for header/controls
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

// View renders the diff viewer
func (m *DiffViewerModel) View() string {
	if !m.visible || m.diff == nil {
		return ""
	}

	// Styles
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("3")). // Yellow border for attention
		Padding(0, 1)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("3"))

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6"))

	addStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("2")) // Green

	delStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("1")) // Red

	contextStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")) // Gray

	// Build header
	var header strings.Builder
	header.WriteString(titleStyle.Render("📝 Diff Preview"))
	header.WriteString("\n")
	header.WriteString(headerStyle.Render(fmt.Sprintf("File: %s", m.path)))
	header.WriteString("\n")
	header.WriteString(fmt.Sprintf("%d additions, %d deletions", m.diff.Stats.Additions, m.diff.Stats.Deletions))

	// Build diff content
	diffContent := m.formatDiffContent()

	// Split and scroll
	lines := strings.Split(diffContent, "\n")
	startY := m.scrollY
	if startY < 0 {
		startY = 0
	}
	endY := startY + m.height - 6 // Reserve lines for header and controls
	if endY > len(lines) {
		endY = len(lines)
	}

	var content strings.Builder
	for i := startY; i < endY; i++ {
		line := lines[i]
		// Apply styling based on line prefix
		if strings.HasPrefix(line, "+") {
			content.WriteString(addStyle.Render(line))
		} else if strings.HasPrefix(line, "-") {
			content.WriteString(delStyle.Render(line))
		} else {
			content.WriteString(contextStyle.Render(line))
		}
		content.WriteString("\n")
	}

	// Build controls
	controlsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("3"))

	var controls strings.Builder
	if m.confirmMode {
		controls.WriteString(controlsStyle.Render("Press [y] to confirm, [n] to cancel, [↑/↓] to scroll"))
	} else {
		controls.WriteString(controlsStyle.Render("Press [ESC] to close"))
	}

	// Combine all parts
	var result strings.Builder
	result.WriteString(header.String())
	result.WriteString("\n")
	result.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(strings.Repeat("─", m.width-4)))
	result.WriteString("\n")
	result.WriteString(content.String())
	result.WriteString("\n")
	result.WriteString(controls.String())

	// Wrap in border
	return borderStyle.
		Width(m.width - 2).
		Height(m.height).
		Render(result.String())
}

func (m *DiffViewerModel) formatDiffContent() string {
	if m.diff == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- Original\n+++ Modified\n\n"))

	for _, op := range m.diff.Ops {
		switch op.Type {
		case "add":
			for _, line := range op.Lines {
				sb.WriteString(fmt.Sprintf("+ %s\n", line))
			}
		case "delete":
			for _, line := range op.Lines {
				sb.WriteString(fmt.Sprintf("- %s\n", line))
			}
		case "equal":
			for _, line := range op.Lines {
				sb.WriteString(fmt.Sprintf("  %s\n", line))
			}
		}
	}

	return sb.String()
}

// FormatDiffForMessage formats a diff for display in a chat message
func FormatDiffForMessage(path string, diff *tools.DiffResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("**Diff Preview: %s**\n", path))
	sb.WriteString(fmt.Sprintf("```diff\n"))
	sb.WriteString(fmt.Sprintf("--- Original\n+++ Modified\n"))

	// Limit to first 50 lines for message display
	lineCount := 0
	maxLines := 50

	for _, op := range diff.Ops {
		if lineCount >= maxLines {
			sb.WriteString(fmt.Sprintf("\n... (%d more lines)\n", len(diff.Ops)-lineCount))
			break
		}

		switch op.Type {
		case "add":
			for _, line := range op.Lines {
				sb.WriteString(fmt.Sprintf("+ %s\n", line))
				lineCount++
			}
		case "delete":
			for _, line := range op.Lines {
				sb.WriteString(fmt.Sprintf("- %s\n", line))
				lineCount++
			}
		case "equal":
			// Skip equal lines in message preview to save space
		}
	}

	sb.WriteString("```\n")
	sb.WriteString(fmt.Sprintf("\n📊 **Stats:** +%d -%d", diff.Stats.Additions, diff.Stats.Deletions))

	return sb.String()
}