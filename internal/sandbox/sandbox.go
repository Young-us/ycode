package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Young-us/ycode/internal/logger"
)

// Config holds sandbox configuration
type Config struct {
	Enabled           bool          `json:"enabled"`
	Timeout           time.Duration `json:"timeout"`
	MaxMemoryMB       int           `json:"max_memory_mb"`       // Max memory in MB (0 = unlimited)
	MaxFileSizeMB     int           `json:"max_file_size_mb"`    // Max file size in MB (0 = unlimited)
	AllowNetwork      bool          `json:"allow_network"`       // Allow network access
	AllowWrite        bool          `json:"allow_write"`         // Allow file writes
	AllowedPaths      []string      `json:"allowed_paths"`       // Allowed directories for writes
	BlockedCommands   []string      `json:"blocked_commands"`    // Commands that are always blocked
	RestrictedEnvVars []string      `json:"restricted_env_vars"` // Environment variables to hide
}

// DefaultConfig returns default sandbox configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:         true,
		Timeout:         30 * time.Second,
		MaxMemoryMB:     512,  // 512MB default
		MaxFileSizeMB:   100,  // 100MB default
		AllowNetwork:    false,
		AllowWrite:      true,
		AllowedPaths:    []string{},  // Empty = allow working directory only
		BlockedCommands: []string{"rm -rf /", "mkfs", "dd if=", "sudo"},
		RestrictedEnvVars: []string{"API_KEY", "SECRET", "PASSWORD", "TOKEN"},
	}
}

// Sandbox provides isolated command execution
type Sandbox struct {
	config    *Config
	workDir   string
	mu        sync.RWMutex
}

// New creates a new sandbox
func New(workDir string, config *Config) *Sandbox {
	if config == nil {
		config = DefaultConfig()
	}

	return &Sandbox{
		config:  config,
		workDir: workDir,
	}
}

// Result holds the result of sandbox execution
type Result struct {
	Stdout     string        `json:"stdout"`
	Stderr     string        `json:"stderr"`
	ExitCode   int           `json:"exit_code"`
	Duration   time.Duration `json:"duration"`
	TimedOut   bool          `json:"timed_out"`
	MemoryUsed int64         `json:"memory_used"` // bytes
}

// Execute runs a command in the sandbox
func (s *Sandbox) Execute(ctx context.Context, command string, args ...string) (*Result, error) {
	if !s.config.Enabled {
		return s.executeDirect(ctx, command, args...)
	}

	// Check if command is blocked
	if s.isBlocked(command) {
		return nil, fmt.Errorf("command is blocked: %s", command)
	}

	// Create command with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, command, args...)
	cmd.Dir = s.workDir

	// Set up environment
	cmd.Env = s.buildEnv()

	// Set up process attributes for platform-specific settings
	s.setupProcess(cmd)

	// Capture output
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	result := &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	// Check for timeout
	if cmdCtx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.ExitCode = -1
		return result, fmt.Errorf("command timed out after %v", s.config.Timeout)
	}

	// Get exit code
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		return result, err
	}

	result.ExitCode = 0
	return result, nil
}

// ExecuteShell runs a shell command in the sandbox
func (s *Sandbox) ExecuteShell(ctx context.Context, shellCommand string) (*Result, error) {
	if !s.config.Enabled {
		return s.executeShellDirect(ctx, shellCommand)
	}

	// Check for blocked patterns in the command
	if s.isBlocked(shellCommand) {
		return nil, fmt.Errorf("command contains blocked pattern")
	}

	// Check for dangerous patterns
	if s.hasDangerousPattern(shellCommand) {
		return nil, fmt.Errorf("command contains dangerous pattern")
	}

	// Use appropriate shell
	var shell string
	var flag string
	if runtime.GOOS == "windows" {
		shell = "cmd"
		flag = "/C"
	} else {
		shell = "/bin/sh"
		flag = "-c"
	}

	return s.Execute(ctx, shell, flag, shellCommand)
}

// executeDirect runs command without sandbox restrictions
func (s *Sandbox) executeDirect(ctx context.Context, command string, args ...string) (*Result, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = s.workDir

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	result := &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		return result, err
	}

	result.ExitCode = 0
	return result, nil
}

// executeShellDirect runs shell command without sandbox restrictions
func (s *Sandbox) executeShellDirect(ctx context.Context, shellCommand string) (*Result, error) {
	var shell string
	var flag string
	if runtime.GOOS == "windows" {
		shell = "cmd"
		flag = "/C"
	} else {
		shell = "/bin/sh"
		flag = "-c"
	}

	return s.executeDirect(ctx, shell, flag, shellCommand)
}

// isBlocked checks if a command is blocked
func (s *Sandbox) isBlocked(command string) bool {
	commandLower := strings.ToLower(command)
	for _, blocked := range s.config.BlockedCommands {
		if strings.Contains(commandLower, strings.ToLower(blocked)) {
			return true
		}
	}
	return false
}

// hasDangerousPattern checks for dangerous patterns
func (s *Sandbox) hasDangerousPattern(command string) bool {
	dangerousPatterns := []string{
		"rm -rf /",
		"rm -rf /*",
		":(){:|:&};:", // Fork bomb
		"> /dev/sda",
		"mkfs",
		"dd if=/dev/zero",
		"chmod -R 777 /",
	}

	commandLower := strings.ToLower(command)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(commandLower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// buildEnv builds a sanitized environment
func (s *Sandbox) buildEnv() []string {
	env := os.Environ()

	// Remove restricted environment variables
	var sanitized []string
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]

		// Check if this variable should be hidden
		restricted := false
		for _, r := range s.config.RestrictedEnvVars {
			if strings.Contains(strings.ToUpper(key), strings.ToUpper(r)) {
				restricted = true
				break
			}
		}

		if !restricted {
			sanitized = append(sanitized, e)
		}
	}

	return sanitized
}

// ValidatePath checks if a path is allowed for write operations
func (s *Sandbox) ValidatePath(path string) error {
	if !s.config.AllowWrite {
		return fmt.Errorf("file writes are disabled in sandbox")
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// If no allowed paths specified, only allow working directory
	if len(s.config.AllowedPaths) == 0 {
		if !strings.HasPrefix(absPath, s.workDir) {
			return fmt.Errorf("path outside working directory not allowed: %s", path)
		}
		return nil
	}

	// Check against allowed paths
	for _, allowed := range s.config.AllowedPaths {
		absAllowed, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absAllowed) {
			return nil
		}
	}

	return fmt.Errorf("path not in allowed directories: %s", path)
}

// CheckNetworkAccess checks if network access is allowed
func (s *Sandbox) CheckNetworkAccess() error {
	if s.config.AllowNetwork {
		return nil
	}
	return fmt.Errorf("network access is disabled in sandbox")
}

// CheckFileSize validates file size against limit
func (s *Sandbox) CheckFileSize(size int64) error {
	if s.config.MaxFileSizeMB <= 0 {
		return nil
	}

	maxBytes := int64(s.config.MaxFileSizeMB) * 1024 * 1024
	if size > maxBytes {
		return fmt.Errorf("file size %d bytes exceeds limit of %d MB", size, s.config.MaxFileSizeMB)
	}
	return nil
}

// KillProcess kills a running process by PID (platform-specific)
func (s *Sandbox) KillProcess(pid int) error {
	return killProcess(pid)
}

// setupProcess configures process attributes for platform-specific settings
func (s *Sandbox) setupProcess(cmd *exec.Cmd) {
	// Platform-specific setup is done in setupProcessPlatform
	setupProcessPlatform(cmd)
}

// SetConfig updates the sandbox configuration
func (s *Sandbox) SetConfig(config *Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}

// GetConfig returns the current configuration
func (s *Sandbox) GetConfig() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// IsEnabled returns whether sandbox is enabled
func (s *Sandbox) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Enabled
}

// Enable enables the sandbox
func (s *Sandbox) Enable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Enabled = true
}

// Disable disables the sandbox
func (s *Sandbox) Disable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Enabled = false
}

// CreateTempFile creates a temporary file within the sandbox
func (s *Sandbox) CreateTempFile(pattern string) (*os.File, error) {
	tempDir := filepath.Join(s.workDir, ".sandbox", "tmp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	return os.CreateTemp(tempDir, pattern)
}

// Cleanup removes temporary sandbox files
func (s *Sandbox) Cleanup() error {
	tempDir := filepath.Join(s.workDir, ".sandbox")
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(tempDir)
}

// Report generates a sandbox report
func (s *Sandbox) Report() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("=== Sandbox Report ===\n")
	sb.WriteString(fmt.Sprintf("Enabled: %v\n", s.config.Enabled))
	sb.WriteString(fmt.Sprintf("Working Directory: %s\n", s.workDir))
	sb.WriteString(fmt.Sprintf("Timeout: %v\n", s.config.Timeout))
	sb.WriteString(fmt.Sprintf("Max Memory: %d MB\n", s.config.MaxMemoryMB))
	sb.WriteString(fmt.Sprintf("Max File Size: %d MB\n", s.config.MaxFileSizeMB))
	sb.WriteString(fmt.Sprintf("Allow Network: %v\n", s.config.AllowNetwork))
	sb.WriteString(fmt.Sprintf("Allow Write: %v\n", s.config.AllowWrite))
	sb.WriteString(fmt.Sprintf("Allowed Paths: %v\n", s.config.AllowedPaths))
	sb.WriteString(fmt.Sprintf("Blocked Commands: %v\n", s.config.BlockedCommands))
	return sb.String()
}

// SafeBashTool wraps the bash tool with sandbox execution
type SafeBashTool struct {
	sandbox *Sandbox
}

// NewSafeBashTool creates a new safe bash tool
func NewSafeBashTool(sandbox *Sandbox) *SafeBashTool {
	return &SafeBashTool{sandbox: sandbox}
}

// Execute runs a bash command safely
func (t *SafeBashTool) Execute(ctx context.Context, command string) (*Result, error) {
	logger.Debug("sandbox", "Executing command: %s", command)
	return t.sandbox.ExecuteShell(ctx, command)
}