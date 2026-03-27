package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Young-us/ycode/internal/agent"
	"github.com/Young-us/ycode/internal/llm"
	"github.com/Young-us/ycode/internal/logger"
)

// Session represents a conversation session
type Session struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Messages    []llm.Message    `json:"messages"`
	Metadata    Metadata         `json:"metadata"`
	Summary     *agent.SessionSummary `json:"summary,omitempty"` // Compressed summary
	IsCompacted bool             `json:"is_compacted"`        // Whether history is compacted
}

// Metadata contains session metadata
type Metadata struct {
	Model       string `json:"model"`
	MaxTokens   int    `json:"max_tokens"`
	TotalTokens int    `json:"total_tokens"`
}

// Manager manages session persistence with auto-save
type Manager struct {
	dataDir       string
	autoSave      bool
	saveInterval  time.Duration
	pendingSave   map[string]*Session
	saveTimer     *time.Timer
	mu            sync.RWMutex
	compactor     *agent.Compactor
}

// ManagerOption is a functional option for Manager
type ManagerOption func(*Manager)

// WithAutoSave enables auto-save with specified interval
func WithAutoSave(interval time.Duration) ManagerOption {
	return func(m *Manager) {
		m.autoSave = true
		m.saveInterval = interval
	}
}

// WithCompactor enables session compaction
func WithCompactor(compactor *agent.Compactor) ManagerOption {
	return func(m *Manager) {
		m.compactor = compactor
	}
}

// NewManager creates a new session manager
func NewManager(opts ...ManagerOption) (*Manager, error) {
	// Use project-level directory: .ycode/sessions
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	dataDir := filepath.Join(cwd, ".ycode", "sessions")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	m := &Manager{
		dataDir:      dataDir,
		autoSave:     false,
		saveInterval: 5 * time.Second,
		pendingSave:  make(map[string]*Session),
	}

	for _, opt := range opts {
		opt(m)
	}

	if m.autoSave {
		m.startAutoSave()
	}

	return m, nil
}

// NewManagerWithPath creates a new session manager with a custom path
func NewManagerWithPath(path string, opts ...ManagerOption) (*Manager, error) {
	dataDir := filepath.Join(path, ".ycode", "sessions")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	m := &Manager{
		dataDir:      dataDir,
		autoSave:     false,
		saveInterval: 5 * time.Second,
		pendingSave:  make(map[string]*Session),
	}

	for _, opt := range opts {
		opt(m)
	}

	if m.autoSave {
		m.startAutoSave()
	}

	return m, nil
}

// startAutoSave starts the auto-save timer
func (m *Manager) startAutoSave() {
	m.saveTimer = time.NewTimer(m.saveInterval)
	go m.autoSaveLoop()
}

// autoSaveLoop periodically saves pending sessions
func (m *Manager) autoSaveLoop() {
	for range m.saveTimer.C {
		m.flushPending()
	}
}

// flushPending saves all pending sessions
func (m *Manager) flushPending() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, session := range m.pendingSave {
		if err := m.saveSync(session); err != nil {
			logger.Error("session", "Failed to auto-save session %s: %v", id, err)
		}
	}
	m.pendingSave = make(map[string]*Session)
}

// GetDataDir returns the session data directory
func (m *Manager) GetDataDir() string {
	return m.dataDir
}

// Create creates a new session
func (m *Manager) Create(title string) (*Session, error) {
	session := &Session{
		ID:        fmt.Sprintf("session-%d", time.Now().Unix()),
		Title:     title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []llm.Message{},
		Metadata:  Metadata{},
	}

	if err := m.Save(session); err != nil {
		return nil, err
	}

	return session, nil
}

// Save saves a session to disk (with debouncing if auto-save is enabled)
func (m *Manager) Save(session *Session) error {
	if m.autoSave {
		m.mu.Lock()
		m.pendingSave[session.ID] = session
		m.mu.Unlock()
		m.saveTimer.Reset(m.saveInterval)
		return nil
	}
	return m.saveSync(session)
}

// saveSync saves a session synchronously
func (m *Manager) saveSync(session *Session) error {
	session.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(m.dataDir, fmt.Sprintf("%s.json", session.ID))

	// Write to temp file first, then rename for atomicity
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tempPath, filePath)
}

// Load loads a session from disk
func (m *Manager) Load(id string) (*Session, error) {
	filePath := filepath.Join(m.dataDir, fmt.Sprintf("%s.json", id))

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// List lists all sessions, sorted by most recently updated first
func (m *Manager) List() ([]Session, error) {
	files, err := filepath.Glob(filepath.Join(m.dataDir, "*.json"))
	if err != nil {
		return nil, err
	}

	var sessions []Session
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		sessions = append(sessions, session)
	}

	// Sort by UpdatedAt descending (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// Delete deletes a session
func (m *Manager) Delete(id string) error {
	filePath := filepath.Join(m.dataDir, fmt.Sprintf("%s.json", id))
	return os.Remove(filePath)
}

// AddMessage adds a message to a session
func (m *Manager) AddMessage(session *Session, msg llm.Message) error {
	session.Messages = append(session.Messages, msg)
	return m.Save(session)
}

// CompactSession compacts a session's message history
func (m *Manager) CompactSession(session *Session, keepRecent int) error {
	if m.compactor == nil {
		return fmt.Errorf("compactor not configured")
	}

	if len(session.Messages) <= keepRecent {
		return nil
	}

	// Create context for compaction
	ctx := context.Background()

	// Compact the messages
	newMessages, err := m.compactor.CompactToHistory(ctx, session.Messages, keepRecent)
	if err != nil {
		return fmt.Errorf("compaction failed: %w", err)
	}

	session.Messages = newMessages
	session.IsCompacted = true

	// Update summary
	session.Summary = &agent.SessionSummary{
		CreatedAt:   time.Now(),
		MessageCount: len(session.Messages),
		TokenCount:  m.compactor.EstimateTokens(session.Messages),
	}

	return m.Save(session)
}

// UpdateTokenUsage updates the token usage for a session
func (m *Manager) UpdateTokenUsage(session *Session, inputTokens, outputTokens int) {
	session.Metadata.TotalTokens = inputTokens + outputTokens
}

// GetRecentSessions returns the N most recent sessions
func (m *Manager) GetRecentSessions(limit int) ([]Session, error) {
	sessions, err := m.List()
	if err != nil {
		return nil, err
	}

	if len(sessions) > limit {
		sessions = sessions[:limit]
	}

	return sessions, nil
}

// SearchSessions searches sessions by title or content
func (m *Manager) SearchSessions(query string) ([]Session, error) {
	sessions, err := m.List()
	if err != nil {
		return nil, err
	}

	var results []Session
	queryLower := []byte(query)

	for _, session := range sessions {
		// Search in title
		if containsIgnoreCase(session.Title, queryLower) {
			results = append(results, session)
			continue
		}

		// Search in messages
		for _, msg := range session.Messages {
			if containsIgnoreCase(msg.Content, queryLower) {
				results = append(results, session)
				break
			}
		}
	}

	return results, nil
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s string, substr []byte) bool {
	// Simple case-insensitive check
	sLower := make([]byte, len(s))
	for i, c := range []byte(s) {
		if c >= 'A' && c <= 'Z' {
			sLower[i] = c + 32
		} else {
			sLower[i] = c
		}
	}

	// Check if sLower contains substr
	return contains(sLower, substr)
}

func contains(s, substr []byte) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			ssc := substr[j]
			if ssc >= 'A' && ssc <= 'Z' {
				ssc += 32
			}
			if sc != ssc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// ExportSession exports a session to a file
func (m *Manager) ExportSession(session *Session, path string) error {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ImportSession imports a session from a file
func (m *Manager) ImportSession(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	// Generate new ID to avoid conflicts
	session.ID = fmt.Sprintf("session-%d", time.Now().Unix())
	session.CreatedAt = time.Now()
	session.UpdatedAt = time.Now()

	if err := m.Save(&session); err != nil {
		return nil, err
	}

	return &session, nil
}

// Close flushes pending saves and stops auto-save
func (m *Manager) Close() error {
	if m.saveTimer != nil {
		m.saveTimer.Stop()
	}
	m.flushPending()
	return nil
}

// GenerateTitle generates a title for the session based on the first message
func GenerateTitle(firstMessage string) string {
	// Truncate and clean up the message
	title := firstMessage
	if len(title) > 50 {
		title = title[:50] + "..."
	}
	return title
}