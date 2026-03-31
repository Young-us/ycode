package ui

import (
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ActionBarPosition defines where the action bar is positioned
type ActionBarPosition int

const (
	ActionBarBottom ActionBarPosition = 0
	ActionBarTop    ActionBarPosition = 1
)

// ActionBarOption defines an option in the action bar
type ActionBarOption struct {
	Key        string // Key to press (e.g., "y", "n", "m")
	Label      string // Display label (e.g., "确认", "取消", "修改")
	Style      string // Style: "success", "danger", "info"
	Highlighted bool // Whether this option is highlighted
}

// ActionBarModel is a reusable action bar component for user decisions
type ActionBarModel struct {
	width      int
	height     int // Fixed small height
	visible    bool
	options    []ActionBarOption
	selected   int    // Currently selected option index
	inputMode  bool   // Whether user is typing input
	inputText  string // User input text
	title      string // Optional title/hint
	position   ActionBarPosition
	context    string // Context identifier (e.g., "plan", "audit")
}

// NewActionBarModel creates a new action bar model
func NewActionBarModel() *ActionBarModel {
	return &ActionBarModel{
		height:   4, // Fixed height: 2 for buttons + 2 for hint
		position: ActionBarBottom,
		options:  []ActionBarOption{},
	}
}

// Init initializes the action bar
func (m *ActionBarModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *ActionBarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

// SetSize sets the width and height
func (m *ActionBarModel) SetSize(width, height int) {
	m.width = width
	// Keep fixed height, ignore passed height
}

// Show shows the action bar with given options
func (m *ActionBarModel) Show(options []ActionBarOption, title string, context string) {
	m.visible = true
	m.options = options
	m.title = title
	m.context = context
	m.selected = 0
	m.inputMode = false
	m.inputText = ""
}

// Hide hides the action bar
func (m *ActionBarModel) Hide() {
	m.visible = false
	m.inputMode = false
	m.inputText = ""
}

// IsVisible returns whether the action bar is visible
func (m *ActionBarModel) IsVisible() bool {
	return m.visible
}

// SetInputMode sets whether user is typing input
func (m *ActionBarModel) SetInputMode(enabled bool) {
	m.inputMode = enabled
}

// SetInputText sets the input text
func (m *ActionBarModel) SetInputText(text string) {
	m.inputText = text
}

// GetInputText returns the current input text
func (m *ActionBarModel) GetInputText() string {
	return m.inputText
}

// IsInputMode returns whether input mode is active
func (m *ActionBarModel) IsInputMode() bool {
	return m.inputMode
}

// GetSelectedOption returns the currently selected option
func (m *ActionBarModel) GetSelectedOption() *ActionBarOption {
	if m.selected >= 0 && m.selected < len(m.options) {
		return &m.options[m.selected]
	}
	return nil
}

// MoveSelection moves the selection by delta
func (m *ActionBarModel) MoveSelection(delta int) {
	m.selected += delta
	if m.selected < 0 {
		m.selected = len(m.options) - 1
	}
	if m.selected >= len(m.options) {
		m.selected = 0
	}
}

// View renders the action bar
func (m *ActionBarModel) View() string {
	if !m.visible || m.width <= 0 {
		return ""
	}

	var b strings.Builder

	// Container style with top border
	containerStyle := lipgloss.NewStyle().
		Width(m.width).
		BorderTop(true).
		BorderForeground(lipgloss.Color("12")) // Cyan border

	// If in input mode, show input field
	if m.inputMode {
		// Input prompt
		inputStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")). // Cyan
			Padding(0, 1)

		b.WriteString(inputStyle.Render("💬 输入修改建议: " + m.inputText + "█"))
		b.WriteString("\n")

		// Hint
		hintStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")). // Gray
			Padding(0, 1)

		b.WriteString(hintStyle.Render("[Enter] 提交   [ESC] 取消输入"))
	} else {
		// Render action buttons in a row
		buttons := m.renderButtons()
		b.WriteString(buttons)
		b.WriteString("\n")

		// Hint line
		hintStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")). // Gray
			Padding(0, 1)

		hintText := m.renderHint()
		b.WriteString(hintStyle.Render(hintText))
	}

	// Apply container style
	return containerStyle.Render(b.String())
}

// renderButtons renders the action buttons
func (m *ActionBarModel) renderButtons() string {
	var buttons []string

	for i, opt := range m.options {
		// Base style with padding and border
		btnStyle := lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.NormalBorder())

		// Add icon based on style
		var icon string
		var borderColor lipgloss.Color
		var fgColor lipgloss.Color
		switch opt.Style {
		case "success":
			icon = "✓"
			borderColor = lipgloss.Color("34")  // Green
			fgColor = lipgloss.Color("34")
		case "danger":
			icon = "✗"
			borderColor = lipgloss.Color("196") // Red
			fgColor = lipgloss.Color("196")
		case "info":
			icon = "✎"
			borderColor = lipgloss.Color("39")  // Blue
			fgColor = lipgloss.Color("39")
		default:
			icon = "○"
			borderColor = lipgloss.Color("243") // Gray
			fgColor = lipgloss.Color("243")
		}

		btnStyle = btnStyle.
			BorderForeground(borderColor).
			Foreground(fgColor)

		// Highlight selected button with background
		if i == m.selected {
			btnStyle = btnStyle.
				Background(lipgloss.Color("235")). // Subtle dark background
				Bold(true)
		}

		label := icon + " [" + opt.Key + "] " + opt.Label
		buttons = append(buttons, btnStyle.Render(label))
	}

	// Join buttons with spacing
	return lipgloss.JoinHorizontal(lipgloss.Top, buttons...)
}

// renderHint renders the hint text
func (m *ActionBarModel) renderHint() string {
	if m.title != "" {
		return m.title
	}

	// Default hint based on context
	switch m.context {
	case "plan":
		return "按对应按键执行操作"
	default:
		return "按对应按键选择"
	}
}

// GetHeight returns the fixed height of the action bar
func (m *ActionBarModel) GetHeight() int {
	return m.height
}

// GetContext returns the current context
func (m *ActionBarModel) GetContext() string {
	return m.context
}

// NewPlanActionBar creates an action bar for plan confirmation
func NewPlanActionBar() *ActionBarModel {
	bar := NewActionBarModel()
	bar.Show([]ActionBarOption{
		{Key: "y", Label: "确认", Style: "success"},
		{Key: "n", Label: "取消", Style: "danger"},
		{Key: "m", Label: "修改", Style: "info"},
	}, "按 Y 执行，N 取消，M 提供修改建议", "plan")
	return bar
}

// NewSensitiveOpActionBar creates an action bar for sensitive operation confirmation
func NewSensitiveOpActionBar(opDescription string) *ActionBarModel {
	bar := NewActionBarModel()
	bar.Show([]ActionBarOption{
		{Key: "y", Label: "确认", Style: "success"},
		{Key: "n", Label: "拒绝", Style: "danger"},
	}, "按 Y 确认执行，N 拒绝操作", "sensitive")
	if opDescription != "" {
		bar.title = opDescription
	}
	return bar
}