package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Young-us/ycode/internal/agent"
	"github.com/Young-us/ycode/internal/audit"
	"github.com/Young-us/ycode/internal/command"
	"github.com/Young-us/ycode/internal/config"
	"github.com/Young-us/ycode/internal/llm"
	"github.com/Young-us/ycode/internal/logger"
	"github.com/Young-us/ycode/internal/session"
	"github.com/Young-us/ycode/internal/tools"
	"github.com/Young-us/ycode/internal/ui/layout"
)

// Layout constants for consistent sizing
const (
	titleBarHeight   = 0 // No title bar
	statusBarHeight  = 2
	inputAreaHeight  = 6
	minContentHeight = 5
	minSidebarWidth  = 25
	maxSidebarWidth  = 40
	minMainWidth     = 40
	scrollbarWidth   = 1
	messageWrapWidth = 80
	// Max thinking display length to prevent UI freeze with very long thinking
	maxThinkingDisplayLen = 5000
)

// Spinner frames for loading animation
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// retryStatusMsg is sent periodically to poll retry status
type retryStatusMsg struct{}

// pollRetryStatus returns a command that sends retry status messages periodically
func pollRetryStatus() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return retryStatusMsg{}
	})
}

// ModernTUIModel represents the modern TUI layout with sidebar and organized panels
type ModernTUIModel struct {
	// Agent
	orchestrator *agent.Orchestrator
	config       *config.Config

	// Command manager
	cmdManager *command.CommandManager

	// Session manager (can be nil)
	sessionManager *session.Manager
	currentSession *session.Session

	// Layout state
	width       int
	height      int
	ready       bool
	loading     bool
	sidebarOpen bool
	activePanel int // 0: chat, 1: files, 2: tools

	// Session state
	sessionTitle string
	hasSetTitle  bool
	sessions     []session.Session // Real session list

	// Chat state
	messages []ChatMessageFinal
	input    string
	cursor   int

	// Cancel context for interrupting operations
	cancelCtx     context.Context
	cancelFunc    context.CancelFunc
	isInterrupted bool

	// Scroll state
	scrollY      int
	totalLines   int
	visibleLines int

	// Welcome state
	showWelcome bool

	// Context stats
	contextTokens int
	contextUsed   float64

	// Anti-duplication - only debounce Enter key
	lastSendTime    time.Time
	minSendInterval time.Duration

	// Streaming support
	streamChan       chan AgentStreamMsgFinal
	streamingContent string
	lastStreamUpdate time.Time
	streamDebounce   time.Duration

	// Tool call status display
	currentToolCall    *ToolCallInfo
	currentToolResult  *ToolResultInfo
	toolCallHistory   []ToolCallInfo // Keep history of tool calls in current turn

	// Spinner animation
	spinnerIndex int

	// ESC double-press tracking
	lastEscPress time.Time
	escCount     int
	escActive    bool // true after first ESC, reset by other keys

	// Command palette
	showPalette         bool
	paletteInput        string
	paletteCursor       int
	paletteSelected     int
	filteredCommands    []*command.Command // Store full command objects for rich display

	// Input history for navigation
	inputHistory      []string
	inputHistoryIndex int
	savedInput        string // Save current input when navigating history

	// Log viewer
	showLogViewer bool
	logLines      []string
	logScrollY    int

	// Help viewer
	showHelpViewer bool
	helpScrollY    int

	// Status indicators
	mcpStatus map[string]string
	lspStatus map[string]string

	// File browser state
	currentDir   string
	files        []string
	selectedFile int
	fileScrollY  int

	// Tool status
	toolStatus map[string]string

	// UI Components (for layout)
	titleBar  *titleBarModel
	sidebar   *sidebarModel
	mainArea  *mainAreaModel
	inputArea *inputAreaModel
	statusBar *statusBarModel

	// Layout container
	layout layout.SplitPaneLayout

	// Retry status
	retryStatus    string // Current retry status message
	retryAttempt   int    // Current retry attempt
	retryCountdown int    // Countdown in seconds
	retryActive    bool   // Whether retry is in progress

	// Diff preview and confirmation
	diffViewer       *DiffViewerModel
	pendingFileOp    *PendingFileOperation // File operation waiting for confirmation

	// Sensitive operation confirmation
	confirmationDialog *ConfirmationDialogModel
	pendingSensitiveOp bool // Flag for pending sensitive operation

	// Autocomplete
	autocomplete *AutoCompleter
	showAutocomplete bool
}

// PendingFileOperation holds a file operation awaiting user confirmation
type PendingFileOperation struct {
	Path      string
	Content   string
	Operation string // "write" or "edit"
	Diff      *tools.DiffResult
}

// Title bar component
type titleBarModel struct {
	width       int
	height      int
	title       string
	model       string
	workingDir  string
	sidebarOpen bool
}

func (t *titleBarModel) Init() tea.Cmd                           { return nil }
func (t *titleBarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return t, nil }
func (t *titleBarModel) View() string {
	if t.width <= 0 {
		return ""
	}

	// Simple title: just show title centered
	titleText := t.title

	// Build the line with padding
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("62")).
		Width(t.width).
		Height(t.height).
		Align(lipgloss.Center)

	return style.Render(titleText)
}
func (t *titleBarModel) SetSize(width, height int) tea.Cmd {
	t.width = width
	t.height = height
	return nil
}
func (t *titleBarModel) GetSize() (int, int) { return t.width, t.height }

// Sidebar component
type sidebarModel struct {
	width              int
	height             int
	sessionTitle       string
	sessions           []session.Session
	currentSess        *session.Session
	selectedIndex      int // Selected session index in RECENT list
	scrollOffset       int // Scroll offset for RECENT list
	mcpStatus          map[string]string
	lspStatus          map[string]string
	tokens             int
	focusMode          string // "chat", "recent" - which section is focused
}

func (s *sidebarModel) Init() tea.Cmd                           { return nil }
func (s *sidebarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return s, nil }
func (s *sidebarModel) View() string {
	if s.width <= 0 || s.height <= 0 {
		return ""
	}

	var lines []string

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("117"))
	decorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	// Content width (leave 1 char for right border)
	contentWidth := s.width - 1
	if contentWidth < 1 {
		contentWidth = 1
	}

	// Helper: pad line to content width (using visual width for CJK support)
	padLine := func(text string) string {
		visualWidth := lipgloss.Width(text)
		if visualWidth > contentWidth {
			// Truncate by visual width
			runes := []rune(text)
			result := ""
			for _, r := range runes {
				if lipgloss.Width(result+string(r)) > contentWidth {
					break
				}
				result += string(r)
			}
			text = result
			visualWidth = lipgloss.Width(text)
		}
		// Pad with spaces to exact content width
		return text + strings.Repeat(" ", contentWidth-visualWidth)
	}

	// Helper: decorative header line
	makeHeader := func(title string) string {
		titleWidth := lipgloss.Width(title)
		totalDash := contentWidth - titleWidth
		if totalDash < 0 {
			totalDash = 0
		}
		left := totalDash / 2
		right := totalDash - left
		return decorStyle.Render(strings.Repeat("-", left)) +
			titleStyle.Render(title) +
			decorStyle.Render(strings.Repeat("-", right))
	}

	// SESSION section
	lines = append(lines, makeHeader(" SESSION "))
	lines = append(lines, padLine("  "+s.sessionTitle))
	lines = append(lines, padLine(""))

	// STATUS section
	lines = append(lines, makeHeader(" STATUS "))
	hasServers := false
	for name, status := range s.mcpStatus {
		hasServers = true
		icon := "[ ]"
		if status == "connected" {
			icon = "[*]"
		}
		lines = append(lines, padLine(fmt.Sprintf("  %s MCP %s", icon, name)))
	}
	for name, status := range s.lspStatus {
		hasServers = true
		icon := "[ ]"
		if status == "connected" {
			icon = "[*]"
		}
		lines = append(lines, padLine(fmt.Sprintf("  %s LSP %s", icon, name)))
	}
	if !hasServers {
		lines = append(lines, padLine("  No servers configured"))
	}
	lines = append(lines, padLine(""))

	// TOKENS section
	lines = append(lines, makeHeader(" TOKENS "))
	lines = append(lines, padLine(fmt.Sprintf("  %s used", formatTokensStatic(s.tokens))))
	lines = append(lines, padLine(""))

	// RECENT section
	lines = append(lines, makeHeader(" RECENT "))
	// Calculate available space for sessions
	usedLines := len(lines) + 2 // +2 for potential fill and bottom padding
	availableForSessions := s.height - usedLines - 3 // Reserve some space at bottom
	if availableForSessions < 3 {
		availableForSessions = 3
	}
	if availableForSessions > 15 {
		availableForSessions = 15 // Cap at 15 to leave room for other sections
	}

	// Show sessions with scroll support
	if len(s.sessions) > 0 {
		// Calculate visible range with scroll
		startIdx := s.scrollOffset
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx > len(s.sessions)-availableForSessions && len(s.sessions) > availableForSessions {
			startIdx = len(s.sessions) - availableForSessions
		}

		endIdx := startIdx + availableForSessions
		if endIdx > len(s.sessions) {
			endIdx = len(s.sessions)
		}

		// Show scroll indicator if needed
		if len(s.sessions) > availableForSessions {
			scrollInfo := fmt.Sprintf(" (%d/%d) ", startIdx+1, len(s.sessions))
			lines = append(lines, padLine("  "+scrollInfo))
		}

		for i := startIdx; i < endIdx; i++ {
			sess := s.sessions[i]
			prefix := "  "
			if s.currentSess != nil && sess.ID == s.currentSess.ID {
				prefix = "● " // Current session marker
			}
			title := sess.Title
			if title == "" || title == "New Session" {
				// Show first message as title if no custom title
				if len(sess.Messages) > 0 {
					firstMsg := sess.Messages[0].Content
					// Truncate by visual width
					maxMsgWidth := contentWidth - 6 // prefix + spacing
					if lipgloss.Width(firstMsg) > maxMsgWidth {
						runes := []rune(firstMsg)
						truncated := ""
						for _, r := range runes {
							if lipgloss.Width(truncated+string(r)) > maxMsgWidth-3 {
								break
							}
							truncated += string(r)
						}
						firstMsg = truncated + "..."
					}
					title = firstMsg
				} else {
					title = "New Session"
				}
			}
			// Truncate title by visual width
			maxTitleWidth := contentWidth - 4 // prefix + spacing
			if lipgloss.Width(title) > maxTitleWidth {
				runes := []rune(title)
				truncated := ""
				for _, r := range runes {
					if lipgloss.Width(truncated+string(r)) > maxTitleWidth-3 {
						break
					}
					truncated += string(r)
				}
				title = truncated + "..."
			}
			line := prefix + title

			// Highlight selected item
			if s.focusMode == "recent" && i == s.selectedIndex {
				selectedStyle := lipgloss.NewStyle().
					Background(lipgloss.Color("62")).
					Foreground(lipgloss.Color("255")).
					Bold(true)
				lines = append(lines, selectedStyle.Render(padLine(line)))
			} else if s.currentSess != nil && sess.ID == s.currentSess.ID {
				// Current session style
				currentStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("117"))
				lines = append(lines, currentStyle.Render(padLine(line)))
			} else {
				lines = append(lines, padLine(line))
			}
		}

		// Show count if more sessions exist
		if len(s.sessions) > endIdx {
			lines = append(lines, padLine(fmt.Sprintf("  ↓ %d more", len(s.sessions)-endIdx)))
		}
	} else {
		lines = append(lines, padLine("  No sessions yet"))
	}

	// Fill remaining height
	for len(lines) < s.height {
		lines = append(lines, padLine(""))
	}

	// Truncate if too tall
	if len(lines) > s.height {
		lines = lines[:s.height]
	}

	// Join and apply right border using lipgloss
	content := strings.Join(lines, "\n")
	style := lipgloss.NewStyle().
		Width(contentWidth).
		Height(s.height).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color("243"))

	return style.Render(content)
}
func (s *sidebarModel) SetSize(width, height int) tea.Cmd {
	s.width = width
	s.height = height
	return nil
}
func (s *sidebarModel) GetSize() (int, int) { return s.width, s.height }

// UpdateLSPStatus updates the LSP server status
func (m *ModernTUIModel) UpdateLSPStatus(name string, connected bool) {
	if connected {
		m.lspStatus[name] = "connected"
	} else {
		m.lspStatus[name] = "disconnected"
	}
	m.sidebar.lspStatus = m.lspStatus
}

type mainAreaModel struct {
	width           int
	height          int
	messages        []ChatMessageFinal
	scrollY         int
	totalLines      int
	showWelcome     bool
	spinnerIndex    int
	currentToolCall *ToolCallInfo    // Current tool being executed
	toolCallHistory []ToolCallInfo   // History of tool calls
}

func (m *mainAreaModel) Init() tea.Cmd                           { return nil }
func (m *mainAreaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m *mainAreaModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	if m.showWelcome {
		return m.renderWelcome()
	}

	return m.renderMessages()
}

func (m *mainAreaModel) renderWelcome() string {
	// Simplified welcome for small terminals
	if m.width < 60 {
		smallWelcome := `

  Welcome to ycode

  Type your request and press Enter
  /help - Show commands
  Ctrl+P - Command palette
  Ctrl+B - Toggle sidebar
  Ctrl+L - Clear chat`

		welcomeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		content := welcomeStyle.Render(smallWelcome)
		lines := strings.Split(content, "\n")

		// Truncate if too many lines
		if len(lines) > m.height {
			lines = lines[:m.height]
		}

		// Pad with empty lines to exact height
		for len(lines) < m.height {
			lines = append(lines, "")
		}

		return strings.Join(lines, "\n")
	}

	// Original ASCII art logo
	logo := `
 ╭─────────────────────────────────────────╮
 │                                         │
 │     ██╗   ██╗ ██████╗ ██████╗ ██████╗ ███████╗
 │     ╚██╗ ██╔╝██╔════╝██╔═══██╗██╔══██╗██╔════╝
 │      ╚████╔╝ ██║     ██║   ██║██║  ██║█████╗
 │       ╚██╔╝  ██║     ██║   ██║██║  ██║██╔══╝
 │        ██║   ╚██████╗╚██████╔╝██████╔╝███████╗
 │        ╚═╝    ╚═════╝ ╚═════╝ ╚═════╝ ╚═════╝
 │                                         │
 ╰─────────────────────────────────────────╯`

	welcome := `

  Welcome to ycode - Your AI Coding Assistant

  I can help you with:
    - Reading and writing code files
    - Executing shell commands
    - Searching through your codebase
    - Git operations

  Quick Start:
    - Type your request and press Enter
    - Type /help to see available commands
    - Press Ctrl+P to open command palette
    - Press Ctrl+B to toggle sidebar
    - Press Ctrl+L to clear chat`

	logoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	welcomeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	content := logoStyle.Render(logo) + welcomeStyle.Render(welcome)

	// Split into lines
	lines := strings.Split(content, "\n")

	// Process each line - wrap if too wide
	var processedLines []string
	for _, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth > m.width {
			// Wrap the line using visual width
			wrapped := wrapTextContent(line, m.width)
			processedLines = append(processedLines, strings.Split(wrapped, "\n")...)
		} else {
			processedLines = append(processedLines, line)
		}
	}

	// Truncate if too many lines
	if len(processedLines) > m.height {
		processedLines = processedLines[:m.height]
	}

	// Pad with empty lines to exact height
	for len(processedLines) < m.height {
		processedLines = append(processedLines, "")
	}

	return strings.Join(processedLines, "\n")
}

func (m *mainAreaModel) renderMessages() string {
	var lines []string
	contentWidth := m.width - 2 // Leave room for padding
	if contentWidth < 20 {
		contentWidth = 20
	}

	if len(m.messages) == 0 {
		// Return empty lines padded to exact height
		for i := 0; i < m.height; i++ {
			lines = append(lines, "")
		}
		// Add placeholder message on first line
		if m.height > 0 {
			lines[0] = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Render("  No messages yet. Start typing to begin...")
		}
		return strings.Join(lines, "\n")
	}

	for i := range m.messages {
		rendered := m.renderMessage(&m.messages[i], contentWidth)
		msgLines := strings.Split(rendered, "\n")
		lines = append(lines, msgLines...)
	}

	// Add tool status bar if there's an active tool call
	if m.currentToolCall != nil {
		toolBar := m.renderToolStatusBar(contentWidth)
		lines = append(lines, "", toolBar)
	}

	// Store total lines for scrollbar
	m.totalLines = len(lines)

	// Calculate scroll position
	// Use the scrollY from parent model, already clamped
	startLine := m.scrollY
	if startLine < 0 {
		startLine = 0
	}
	// Don't re-clamp here - parent model handles it

	// Get visible lines
	endLine := startLine + m.height
	if endLine > len(lines) {
		endLine = len(lines)
	}

	var visibleLines []string
	if startLine < len(lines) {
		if endLine <= len(lines) {
			visibleLines = lines[startLine:endLine]
		} else {
			visibleLines = lines[startLine:]
		}
	}

	// Fill remaining height with empty lines
	for len(visibleLines) < m.height {
		visibleLines = append(visibleLines, "")
	}

	return strings.Join(visibleLines, "\n")
}

// renderToolStatusBar renders a status bar showing current tool activity
func (m *mainAreaModel) renderToolStatusBar(width int) string {
	if m.currentToolCall == nil {
		return ""
	}

	// Spinner characters
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinnerChar := spinner[m.spinnerIndex%len(spinner)]

	// Tool icon based on tool name
	toolIcon := "🔧"
	toolName := m.currentToolCall.Name
	switch {
	case toolName == "read_file":
		toolIcon = "📄"
	case toolName == "write_file":
		toolIcon = "✏️"
	case toolName == "edit_file":
		toolIcon = "📝"
	case toolName == "bash":
		toolIcon = "⚡"
	case toolName == "git":
		toolIcon = "🔀"
	case toolName == "grep":
		toolIcon = "🔍"
	case toolName == "glob":
		toolIcon = "📂"
	case toolName == "lsp":
		toolIcon = "💡"
	case toolName == "ast":
		toolIcon = "🌳"
	case strings.HasPrefix(toolName, "mcp_"):
		toolIcon = "🔌"
	}

	// Build status line
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true)

	spinnerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214"))

	toolStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("117"))

	// Build the status bar
	status := fmt.Sprintf("  %s %s %s %s",
		spinnerStyle.Render(spinnerChar),
		toolIcon,
		toolStyle.Render(toolName),
		statusStyle.Render("..."),
	)

	// Show tool arguments summary if relevant
	if m.currentToolCall.Arguments != nil {
		if path, ok := m.currentToolCall.Arguments["path"].(string); ok && path != "" {
			argStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
			status += " " + argStyle.Render(truncatePath(path, width-len(status)-10))
		} else if cmd, ok := m.currentToolCall.Arguments["command"].(string); ok && cmd != "" {
			argStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
			status += " " + argStyle.Render(truncateString(cmd, width-len(status)-10))
		}
	}

	return status
}

// truncatePath shortens a file path for display
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	// Keep the filename and show "..." for truncated parts
	parts := strings.Split(path, string(os.PathSeparator))
	if len(parts) > 2 {
		return "..." + string(os.PathSeparator) + parts[len(parts)-1]
	}
	return path[:maxLen-3] + "..."
}

// truncateString shortens a string for display
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func (m *mainAreaModel) renderScrollbar(height, total, offset int) string {
	if total <= height || height <= 0 {
		return ""
	}

	thumbSize := max(1, height*height/total)
	thumbPos := 0
	if total > height {
		thumbPos = offset * (height - thumbSize) / (total - height)
	}
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos > height-thumbSize {
		thumbPos = height - thumbSize
	}

	var scrollbar strings.Builder
	for i := 0; i < height; i++ {
		if i >= thumbPos && i < thumbPos+thumbSize {
			scrollbar.WriteString("|") // Use ASCII for Windows CMD compatibility
		} else {
			scrollbar.WriteString(":")
		}
	}
	return scrollbar.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *mainAreaModel) renderMessage(msg *ChatMessageFinal, maxWidth int) string {
	var contentStyle lipgloss.Style

	// Use cached wrapped content if available and width matches
	wrappedContent := msg.WrappedContent
	if wrappedContent == "" || msg.WrapWidth != maxWidth {
		wrappedContent = wrapTextContent(msg.Content, maxWidth-4)
		// Cache the wrapped content
		msg.WrappedContent = wrappedContent
		msg.WrapWidth = maxWidth
	}

	switch msg.Role {
	case "user":
		// User message with cyan accent and left border
		userStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("117"))
		contentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
		// Add spacing before user message
		return "\n" + userStyle.Render("> ") + contentStyle.Render(wrappedContent) + "\n"

	case "assistant":
		// Assistant message with green accent
		assistantStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))
		contentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

		// Thinking style for extended thinking
		thinkingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)

		var result strings.Builder

		// Show thinking content if available
		if msg.Thinking != "" {
			// Truncate thinking for display to prevent UI freeze
			displayThinking := truncateThinking(msg.Thinking, maxThinkingDisplayLen)
			// Use cached wrapped thinking if available and width matches
			thinkingWrapped := msg.WrappedThinking
			if thinkingWrapped == "" || msg.WrapWidth != maxWidth {
				thinkingWrapped = wrapTextContent(displayThinking, maxWidth-6)
				msg.WrappedThinking = thinkingWrapped
			}
			thinkingLines := strings.Split(thinkingWrapped, "\n")
			for _, line := range thinkingLines {
				result.WriteString(thinkingStyle.Render("  │ " + line) + "\n")
			}
			result.WriteString("\n") // Add spacing between thinking and content
		}

		result.WriteString(assistantStyle.Render("< ") + contentStyle.Render(wrappedContent) + "\n")
		return result.String()

	case "error":
		// Error message with red accent
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
		contentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
		return errorStyle.Render("! ") + contentStyle.Render(wrappedContent) + "\n"

	case "loading":
		// Loading with spinner animation
		spinner := spinnerFrames[m.spinnerIndex%len(spinnerFrames)]
		spinnerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		contentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

		// Thinking style - smaller, dimmer text for extended thinking
		thinkingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)

		var result strings.Builder

		// Show thinking content if available
		if msg.Thinking != "" {
			// Truncate thinking for display to prevent UI freeze
			displayThinking := truncateThinking(msg.Thinking, maxThinkingDisplayLen)
			// Use cached wrapped thinking if available and width matches
			thinkingWrapped := msg.WrappedThinking
			if thinkingWrapped == "" || msg.WrapWidth != maxWidth {
				thinkingWrapped = wrapTextContent(displayThinking, maxWidth-6)
				msg.WrappedThinking = thinkingWrapped
			}
			thinkingLines := strings.Split(thinkingWrapped, "\n")
			for _, line := range thinkingLines {
				result.WriteString(thinkingStyle.Render("  │ " + line) + "\n")
			}
		}

		if msg.Content != "" {
			result.WriteString(spinnerStyle.Render(spinner+" ") + contentStyle.Render(wrappedContent))
		} else if msg.Thinking == "" {
			result.WriteString(spinnerStyle.Render(spinner + " Thinking..."))
		}
		return result.String()

	default:
		return "  " + wrappedContent
	}
}

// wrapTextContent wraps text to fit within maxWidth (visual width)
func wrapTextContent(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		// Wrap long lines using visual width
		runes := []rune(line)
		currentWidth := 0
		lineStart := 0

		for j, r := range runes {
			charWidth := lipgloss.Width(string(r))
			if currentWidth+charWidth > maxWidth {
				// Find a good break point (look back for space/punctuation)
				breakPoint := j
				for k := j - 1; k > lineStart+(j-lineStart)/2; k-- {
					if runes[k] == ' ' || runes[k] == '-' || runes[k] == ',' || runes[k] == '.' || runes[k] == '，' || runes[k] == '。' || runes[k] == '、' {
						breakPoint = k + 1
						break
					}
				}
				if breakPoint == j && j > lineStart {
					breakPoint = j
				}
				result.WriteString(string(runes[lineStart:breakPoint]))
				result.WriteString("\n")
				lineStart = breakPoint
				currentWidth = 0
				// Recalculate width from lineStart to current position
				for k := lineStart; k <= j; k++ {
					currentWidth += lipgloss.Width(string(runes[k]))
				}
				// Skip leading space on new line
				if lineStart < len(runes) && runes[lineStart] == ' ' {
					lineStart++
					currentWidth = 0
					for k := lineStart; k <= j; k++ {
						currentWidth += lipgloss.Width(string(runes[k]))
					}
				}
			} else {
				currentWidth += charWidth
			}
		}
		if lineStart < len(runes) {
			result.WriteString(string(runes[lineStart:]))
		}
	}

	return result.String()
}

// truncateThinking truncates thinking content for display to prevent UI freeze
func truncateThinking(thinking string, maxLen int) string {
	if len(thinking) <= maxLen {
		return thinking
	}
	// Truncate by rune count to avoid breaking multi-byte characters
	runes := []rune(thinking)
	if len(runes) <= maxLen {
		return thinking
	}
	return string(runes[:maxLen]) + "\n... (thinking truncated for performance)"
}

func (m *mainAreaModel) SetSize(width, height int) tea.Cmd {
	m.width = width
	m.height = height
	return nil
}
func (m *mainAreaModel) GetSize() (int, int) { return m.width, m.height }

// Input area component
type inputAreaModel struct {
	width   int
	height  int
	input   string
	cursor  int
	loading bool
}

func (i *inputAreaModel) Init() tea.Cmd                           { return nil }
func (i *inputAreaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return i, nil }
func (i *inputAreaModel) View() string {
	if i.width <= 0 || i.height <= 0 {
		return ""
	}

	// Build input lines
	runes := []rune(i.input)
	maxWidth := i.width - 8
	if maxWidth < 5 {
		maxWidth = 5
	}

	var inputLines []string
	var current strings.Builder
	for _, r := range runes {
		current.WriteRune(r)
		if current.Len() >= maxWidth {
			inputLines = append(inputLines, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		inputLines = append(inputLines, current.String())
	}
	if len(inputLines) == 0 {
		inputLines = []string{""}
	}

	// Calculate which line the cursor is on
	cursorLine := 0
	cursorPos := i.cursor
	for idx, line := range inputLines {
		lineLen := len([]rune(line))
		if cursorPos <= lineLen {
			cursorLine = idx
			break
		}
		cursorPos -= lineLen
		if idx == len(inputLines)-1 {
			cursorLine = idx
		}
	}

	// Build display
	var displayLines []string

	// Prompt style
	promptStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("117"))

	cursorStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("255")).
		Foreground(lipgloss.Color("235"))

	// Render each line with cursor on the correct line
	for idx, line := range inputLines {
		if idx == 0 {
			// First line has prompt
			if cursorLine == idx {
				// Cursor is on this line
				lineRunes := []rune(line)
				if cursorPos <= len(lineRunes) {
					before := string(lineRunes[:cursorPos])
					after := string(lineRunes[cursorPos:])
					displayLines = append(displayLines,
						promptStyle.Render("  > ")+before+cursorStyle.Render(" ")+after)
				} else {
					displayLines = append(displayLines,
						promptStyle.Render("  > ")+line+cursorStyle.Render(" "))
				}
			} else {
				displayLines = append(displayLines, promptStyle.Render("  > ")+line)
			}
		} else {
			// Subsequent lines
			if cursorLine == idx {
				// Cursor is on this line
				lineRunes := []rune(line)
				if cursorPos <= len(lineRunes) {
					before := string(lineRunes[:cursorPos])
					after := string(lineRunes[cursorPos:])
					displayLines = append(displayLines,
						"     "+before+cursorStyle.Render(" ")+after)
				} else {
					displayLines = append(displayLines,
						"     "+line+cursorStyle.Render(" "))
				}
			} else {
				displayLines = append(displayLines, "     "+line)
			}
		}
	}

	// Fill remaining height (leave room for shortcuts at bottom and top border)
	// Final content should be exactly i.height - 1 lines (border adds 1 line = i.height total)
	targetLines := i.height - 1
	for len(displayLines) < targetLines-1 {
		displayLines = append(displayLines, "")
	}

	// Shortcuts bar at bottom right
	shortcutStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))
	shortcuts := "Ctrl+P:cmds  Ctrl+G:logs  Ctrl+B:sidebar  Ctrl+K:keys"
	// Right-align shortcuts
	padding := i.width - lipgloss.Width(shortcuts) - 2
	if padding < 0 {
		padding = 0
	}
	// Only add shortcuts if we have room
	if len(displayLines) < targetLines {
		displayLines = append(displayLines, strings.Repeat(" ", padding)+shortcutStyle.Render(shortcuts))
	}

	// Ensure exactly targetLines lines
	for len(displayLines) < targetLines {
		displayLines = append(displayLines, "")
	}
	if len(displayLines) > targetLines {
		displayLines = displayLines[:targetLines]
	}

	content := strings.Join(displayLines, "\n")

	// Apply style with top border
	style := lipgloss.NewStyle().
		Width(i.width).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color("243"))

	return style.Render(content)
}

func (i *inputAreaModel) SetSize(width, height int) tea.Cmd {
	i.width = width
	i.height = height
	return nil
}
func (i *inputAreaModel) GetSize() (int, int) { return i.width, i.height }

// Status bar component
type statusBarModel struct {
	width       int
	height      int
	model       string
	panel       string
	sidebarOpen bool
	tokens      int
	cost        float64
	escHint     string // Dynamic ESC hint: "ESC x2: interrupt" or "ESC again: interrupt"
	// Retry status
	retryStatus  string
	retryActive  bool
}

func (s *statusBarModel) Init() tea.Cmd                           { return nil }
func (s *statusBarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return s, nil }
func (s *statusBarModel) View() string {
	if s.width <= 0 {
		return ""
	}

	// Show retry status if active
	if s.retryActive && s.retryStatus != "" {
		// Retry status line with yellow color
		retryLine := lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")). // Yellow
			Render(fmt.Sprintf(" ⚠ %s ", s.retryStatus))
		borderLine := strings.Repeat("-", s.width)
		return borderLine + "\n" + retryLine
	}

	// Left section - dynamic ESC hint (no background for CMD compatibility)
	escHint := s.escHint
	if escHint == "" {
		escHint = "ESC x2: interrupt"
	}
	leftContent := " " + escHint + " "

	// Center section - tokens
	centerContent := fmt.Sprintf("  %s tokens", formatTokensStatic(s.tokens))

	// Right section - model and status
	sidebarStatus := "sidebar: off"
	if s.sidebarOpen {
		sidebarStatus = "sidebar: on"
	}
	rightContent := fmt.Sprintf("%s | %s", s.model, sidebarStatus)

	// Calculate spacing using actual display width (no ANSI codes)
	leftWidth := len(leftContent)
	centerWidth := len(centerContent)
	rightWidth := len(rightContent)

	padding1 := (s.width - leftWidth - centerWidth - rightWidth) / 2
	if padding1 < 1 {
		padding1 = 1
	}
	padding2 := s.width - leftWidth - padding1 - centerWidth - rightWidth
	if padding2 < 1 {
		padding2 = 1
	}

	// Build two lines: border line and content line
	borderLine := strings.Repeat("-", s.width)
	contentLine := leftContent + strings.Repeat(" ", padding1) + centerContent + strings.Repeat(" ", padding2) + rightContent

	// Truncate content line if needed
	contentRunes := []rune(contentLine)
	if len(contentRunes) > s.width {
		contentLine = string(contentRunes[:s.width])
	}
	// Pad content line if needed
	for len(contentRunes) < s.width {
		contentLine += " "
		contentRunes = append(contentRunes, ' ')
	}

	// Apply styles to each section separately
	leftStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
	centerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	rightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	styledLeft := leftStyle.Render(leftContent)
	styledCenter := centerStyle.Render(centerContent)
	styledRight := rightStyle.Render(rightContent)

	styledLine := styledLeft + strings.Repeat(" ", padding1) + styledCenter + strings.Repeat(" ", padding2) + styledRight

	return borderLine + "\n" + styledLine
}

func (s *statusBarModel) SetSize(width, height int) tea.Cmd {
	s.width = width
	s.height = height
	return nil
}
func (s *statusBarModel) GetSize() (int, int) { return s.width, s.height }

// Command palette component
type commandPaletteModel struct {
	width        int
	height       int
	input        string
	cursor       int
	selected     int
	filteredCmds []*command.Command // Store full command objects for rich display
	allCmds      []*command.Command
}

func (c *commandPaletteModel) Init() tea.Cmd                           { return nil }
func (c *commandPaletteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return c, nil }
func (c *commandPaletteModel) View() string {
	// Define colors
	borderColor := lipgloss.Color("62")
	titleColor := lipgloss.Color("117")
	selectedBg := lipgloss.Color("62")
	selectedFg := lipgloss.Color("255")
	normalFg := lipgloss.Color("252")
	descFg := lipgloss.Color("243")

	// Calculate inner width (border adds 2 chars)
	innerWidth := c.width - 2
	if innerWidth < 20 {
		innerWidth = 20
	}

	// Helper: pad string to exact width
	padToWidth := func(text string, width int) string {
		textWidth := lipgloss.Width(text)
		if textWidth > width {
			runes := []rune(text)
			result := ""
			for _, r := range runes {
				if lipgloss.Width(result+string(r)) > width-3 {
					break
				}
				result += string(r)
			}
			return result + "..."
		}
		return text + strings.Repeat(" ", width-textWidth)
	}

	nameWidth := 18 // Fixed width for command name
	descWidth := innerWidth - nameWidth - 6 // Space for prefix and spacing
	if descWidth < 10 {
		descWidth = 10
	}

	// Number of visible commands (reserve lines for title, separator, input, footer)
	visibleCount := c.height - 6
	if visibleCount < 3 {
		visibleCount = 3
	}

	// Calculate scroll
	startIdx := 0
	if c.selected >= visibleCount {
		startIdx = c.selected - visibleCount + 1
	}
	endIdx := startIdx + visibleCount
	if endIdx > len(c.filteredCmds) {
		endIdx = len(c.filteredCmds)
	}

	var lines []string

	// Title bar
	titleStyle := lipgloss.NewStyle().Foreground(titleColor).Bold(true)
	lines = append(lines, titleStyle.Render(padToWidth("  Command Palette", innerWidth)))

	// Separator line
	sepStyle := lipgloss.NewStyle().Foreground(borderColor)
	separator := strings.Repeat("─", innerWidth)
	lines = append(lines, sepStyle.Render(separator))

	// Input line with prompt
	inputStyle := lipgloss.NewStyle().Foreground(normalFg)
	inputLine := "  > " + c.input
	lines = append(lines, inputStyle.Render(padToWidth(inputLine, innerWidth)))

	// Commands list
	for i := startIdx; i < endIdx; i++ {
		cmd := c.filteredCmds[i]

		// Format command name
		cmdName := "/" + cmd.Name
		if len(cmdName) > nameWidth {
			cmdName = string([]rune(cmdName)[:nameWidth-3]) + "..."
		}

		// Format description
		desc := cmd.Description
		if len(desc) > descWidth {
			desc = string([]rune(desc)[:descWidth-3]) + "..."
		}

		line := fmt.Sprintf("  %-18s  %s", cmdName, desc)

		if i == c.selected {
			// Selected item with background
			selectedStyle := lipgloss.NewStyle().
				Foreground(selectedFg).
				Background(selectedBg).
				Bold(true)
			lines = append(lines, selectedStyle.Render(padToWidth(line, innerWidth)))
		} else {
			// Normal item
			nameStyle := lipgloss.NewStyle().Foreground(normalFg)
			descStyle := lipgloss.NewStyle().Foreground(descFg)
			styledLine := nameStyle.Render(fmt.Sprintf("  %-18s", cmdName)) + "  " + descStyle.Render(desc)
			lines = append(lines, padToWidth(styledLine, innerWidth))
		}
	}

	// Fill remaining space with empty lines of correct width
	for len(lines) < c.height-2 {
		lines = append(lines, strings.Repeat(" ", innerWidth))
	}

	// Footer
	lines = append(lines, sepStyle.Render(separator))
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	footer := fmt.Sprintf("  %d commands | Up/Down: navigate | Enter: select | Esc: close", len(c.filteredCmds))
	lines = append(lines, footerStyle.Render(padToWidth(footer, innerWidth)))

	// Ensure exact height
	for len(lines) > c.height {
		lines = lines[:c.height]
	}

	// Wrap in border with explicit total width (including border chars)
	// c.width is the total width including borders
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		Width(c.width - 2). // content width = total - 2 border chars
		Height(c.height)

	return borderStyle.Render(strings.Join(lines, "\n"))
}

func (c *commandPaletteModel) SetSize(width, height int) tea.Cmd {
	c.width = width
	c.height = height
	return nil
}
func (c *commandPaletteModel) GetSize() (int, int) { return c.width, c.height }

// Log viewer component
type logViewerModel struct {
	width    int
	height   int
	lines    []string
	scrollY  int
	maxLines int // Maximum lines to keep
}

func (l *logViewerModel) Init() tea.Cmd                           { return nil }
func (l *logViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return l, nil }
func (l *logViewerModel) View() string {
	if l.width <= 0 || l.height <= 0 {
		return ""
	}

	// Define colors
	borderColor := lipgloss.Color("62")
	titleColor := lipgloss.Color("117")
	timestampColor := lipgloss.Color("243")
	infoColor := lipgloss.Color("252")
	warnColor := lipgloss.Color("214")
	errorColor := lipgloss.Color("196")

	// Calculate inner width (account for border: 2 chars for left+right)
	// When Width() is set, lipgloss includes border in that width
	// So content width = total width - 2 (border chars)
	innerWidth := l.width - 2
	if innerWidth < 20 {
		innerWidth = 20
	}

	// Helper: pad string to exact width (handles ANSI codes)
	padToWidth := func(text string, width int) string {
		textWidth := lipgloss.Width(text)
		if textWidth > width {
			// For log viewer, we want to show full content with wrapping
			// Don't truncate - just return as is, caller will handle wrapping
			return text
		}
		return text + strings.Repeat(" ", width-textWidth)
	}

	// Helper: wrap long text to multiple lines
	wrapLine := func(text string, width int) []string {
		textWidth := lipgloss.Width(text)
		if textWidth <= width {
			return []string{text + strings.Repeat(" ", width-textWidth)}
		}

		// Need to wrap - handle ANSI codes properly
		var lines []string
		currentLine := ""
		currentWidth := 0

		runes := []rune(text)
		for i := 0; i < len(runes); i++ {
			r := runes[i]

			// Handle ANSI escape sequences (don't count them in width)
			if r == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
				// Find end of ANSI sequence
				j := i + 2
				for j < len(runes) && !(runes[j] >= 'A' && runes[j] <= 'Z' || runes[j] >= 'a' && runes[j] <= 'z') {
					j++
				}
				if j < len(runes) {
					ansiSeq := string(runes[i : j+1])
					currentLine += ansiSeq
					i = j
					continue
				}
			}

			charWidth := lipgloss.Width(string(r))
			if currentWidth+charWidth > width {
				// Wrap this line
				lines = append(lines, currentLine+strings.Repeat(" ", width-currentWidth))
				currentLine = string(r)
				currentWidth = charWidth
			} else {
				currentLine += string(r)
				currentWidth += charWidth
			}
		}

		if currentWidth > 0 {
			lines = append(lines, currentLine+strings.Repeat(" ", width-currentWidth))
		}

		return lines
	}

	var lines []string

	// Title bar
	titleStyle := lipgloss.NewStyle().Foreground(titleColor).Bold(true)
	titleText := "  Log Viewer"
	lines = append(lines, titleStyle.Render(padToWidth(titleText, innerWidth)))

	// Separator
	sepStyle := lipgloss.NewStyle().Foreground(borderColor)
	separator := strings.Repeat("─", innerWidth)
	lines = append(lines, sepStyle.Render(separator))

	// Calculate visible log lines
	headerLines := 2
	footerLines := 2
	availableLines := l.height - headerLines - footerLines
	if availableLines < 3 {
		availableLines = 3
	}

	// Get visible range
	totalLogs := len(l.lines)
	startIdx := l.scrollY
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > totalLogs-availableLines && totalLogs > availableLines {
		startIdx = totalLogs - availableLines
	}
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + availableLines
	if endIdx > totalLogs {
		endIdx = totalLogs
	}

	// Display log lines with consistent width
	if totalLogs == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		lines = append(lines, emptyStyle.Render(padToWidth("  No logs yet...", innerWidth)))
	} else {
		// First, wrap all log lines and collect them
		var wrappedLines []struct {
			text  string
			level string // "error", "warn", "info", "debug"
		}

		for i := 0; i < totalLogs; i++ {
			logLine := "  " + l.lines[i]
			lowerLine := strings.ToLower(l.lines[i])

			// Determine log level
			level := "debug"
			if strings.Contains(lowerLine, "[error]") {
				level = "error"
			} else if strings.Contains(lowerLine, "[warn]") {
				level = "warn"
			} else if strings.Contains(lowerLine, "[info]") {
				level = "info"
			} else if strings.Contains(lowerLine, "[debug]") {
				level = "debug"
			} else if strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "fail") {
				level = "error"
			} else if strings.Contains(lowerLine, "warn") || strings.Contains(lowerLine, "warning") {
				level = "warn"
			}

			// Wrap the line and add each wrapped line
			wrapped := wrapLine(logLine, innerWidth)
			for _, w := range wrapped {
				wrappedLines = append(wrappedLines, struct {
					text  string
					level string
				}{text: w, level: level})
			}
		}

		// Calculate visible range for wrapped lines
		totalWrapped := len(wrappedLines)
		startWrappedIdx := l.scrollY
		if startWrappedIdx < 0 {
			startWrappedIdx = 0
		}
		if startWrappedIdx > totalWrapped-availableLines && totalWrapped > availableLines {
			startWrappedIdx = totalWrapped - availableLines
		}
		endWrappedIdx := startWrappedIdx + availableLines
		if endWrappedIdx > totalWrapped {
			endWrappedIdx = totalWrapped
		}

		// Display wrapped lines with appropriate styling
		for i := startWrappedIdx; i < endWrappedIdx; i++ {
			wl := wrappedLines[i]
			var styledLine string
			switch wl.level {
			case "error":
				styledLine = lipgloss.NewStyle().Foreground(errorColor).Render(wl.text)
			case "warn":
				styledLine = lipgloss.NewStyle().Foreground(warnColor).Render(wl.text)
			case "info":
				styledLine = lipgloss.NewStyle().Foreground(infoColor).Render(wl.text)
			default:
				styledLine = lipgloss.NewStyle().Foreground(timestampColor).Render(wl.text)
			}
			lines = append(lines, styledLine)
		}
	}

	// Fill remaining space with empty lines of correct width
	for len(lines) < l.height-2 {
		lines = append(lines, strings.Repeat(" ", innerWidth))
	}

	// Footer with scroll info
	lines = append(lines, sepStyle.Render(separator))
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	scrollInfo := fmt.Sprintf("  %d/%d logs | Up/Down: scroll | Esc: close", endIdx, totalLogs)
	if totalLogs == 0 {
		scrollInfo = "  0 logs | Esc: close"
	}
	lines = append(lines, footerStyle.Render(padToWidth(scrollInfo, innerWidth)))

	// Ensure exact height
	for len(lines) > l.height {
		lines = lines[:l.height]
	}

	// Wrap in border with explicit total width (including border chars)
	// l.width is the total width including borders, so border style should use it directly
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		Width(l.width - 2). // content width = total - 2 border chars
		Height(l.height)

	result := borderStyle.Render(strings.Join(lines, "\n"))

	return result
}

func (l *logViewerModel) SetSize(width, height int) tea.Cmd {
	l.width = width
	l.height = height
	return nil
}

func (l *logViewerModel) AddLine(line string) {
	l.lines = append(l.lines, line)
	// Keep only last maxLines
	if len(l.lines) > l.maxLines {
		l.lines = l.lines[len(l.lines)-l.maxLines:]
	}
	// Auto-scroll to bottom
	l.scrollY = len(l.lines)
}

func (l *logViewerModel) Scroll(delta int) {
	l.scrollY += delta
	maxScroll := len(l.lines)
	if l.scrollY < 0 {
		l.scrollY = 0
	}
	if l.scrollY > maxScroll {
		l.scrollY = maxScroll
	}
}

// helpViewerModel displays keyboard shortcuts and commands as an overlay
type helpViewerModel struct {
	width   int
	height  int
	scrollY int
}

func (h *helpViewerModel) Init() tea.Cmd                           { return nil }
func (h *helpViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return h, nil }

func (h *helpViewerModel) View() string {
	if h.width <= 0 || h.height <= 0 {
		return ""
	}

	// Define colors
	borderColor := lipgloss.Color("62")
	titleColor := lipgloss.Color("117")
	keyColor := lipgloss.Color("214")
	descColor := lipgloss.Color("252")
	sectionColor := lipgloss.Color("117")

	innerWidth := h.width - 2
	if innerWidth < 20 {
		innerWidth = 20
	}

	// Helper: pad string to exact width
	padToWidth := func(text string, width int) string {
		textWidth := lipgloss.Width(text)
		if textWidth > width {
			runes := []rune(text)
			result := ""
			for _, r := range runes {
				if lipgloss.Width(result+string(r)) > width-3 {
					break
				}
				result += string(r)
			}
			return result + "..."
		}
		return text + strings.Repeat(" ", width-textWidth)
	}

	var lines []string

	// Title bar
	titleStyle := lipgloss.NewStyle().Foreground(titleColor).Bold(true)
	lines = append(lines, titleStyle.Render(padToWidth("  ⌨ Keyboard Shortcuts", innerWidth)))

	// Separator
	sepStyle := lipgloss.NewStyle().Foreground(borderColor)
	separator := strings.Repeat("─", innerWidth)
	lines = append(lines, sepStyle.Render(separator))

	// Build content
	keyStyle := lipgloss.NewStyle().Foreground(keyColor)
	descStyle := lipgloss.NewStyle().Foreground(descColor)
	sectionStyle := lipgloss.NewStyle().Foreground(sectionColor).Bold(true)

	var contentLines []string

	// Keyboard shortcuts section
	contentLines = append(contentLines, sectionStyle.Render("  Shortcuts"))
	contentLines = append(contentLines, "")
	shortcuts := []struct {
		key  string
		desc string
	}{
		{"Ctrl+P", "Command palette"},
		{"Ctrl+G", "Log viewer"},
		{"Ctrl+K", "Help (this panel)"},
		{"Ctrl+B", "Toggle sidebar"},
		{"Ctrl+L", "Clear chat"},
		{"Ctrl+U", "Clear input"},
		{"Ctrl+A/E", "Cursor to start/end of line"},
		{"Ctrl+W", "Delete word before cursor"},
		{"Ctrl+←/→", "Jump by word"},
		{"↑/↓", "History (empty input)"},
		{"Enter", "Accept suggestion"},
		{"Esc x2", "Interrupt operation"},
	}

	for _, s := range shortcuts {
		line := fmt.Sprintf("    %s  %s",
			keyStyle.Render(fmt.Sprintf("%-12s", s.key)),
			descStyle.Render(s.desc))
		contentLines = append(contentLines, padToWidth(line, innerWidth))
	}

	// Calculate visible range
	headerLines := 2
	footerLines := 2
	availableLines := h.height - headerLines - footerLines
	if availableLines < 3 {
		availableLines = 3
	}

	totalContent := len(contentLines)
	startIdx := h.scrollY
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > totalContent-availableLines && totalContent > availableLines {
		startIdx = totalContent - availableLines
	}
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + availableLines
	if endIdx > totalContent {
		endIdx = totalContent
	}

	// Add visible content lines
	for i := startIdx; i < endIdx; i++ {
		lines = append(lines, contentLines[i])
	}

	// Fill remaining height
	for len(lines) < h.height-footerLines {
		lines = append(lines, padToWidth("", innerWidth))
	}

	// Footer with scroll info
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	scrollInfo := fmt.Sprintf("  %d/%d | ↑/↓: scroll | Esc: close", startIdx+1, totalContent)
	if totalContent <= availableLines {
		scrollInfo = "  Esc: close"
	}
	lines = append(lines, footerStyle.Render(padToWidth(scrollInfo, innerWidth)))

	// Ensure exact height
	for len(lines) > h.height {
		lines = lines[:h.height]
	}

	// Wrap in border
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		Width(h.width - 2).
		Height(h.height)

	return borderStyle.Render(strings.Join(lines, "\n"))
}

func formatTokensStatic(tokens int) string {
	if tokens < 1000 {
		return fmt.Sprintf("%d", tokens)
	} else if tokens < 1000000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1000)
	} else {
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
	}
}

// NewModernTUIModel creates a new modern TUI model
func NewModernTUIModel(orch *agent.Orchestrator, cfg *config.Config, cmdManager *command.CommandManager, connectedLSPServers []string, connectedMCPServers []string, sensitiveOpManager *audit.SensitiveOperationManager) *ModernTUIModel {
	_ = sensitiveOpManager // Will be used for sensitive operation confirmation
	// Initialize session manager
	var sessionManager *session.Manager
	var currentSession *session.Session

	sm, err := session.NewManager()
	if err == nil {
		sessionManager = sm
		sess, err := sessionManager.Create("New Session")
		if err == nil {
			currentSession = sess
		}
	}

	// Initialize status maps
	mcpStatus := make(map[string]string)
	lspStatus := make(map[string]string)
	toolStatus := make(map[string]string)

	for _, server := range cfg.MCP.Servers {
		if server.Enabled {
			// Check if this server is in the connected list
			connected := false
			for _, name := range connectedMCPServers {
				if name == server.Name {
					connected = true
					break
				}
			}
			if connected {
				mcpStatus[server.Name] = "connected"
			} else {
				mcpStatus[server.Name] = "disconnected"
			}
		}
	}

	for _, server := range cfg.LSP.Servers {
		if server.Enabled {
			// Check if this server is in the connected list
			connected := false
			for _, name := range connectedLSPServers {
				if name == server.Name {
					connected = true
					break
				}
			}
			if connected {
				lspStatus[server.Name] = "connected"
			} else {
				lspStatus[server.Name] = "disconnected"
			}
		}
	}

	// Load sessions
	var sessions []session.Session
	if sessionManager != nil {
		loadedSessions, err := sessionManager.List()
		if err == nil {
			sessions = loadedSessions
		}
	}

	// Create UI components
	// Get working directory for title bar
	workDir, _ := os.Getwd()
	titleBar := &titleBarModel{
		title:      " ycode v1.0.0 ",
		model:      cfg.LLM.Model,
		workingDir: workDir,
	}
	sidebar := &sidebarModel{
		sessionTitle: "New Session",
		sessions:     sessions,
		currentSess:  currentSession,
		mcpStatus:    mcpStatus,
		lspStatus:    lspStatus,
	}
	mainArea := &mainAreaModel{
		messages:    []ChatMessageFinal{},
		showWelcome: true,
	}
	inputArea := &inputAreaModel{}
	statusBar := &statusBarModel{
		model:       cfg.LLM.Model,
		sidebarOpen: true,
	}

	return &ModernTUIModel{
		orchestrator:     orch,
		config:          cfg,
		cmdManager:      cmdManager,
		sessionManager:  sessionManager,
		currentSession:  currentSession,
		sessionTitle:    "New Session",
		sessions:        sessions,
		messages:        []ChatMessageFinal{},
		showWelcome:     true,
		sidebarOpen:     true,
		activePanel:     0,
		minSendInterval: 500 * time.Millisecond,
		streamDebounce:  50 * time.Millisecond,
		mcpStatus:       mcpStatus,
		lspStatus:       lspStatus,
		toolStatus:      toolStatus,
		currentDir:      ".",
		files:           []string{},
		selectedFile:    0,
		logLines:        []string{},
		titleBar:        titleBar,
		sidebar:         sidebar,
		mainArea:        mainArea,
		inputArea:       inputArea,
		statusBar:       statusBar,
		diffViewer:      NewDiffViewerModel(),

		// Confirmation dialog
		confirmationDialog: NewConfirmationDialogModel(),

		// Autocomplete
		autocomplete:    NewAutoCompleter(cmdManager, ""),
		showAutocomplete: false,
	}
}

// SetRetryCallback sets up the retry callback on the LLM client
func (m *ModernTUIModel) SetRetryCallback() {
	// Get the anthropic client from the orchestrator and set up retry callback
	if m.orchestrator != nil {
		// We need to access the underlying AnthropicClient to set the callback
		// This is done through the LLMClient interface using type assertion
	}
}

// OnRetry is called when a retry occurs (to be connected via callback)
func (m *ModernTUIModel) OnRetry(attempt int, reason string, delay time.Duration) {
	m.retryActive = true
	m.retryAttempt = attempt
	m.retryStatus = fmt.Sprintf("Retrying (%d/3): %s in %.0fs", attempt, reason, delay.Seconds())
	m.retryCountdown = int(delay.Seconds())
	m.updateComponents()
}

// ClearRetryStatus clears the retry status
func (m *ModernTUIModel) ClearRetryStatus() {
	m.retryActive = false
	m.retryStatus = ""
	m.retryAttempt = 0
	m.retryCountdown = 0
	m.updateComponents()
}

// Init initializes the model
func (m *ModernTUIModel) Init() tea.Cmd {
	// Load existing logs that were collected before TUI started
	existingEntries := logger.GetEntries()
	for _, entry := range existingEntries {
		timeStr := entry.Time.Format("15:04:05")
		logLine := fmt.Sprintf("[%s] [%s] [%s] %s", timeStr, strings.ToUpper(entry.Level), entry.Source, entry.Message)
		m.logLines = append(m.logLines, logLine)
	}

	// Set up logger callback to collect new logs
	logger.SetCallback(func(entry logger.LogEntry) {
		timeStr := entry.Time.Format("15:04:05")
		logLine := fmt.Sprintf("[%s] [%s] [%s] %s", timeStr, strings.ToUpper(entry.Level), entry.Source, entry.Message)
		m.logLines = append(m.logLines, logLine)
		// Keep only last 1000 lines
		if len(m.logLines) > 1000 {
			m.logLines = m.logLines[len(m.logLines)-1000:]
		}
	})

	// Start polling retry status and enable mouse for scroll
	return tea.Batch(pollRetryStatus(), func() tea.Msg { return tea.EnableMouseCellMotion() })
}

// Update handles messages
func (m *ModernTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.recalculateLayout()
		m.updateComponents()

	case retryStatusMsg:
		// Poll retry status from LLM client
		if m.orchestrator != nil {
			if rs := m.orchestrator.GetRetryStatus(); rs != nil {
				active, attempt, reason, delay := rs.Get()
				if active {
					m.retryActive = true
					m.retryAttempt = attempt
					m.retryStatus = fmt.Sprintf("Retrying (%d/3): %s in %.0fs", attempt, reason, delay.Seconds())
				} else {
					m.retryActive = false
					m.retryStatus = ""
				}
				m.updateComponents()
			}
		}
		return m, pollRetryStatus()

	case tea.KeyMsg:
		// Handle diff viewer confirmation keys first
		if m.diffViewer != nil && m.diffViewer.IsVisible() {
			return m.updateDiffViewer(msg)
		}

		// Handle confirmation dialog
		if m.confirmationDialog != nil && m.confirmationDialog.IsVisible() {
			return m.updateConfirmationDialog(msg)
		}

		if m.showPalette {
			return m.updatePalette(msg)
		}
		if m.showLogViewer {
			return m.updateLogViewer(msg)
		}
		if m.showHelpViewer {
			return m.updateHelpViewer(msg)
		}

		switch msg.Type {
		case tea.KeyEsc:
			// Cancel autocomplete if active
			if m.showAutocomplete {
				m.autoCompleteCancel()
				m.showAutocomplete = false
				return m, nil
			}

			// Double-press ESC to interrupt current action
			now := time.Now()
			if m.escActive && now.Sub(m.lastEscPress) < 500*time.Millisecond {
				// Second ESC press - interrupt current action
				m.escActive = false
				m.escCount = 0
				m.isInterrupted = true
				// Clear retry status
				m.retryActive = false
				m.retryStatus = ""
				// Cancel any ongoing operation
				if m.cancelFunc != nil {
					m.cancelFunc()
					m.cancelFunc = nil
				}
				// Get partial content from interrupted stream
				if m.orchestrator != nil {
					if content, thinking := m.orchestrator.GetPartialContent(); content != "" || thinking != "" {
						// Add partial content as a message
						partialMsg := "**[Interrupted - Partial Response]**\n\n"
						if thinking != "" {
							partialMsg += "**Thinking:**\n" + thinking + "\n\n"
						}
						if content != "" {
							partialMsg += content
						}
						m.messages = append(m.messages, ChatMessageFinal{
							Role:      "assistant",
							Content:   partialMsg,
							Timestamp: time.Now(),
						})
						m.orchestrator.ClearPartialContent()
					}
				}
				m.updateComponents()
				return m, nil
			} else {
				// First ESC press - show "press again" hint
				m.escActive = true
				m.escCount = 1
				m.lastEscPress = now
				m.updateComponents()
				return m, nil
			}

		case tea.KeyCtrlP:
			m.showPalette = true
			m.paletteInput = ""
			m.paletteCursor = 0
			m.paletteSelected = 0
			m.updateFilteredCommands()
			return m, nil

		case tea.KeyCtrlG:
			// Toggle log viewer
			m.showLogViewer = !m.showLogViewer
			if m.showLogViewer {
				m.logScrollY = len(m.logLines)
			}
			return m, nil

		case tea.KeyCtrlB:
			m.sidebarOpen = !m.sidebarOpen
			m.recalculateLayout()
			m.updateComponents()
			return m, nil

		case tea.KeyCtrlS:
			// Toggle session selection mode
			m.resetESCState()
			if m.sidebar.focusMode == "recent" {
				m.sidebar.focusMode = "chat"
			} else {
				m.sidebar.focusMode = "recent"
				m.sidebar.selectedIndex = 0
			}
			m.updateComponents()
			return m, nil

		case tea.KeyCtrlL:
			m.messages = []ChatMessageFinal{}
			m.mainArea.messages = m.messages
			m.showWelcome = true
			m.mainArea.showWelcome = true
			m.updateContextStats()
			m.recalculateLayout()
			m.updateComponents()
			return m, nil

		case tea.KeyCtrlU:
			m.resetESCState()
			m.input = ""
			m.cursor = 0
			m.updateComponents()
			return m, nil

		case tea.KeyCtrlA:
			m.resetESCState()
			m.cursor = 0
			m.updateComponents()
			return m, nil

		case tea.KeyCtrlE:
			m.resetESCState()
			m.cursor = len([]rune(m.input))
			m.updateComponents()
			return m, nil

		case tea.KeyCtrlW:
			// Delete word before cursor
			m.resetESCState()
			m.deleteWordBeforeCursor()
			return m, nil

		case tea.KeyCtrlK:
			// Toggle help viewer
			m.showHelpViewer = !m.showHelpViewer
			m.helpScrollY = 0
			return m, nil

		case tea.KeyCtrlLeft:
			// Move cursor left by one word
			m.resetESCState()
			m.moveWordLeft()
			return m, nil

		case tea.KeyCtrlRight:
			// Move cursor right by one word
			m.resetESCState()
			m.moveWordRight()
			return m, nil

		case tea.KeyEnter:
			m.resetESCState()

			// Clean input: remove null bytes and other control characters, then trim
			// This handles Windows CMD terminal encoding issues
			cleanInput := strings.Map(func(r rune) rune {
				if r == 0 || r < 32 && r != '\t' && r != '\n' && r != '\r' {
					return -1 // Remove control characters including null bytes
				}
				return r
			}, m.input)
			inputTrimmed := strings.TrimSpace(cleanInput)

			if len(inputTrimmed) == 0 {
				return m, nil
			}

			firstRune := []rune(inputTrimmed)[0]

			// Check for half-width slash /
			if strings.HasPrefix(inputTrimmed, "/") {
				cmdName := strings.TrimPrefix(inputTrimmed, "/")
				logger.Debug("ui", "Checking command: cmdName='%s', hasCommand=%v", cmdName, m.cmdManager.HasCommand(cmdName))
				// Check if it's a valid command (not a file path)
				if !strings.Contains(cmdName, "/") && !strings.Contains(cmdName, "\\") && m.cmdManager.HasCommand(cmdName) {
					// Valid command - cancel autocomplete and execute
					logger.Debug("ui", "Executing command: %s", cmdName)
					m.showAutocomplete = false
					if m.autocomplete != nil {
						m.autocomplete.Cancel()
					}
					m.showWelcome = false
					m.mainArea.showWelcome = false
					m.lastSendTime = time.Now()
					m.input = inputTrimmed // Use cleaned input
					return m, m.handleCommand(inputTrimmed)
				}
			} else if firstRune == '／' {
				// Full-width slash (Japanese/Chinese IME) - convert and try again
				runes := []rune(inputTrimmed)
				cmdName := string(runes[1:])
				logger.Debug("ui", "Full-width slash detected, cmdName='%s', hasCommand=%v", cmdName, m.cmdManager.HasCommand(cmdName))
				if m.cmdManager.HasCommand(cmdName) {
					logger.Debug("ui", "Executing command (full-width): %s", cmdName)
					m.showAutocomplete = false
					if m.autocomplete != nil {
						m.autocomplete.Cancel()
					}
					m.showWelcome = false
					m.mainArea.showWelcome = false
					m.lastSendTime = time.Now()
					return m, m.handleCommand("/" + cmdName)
				}
			}

			// Handle autocomplete acceptance (only if no exact command match)
			if m.showAutocomplete && m.autocomplete != nil && m.autocomplete.GetState().Active {
				newInput, newCursor := m.autocomplete.Accept(m.input, m.cursor)
				m.input = newInput
				m.cursor = newCursor
				m.showAutocomplete = false
				m.updateComponents()
				return m, nil
			}

			// If in session selection mode, switch to selected session
			if m.sidebar.focusMode == "recent" && m.sidebarOpen && len(m.sidebar.sessions) > 0 {
				if m.sidebar.selectedIndex < len(m.sidebar.sessions) {
					selectedSession := m.sidebar.sessions[m.sidebar.selectedIndex]
					m.sidebar.focusMode = "chat"
					return m, m.switchToSession(selectedSession.ID)
				}
			}

			if time.Since(m.lastSendTime) < m.minSendInterval {
				return m, nil
			}
			if m.input != "" && !m.loading {
				m.showWelcome = false
				m.mainArea.showWelcome = false
				m.lastSendTime = time.Now()

				// Check if input starts with "/" but is actually a file path (contains path separators)
				if strings.HasPrefix(m.input, "/") {
					// If it contains additional path separators, it's a file path, not a command
					remaining := m.input[1:]
					if strings.Contains(remaining, "/") || strings.Contains(remaining, "\\") {
						// It's a file path, send as regular message
						return m, m.sendMessage()
					}
					return m, m.handleCommand(m.input)
				}

				return m, m.sendMessage()
			}

		case tea.KeyBackspace, tea.KeyCtrlH:
			if len(m.input) > 0 && m.cursor > 0 {
				runes := []rune(m.input)
				if m.cursor <= len(runes) {
					m.input = string(runes[:m.cursor-1]) + string(runes[m.cursor:])
					m.cursor--
					m.updateComponents()
					// Update autocomplete suggestions after backspace
					m.updateAutocompleteOnChange()
				}
			}

		case tea.KeyDelete:
			if len(m.input) > 0 && m.cursor < len([]rune(m.input)) {
				runes := []rune(m.input)
				m.input = string(runes[:m.cursor]) + string(runes[m.cursor+1:])
				m.updateComponents()
				// Update autocomplete suggestions after delete
				m.updateAutocompleteOnChange()
			}

		case tea.KeyLeft:
			if m.cursor > 0 {
				m.cursor--
				m.updateComponents()
			}

		case tea.KeyRight:
			if m.cursor < len([]rune(m.input)) {
				m.cursor++
				m.updateComponents()
			}

		case tea.KeyUp:
			// Handle autocomplete navigation first
			if m.showAutocomplete && m.autocomplete != nil && m.autocomplete.GetState().Active {
				m.autoCompleteNavigate(-1)
				return m, nil
			}

			if m.sidebar.focusMode == "recent" && m.sidebarOpen {
				if m.sidebar.selectedIndex > 0 {
					m.sidebar.selectedIndex--
					// Update scroll offset if needed
					if m.sidebar.selectedIndex < m.sidebar.scrollOffset {
						m.sidebar.scrollOffset = m.sidebar.selectedIndex
					}
					m.updateComponents()
				}
			} else if len(m.inputHistory) > 0 {
				// Navigate input history
				m.navigateInputHistory(-1)
			}

		case tea.KeyDown:
			// Handle autocomplete navigation first
			if m.showAutocomplete && m.autocomplete != nil && m.autocomplete.GetState().Active {
				m.autoCompleteNavigate(1)
				return m, nil
			}

			if m.sidebar.focusMode == "recent" && m.sidebarOpen {
				if m.sidebar.selectedIndex < len(m.sidebar.sessions)-1 {
					m.sidebar.selectedIndex++
					// Update scroll offset if needed (visible count = 15 or less)
					visibleCount := 10
					if m.sidebar.selectedIndex >= m.sidebar.scrollOffset+visibleCount {
						m.sidebar.scrollOffset = m.sidebar.selectedIndex - visibleCount + 1
					}
					m.updateComponents()
				}
			} else if m.inputHistoryIndex < len(m.inputHistory)-1 {
				// Navigate input history
				m.navigateInputHistory(1)
			}

		case tea.KeyPgUp:
			m.scroll(-m.visibleLines)
			m.updateComponents()

		case tea.KeyPgDown:
			m.scroll(m.visibleLines)
			m.updateComponents()

		case tea.KeyHome:
			m.scrollY = 0
			m.updateComponents()

		case tea.KeyEnd:
			m.scrollY = m.totalLines
			m.updateComponents()

		case tea.KeyRunes:
			runes := []rune(m.input)
			m.input = string(runes[:m.cursor]) + string(msg.Runes) + string(runes[m.cursor:])
			m.cursor += len(msg.Runes)
			m.updateComponents()

			// Auto-trigger autocomplete based on input
			if m.autocomplete == nil {
				m.autocomplete = NewAutoCompleter(m.cmdManager, "")
			}
			// Update working directory
			if cwd, err := os.Getwd(); err == nil {
				m.autocomplete.SetWorkingDir(cwd)
			}
			// Check if autocomplete should auto-trigger
			if m.autocomplete.ShouldTrigger(m.input, m.cursor) {
				if m.autocomplete.Trigger(m.input, m.cursor) {
					m.showAutocomplete = true
				} else {
					m.showAutocomplete = false
				}
			} else {
				// Hide autocomplete if input no longer matches trigger conditions
				if m.showAutocomplete {
					m.autocomplete.Cancel()
					m.showAutocomplete = false
				}
			}
		}

	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			// If log viewer is open, scroll it instead of main area
			if m.showLogViewer {
				if m.logScrollY > 0 {
					m.logScrollY -= 3
					if m.logScrollY < 0 {
						m.logScrollY = 0
					}
				}
			} else if m.showHelpViewer {
				if m.helpScrollY > 0 {
					m.helpScrollY -= 3
					if m.helpScrollY < 0 {
						m.helpScrollY = 0
					}
				}
			} else {
				m.scroll(-3)
				m.updateComponents()
			}
		case tea.MouseWheelDown:
			// If log viewer is open, scroll it instead of main area
			if m.showLogViewer {
				m.logScrollY += 3
			} else if m.showHelpViewer {
				m.helpScrollY += 3
			} else {
				m.scroll(3)
				m.updateComponents()
			}
		}

	case AgentResponseMsgFinal:
		m.removeLoadingMessage()

		m.messages = append(m.messages, ChatMessageFinal{
			Role:      "assistant",
			Content:   msg.Content,
			Timestamp: time.Now(),
		})
		m.mainArea.messages = m.messages
		m.loading = false
		m.inputArea.loading = false
		m.recalculateLayout()
		m.scrollToBottom()
		m.updateComponents()
		m.saveSession()

	case AgentStreamMsgFinal:
		if msg.Error != nil {
			// Handle error during streaming
			m.removeLoadingMessage()
			errorContent := m.formatError(msg.Error)
			m.messages = append(m.messages, ChatMessageFinal{
				Role:      "error",
				Content:   errorContent,
				Timestamp: time.Now(),
			})
			m.mainArea.messages = m.messages
			m.loading = false
			m.inputArea.loading = false
			m.currentToolCall = nil // Clear tool state on error
			m.recalculateLayout()
			m.scrollToBottom()
			m.updateComponents()
		} else if msg.Done {
			// Streaming complete
			m.removeLoadingMessage()
			if msg.Content != "" {
				m.messages = append(m.messages, ChatMessageFinal{
					Role:      "assistant",
					Content:   msg.Content,
					Thinking:  msg.Thinking,
					Timestamp: time.Now(),
				})
			}
			m.mainArea.messages = m.messages
			m.loading = false
			m.inputArea.loading = false
			m.currentToolCall = nil // Clear tool state on complete
			m.toolCallHistory = nil // Clear tool history
			m.recalculateLayout()
			m.scrollToBottom()
			m.updateComponents()
			m.saveSession()
		} else {
			// Handle tool call status
			if msg.ToolCall != nil {
				m.currentToolCall = msg.ToolCall
				m.toolCallHistory = append(m.toolCallHistory, *msg.ToolCall)
				// Sync to mainArea for display
				m.mainArea.currentToolCall = msg.ToolCall
				m.mainArea.toolCallHistory = m.toolCallHistory
			}
			// Update streaming content with debouncing to avoid UI freeze
			// when thinking content is very long
			if time.Since(m.lastStreamUpdate) >= m.streamDebounce {
				m.updateStreamingContentWithThinking(msg.Content, msg.Thinking)
				m.lastStreamUpdate = time.Now()
			} else {
				// Still update the content but don't recalculate layout
				for i := len(m.messages) - 1; i >= 0; i-- {
					if m.messages[i].Role == "loading" {
						m.messages[i].Content = msg.Content
						m.messages[i].Thinking = msg.Thinking
						break
					}
				}
			}
			return m, m.readStreamChan(m.streamChan)
		}

	case AgentDoneMsgFinal:
		m.removeLoadingMessage()
		m.loading = false
		m.inputArea.loading = false
		m.updateComponents()

	case AgentErrorMsgFinal:
		m.removeLoadingMessage()

		errorContent := m.formatError(msg.Error)
		m.messages = append(m.messages, ChatMessageFinal{
			Role:      "error",
			Content:   errorContent,
			Timestamp: time.Now(),
		})
		m.mainArea.messages = m.messages
		m.loading = false
		m.inputArea.loading = false
		m.recalculateLayout()
		m.scrollToBottom()
		m.updateComponents()

	case StreamTickMsg:
		if m.loading && m.streamingContent != "" {
			m.updateStreamingContent(m.streamingContent)
		}
		return m, streamTickCmd(m.streamDebounce)
	}

	return m, nil
}

// updateComponents syncs state to UI components
func (m *ModernTUIModel) updateComponents() {
	m.titleBar.title = " ycode v1.0.0 "
	m.sidebar.sessionTitle = m.sessionTitle
	m.sidebar.sessions = m.sessions
	m.sidebar.currentSess = m.currentSession
	m.sidebar.mcpStatus = m.mcpStatus
	m.sidebar.lspStatus = m.lspStatus
	m.sidebar.tokens = m.contextTokens
	m.mainArea.messages = m.messages
	m.mainArea.scrollY = m.scrollY
	m.mainArea.showWelcome = m.showWelcome
	m.inputArea.input = m.input
	m.inputArea.cursor = m.cursor
	m.inputArea.loading = m.loading
	m.statusBar.model = m.config.LLM.Model
	m.statusBar.sidebarOpen = m.sidebarOpen
	m.statusBar.tokens = m.contextTokens
	// Update dynamic ESC hint
	if m.escActive {
		m.statusBar.escHint = "ESC again: interrupt"
	} else {
		m.statusBar.escHint = "ESC x2: interrupt"
	}
	// Update retry status
	m.statusBar.retryStatus = m.retryStatus
	m.statusBar.retryActive = m.retryActive
}

// View renders the modern layout
func (m *ModernTUIModel) View() string {
	if !m.ready {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Render("Initializing ycode...")
	}

	// Calculate dimensions using constants
	contentHeight := m.height - titleBarHeight - statusBarHeight - inputAreaHeight
	if contentHeight < minContentHeight {
		contentHeight = minContentHeight
	}

	sidebarWidth := 0
	if m.sidebarOpen {
		sidebarWidth = m.width / 4
		if sidebarWidth < minSidebarWidth {
			sidebarWidth = minSidebarWidth
		}
		if sidebarWidth > maxSidebarWidth {
			sidebarWidth = maxSidebarWidth
		}
	}
	mainWidth := m.width - sidebarWidth
	if mainWidth < minMainWidth {
		mainWidth = minMainWidth
	}

	// Set component sizes
	m.titleBar.SetSize(m.width, titleBarHeight)
	// Sidebar internally handles the border width adjustment
	m.sidebar.SetSize(sidebarWidth, contentHeight)
	m.mainArea.SetSize(mainWidth, contentHeight)
	m.inputArea.SetSize(m.width, inputAreaHeight)
	m.statusBar.SetSize(m.width, statusBarHeight)

	// Render content area (sidebar + main)
	sidebarView := m.sidebar.View()
	mainView := m.mainArea.View()
	var contentView string
	if m.sidebarOpen {
		contentView = lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, mainView)
	} else {
		contentView = mainView
	}

	// Render input area
	inputView := m.inputArea.View()

	// Render status bar
	statusView := m.statusBar.View()

	// Combine all vertically (no title bar)
	result := lipgloss.JoinVertical(lipgloss.Left,
		contentView,
		inputView,
		statusView,
	)

	// Overlay command palette if open
	if m.showPalette {
		// Larger palette for better visibility
		paletteWidth := m.width * 4 / 5
		if paletteWidth < 60 {
			paletteWidth = 60
		}
		paletteHeight := m.height * 2 / 3
		if paletteHeight < 15 {
			paletteHeight = 15
		}
		if paletteHeight > m.height-4 {
			paletteHeight = m.height - 4
		}

		palette := &commandPaletteModel{
			width:        paletteWidth,
			height:       paletteHeight,
			input:        m.paletteInput,
			cursor:       m.paletteCursor,
			selected:     m.paletteSelected,
			filteredCmds: m.filteredCommands,
			allCmds:      m.cmdManager.List(),
		}

		paletteView := palette.View()
		paletteW := lipgloss.Width(paletteView)
		paletteH := lipgloss.Height(paletteView)
		x := (m.width - paletteW) / 2
		y := (m.height - paletteH) / 2

		return layout.PlaceOverlay(x, y, paletteView, result, true)
	}

	// Overlay log viewer if open
	if m.showLogViewer {
		logWidth := m.width * 4 / 5
		if logWidth < 60 {
			logWidth = 60
		}
		logHeight := m.height * 2 / 3
		if logHeight < 15 {
			logHeight = 15
		}
		if logHeight > m.height-4 {
			logHeight = m.height - 4
		}

		logViewer := &logViewerModel{
			width:    logWidth,
			height:   logHeight,
			lines:    m.logLines,
			scrollY:  m.logScrollY,
			maxLines: 1000,
		}

		logView := logViewer.View()
		logW := lipgloss.Width(logView)
		logH := lipgloss.Height(logView)
		x := (m.width - logW) / 2
		y := (m.height - logH) / 2

		return layout.PlaceOverlay(x, y, logView, result, true)
	}

	// Overlay help viewer if open
	if m.showHelpViewer {
		helpWidth := m.width * 4 / 5
		if helpWidth < 60 {
			helpWidth = 60
		}
		helpHeight := m.height * 2 / 3
		if helpHeight < 15 {
			helpHeight = 15
		}
		if helpHeight > m.height-4 {
			helpHeight = m.height - 4
		}

		helpViewer := &helpViewerModel{
			width:   helpWidth,
			height:  helpHeight,
			scrollY: m.helpScrollY,
		}

		helpView := helpViewer.View()
		helpW := lipgloss.Width(helpView)
		helpH := lipgloss.Height(helpView)
		x := (m.width - helpW) / 2
		y := (m.height - helpH) / 2

		return layout.PlaceOverlay(x, y, helpView, result, true)
	}

	// Overlay diff viewer if open
	if m.diffViewer != nil && m.diffViewer.IsVisible() {
		m.diffViewer.width = m.width * 4 / 5
		if m.diffViewer.width < 60 {
			m.diffViewer.width = 60
		}
		m.diffViewer.height = m.height * 2 / 3
		if m.diffViewer.height < 15 {
			m.diffViewer.height = 15
		}
		if m.diffViewer.height > m.height-4 {
			m.diffViewer.height = m.height - 4
		}

		diffView := m.diffViewer.View()
		diffW := lipgloss.Width(diffView)
		diffH := lipgloss.Height(diffView)
		x := (m.width - diffW) / 2
		y := (m.height - diffH) / 2

		return layout.PlaceOverlay(x, y, diffView, result, true)
	}

	// Show confirmation dialog if visible
	if m.confirmationDialog != nil && m.confirmationDialog.IsVisible() {
		m.confirmationDialog.width = m.width * 3 / 5
		if m.confirmationDialog.width < 50 {
			m.confirmationDialog.width = 50
		}
		if m.confirmationDialog.width > 80 {
			m.confirmationDialog.width = 80
		}

		confirmView := m.confirmationDialog.View()
		confirmW := lipgloss.Width(confirmView)
		confirmH := lipgloss.Height(confirmView)
		x := (m.width - confirmW) / 2
		y := (m.height - confirmH) / 2

		return layout.PlaceOverlay(x, y, confirmView, result, true)
	}

	// Show autocomplete suggestions if active
	if m.showAutocomplete && m.autocomplete != nil && m.autocomplete.GetState().Active {
		autocompleteView := m.renderAutocomplete()
		if autocompleteView != "" {
			// Position above the input area
			autocompleteH := lipgloss.Height(autocompleteView)
			y := m.height - inputAreaHeight - autocompleteH - 2
			if y < 0 {
				y = 0
			}
			x := 2

			return layout.PlaceOverlay(x, y, autocompleteView, result, true)
		}
	}

	return result
}

// Helper methods (same as before but updated)
func (m *ModernTUIModel) recalculateLayout() {
	m.updateContextStats()
	m.totalLines = 0

	// Calculate actual main content width
	sidebarWidth := 0
	if m.sidebarOpen {
		sidebarWidth = m.width / 4
		if sidebarWidth < minSidebarWidth {
			sidebarWidth = minSidebarWidth
		}
		if sidebarWidth > maxSidebarWidth {
			sidebarWidth = maxSidebarWidth
		}
	}
	mainWidth := m.width - sidebarWidth
	if mainWidth < minMainWidth {
		mainWidth = minMainWidth
	}

	for i := range m.messages {
		m.messages[i].RenderedLines = m.calculateMessageLines(m.messages[i], mainWidth-2)
		m.totalLines += m.messages[i].RenderedLines
	}

	// Match the actual content area height
	contentHeight := m.height - titleBarHeight - statusBarHeight - inputAreaHeight

	m.visibleLines = contentHeight
	if m.visibleLines < 1 {
		m.visibleLines = 1
	}

	m.clampScrollY()
}

func (m *ModernTUIModel) clampScrollY() {
	if m.scrollY < 0 {
		m.scrollY = 0
	}
	if m.scrollY > m.totalLines-m.visibleLines {
		m.scrollY = m.totalLines - m.visibleLines
	}
	if m.scrollY < 0 {
		m.scrollY = 0
	}
}

func (m *ModernTUIModel) scroll(delta int) {
	m.scrollY += delta
	m.clampScrollY()
}

func (m *ModernTUIModel) scrollToBottom() {
	m.scrollY = m.totalLines
	m.clampScrollY()
}

// resetESCState resets the ESC double-press state when other keys are pressed
func (m *ModernTUIModel) resetESCState() {
	if m.escActive {
		m.escActive = false
		m.escCount = 0
		m.updateComponents()
	}
}

func (m *ModernTUIModel) calculateMessageLines(msg ChatMessageFinal, maxWidth int) int {
	if msg.RenderedLines > 0 {
		return msg.RenderedLines
	}

	totalLines := 0

	// Calculate thinking lines if present
	if msg.Thinking != "" {
		thinkingDisplay := truncateThinking(msg.Thinking, maxThinkingDisplayLen)
		thinkingLines := strings.Split(thinkingDisplay, "\n")
		for _, line := range thinkingLines {
			lineWidth := lipgloss.Width(line)
			if lineWidth == 0 {
				totalLines++
			} else {
				wrappedLines := (lineWidth + maxWidth - 6 - 1) / (maxWidth - 6)
				if wrappedLines < 1 {
					wrappedLines = 1
				}
				totalLines += wrappedLines
			}
		}
		// Add spacing between thinking and content
		totalLines += 1
	}

	// Calculate content lines
	lines := strings.Split(msg.Content, "\n")
	for _, line := range lines {
		// Use visual width for accurate line counting (handles CJK characters)
		lineWidth := lipgloss.Width(line)
		if lineWidth == 0 {
			totalLines++
		} else {
			wrappedLines := (lineWidth + maxWidth - 4 - 1) / (maxWidth - 4)
			if wrappedLines < 1 {
				wrappedLines = 1
			}
			totalLines += wrappedLines
		}
	}

	// Add extra line for message separator/spacing
	totalLines += 2
	return totalLines
}

func (m *ModernTUIModel) updateContextStats() {
	// Use real token counts from API if orchestrator is available
	if m.orchestrator != nil {
		inputTokens, outputTokens := m.orchestrator.GetTokenUsage()
		// Total tokens is sum of input and output tokens from API
		m.contextTokens = inputTokens + outputTokens
	} else {
		// Fallback to estimation if orchestrator not available
		totalChars := 0
		for _, msg := range m.messages {
			totalChars += len([]rune(msg.Content))
		}
		m.contextTokens = totalChars / 4
	}

	maxTokens := m.config.LLM.GetMaxTokens()
	if maxTokens > 0 {
		m.contextUsed = float64(m.contextTokens) / float64(maxTokens)
		if m.contextUsed > 1.0 {
			m.contextUsed = 1.0
		}
	}
}

func (m *ModernTUIModel) formatTokens(tokens int) string {
	return formatTokensStatic(tokens)
}

func (m *ModernTUIModel) handleCommand(input string) tea.Cmd {
	// Debug: show actual input bytes
	logger.Debug("ui", "handleCommand: input='%s', len=%d, bytes=%v", input, len(input), []byte(input))

	// Add to input history
	m.addToInputHistory(input)

	m.messages = append(m.messages, ChatMessageFinal{
		Role:      "user",
		Content:   input,
		Timestamp: time.Now(),
	})
	m.mainArea.messages = m.messages
	m.recalculateLayout()
	m.scrollToBottom()

	// Clear input after command
	m.input = ""
	m.cursor = 0
	m.updateComponents()

	// Check for exit/quit first (before executing command)
	inputLower := strings.ToLower(strings.TrimSpace(input))
	if inputLower == "/exit" || inputLower == "/quit" {
		m.saveSession()
		return tea.Quit
	}

	// Parse command name
	parts := strings.Fields(input)
	if len(parts) > 0 {
		cmdName := strings.TrimPrefix(parts[0], "/")
		cmd, exists := m.cmdManager.Get(cmdName)
		if exists && cmd.Subtask && cmd.Template != "" {
			// Subtask commands go through agent streaming
			m.messages = append(m.messages, ChatMessageFinal{
				Role:      "loading",
				Content:   "",
				Timestamp: time.Now(),
			})
			m.loading = true
			m.inputArea.loading = true
			m.mainArea.messages = m.messages
			m.recalculateLayout()
			m.scrollToBottom()
			m.updateComponents()

			// Extract user arguments (everything after the command)
			var argsStr string
			if len(parts) > 1 {
				argsStr = strings.Join(parts[1:], " ")
			}
			// Process template with user arguments
			processedTemplate := command.ProcessTemplate(cmd.Template, argsStr, parts[1:])

			// Debug: log history before skill command
			historyBefore := m.orchestrator.GetHistory()
			logger.Info("ui", "Skill command '%s' starting. History before: %d messages", cmdName, len(historyBefore))

			// Create cancellable context
			m.cancelCtx, m.cancelFunc = context.WithCancel(context.Background())

			// Create channel for streaming
			m.streamChan = make(chan AgentStreamMsgFinal, 100)

			// Start the agent with the processed template as input
			go func() {
				var fullContent strings.Builder
				var thinkingContent strings.Builder

				m.orchestrator.SetCallback(func(event llm.StreamEvent) {
					switch event.Type {
					case "content":
						fullContent.WriteString(event.Content)
						select {
						case m.streamChan <- AgentStreamMsgFinal{Content: fullContent.String(), Thinking: thinkingContent.String(), Done: false}:
						case <-m.cancelCtx.Done():
						}
					case "thinking":
						thinkingContent.WriteString(event.Content)
						select {
						case m.streamChan <- AgentStreamMsgFinal{Content: fullContent.String(), Thinking: thinkingContent.String(), Done: false}:
						case <-m.cancelCtx.Done():
						}
					case "tool_call":
						if event.ToolCall != nil {
							select {
							case m.streamChan <- AgentStreamMsgFinal{
								Content:  fullContent.String(),
								Thinking: thinkingContent.String(),
								Done:     false,
								ToolCall: &ToolCallInfo{
									Name:      event.ToolCall.Name,
									Arguments: event.ToolCall.Arguments,
								},
							}:
							case <-m.cancelCtx.Done():
							}
						}
					}
				})

				err := m.orchestrator.Run(m.cancelCtx, processedTemplate, "")

				// Send final message through channel
				if m.cancelCtx.Err() != nil {
					m.streamChan <- AgentStreamMsgFinal{Content: fullContent.String(), Thinking: thinkingContent.String(), Done: true, Error: fmt.Errorf("interrupted")}
				} else if err != nil {
					m.streamChan <- AgentStreamMsgFinal{Content: "", Thinking: "", Done: true, Error: err}
				} else {
					// Get response from history or use streaming content
					// This ensures we capture the complete response including any thinking
					history := m.orchestrator.GetHistory()
					logger.Info("ui", "Skill command completed. History after: %d messages", len(history))
					if len(history) > 0 {
						lastMsg := history[len(history)-1]
						if lastMsg.Role == "assistant" {
							// Use thinking from history if available, otherwise use accumulated
							thinking := lastMsg.Thinking
							if thinking == "" {
								thinking = thinkingContent.String()
							}
							logger.Debug("ui", "Skill command completed, sending final message from history (content_len=%d)", len(lastMsg.Content))
							m.streamChan <- AgentStreamMsgFinal{Content: lastMsg.Content, Thinking: thinking, Done: true}
						} else {
							m.streamChan <- AgentStreamMsgFinal{Content: fullContent.String(), Thinking: thinkingContent.String(), Done: true}
						}
					} else if fullContent.Len() > 0 {
						m.streamChan <- AgentStreamMsgFinal{Content: fullContent.String(), Thinking: thinkingContent.String(), Done: true}
					} else {
						m.streamChan <- AgentStreamMsgFinal{Content: "", Thinking: "", Done: true}
					}
				}
				close(m.streamChan)
			}()

			return m.readStreamChan(m.streamChan)
		}
	}

	handled, result, err := m.cmdManager.Execute(input)

	if !handled {
		m.messages = append(m.messages, ChatMessageFinal{
			Role:      "assistant",
			Content:   fmt.Sprintf("Unknown command: %s. Type /help to see available commands.", input),
			Timestamp: time.Now(),
		})
	} else if err != nil {
		m.messages = append(m.messages, ChatMessageFinal{
			Role:      "assistant",
			Content:   fmt.Sprintf("Command failed: %v", err),
			Timestamp: time.Now(),
		})
	} else {
		if input == "/clear" {
			m.messages = []ChatMessageFinal{}
			m.mainArea.messages = m.messages
			m.showWelcome = true
			m.mainArea.showWelcome = true
			m.updateContextStats()
			m.recalculateLayout()
			m.updateComponents()
			return nil
		}

		// Handle reload command
		if result == "RELOAD_COMMANDS" || input == "/reload" {
			count := m.cmdManager.ReloadFromSkills()
			result = fmt.Sprintf("已重新加载命令\n新增 %d 个 skill 命令\n总命令数: %d", count, m.cmdManager.Count())
		}

		// Handle compact command
		if strings.HasPrefix(result, "COMPACT_HISTORY:") {
			keepRecent := 6
			fmt.Sscanf(result, "COMPACT_HISTORY:%d", &keepRecent)

			// Perform compaction
			ctx := context.Background()
			compactionResult, err := m.orchestrator.CompactHistory(ctx, keepRecent)
			if err != nil {
				result = fmt.Sprintf("压缩失败: %v", err)
			} else {
				result = fmt.Sprintf("✅ 对话已压缩\n\n"+
					"移除消息: %d 条\n"+
					"保留消息: %d 条\n"+
					"预计节省: %d tokens",
					compactionResult.RemovedCount,
					len(compactionResult.KeptMessages),
					compactionResult.SavedTokens)

				// Update UI messages to reflect compacted history
				history := m.orchestrator.GetHistory()
				m.messages = []ChatMessageFinal{}
				for _, msg := range history {
					m.messages = append(m.messages, ChatMessageFinal{
						Role:      msg.Role,
						Content:   msg.Content,
						Timestamp: time.Now(),
					})
				}
				m.mainArea.messages = m.messages
				m.updateContextStats()
			}
		}

		if result != "" {
			m.messages = append(m.messages, ChatMessageFinal{
				Role:      "assistant",
				Content:   result,
				Timestamp: time.Now(),
			})
		}
	}

	m.mainArea.messages = m.messages
	m.recalculateLayout()
	m.scrollToBottom()
	m.updateComponents()
	return nil
}

func (m *ModernTUIModel) sendMessage() tea.Cmd {
	userInput := m.input
	m.input = ""
	m.cursor = 0
	m.isInterrupted = false
	m.streamingContent = ""
	m.updateComponents()

	if !m.hasSetTitle {
		m.sessionTitle = generateTitleFinal(userInput)
		m.hasSetTitle = true
		m.sidebar.sessionTitle = m.sessionTitle

		if m.sessionManager != nil && m.currentSession != nil {
			m.currentSession.Title = m.sessionTitle
			m.sessionManager.Save(m.currentSession)
			m.refreshSessionList()
		}
	}

	m.messages = append(m.messages, ChatMessageFinal{
		Role:      "user",
		Content:   userInput,
		Timestamp: time.Now(),
	})

	m.messages = append(m.messages, ChatMessageFinal{
		Role:      "loading",
		Content:   "",
		Timestamp: time.Now(),
	})

	m.loading = true
	m.inputArea.loading = true
	m.mainArea.messages = m.messages
	m.recalculateLayout()
	m.scrollToBottom()
	m.updateComponents()

	// Create cancellable context for interrupting
	m.cancelCtx, m.cancelFunc = context.WithCancel(context.Background())

	// Create channel for streaming events
	m.streamChan = make(chan AgentStreamMsgFinal, 100)

	// Start the agent in background - sends ALL messages through m.streamChan
	go func() {
		var fullContent strings.Builder
		var thinkingContent strings.Builder

		// Set callback for streaming
		m.orchestrator.SetCallback(func(event llm.StreamEvent) {
			switch event.Type {
			case "content":
				fullContent.WriteString(event.Content)
				// Send streaming update
				select {
				case m.streamChan <- AgentStreamMsgFinal{Content: fullContent.String(), Thinking: thinkingContent.String(), Done: false}:
				case <-m.cancelCtx.Done():
				}
			case "thinking":
				thinkingContent.WriteString(event.Content)
				// Send thinking update with special type
				select {
				case m.streamChan <- AgentStreamMsgFinal{Content: fullContent.String(), Thinking: thinkingContent.String(), Done: false}:
				case <-m.cancelCtx.Done():
				}
			case "tool_call":
				// Send tool call status
				if event.ToolCall != nil {
					select {
					case m.streamChan <- AgentStreamMsgFinal{
						Content:  fullContent.String(),
						Thinking: thinkingContent.String(),
						Done:     false,
						ToolCall: &ToolCallInfo{
							Name:      event.ToolCall.Name,
							Arguments: event.ToolCall.Arguments,
						},
					}:
					case <-m.cancelCtx.Done():
					}
				}
			}
		})

		err := m.orchestrator.Run(m.cancelCtx, userInput, "")

		// Send final message through channel
		if m.isInterrupted || m.cancelCtx.Err() != nil {
			m.streamChan <- AgentStreamMsgFinal{Content: fullContent.String(), Thinking: thinkingContent.String(), Done: true, Error: fmt.Errorf("interrupted")}
		} else if err != nil {
			m.streamChan <- AgentStreamMsgFinal{Content: "", Thinking: "", Done: true, Error: err}
		} else {
			// Get response from history or use streaming content
			history := m.orchestrator.GetHistory()
			if len(history) > 0 {
				lastMsg := history[len(history)-1]
				if lastMsg.Role == "assistant" {
					// Use thinking from history if available, otherwise use accumulated
					thinking := lastMsg.Thinking
					if thinking == "" {
						thinking = thinkingContent.String()
					}
					m.streamChan <- AgentStreamMsgFinal{Content: lastMsg.Content, Thinking: thinking, Done: true}
				} else {
					m.streamChan <- AgentStreamMsgFinal{Content: fullContent.String(), Thinking: thinkingContent.String(), Done: true}
				}
			} else if fullContent.Len() > 0 {
				m.streamChan <- AgentStreamMsgFinal{Content: fullContent.String(), Thinking: thinkingContent.String(), Done: true}
			} else {
				m.streamChan <- AgentStreamMsgFinal{Content: "", Thinking: "", Done: true}
			}
		}
		close(m.streamChan)
	}()

	// Return a command that reads the first message from the stream
	return m.readStreamChan(m.streamChan)
}

// readStreamChan creates a command that reads from the stream channel
func (m *ModernTUIModel) readStreamChan(streamChan chan AgentStreamMsgFinal) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-streamChan
		if !ok {
			return AgentDoneMsgFinal{}
		}
		return msg
	}
}

func (m *ModernTUIModel) removeLoadingMessage() {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "loading" {
			m.messages = append(m.messages[:i], m.messages[i+1:]...)
			break
		}
	}
	m.mainArea.messages = m.messages
	m.recalculateLayout()
}

func (m *ModernTUIModel) updateStreamingContent(content string) {
	m.updateStreamingContentWithThinking(content, "")
}

func (m *ModernTUIModel) updateStreamingContentWithThinking(content string, thinking string) {
	// Find and update the loading message
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "loading" {
			// Only update if content actually changed
			if m.messages[i].Content == content && m.messages[i].Thinking == thinking {
				return
			}
			m.messages[i].Content = content
			m.messages[i].Thinking = thinking
			// Invalidate all cached rendering data since content changed
			m.messages[i].WrappedContent = ""
			m.messages[i].WrappedThinking = ""
			m.messages[i].RenderedLines = 0 // Important: reset so line count is recalculated
			m.mainArea.messages = m.messages
			m.recalculateLayout()
			m.scrollToBottom()
			m.updateComponents()
			return
		}
	}
}

func (m *ModernTUIModel) saveSession() {
	if m.sessionManager == nil || m.currentSession == nil {
		return
	}

	var messages []llm.Message
	for _, msg := range m.messages {
		if msg.Role != "loading" {
			messages = append(messages, llm.Message{
				Role:     msg.Role,
				Content:  msg.Content,
				Thinking: msg.Thinking,
			})
		}
	}

	m.currentSession.Messages = messages
	m.sessionManager.Save(m.currentSession)
}

func (m *ModernTUIModel) formatError(err error) string {
	errStr := err.Error()

	// Handle interrupt specially
	if strings.Contains(errStr, "interrupted") {
		return "Operation cancelled by user"
	}

	errorCode := "ERROR"
	if strings.Contains(errStr, ":") {
		parts := strings.SplitN(errStr, ":", 2)
		if len(parts) == 2 {
			errorCode = strings.TrimSpace(parts[0])
			errStr = strings.TrimSpace(parts[1])
		}
	}

	return fmt.Sprintf("[%s] %s", errorCode, errStr)
}

func (m *ModernTUIModel) updatePalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.showPalette = false
		return m, nil

	case tea.KeyEnter:
		if len(m.filteredCommands) > 0 && m.paletteSelected < len(m.filteredCommands) {
			selectedCmd := m.filteredCommands[m.paletteSelected]
			m.showPalette = false
			m.input = "/" + selectedCmd.Name
			m.cursor = len([]rune(m.input))
			m.updateComponents()
		}
		return m, nil

	case tea.KeyUp:
		if m.paletteSelected > 0 {
			m.paletteSelected--
		}
		return m, nil

	case tea.KeyDown:
		if m.paletteSelected < len(m.filteredCommands)-1 {
			m.paletteSelected++
		}
		return m, nil

	case tea.KeyBackspace, tea.KeyCtrlH:
		if len(m.paletteInput) > 0 && m.paletteCursor > 0 {
			runes := []rune(m.paletteInput)
			m.paletteInput = string(runes[:m.paletteCursor-1]) + string(runes[m.paletteCursor:])
			m.paletteCursor--
			m.updateFilteredCommands()
		}
		return m, nil

	case tea.KeyRunes:
		runes := []rune(m.paletteInput)
		m.paletteInput = string(runes[:m.paletteCursor]) + string(msg.Runes) + string(runes[m.paletteCursor:])
		m.paletteCursor += len(msg.Runes)
		m.updateFilteredCommands()
		return m, nil
	}

	return m, nil
}

func (m *ModernTUIModel) updateLogViewer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.showLogViewer = false
		return m, nil

	case tea.KeyUp:
		if m.logScrollY > 0 {
			m.logScrollY--
		}
		return m, nil

	case tea.KeyDown:
		m.logScrollY++
		return m, nil

	case tea.KeyPgUp:
		m.logScrollY -= 10
		if m.logScrollY < 0 {
			m.logScrollY = 0
		}
		return m, nil

	case tea.KeyPgDown:
		m.logScrollY += 10
		return m, nil

	case tea.KeyHome:
		m.logScrollY = 0
		return m, nil

	case tea.KeyEnd:
		m.logScrollY = len(m.logLines)
		return m, nil
	}

	return m, nil
}

// updateHelpViewer handles keyboard input for the help viewer
func (m *ModernTUIModel) updateHelpViewer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.showHelpViewer = false
		return m, nil

	case tea.KeyUp:
		if m.helpScrollY > 0 {
			m.helpScrollY--
		}
		return m, nil

	case tea.KeyDown:
		m.helpScrollY++
		return m, nil

	case tea.KeyPgUp:
		m.helpScrollY -= 10
		if m.helpScrollY < 0 {
			m.helpScrollY = 0
		}
		return m, nil

	case tea.KeyPgDown:
		m.helpScrollY += 10
		return m, nil

	case tea.KeyHome:
		m.helpScrollY = 0
		return m, nil

	case tea.KeyEnd:
		// Calculate total lines in help content
		m.helpScrollY = 999 // Will be clamped
		return m, nil
	}

	return m, nil
}

func (m *ModernTUIModel) updateFilteredCommands() {
	allCommands := m.cmdManager.List()
	m.filteredCommands = []*command.Command{}

	if m.paletteInput == "" {
		m.filteredCommands = allCommands
	} else {
		inputLower := strings.ToLower(m.paletteInput)
		for _, cmd := range allCommands {
			if strings.Contains(strings.ToLower(cmd.Name), inputLower) ||
				strings.Contains(strings.ToLower(cmd.Description), inputLower) {
				m.filteredCommands = append(m.filteredCommands, cmd)
			}
		}
	}

	if m.paletteSelected >= len(m.filteredCommands) {
		m.paletteSelected = 0
	}
}

// updateDiffViewer handles keyboard input for the diff viewer
func (m *ModernTUIModel) updateDiffViewer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		// Cancel the operation
		m.diffViewer.Hide()
		m.pendingFileOp = nil
		return m, nil

	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "y", "Y":
			// Confirm and execute the operation
			if m.pendingFileOp != nil {
				// Execute the confirmed file operation
				m.executeConfirmedFileOp()
			}
			m.diffViewer.Hide()
			return m, nil
		case "n", "N":
			// Cancel the operation
			m.diffViewer.Hide()
			m.pendingFileOp = nil
			return m, nil
		}

	case tea.KeyUp:
		m.diffViewer.Scroll(-1)
		return m, nil

	case tea.KeyDown:
		m.diffViewer.Scroll(1)
		return m, nil

	case tea.KeyPgUp:
		m.diffViewer.Scroll(-10)
		return m, nil

	case tea.KeyPgDown:
		m.diffViewer.Scroll(10)
		return m, nil
	}

	return m, nil
}

// updateConfirmationDialog handles confirmation dialog input
func (m *ModernTUIModel) updateConfirmationDialog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		// Cancel the operation
		m.confirmationDialog.Hide()
		m.pendingSensitiveOp = false
		return m, nil

	case tea.KeyUp:
		m.confirmationDialog.MoveSelection(-1)
		return m, nil

	case tea.KeyDown:
		m.confirmationDialog.MoveSelection(1)
		return m, nil

	case tea.KeyTab:
		m.confirmationDialog.MoveSelection(1)
		return m, nil

	case tea.KeyEnter:
		// Select current option
		if m.confirmationDialog.selected == 2 {
			// Toggle remember checkbox
			m.confirmationDialog.ToggleRemember()
			return m, nil
		}
		// Get result and process
		result := m.confirmationDialog.GetResult()
		m.processConfirmationResult(result)
		m.confirmationDialog.Hide()
		m.pendingSensitiveOp = false
		return m, nil

	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "y", "Y":
			// Quick confirm
			result := m.confirmationDialog.GetResult()
			result.Confirmed = true
			m.processConfirmationResult(result)
			m.confirmationDialog.Hide()
			m.pendingSensitiveOp = false
			return m, nil
		case "n", "N":
			// Quick deny
			result := m.confirmationDialog.GetResult()
			result.Confirmed = false
			m.processConfirmationResult(result)
			m.confirmationDialog.Hide()
			m.pendingSensitiveOp = false
			return m, nil
		}
	}

	return m, nil
}

// processConfirmationResult handles the result of a confirmation dialog
func (m *ModernTUIModel) processConfirmationResult(result audit.ConfirmationResult) {
	// This will be connected to the sensitive operation manager
	// For now, just log the decision
	logger.Info("confirmation", "confirmed=%t, remember=%t", result.Confirmed, result.Remember)
}

// ShowConfirmationDialog shows the confirmation dialog for a sensitive operation
func (m *ModernTUIModel) ShowConfirmationDialog(op audit.SensitiveOperation) {
	m.confirmationDialog.Show(op)
	m.pendingSensitiveOp = true
}

// ShowDiffPreview shows a diff preview for user confirmation
func (m *ModernTUIModel) ShowDiffPreview(path string, diff *tools.DiffResult, content string, operation string) {
	m.diffViewer.SetDiff(path, diff)
	m.pendingFileOp = &PendingFileOperation{
		Path:      path,
		Content:   content,
		Operation: operation,
		Diff:      diff,
	}
}

// navigateInputHistory navigates through input history
func (m *ModernTUIModel) navigateInputHistory(direction int) {
	if len(m.inputHistory) == 0 {
		return
	}

	// Save current input when starting navigation
	if m.inputHistoryIndex == -1 {
		m.savedInput = m.input
	}

	newIndex := m.inputHistoryIndex + direction
	if newIndex < -1 {
		newIndex = -1
	}
	if newIndex >= len(m.inputHistory) {
		newIndex = len(m.inputHistory) - 1
	}

	m.inputHistoryIndex = newIndex

	if newIndex == -1 {
		// Back to current input
		m.input = m.savedInput
	} else {
		m.input = m.inputHistory[len(m.inputHistory)-1-newIndex]
	}
	m.cursor = len([]rune(m.input))
	m.updateComponents()
}

// addToInputHistory adds input to history
func (m *ModernTUIModel) addToInputHistory(input string) {
	if input == "" {
		return
	}

	// Don't add duplicate consecutive entries
	if len(m.inputHistory) > 0 && m.inputHistory[len(m.inputHistory)-1] == input {
		return
	}

	m.inputHistory = append(m.inputHistory, input)

	// Keep only last 100 entries
	if len(m.inputHistory) > 100 {
		m.inputHistory = m.inputHistory[len(m.inputHistory)-100:]
	}

	// Reset history index
	m.inputHistoryIndex = -1
}

func (m *ModernTUIModel) deleteWordBeforeCursor() {
	if m.cursor == 0 {
		return
	}

	runes := []rune(m.input)
	start := m.cursor - 1

	// Skip trailing spaces
	for start >= 0 && runes[start] == ' ' {
		start--
	}

	// Find word boundary
	for start >= 0 && runes[start] != ' ' {
		start--
	}
	start++

	m.input = string(runes[:start]) + string(runes[m.cursor:])
	m.cursor = start
	m.updateComponents()
}

// deleteToEndOfLine deletes from cursor to end of line
func (m *ModernTUIModel) deleteToEndOfLine() {
	if m.cursor >= len([]rune(m.input)) {
		return
	}

	runes := []rune(m.input)
	m.input = string(runes[:m.cursor])
	m.updateComponents()
}

// moveWordLeft moves cursor left by one word
func (m *ModernTUIModel) moveWordLeft() {
	if m.cursor == 0 {
		return
	}

	runes := []rune(m.input)
	pos := m.cursor - 1

	// Skip trailing spaces
	for pos >= 0 && runes[pos] == ' ' {
		pos--
	}

	// Find word boundary
	for pos >= 0 && runes[pos] != ' ' {
		pos--
	}

	m.cursor = pos + 1
	m.updateComponents()
}

// moveWordRight moves cursor right by one word
func (m *ModernTUIModel) moveWordRight() {
	runes := []rune(m.input)
	if m.cursor >= len(runes) {
		return
	}

	pos := m.cursor

	// Skip current word
	for pos < len(runes) && runes[pos] != ' ' {
		pos++
	}

	// Skip spaces
	for pos < len(runes) && runes[pos] == ' ' {
		pos++
	}

	m.cursor = pos
	m.updateComponents()
}

// executeConfirmedFileOp executes a confirmed file operation
func (m *ModernTUIModel) executeConfirmedFileOp() {
	if m.pendingFileOp == nil {
		return
	}

	// Get file ops manager from tool manager
	// This would need to be connected through the orchestrator
	// For now, just log the confirmation
	logger.Info("ui", "File operation confirmed: %s on %s", m.pendingFileOp.Operation, m.pendingFileOp.Path)

	// Clear the pending operation
	m.pendingFileOp = nil
}

func (m *ModernTUIModel) autoCompleteCommand() {
	// Use the new autocomplete system
	if m.autocomplete == nil {
		m.autocomplete = NewAutoCompleter(m.cmdManager, "")
	}

	// If autocomplete is not active, trigger it
	if !m.autocomplete.GetState().Active {
		if m.autocomplete.Trigger(m.input, m.cursor) {
			m.showAutocomplete = true
		}
		return
	}

	// If active, accept the current selection
	newInput, newCursor := m.autocomplete.Accept(m.input, m.cursor)
	m.input = newInput
	m.cursor = len([]rune(newInput))
	if m.cursor != newCursor {
		m.cursor = newCursor
	}
	m.showAutocomplete = false
	m.updateComponents()
}

// autoCompleteNavigate navigates through autocomplete suggestions
func (m *ModernTUIModel) autoCompleteNavigate(direction int) {
	if m.autocomplete != nil && m.autocomplete.GetState().Active {
		m.autocomplete.Navigate(direction)
	}
}

// autoCompleteCancel cancels the current autocomplete
func (m *ModernTUIModel) autoCompleteCancel() {
	if m.autocomplete != nil {
		m.autocomplete.Cancel()
	}
	m.showAutocomplete = false
}

// triggerAutocomplete triggers autocomplete based on current input context
func (m *ModernTUIModel) triggerAutocomplete() bool {
	if m.autocomplete == nil {
		m.autocomplete = NewAutoCompleter(m.cmdManager, "")
	}

	// Set working directory
	if cwd, err := os.Getwd(); err == nil {
		m.autocomplete.SetWorkingDir(cwd)
	}

	if m.autocomplete.Trigger(m.input, m.cursor) {
		m.showAutocomplete = true
		return true
	}
	return false
}

// renderAutocomplete renders the autocomplete suggestions
func (m *ModernTUIModel) renderAutocomplete() string {
	state := m.autocomplete.GetState()
	if !state.Active || len(state.Suggestions) == 0 {
		return ""
	}

	// Styles
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("6")).
		Padding(0, 1).
		Background(lipgloss.Color("235"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("27")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	typeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("243"))

	// Build suggestions list
	var lines []string
	maxVisible := 8
	start := 0

	if state.SelectedIndex >= maxVisible {
		start = state.SelectedIndex - maxVisible + 1
	}

	end := start + maxVisible
	if end > len(state.Suggestions) {
		end = len(state.Suggestions)
	}

	for i := start; i < end; i++ {
		suggestion := state.Suggestions[i]

		// Determine type label based on suggestion type
		var typeLabel string
		switch suggestion.Type {
		case "command":
			typeLabel = "[cmd]"
		case "directory":
			typeLabel = "[dir]"
		case "file":
			typeLabel = "[file]"
		default:
			typeLabel = ""
		}

		// Build the display line
		display := suggestion.Display
		if display == "" {
			display = suggestion.Text
		}

		// Add description if available (for commands)
		desc := ""
		if suggestion.Description != "" && suggestion.Type == "command" {
			// Truncate description by visual width (handles CJK characters)
			maxDescWidth := 30
			descRunes := []rune(suggestion.Description)
			descVisualWidth := lipgloss.Width(suggestion.Description)
			if descVisualWidth > maxDescWidth {
				// Truncate by visual width
				truncated := ""
				for _, r := range descRunes {
					if lipgloss.Width(truncated+string(r)) > maxDescWidth-3 {
						break
					}
					truncated += string(r)
				}
				desc = " " + descStyle.Render(truncated + "...")
			} else {
				desc = " " + descStyle.Render(suggestion.Description)
			}
		}

		if i == state.SelectedIndex {
			line := selectedStyle.Render("▶ "+display) + typeStyle.Render(" "+typeLabel) + desc
			lines = append(lines, line)
		} else {
			line := normalStyle.Render("  "+display) + typeStyle.Render(" "+typeLabel) + desc
			lines = append(lines, line)
		}
	}

	// Add scroll indicator if there are more items
	if len(state.Suggestions) > maxVisible {
		indicator := fmt.Sprintf(" (%d/%d)", state.SelectedIndex+1, len(state.Suggestions))
		lines = append(lines, typeStyle.Render(indicator))
	}

	content := strings.Join(lines, "\n")

	// Calculate width based on display text (use visual width for CJK support)
	maxWidth := 50
	for _, s := range state.Suggestions {
		displayText := s.Display
		if displayText == "" {
			displayText = s.Text
		}
		displayWidth := lipgloss.Width(displayText)
		if displayWidth+15 > maxWidth {
			maxWidth = displayWidth + 15
		}
	}

	return boxStyle.Width(maxWidth).Render(content)
}

// updateAutocompleteOnChange updates autocomplete suggestions when input changes
func (m *ModernTUIModel) updateAutocompleteOnChange() {
	if m.autocomplete == nil {
		return
	}

	// Check if autocomplete should be shown for current input
	if m.autocomplete.ShouldTrigger(m.input, m.cursor) {
		if m.autocomplete.Trigger(m.input, m.cursor) {
			m.showAutocomplete = true
		} else {
			m.showAutocomplete = false
		}
	} else {
		// Hide autocomplete if input no longer matches trigger conditions
		if m.showAutocomplete {
			m.autocomplete.Cancel()
			m.showAutocomplete = false
		}
	}
}

func (m *ModernTUIModel) updateStreamingContentDebounced(content string) {
	m.streamingContent = content
	m.updateStreamingContent(content)
}

func (m *ModernTUIModel) refreshSessionList() {
	if m.sessionManager == nil {
		return
	}

	sessions, err := m.sessionManager.List()
	if err != nil {
		return
	}

	m.sessions = sessions
	m.sidebar.sessions = sessions
}

// switchToSession switches to a different session
func (m *ModernTUIModel) switchToSession(sessionID string) tea.Cmd {
	if m.sessionManager == nil {
		return nil
	}

	// Load the session
	sess, err := m.sessionManager.Load(sessionID)
	if err != nil {
		return nil
	}

	// Save current session first
	m.saveSession()

	// Switch to new session
	m.currentSession = sess
	m.sessionTitle = sess.Title
	m.hasSetTitle = true

	// Load messages from session
	m.messages = []ChatMessageFinal{}
	for _, msg := range sess.Messages {
		m.messages = append(m.messages, ChatMessageFinal{
			Role:      msg.Role,
			Content:   msg.Content,
			Thinking:  msg.Thinking,
			Timestamp: time.Now(),
		})
	}

	// Update history in orchestrator - smart load to save tokens
	if m.orchestrator != nil {
		// Get current agent and restore history
		currentAgent := m.orchestrator.GetCurrentAgent()
		agentInfo, err := m.orchestrator.GetAgentInfo(currentAgent)
		if err == nil && agentInfo.Loop != nil {
			// Check if session has a stored summary (already compacted)
			if sess.Summary != nil && sess.Summary.Summary != "" {
				// Build compacted history: summary + recent messages
				var compactedHistory []llm.Message
				compactedHistory = append(compactedHistory, llm.Message{
					Role:    "system",
					Content: fmt.Sprintf("[对话摘要]\n%s", sess.Summary.Summary),
				})

				// Add messages that came after the summary
				summaryMsgCount := sess.Summary.MessageCount
				if summaryMsgCount < len(sess.Messages) {
					compactedHistory = append(compactedHistory, sess.Messages[summaryMsgCount:]...)
				}

				agentInfo.Loop.SetHistory(compactedHistory)
			} else {
				// Check if history is too large and should be auto-compacted
				maxTokens := m.config.LLM.GetMaxTokens()
				threshold := m.config.Agent.CompactThreshold
				if threshold <= 0 {
					threshold = 0.7 // Default 70%
				}

				// Estimate token count
				totalChars := 0
				for _, msg := range sess.Messages {
					totalChars += len(msg.Content)
				}
				estimatedTokens := totalChars / 4

				// If history is large, keep only recent messages for efficiency
				if float64(estimatedTokens)/float64(maxTokens) > threshold {
					// Keep last 10 messages (5 turns) by default for quick switch
					keepRecent := 10
					if len(sess.Messages) > keepRecent {
						// Create a quick summary message
						quickSummary := fmt.Sprintf("[历史对话] 共 %d 条消息，加载最近 %d 条以节省上下文。使用 /compact 可生成完整摘要。",
							len(sess.Messages), keepRecent)

						var smartHistory []llm.Message
						smartHistory = append(smartHistory, llm.Message{
							Role:    "system",
							Content: quickSummary,
						})
						smartHistory = append(smartHistory, sess.Messages[len(sess.Messages)-keepRecent:]...)

						agentInfo.Loop.SetHistory(smartHistory)
					} else {
						agentInfo.Loop.SetHistory(sess.Messages)
					}
				} else {
					// History is small enough, load everything
					agentInfo.Loop.SetHistory(sess.Messages)
				}
			}
		}
	}

	// Update UI
	m.mainArea.messages = m.messages
	m.sidebar.currentSess = sess
	m.sidebar.sessionTitle = sess.Title
	m.showWelcome = len(m.messages) == 0
	m.mainArea.showWelcome = len(m.messages) == 0

	// Update context stats
	m.updateContextStats()

	// Reset scroll
	m.scrollY = 0
	m.recalculateLayout()
	m.updateComponents()

	return nil
}
