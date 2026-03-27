package logger

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Time    time.Time
	Level   string
	Message string
	Source  string // plugin, agent, tool, etc.
}

// Logger is a custom logger that stores logs in memory
type Logger struct {
	mu       sync.RWMutex
	entries  []LogEntry
	maxSize  int
	callback func(entry LogEntry)
}

// Global logger instance
var globalLogger = &Logger{
	entries: make([]LogEntry, 0),
	maxSize: 1000,
}

// SetCallback sets a callback function to be called when a new log is added
func (l *Logger) SetCallback(cb func(entry LogEntry)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.callback = cb
}

// Add adds a new log entry
func (l *Logger) Add(level, source, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: message,
		Source:  source,
	}

	l.entries = append(l.entries, entry)

	// Keep only the last maxSize entries
	if len(l.entries) > l.maxSize {
		l.entries = l.entries[len(l.entries)-l.maxSize:]
	}

	// Call callback if set
	if l.callback != nil {
		l.callback(entry)
	}
}

// GetEntries returns all log entries
func (l *Logger) GetEntries() []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]LogEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

// GetLines returns log entries as formatted strings
func (l *Logger) GetLines() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	lines := make([]string, len(l.entries))
	for i, entry := range l.entries {
		timeStr := entry.Time.Format("15:04:05")
		lines[i] = fmt.Sprintf("[%s] [%s] [%s] %s", timeStr, strings.ToUpper(entry.Level), strings.ToUpper(entry.Source), entry.Message)
	}
	return lines
}

// Clear clears all log entries
func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make([]LogEntry, 0)
}

// Global functions
func SetCallback(cb func(entry LogEntry)) {
	globalLogger.SetCallback(cb)
}

func Info(source, format string, args ...interface{}) {
	globalLogger.Add("info", source, fmt.Sprintf(format, args...))
}

func Warn(source, format string, args ...interface{}) {
	globalLogger.Add("warn", source, fmt.Sprintf(format, args...))
}

func Error(source, format string, args ...interface{}) {
	globalLogger.Add("error", source, fmt.Sprintf(format, args...))
}

func Debug(source, format string, args ...interface{}) {
	globalLogger.Add("debug", source, fmt.Sprintf(format, args...))
}

func Plugin(format string, args ...interface{}) {
	globalLogger.Add("info", "plugin", fmt.Sprintf(format, args...))
}

func PluginError(format string, args ...interface{}) {
	globalLogger.Add("error", "plugin", fmt.Sprintf(format, args...))
}

func Agent(format string, args ...interface{}) {
	globalLogger.Add("info", "agent", fmt.Sprintf(format, args...))
}

func Tool(format string, args ...interface{}) {
	globalLogger.Add("info", "tool", fmt.Sprintf(format, args...))
}

func GetEntries() []LogEntry {
	return globalLogger.GetEntries()
}

func GetLines() []string {
	return globalLogger.GetLines()
}

func Clear() {
	globalLogger.Clear()
}