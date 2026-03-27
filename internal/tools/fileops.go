package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileOperation represents a file operation record for undo
type FileOperation struct {
	ID          string
	Path        string
	AbsPath     string
	Operation   string // "write", "edit", "delete"
	OldContent  string
	NewContent  string
	Timestamp   time.Time
	Checksum    string
}

// FileOpsManager manages file operations with safety features
type FileOpsManager struct {
	WorkingDir   string
	BackupDir    string
	operations   []FileOperation
	currentIndex int
	mu           sync.RWMutex
	maxHistory   int
	plugins      PluginHookTrigger
}

// NewFileOpsManager creates a new file operations manager
func NewFileOpsManager(workingDir string) *FileOpsManager {
	backupDir := filepath.Join(workingDir, ".ycode", "backups")
	os.MkdirAll(backupDir, 0755)

	return &FileOpsManager{
		WorkingDir:   workingDir,
		BackupDir:    backupDir,
		operations:   make([]FileOperation, 0),
		currentIndex: -1,
		maxHistory:   100,
	}
}

// SetPluginManager sets the plugin manager
func (m *FileOpsManager) SetPluginManager(plugins PluginHookTrigger) {
	m.plugins = plugins
}

// PreviewOperation returns a diff preview of the operation without executing it
func (m *FileOpsManager) PreviewOperation(path, newContent string) (*DiffResult, error) {
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(m.WorkingDir, path)
	}

	// Security: prevent directory traversal
	relPath, err := filepath.Rel(m.WorkingDir, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return nil, fmt.Errorf("path '%s' is outside working directory", path)
	}

	// Read existing content
	oldContent := ""
	if data, err := os.ReadFile(absPath); err == nil {
		oldContent = string(data)
	}

	// Compute diff
	return ComputeDiff(oldContent, newContent), nil
}

// WriteFileWithBackup writes a file with backup for undo
func (m *FileOpsManager) WriteFileWithBackup(ctx context.Context, path, content string, preview bool) (*ToolResult, *DiffResult, error) {
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(m.WorkingDir, path)
	}

	// Security: prevent directory traversal
	relPath, err := filepath.Rel(m.WorkingDir, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return &ToolResult{
			Content: fmt.Sprintf("Error: path '%s' is outside working directory", path),
			IsError: true,
		}, nil, nil
	}

	// Read existing content for backup
	oldContent := ""
	fileExists := false
	if data, err := os.ReadFile(absPath); err == nil {
		oldContent = string(data)
		fileExists = true
	}

	// Compute diff for preview
	diff := ComputeDiff(oldContent, content)

	// If preview only, return diff without writing
	if preview {
		return nil, diff, nil
	}

	// Trigger on_file_write hook
	if m.plugins != nil && m.plugins.Enabled() {
		hookResult, err := m.plugins.Trigger(ctx, "on_file_write", map[string]interface{}{
			"path":     path,
			"abs_path": absPath,
			"content":  content,
		})
		if err != nil {
			return &ToolResult{
				Content: fmt.Sprintf("Plugin hook error: %v", err),
				IsError: true,
			}, nil, nil
		}
		if modifiedContent, ok := hookResult["content"].(string); ok {
			content = modifiedContent
		}
		if skip, ok := hookResult["skip"].(bool); ok && skip {
			return &ToolResult{Content: "Write skipped by plugin", IsError: false}, nil, nil
		}
	}

	// Create backup before writing
	backupPath := ""
	if fileExists && oldContent != "" {
		backupPath, err = m.createBackup(absPath, oldContent)
		if err != nil {
			return &ToolResult{
				Content: fmt.Sprintf("Error creating backup: %v", err),
				IsError: true,
			}, nil, nil
		}
	}

	// Create parent directories if needed
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error creating directory '%s': %v", dir, err),
			IsError: true,
		}, nil, nil
	}

	// Atomic write: write to temp file first, then rename
	tempPath := absPath + ".tmp"
	if err := os.WriteFile(tempPath, []byte(content), 0644); err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error writing file '%s': %v", path, err),
			IsError: true,
		}, nil, nil
	}

	// Rename temp file to target (atomic on most filesystems)
	if err := os.Rename(tempPath, absPath); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return &ToolResult{
			Content: fmt.Sprintf("Error renaming file '%s': %v", path, err),
			IsError: true,
		}, nil, nil
	}

	// Record operation for undo
	checksum := computeChecksum(content)
	m.recordOperation(FileOperation{
		ID:         generateOpID(),
		Path:       path,
		AbsPath:    absPath,
		Operation:  "write",
		OldContent: oldContent,
		NewContent: content,
		Timestamp:  time.Now(),
		Checksum:   checksum,
	})

	// Clean up old backup after successful write
	if backupPath != "" {
		// Keep backup for undo, just log
		// Don't remove: backup is needed for undo functionality
	}

	if fileExists {
		return &ToolResult{
			Content: fmt.Sprintf("File '%s' overwritten successfully", path),
			IsError: false,
		}, diff, nil
	}

	return &ToolResult{
		Content: fmt.Sprintf("File '%s' created successfully", path),
		IsError: false,
	}, diff, nil
}

// EditFileWithBackup edits a file with backup for undo
func (m *FileOpsManager) EditFileWithBackup(ctx context.Context, path, edit string, preview bool) (*ToolResult, *DiffResult, error) {
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(m.WorkingDir, path)
	}

	// Security check
	relPath, err := filepath.Rel(m.WorkingDir, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return &ToolResult{
			Content: fmt.Sprintf("Error: path '%s' is outside working directory", path),
			IsError: true,
		}, nil, nil
	}

	// Read existing content
	oldContent, err := os.ReadFile(absPath)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error reading file '%s': %v", path, err),
			IsError: true,
		}, nil, nil
	}

	// Parse edit blocks
	blocks, err := parseEditBlocks(edit)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("Error parsing edit: %v", err),
			IsError: true,
		}, nil, nil
	}

	// Apply edits
	result := string(oldContent)
	for _, block := range blocks {
		if !strings.Contains(result, block.Search) {
			return &ToolResult{
				Content: fmt.Sprintf("Error: SEARCH text not found in file:\n%s", block.Search),
				IsError: true,
			}, nil, nil
		}
		result = strings.Replace(result, block.Search, block.Replace, 1)
	}

	// Compute diff
	diff := ComputeDiff(string(oldContent), result)

	// If preview only, return diff without writing
	if preview {
		return nil, diff, nil
	}

	// Write with backup
	return m.WriteFileWithBackup(ctx, path, result, false)
}

// Undo reverts the last file operation
func (m *FileOpsManager) Undo() (*ToolResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.operations) == 0 || m.currentIndex < 0 {
		return &ToolResult{
			Content: "No operations to undo",
			IsError: true,
		}, nil
	}

	op := m.operations[m.currentIndex]

	// Restore old content
	if op.OldContent != "" {
		if err := os.WriteFile(op.AbsPath, []byte(op.OldContent), 0644); err != nil {
			return &ToolResult{
				Content: fmt.Sprintf("Error undoing operation: %v", err),
				IsError: true,
			}, nil
		}
	} else {
		// File was created, delete it
		if err := os.Remove(op.AbsPath); err != nil {
			return &ToolResult{
				Content: fmt.Sprintf("Error removing created file: %v", err),
				IsError: true,
			}, nil
		}
	}

	m.currentIndex--

	return &ToolResult{
		Content: fmt.Sprintf("Undone: %s on '%s'", op.Operation, op.Path),
		IsError: false,
	}, nil
}

// Redo re-applies a previously undone operation
func (m *FileOpsManager) Redo() (*ToolResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentIndex >= len(m.operations)-1 {
		return &ToolResult{
			Content: "No operations to redo",
			IsError: true,
		}, nil
	}

	m.currentIndex++
	op := m.operations[m.currentIndex]

	// Re-apply new content
	if err := os.WriteFile(op.AbsPath, []byte(op.NewContent), 0644); err != nil {
		m.currentIndex--
		return &ToolResult{
			Content: fmt.Sprintf("Error redoing operation: %v", err),
			IsError: true,
		}, nil
	}

	return &ToolResult{
		Content: fmt.Sprintf("Redone: %s on '%s'", op.Operation, op.Path),
		IsError: false,
	}, nil
}

// GetHistory returns the operation history
func (m *FileOpsManager) GetHistory() []FileOperation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]FileOperation, len(m.operations))
	copy(result, m.operations)
	return result
}

// CanUndo returns true if there are operations to undo
func (m *FileOpsManager) CanUndo() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentIndex >= 0
}

// CanRedo returns true if there are operations to redo
func (m *FileOpsManager) CanRedo() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentIndex < len(m.operations)-1
}

// createBackup creates a backup of the file
func (m *FileOpsManager) createBackup(absPath, content string) (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	relPath, _ := filepath.Rel(m.WorkingDir, absPath)
	backupName := fmt.Sprintf("%s.%s.bak", strings.ReplaceAll(relPath, string(filepath.Separator), "_"), timestamp)
	backupPath := filepath.Join(m.BackupDir, backupName)

	if err := os.WriteFile(backupPath, []byte(content), 0644); err != nil {
		return "", err
	}

	return backupPath, nil
}

// recordOperation records a file operation for undo
func (m *FileOpsManager) recordOperation(op FileOperation) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If we're not at the end of history, truncate
	if m.currentIndex < len(m.operations)-1 {
		m.operations = m.operations[:m.currentIndex+1]
	}

	m.operations = append(m.operations, op)
	m.currentIndex = len(m.operations) - 1

	// Trim history if too long
	if len(m.operations) > m.maxHistory {
		m.operations = m.operations[len(m.operations)-m.maxHistory:]
		m.currentIndex = len(m.operations) - 1
	}
}

// parseEditBlocks parses SEARCH/REPLACE edit blocks
func parseEditBlocks(edit string) ([]editBlock, error) {
	var blocks []editBlock

	parts := strings.Split(edit, "<<<<<<< SEARCH")

	for i := 1; i < len(parts); i++ {
		part := parts[i]

		separatorIdx := strings.Index(part, "=======")
		if separatorIdx == -1 {
			return nil, fmt.Errorf("missing ======= separator in edit block %d", i)
		}

		endIdx := strings.Index(part, ">>>>>>> REPLACE")
		if endIdx == -1 {
			return nil, fmt.Errorf("missing >>>>>>> REPLACE marker in edit block %d", i)
		}

		search := strings.TrimSpace(part[:separatorIdx])
		replace := strings.TrimSpace(part[separatorIdx+7 : endIdx])

		blocks = append(blocks, editBlock{
			Search:  search,
			Replace: replace,
		})
	}

	if len(blocks) == 0 {
		return nil, fmt.Errorf("no valid SEARCH/REPLACE blocks found")
	}

	return blocks, nil
}

// Helper functions
func computeChecksum(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])[:8]
}

func generateOpID() string {
	return fmt.Sprintf("op-%d", time.Now().UnixNano())
}