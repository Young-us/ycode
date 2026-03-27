package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Young-us/ycode/internal/agent"
	"github.com/Young-us/ycode/internal/command"
	"github.com/Young-us/ycode/internal/config"
	"github.com/Young-us/ycode/internal/llm"
	"github.com/Young-us/ycode/internal/session"
)

// TUIFinalModel represents the final TUI layout
type TUIFinalModel struct {
	// Agent
	agent  *agent.Loop
	config *config.Config

	// Command manager
	cmdManager *command.CommandManager

	// Session manager (can be nil)
	sessionManager *session.Manager
	currentSession *session.Session

	// State
	width   int
	height  int
	ready   bool
	loading bool

	// Session state
	sessionTitle string
	hasSetTitle  bool

	// Chat state
	messages []ChatMessageFinal
	input    string
	cursor   int // 光标位置

	// Scroll state (in lines, not messages)
	scrollY      int // 当前滚动位置（行）
	totalLines   int // 总行数
	visibleLines int // 可见行数

	// Show welcome
	showWelcome bool

	// Context stats
	contextTokens int
	contextUsed   float64

	// Anti-duplication
	lastSendTime    time.Time
	minSendInterval time.Duration

	// Streaming support
	streamingContent string
	lastStreamUpdate time.Time
	streamDebounce   time.Duration

	// Command palette
	showPalette      bool
	paletteInput     string
	paletteCursor    int
	paletteSelected  int
	filteredCommands []string

	// Status indicators
	mcpStatus map[string]string // server name -> status
	lspStatus map[string]string // server name -> status

	// Styles
	styles *TUIFinalStyles
}

// ChatMessageFinal represents a chat message
type ChatMessageFinal struct {
	Role          string // "user", "assistant", "loading"
	Content       string
	Thinking      string // Extended thinking content
	Timestamp     time.Time
	RenderedLines int // 渲染后的行数
	// Cached wrapped content to avoid re-computing on every render
	WrappedContent  string
	WrappedThinking string
	WrapWidth       int // Width used for wrapping, to detect if re-wrap needed
}

// TUIFinalStyles contains all TUI styles
type TUIFinalStyles struct {
	TitleBar           lipgloss.Style
	TitleBarBorder     lipgloss.Style
	InputBox           lipgloss.Style
	InputBoxBorder     lipgloss.Style
	ChatArea           lipgloss.Style
	UserMsg            lipgloss.Style
	AssistantMsg       lipgloss.Style
	LoadingMsg         lipgloss.Style
	ErrorMsg           lipgloss.Style
	Welcome            lipgloss.Style
	Muted              lipgloss.Style
	Bold               lipgloss.Style
	DimText            lipgloss.Style
	Palette            lipgloss.Style
	PaletteBorder      lipgloss.Style
	PaletteItem        lipgloss.Style
	PaletteSelected    lipgloss.Style
	StatusConnected    lipgloss.Style
	StatusDisconnected lipgloss.Style
	StatusError        lipgloss.Style
}

// NewTUIFinalStyles creates TUI styles
func NewTUIFinalStyles() *TUIFinalStyles {
	return &TUIFinalStyles{
		TitleBar: lipgloss.NewStyle().
			Background(lipgloss.Color("#111827")).
			Foreground(lipgloss.Color("#F9FAFB")).
			Bold(true),

		TitleBarBorder: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")),

		InputBox: lipgloss.NewStyle().
			Background(lipgloss.Color("#111827")).
			Foreground(lipgloss.Color("#F9FAFB")),

		InputBoxBorder: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")),

		ChatArea: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB")),

		UserMsg: lipgloss.NewStyle().
			Background(lipgloss.Color("#111827")).
			Foreground(lipgloss.Color("#F9FAFB")).
			Padding(0, 1),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB")),

		LoadingMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Italic(true),

		ErrorMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true),

		Welcome: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")),

		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")),

		Bold: lipgloss.NewStyle().
			Bold(true),

		DimText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Italic(true),

		Palette: lipgloss.NewStyle().
			Background(lipgloss.Color("#1F2937")).
			Foreground(lipgloss.Color("#F9FAFB")).
			Padding(1, 2),

		PaletteBorder: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6366F1")),

		PaletteItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB")),

		PaletteSelected: lipgloss.NewStyle().
			Background(lipgloss.Color("#4F46E5")).
			Foreground(lipgloss.Color("#F9FAFB")).
			Bold(true),

		StatusConnected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")),

		StatusDisconnected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")),

		StatusError: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")),
	}
}

// NewTUIFinalModel creates a new TUI model
func NewTUIFinalModel(agentLoop *agent.Loop, cfg *config.Config, cmdManager *command.CommandManager) *TUIFinalModel {
	// Initialize session manager (can be nil if fails)
	var sessionManager *session.Manager
	var currentSession *session.Session

	sm, err := session.NewManager()
	if err == nil {
		sessionManager = sm
		// Create new session
		sess, err := sessionManager.Create("New Session")
		if err == nil {
			currentSession = sess
		}
	}

	// Initialize status maps
	mcpStatus := make(map[string]string)
	lspStatus := make(map[string]string)

	// Set MCP server statuses
	for _, server := range cfg.MCP.Servers {
		if server.Enabled {
			mcpStatus[server.Name] = "disconnected"
		}
	}

	// Set LSP server statuses
	for _, server := range cfg.LSP.Servers {
		if server.Enabled {
			lspStatus[server.Name] = "disconnected"
		}
	}

	return &TUIFinalModel{
		agent:           agentLoop,
		config:          cfg,
		cmdManager:      cmdManager,
		sessionManager:  sessionManager,
		currentSession:  currentSession,
		sessionTitle:    "New Session",
		messages:        []ChatMessageFinal{},
		showWelcome:     true,
		styles:          NewTUIFinalStyles(),
		minSendInterval: 500 * time.Millisecond, // 防抖间隔
		streamDebounce:  50 * time.Millisecond,  // 流式更新防抖
		mcpStatus:       mcpStatus,
		lspStatus:       lspStatus,
	}
}

// UpdateMCPStatus updates the status of an MCP server
func (m *TUIFinalModel) UpdateMCPStatus(name string, status string) {
	m.mcpStatus[name] = status
}

// UpdateLSPStatus updates the status of an LSP server
func (m *TUIFinalModel) UpdateLSPStatus(name string, status string) {
	m.lspStatus[name] = status
}

// Init initializes the model
func (m *TUIFinalModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *TUIFinalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.recalculateLayout()

	case tea.KeyMsg:
		// Handle command palette first if open
		if m.showPalette {
			return m.updatePalette(msg)
		}

		// 防抖检查
		if time.Since(m.lastSendTime) < m.minSendInterval {
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			// 保存会话
			m.saveSession()
			return m, tea.Quit

		case tea.KeyCtrlP:
			// Open command palette
			m.showPalette = true
			m.paletteInput = ""
			m.paletteCursor = 0
			m.paletteSelected = 0
			m.updateFilteredCommands()
			return m, nil

		case tea.KeyCtrlL:
			// Clear screen
			m.messages = []ChatMessageFinal{}
			m.updateContextStats()
			m.recalculateLayout()
			return m, nil

		case tea.KeyCtrlU:
			// Clear input
			m.input = ""
			m.cursor = 0
			return m, nil

		case tea.KeyCtrlA:
			// Move to start of line
			m.cursor = 0
			return m, nil

		case tea.KeyCtrlE:
			// Move to end of line
			m.cursor = len([]rune(m.input))
			return m, nil

		case tea.KeyTab:
			// Auto-complete command
			if strings.HasPrefix(m.input, "/") {
				m.autoCompleteCommand()
			}
			return m, nil

		case tea.KeyEnter:
			if m.input != "" && !m.loading {
				m.showWelcome = false
				m.lastSendTime = time.Now()

				// Check if it's a command
				if strings.HasPrefix(m.input, "/") {
					return m, m.handleCommand(m.input)
				}

				return m, m.sendMessage()
			}

		case tea.KeyBackspace, tea.KeyCtrlH:
			// Handle UTF-8 properly for Chinese characters
			if len(m.input) > 0 && m.cursor > 0 {
				runes := []rune(m.input)
				if m.cursor <= len(runes) {
					m.input = string(runes[:m.cursor-1]) + string(runes[m.cursor:])
					m.cursor--
				}
			}

		case tea.KeyDelete:
			if len(m.input) > 0 && m.cursor < len([]rune(m.input)) {
				runes := []rune(m.input)
				m.input = string(runes[:m.cursor]) + string(runes[m.cursor+1:])
			}

		case tea.KeyLeft:
			if m.cursor > 0 {
				m.cursor--
			}

		case tea.KeyRight:
			if m.cursor < len([]rune(m.input)) {
				m.cursor++
			}

		case tea.KeyUp:
			m.scroll(-1)

		case tea.KeyDown:
			m.scroll(1)

		case tea.KeyPgUp:
			m.scroll(-m.visibleLines)

		case tea.KeyPgDown:
			m.scroll(m.visibleLines)

		case tea.KeyHome:
			m.scrollY = 0

		case tea.KeyEnd:
			m.scrollY = m.totalLines

		case tea.KeyRunes:
			runes := []rune(m.input)
			m.input = string(runes[:m.cursor]) + string(msg.Runes) + string(runes[m.cursor:])
			m.cursor += len(msg.Runes)
		}

	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			m.scroll(-3)
		case tea.MouseWheelDown:
			m.scroll(3)
		}

	case AgentResponseMsgFinal:
		// Remove loading message
		m.removeLoadingMessage()

		m.messages = append(m.messages, ChatMessageFinal{
			Role:      "assistant",
			Content:   msg.Content,
			Timestamp: time.Now(),
		})
		m.loading = false
		m.recalculateLayout()
		m.scrollToBottom()

		// Save to session
		m.saveSession()

	case AgentStreamMsgFinal:
		// Update loading message or add new content with debouncing
		m.updateStreamingContentDebounced(msg.Content)

	case AgentDoneMsgFinal:
		m.removeLoadingMessage()
		m.loading = false

	case AgentErrorMsgFinal:
		m.removeLoadingMessage()

		// Format error with icon and code
		errorContent := m.formatError(msg.Error)
		m.messages = append(m.messages, ChatMessageFinal{
			Role:      "error",
			Content:   errorContent,
			Timestamp: time.Now(),
		})
		m.loading = false
		m.recalculateLayout()
		m.scrollToBottom()

	case StreamTickMsg:
		// Periodic tick for streaming updates
		if m.loading && m.streamingContent != "" {
			m.updateStreamingContent(m.streamingContent)
		}
		return m, streamTickCmd(m.streamDebounce)
	}

	return m, nil
}

// View renders the model
func (m *TUIFinalModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Calculate heights
	titleHeight := 3
	inputHeight := 4
	chatHeight := m.height - titleHeight - inputHeight - 2 // -2 for small gaps

	if chatHeight < 1 {
		chatHeight = 1
	}

	// Render sections
	titleBar := m.renderTitleBar(m.width, titleHeight)
	chatArea := m.renderChatArea(m.width, chatHeight)
	inputBox := m.renderInputBox(m.width, inputHeight)

	// Combine vertically with minimal spacing
	mainView := titleBar + "\n" + chatArea + "\n" + inputBox

	// Render command palette overlay if open
	if m.showPalette {
		palette := m.renderCommandPalette(m.width, m.height)
		return m.overlayView(mainView, palette)
	}

	return mainView
}

// overlayView overlays one view on top of another
func (m *TUIFinalModel) overlayView(base, overlay string) string {
	// For simplicity, just return the overlay when palette is open
	// In a more sophisticated implementation, we would properly composite them
	return overlay
}

func (m *TUIFinalModel) renderTitleBar(width, height int) string {
	if height <= 0 {
		return ""
	}

	// Title (dynamic)
	title := m.sessionTitle

	// MCP status
	mcpConnected := 0
	for _, s := range m.mcpStatus {
		if s == "connected" {
			mcpConnected++
		}
	}

	// LSP status
	lspConnected := 0
	for _, s := range m.lspStatus {
		if s == "connected" {
			lspConnected++
		}
	}

	// Status icons
	mcpIcon := "○"
	mcpIconStyle := m.styles.StatusDisconnected
	if mcpConnected > 0 {
		mcpIcon = "●"
		mcpIconStyle = m.styles.StatusConnected
	}

	lspIcon := "○"
	lspIconStyle := m.styles.StatusDisconnected
	if lspConnected > 0 {
		lspIcon = "●"
		lspIconStyle = m.styles.StatusConnected
	}

	// Build the content line
	leftContent := title
	rightContent := fmt.Sprintf("MCP:%s  LSP:%s  %d tok",
		mcpIconStyle.Render(mcpIcon),
		lspIconStyle.Render(lspIcon),
		m.contextTokens)

	// Calculate padding to push right content to edge
	// Simple approach: pad with spaces
	padding := width - len(leftContent) - len(rightContent) - 2
	if padding < 3 {
		padding = 3
	}

	content := leftContent + strings.Repeat(" ", padding) + rightContent

	// Truncate if still too long
	if len(content) > width-2 {
		content = content[:width-4] + ".."
	}

	// Build the title bar with borders
	borderChar := "|"
	var result strings.Builder
	for i := 0; i < height; i++ {
		if i == height/2 {
			result.WriteString(borderChar + " " + content)
			// Fill remaining width
			remaining := width - 2 - len(content) - 1
			if remaining > 0 {
				result.WriteString(strings.Repeat(" ", remaining))
			}
		}
		result.WriteString(borderChar + "\n")
	}

	// Remove last newline and add closing border
	output := result.String()
	if len(output) > 0 && output[len(output)-1] == '\n' {
		output = output[:len(output)-1]
	}
	// Replace first border with styled border
	output = m.styles.TitleBarBorder.Render("|") + output[1:]

	return output
}

func (m *TUIFinalModel) renderChatArea(width, height int) string {
	if m.showWelcome {
		return m.renderWelcome(width, height)
	}

	if len(m.messages) == 0 {
		var result strings.Builder
		for i := 0; i < height; i++ {
			if i > 0 {
				result.WriteString("\n")
			}
			result.WriteString("|")
		}
		return result.String()
	}

	// Reserve space for scrollbar (1 char)
	contentWidth := width - 2
	if contentWidth < 1 {
		contentWidth = 1
	}

	// Calculate visible messages based on scroll position
	visibleMessages := m.getVisibleMessages(height)

	var content strings.Builder
	for i, msg := range visibleMessages {
		// Add small spacing between different message types
		if i > 0 {
			content.WriteString("\n")
		}
		rendered := m.renderMessage(msg, contentWidth-2)
		content.WriteString(rendered)
	}

	// Build chat area with border
	var result strings.Builder
	result.WriteString("|" + content.String())

	// Fill remaining height
	for i := 1; i < height; i++ {
		result.WriteString("\n|")
	}

	return result.String()
}

func (m *TUIFinalModel) renderMessage(msg ChatMessageFinal, width int) string {
	if width < 1 {
		width = 1
	}

	switch msg.Role {
	case "user":
		// User input with left border
		userBorder := m.styles.InputBoxBorder.Render("|")
		userContent := m.styles.UserMsg.Render(" " + msg.Content)
		return lipgloss.JoinHorizontal(lipgloss.Left, userBorder, userContent)

	case "assistant":
		// Assistant message with left margin
		lines := strings.Split(msg.Content, "\n")
		var result strings.Builder
		for _, line := range lines {
			result.WriteString(m.styles.AssistantMsg.Render("  " + line))
			result.WriteString("\n")
		}
		return result.String()

	case "error":
		// Error message with red highlighting and icon
		lines := strings.Split(msg.Content, "\n")
		var result strings.Builder
		for _, line := range lines {
			result.WriteString(m.styles.ErrorMsg.Render("  " + line))
			result.WriteString("\n")
		}
		return result.String()

	case "loading":
		// Loading indicator with thinking support
		var result strings.Builder

		// Show thinking content if available
		if msg.Thinking != "" {
			// Thinking style - dimmer, italic text
			thinkingLines := strings.Split(msg.Thinking, "\n")
			for _, line := range thinkingLines {
				result.WriteString(m.styles.DimText.Render("  │ " + line))
				result.WriteString("\n")
			}
		}

		if msg.Content != "" {
			result.WriteString(m.styles.LoadingMsg.Render("  " + msg.Content))
			result.WriteString("\n")
		} else if msg.Thinking == "" {
			result.WriteString(m.styles.LoadingMsg.Render("  Thinking..."))
			result.WriteString("\n")
		}
		return result.String()

	default:
		return msg.Content + "\n"
	}
}

func (m *TUIFinalModel) renderWelcome(width, height int) string {
	lines := []string{
		"ycode - Your AI Coding Assistant",
		"",
		"I can help you with:",
		"* Reading and writing code files",
		"* Executing shell commands",
		"* Searching through your codebase",
		"* Git operations",
		"",
		"Type your request and press Enter to get started.",
		"Type /help to see available commands.",
	}

	var result strings.Builder
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString("| " + line)
	}

	// Fill remaining height
	for i := len(lines); i < height; i++ {
		result.WriteString("\n|")
	}

	return result.String()
}

func (m *TUIFinalModel) renderInputBox(width, height int) string {
	if height <= 0 {
		return ""
	}

	// Build input display with cursor
	runes := []rune(m.input)
	maxWidth := width - 4
	if maxWidth < 5 {
		maxWidth = 5
	}

	// Simple word wrap
	var lines []string
	var current strings.Builder
	for _, r := range string(runes) {
		current.WriteRune(r)
		if current.Len() >= maxWidth {
			lines = append(lines, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	if len(lines) == 0 {
		lines = []string{""}
	}

	// Insert cursor into the first line (simplified)
	display := lines[0]
	if m.cursor <= len([]rune(display)) {
		runes2 := []rune(display)
		display = string(runes2[:m.cursor]) + "█" + string(runes2[m.cursor:])
	} else {
		display = display + "█"
	}

	// Model info at bottom
	modelInfo := "Model: " + m.config.LLM.Model + " | Ctrl+P:cmds | Ctrl+L:clear | Ctrl+C:quit"

	// Build content
	var content strings.Builder

	// Line 1: input with cursor
	content.WriteString(" " + display)

	// Middle lines: remaining input lines or empty
	for i := 1; i < len(lines) && i < height-2; i++ {
		content.WriteString("\n " + lines[i])
	}

	// Fill remaining space
	linesUsed := len(lines)
	if linesUsed > height-1 {
		linesUsed = height - 1
	}
	for i := linesUsed; i < height-1; i++ {
		content.WriteString("\n")
	}

	// Last line: model info (always at bottom)
	content.WriteString(m.styles.Muted.Render("\n " + modelInfo))

	// Build border
	var result strings.Builder
	for i := 0; i < height; i++ {
		if i == 0 {
			result.WriteString("|" + content.String())
		} else if i < height-1 {
			result.WriteString("\n|")
		} else {
			result.WriteString("\n|")
		}
	}

	return result.String()
}

// wrapText wraps text to fit within maxWidth
func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}

	var lines []string
	var currentLine strings.Builder
	width := 0

	for _, r := range text {
		rWidth := 1
		if r >= 0x4E00 && r <= 0x9FFF {
			rWidth = 2 // Chinese characters are wider
		}

		if width+rWidth > maxWidth {
			if currentLine.Len() > 0 {
				lines = append(lines, currentLine.String())
				currentLine.Reset()
				width = 0
			}
		}

		currentLine.WriteRune(r)
		width += rWidth
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	if len(lines) == 0 {
		lines = []string{""}
	}

	return lines
}

func (m *TUIFinalModel) renderScrollbar(height int) string {
	if height <= 0 || m.totalLines == 0 {
		return strings.Repeat(" ", height)
	}

	// Calculate scrollbar position based on actual lines
	visibleRatio := float64(m.visibleLines) / float64(m.totalLines)
	if visibleRatio > 1 {
		visibleRatio = 1
	}

	scrollPos := float64(m.scrollY) / float64(m.totalLines)
	if scrollPos > 1 {
		scrollPos = 1
	}

	// Build scrollbar
	scrollbar := make([]rune, height)
	thumbSize := int(float64(height) * visibleRatio)
	if thumbSize < 1 {
		thumbSize = 1
	}
	thumbPos := int(scrollPos * float64(height))

	for i := 0; i < height; i++ {
		if i >= thumbPos && i < thumbPos+thumbSize {
			scrollbar[i] = '#' // Thumb
		} else {
			scrollbar[i] = '|' // Track
		}
	}

	return string(scrollbar)
}

func (m *TUIFinalModel) getVisibleMessages(maxHeight int) []ChatMessageFinal {
	if len(m.messages) == 0 {
		return nil
	}

	// Calculate visible range based on lines, not messages
	startLine := m.scrollY
	endLine := startLine + maxHeight

	// Find messages that fit in the visible range
	var visibleMessages []ChatMessageFinal
	currentLine := 0

	for _, msg := range m.messages {
		msgLines := m.calculateMessageLines(msg, maxHeight)
		msgEndLine := currentLine + msgLines

		// Check if message overlaps with visible range
		if msgEndLine > startLine && currentLine < endLine {
			visibleMessages = append(visibleMessages, msg)
		}

		currentLine = msgEndLine

		// Stop if we've passed the visible range
		if currentLine >= endLine {
			break
		}
	}

	return visibleMessages
}

func (m *TUIFinalModel) calculateMessageLines(msg ChatMessageFinal, maxWidth int) int {
	if msg.RenderedLines > 0 {
		return msg.RenderedLines
	}

	// Calculate lines based on content
	lines := strings.Split(msg.Content, "\n")
	totalLines := 0

	for _, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth == 0 {
			totalLines++
		} else {
			// Calculate wrapped lines
			wrappedLines := (lineWidth + maxWidth - 1) / maxWidth
			if wrappedLines < 1 {
				wrappedLines = 1
			}
			totalLines += wrappedLines
		}
	}

	// Add padding
	totalLines++

	return totalLines
}

func (m *TUIFinalModel) recalculateLayout() {
	// Update context stats
	m.updateContextStats()

	// Calculate total lines
	m.totalLines = 0
	for i := range m.messages {
		m.messages[i].RenderedLines = m.calculateMessageLines(m.messages[i], m.width-4)
		m.totalLines += m.messages[i].RenderedLines
	}

	// Calculate visible lines
	titleHeight := 3
	inputHeight := 5
	m.visibleLines = m.height - titleHeight - inputHeight
	if m.visibleLines < 1 {
		m.visibleLines = 1
	}

	// Ensure scroll position is valid
	m.clampScrollY()
}

func (m *TUIFinalModel) clampScrollY() {
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

func (m *TUIFinalModel) scroll(delta int) {
	m.scrollY += delta
	m.clampScrollY()
}

func (m *TUIFinalModel) scrollToBottom() {
	m.scrollY = m.totalLines
	m.clampScrollY()
}

func (m *TUIFinalModel) handleCommand(input string) tea.Cmd {
	// Add user message (command)
	m.messages = append(m.messages, ChatMessageFinal{
		Role:      "user",
		Content:   input,
		Timestamp: time.Now(),
	})
	m.recalculateLayout()
	m.scrollToBottom()

	// Execute command
	handled, result, err := m.cmdManager.Execute(input)

	// Distinguish between "command not found" and "command execution failed"
	if !handled {
		// Command not found
		m.messages = append(m.messages, ChatMessageFinal{
			Role:      "assistant",
			Content:   fmt.Sprintf("Unknown command: %s. Type /help to see available commands.", input),
			Timestamp: time.Now(),
		})
	} else if err != nil {
		// Command execution failed
		m.messages = append(m.messages, ChatMessageFinal{
			Role:      "assistant",
			Content:   fmt.Sprintf("Command failed: %v", err),
			Timestamp: time.Now(),
		})
	} else {
		// Special handling for certain commands
		if input == "/clear" {
			m.messages = []ChatMessageFinal{}
			m.updateContextStats()
			m.recalculateLayout()
			return nil
		}
		if input == "/exit" || input == "/quit" {
			m.saveSession()
			return tea.Quit
		}

		// Show command result
		if result != "" {
			m.messages = append(m.messages, ChatMessageFinal{
				Role:      "assistant",
				Content:   result,
				Timestamp: time.Now(),
			})
		}
	}

	m.recalculateLayout()
	m.scrollToBottom()
	return nil
}

func (m *TUIFinalModel) sendMessage() tea.Cmd {
	userInput := m.input
	m.input = ""
	m.cursor = 0

	// Set session title from first message
	if !m.hasSetTitle {
		m.sessionTitle = generateTitleFinal(userInput)
		m.hasSetTitle = true

		// Update session title
		if m.sessionManager != nil && m.currentSession != nil {
			m.currentSession.Title = m.sessionTitle
			m.sessionManager.Save(m.currentSession)
		}
	}

	// Add user message
	m.messages = append(m.messages, ChatMessageFinal{
		Role:      "user",
		Content:   userInput,
		Timestamp: time.Now(),
	})

	// Add loading message
	m.messages = append(m.messages, ChatMessageFinal{
		Role:      "loading",
		Content:   "",
		Timestamp: time.Now(),
	})

	m.loading = true
	m.recalculateLayout()
	m.scrollToBottom()

	// Run agent in background with panic recovery
	return func() (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				msg = AgentErrorMsgFinal{Error: fmt.Errorf("panic: %v", r)}
			}
		}()

		ctx := context.Background()

		// Collect streaming content
		var streamingContent strings.Builder

		err := m.agent.Run(ctx, userInput, func(event llm.StreamEvent) {
			// Handle streaming events
			if event.Type == "content" {
				streamingContent.WriteString(event.Content)
			}
		})

		if err != nil {
			return AgentErrorMsgFinal{Error: err}
		}

		// Get response from history
		history := m.agent.GetHistory()
		if len(history) > 0 {
			lastMsg := history[len(history)-1]
			if lastMsg.Role == "assistant" {
				return AgentResponseMsgFinal{Content: lastMsg.Content}
			}
		}

		// If we have streaming content, use it
		if streamingContent.Len() > 0 {
			return AgentResponseMsgFinal{Content: streamingContent.String()}
		}

		return AgentDoneMsgFinal{}
	}
}

func (m *TUIFinalModel) removeLoadingMessage() {
	// Remove the last loading message
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "loading" {
			m.messages = append(m.messages[:i], m.messages[i+1:]...)
			break
		}
	}
	m.recalculateLayout()
}

func (m *TUIFinalModel) updateStreamingContent(content string) {
	// Find and update loading message or add new content
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "loading" {
			// Update loading message with streaming content
			m.messages[i].Content = content
			m.messages[i].Role = "assistant"
			m.recalculateLayout()
			m.scrollToBottom()
			return
		}
	}

	// If no loading message, add new assistant message
	m.messages = append(m.messages, ChatMessageFinal{
		Role:      "assistant",
		Content:   content,
		Timestamp: time.Now(),
	})
	m.recalculateLayout()
	m.scrollToBottom()
}

func (m *TUIFinalModel) saveSession() {
	if m.sessionManager == nil || m.currentSession == nil {
		return
	}

	// Convert messages to llm.Message
	var messages []llm.Message
	for _, msg := range m.messages {
		if msg.Role != "loading" {
			messages = append(messages, llm.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	m.currentSession.Messages = messages
	m.sessionManager.Save(m.currentSession)
}

func (m *TUIFinalModel) updateContextStats() {
	// Calculate total characters from all messages
	totalChars := 0
	for _, msg := range m.messages {
		// Count runes for proper Unicode support
		totalChars += len([]rune(msg.Content))
	}

	// Rough token estimation: ~4 characters per token
	m.contextTokens = totalChars / 4

	// Calculate usage percentage
	maxTokens := m.config.LLM.GetMaxTokens()
	if maxTokens > 0 {
		m.contextUsed = float64(m.contextTokens) / float64(maxTokens)
		if m.contextUsed > 1.0 {
			m.contextUsed = 1.0
		}
	}
}

// generateTitleFinal generates a title from the first message
func generateTitleFinal(message string) string {
	// Truncate and clean up the message
	title := message
	if len(title) > 50 {
		title = title[:50] + "..."
	}
	return title
}

// Message types
type (
	AgentResponseMsgFinal struct {
		Content string
	}

	AgentStreamMsgFinal struct {
		Content     string
		Thinking    string
		Done        bool
		Error       error
		ToolCall    *ToolCallInfo // Current tool being executed
		ToolResult  *ToolResultInfo // Result of tool execution
	}

	// ToolCallInfo represents a tool being called
	ToolCallInfo struct {
		Name      string
		Arguments map[string]interface{}
	}

	// ToolResultInfo represents the result of a tool call
	ToolResultInfo struct {
		Name    string
		Success bool
		Content string
	}

	AgentDoneMsgFinal struct{}

	AgentErrorMsgFinal struct {
		Error error
	}

	StreamTickMsg struct{}
)

// streamTickCmd creates a command that sends a tick message after a delay
func streamTickCmd(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(t time.Time) tea.Msg {
		return StreamTickMsg{}
	})
}

// updateStreamingContentDebounced updates streaming content with debouncing
func (m *TUIFinalModel) updateStreamingContentDebounced(content string) {
	m.streamingContent = content
	m.lastStreamUpdate = time.Now()

	// Update immediately if enough time has passed
	if time.Since(m.lastStreamUpdate) >= m.streamDebounce {
		m.updateStreamingContent(content)
	}
}

// formatError formats an error with icon and styling
func (m *TUIFinalModel) formatError(err error) string {
	errorIcon := "❌"
	errorCode := "ERROR"

	// Try to extract error code if available
	errStr := err.Error()
	if strings.Contains(errStr, ":") {
		parts := strings.SplitN(errStr, ":", 2)
		if len(parts) == 2 {
			errorCode = strings.TrimSpace(parts[0])
			errStr = strings.TrimSpace(parts[1])
		}
	}

	return fmt.Sprintf("%s [%s] %s", errorIcon, errorCode, errStr)
}

// updatePalette handles command palette input
func (m *TUIFinalModel) updatePalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.showPalette = false
		return m, nil

	case tea.KeyEnter:
		if len(m.filteredCommands) > 0 && m.paletteSelected < len(m.filteredCommands) {
			selectedCmd := m.filteredCommands[m.paletteSelected]
			m.showPalette = false
			m.input = selectedCmd
			m.cursor = len([]rune(selectedCmd))
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

// updateFilteredCommands filters commands based on palette input
func (m *TUIFinalModel) updateFilteredCommands() {
	allCommands := m.cmdManager.List()
	m.filteredCommands = []string{}

	if m.paletteInput == "" {
		// Show all commands
		for _, cmd := range allCommands {
			m.filteredCommands = append(m.filteredCommands, cmd.Name)
		}
	} else {
		// Fuzzy match
		inputLower := strings.ToLower(m.paletteInput)
		for _, cmd := range allCommands {
			if strings.Contains(strings.ToLower(cmd.Name), inputLower) ||
				strings.Contains(strings.ToLower(cmd.Description), inputLower) {
				m.filteredCommands = append(m.filteredCommands, cmd.Name)
			}
		}
	}

	// Reset selection if out of bounds
	if m.paletteSelected >= len(m.filteredCommands) {
		m.paletteSelected = 0
	}
}

// autoCompleteCommand auto-completes the current command input
func (m *TUIFinalModel) autoCompleteCommand() {
	if !strings.HasPrefix(m.input, "/") {
		return
	}

	allCommands := m.cmdManager.List()
	inputLower := strings.ToLower(m.input)

	var matches []string
	for _, cmd := range allCommands {
		if strings.HasPrefix(strings.ToLower(cmd.Name), inputLower) {
			matches = append(matches, cmd.Name)
		}
	}

	if len(matches) == 1 {
		// Single match - complete it
		m.input = matches[0] + " "
		m.cursor = len([]rune(m.input))
	} else if len(matches) > 1 {
		// Find common prefix
		commonPrefix := matches[0]
		for _, match := range matches[1:] {
			for !strings.HasPrefix(match, commonPrefix) {
				commonPrefix = commonPrefix[:len(commonPrefix)-1]
			}
		}
		if len(commonPrefix) > len(m.input) {
			m.input = commonPrefix
			m.cursor = len([]rune(m.input))
		}
	}
}

// renderCommandPalette renders the command palette overlay
func (m *TUIFinalModel) renderCommandPalette(width, height int) string {
	if !m.showPalette {
		return ""
	}

	// Calculate palette dimensions
	paletteWidth := width * 2 / 3
	if paletteWidth < 40 {
		paletteWidth = 40
	}
	paletteHeight := 15
	if paletteHeight > height-4 {
		paletteHeight = height - 4
	}

	// Build palette content
	var content strings.Builder

	// Header
	header := fmt.Sprintf(" Command Palette (Ctrl+P) ")
	content.WriteString(m.styles.Bold.Render(header))
	content.WriteString("\n\n")

	// Input field
	inputDisplay := "> " + m.paletteInput + "█"
	content.WriteString(inputDisplay)
	content.WriteString("\n\n")

	// Commands list
	visibleCount := paletteHeight - 5
	if visibleCount < 1 {
		visibleCount = 1
	}

	startIdx := 0
	if m.paletteSelected >= visibleCount {
		startIdx = m.paletteSelected - visibleCount + 1
	}

	endIdx := startIdx + visibleCount
	if endIdx > len(m.filteredCommands) {
		endIdx = len(m.filteredCommands)
	}

	for i := startIdx; i < endIdx; i++ {
		cmd := m.filteredCommands[i]
		if i == m.paletteSelected {
			content.WriteString(m.styles.PaletteSelected.Render("▸ " + cmd))
		} else {
			content.WriteString(m.styles.PaletteItem.Render("  " + cmd))
		}
		content.WriteString("\n")
	}

	// Footer
	content.WriteString("\n")
	footer := fmt.Sprintf(" %d commands | ↑↓ navigate | Enter select | Esc close ", len(m.filteredCommands))
	content.WriteString(m.styles.Muted.Render(footer))

	// Render with border
	palette := m.styles.Palette.
		Width(paletteWidth).
		Height(paletteHeight).
		Render(content.String())

	// Add border
	borderStyle := m.styles.PaletteBorder
	border := borderStyle.Render("┌" + strings.Repeat("─", paletteWidth-2) + "┐")
	border += "\n"
	for i := 0; i < paletteHeight-2; i++ {
		border += borderStyle.Render("│") + "\n"
	}
	border += borderStyle.Render("└" + strings.Repeat("─", paletteWidth-2) + "┘")

	// Center the palette
	paddingTop := (height - paletteHeight) / 2
	paddingLeft := (width - paletteWidth) / 2

	result := strings.Repeat("\n", paddingTop)
	result += strings.Repeat(" ", paddingLeft)
	result += palette

	return result
}

// renderStatusIndicators renders MCP and LSP status indicators
func (m *TUIFinalModel) renderStatusIndicators() string {
	var indicators []string

	// MCP status
	for name, status := range m.mcpStatus {
		var icon string
		var style lipgloss.Style
		switch status {
		case "connected":
			icon = "●"
			style = m.styles.StatusConnected
		case "error":
			icon = "✖"
			style = m.styles.StatusError
		default:
			icon = "○"
			style = m.styles.StatusDisconnected
		}
		indicators = append(indicators, fmt.Sprintf("MCP:%s%s", style.Render(icon), name))
	}

	// LSP status
	for name, status := range m.lspStatus {
		var icon string
		var style lipgloss.Style
		switch status {
		case "connected":
			icon = "●"
			style = m.styles.StatusConnected
		case "error":
			icon = "✖"
			style = m.styles.StatusError
		default:
			icon = "○"
			style = m.styles.StatusDisconnected
		}
		indicators = append(indicators, fmt.Sprintf("LSP:%s%s", style.Render(icon), name))
	}

	if len(indicators) == 0 {
		return ""
	}

	return strings.Join(indicators, " ")
}
