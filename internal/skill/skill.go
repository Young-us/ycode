package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Young-us/ycode/internal/logger"
	"gopkg.in/yaml.v3"
)

// SkillErrorCode represents the type of skill error
type SkillErrorCode string

const (
	// ErrCodeSkillNotFound indicates the skill was not found
	ErrCodeSkillNotFound SkillErrorCode = "SKILL_NOT_FOUND"
	// ErrCodeSkillLoadFailed indicates skill loading failed
	ErrCodeSkillLoadFailed SkillErrorCode = "SKILL_LOAD_FAILED"
	// ErrCodeSkillParseFailed indicates skill parsing failed
	ErrCodeSkillParseFailed SkillErrorCode = "SKILL_PARSE_FAILED"
	// ErrCodeSkillPermissionDenied indicates permission denied for skill
	ErrCodeSkillPermissionDenied SkillErrorCode = "SKILL_PERMISSION_DENIED"
	// ErrCodeSkillDirNotFound indicates skill directory not found
	ErrCodeSkillDirNotFound SkillErrorCode = "SKILL_DIR_NOT_FOUND"
)

// SkillError represents a structured skill error
type SkillError struct {
	Code    SkillErrorCode
	Message string
	Path    string
	Err     error
}

func (e *SkillError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("skill error [%s]: %s (path: %s): %v", e.Code, e.Message, e.Path, e.Err)
	}
	return fmt.Sprintf("skill error [%s]: %s (path: %s)", e.Code, e.Message, e.Path)
}

func (e *SkillError) Unwrap() error {
	return e.Err
}

// NewSkillError creates a new SkillError
func NewSkillError(code SkillErrorCode, message, path string, err error) *SkillError {
	return &SkillError{
		Code:    code,
		Message: message,
		Path:    path,
		Err:     err,
	}
}

// Skill represents a ycode skill
type Skill struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Triggers     []string `yaml:"triggers,omitempty"`    // Keywords that trigger this skill
	Commands     []string `yaml:"commands,omitempty"`    // Slash commands (e.g., "/review")
	Permissions  []string `yaml:"permissions,omitempty"` // Required permissions (e.g., ["bash", "file_write"])
	URL          string   `yaml:"url,omitempty"`         // Remote skill URL (placeholder)
	Instructions string   `yaml:"-"`                     // Loaded from SKILL.md content
	Directory    string   `yaml:"-"`                     // Path to skill directory
	Files        []string `yaml:"-"`                     // Supporting files
}

// AgentPermissions defines what permissions an agent has
type AgentPermissions struct {
	Permissions []string
}

// HasPermission checks if agent has a specific permission
func (a *AgentPermissions) HasPermission(perm string) bool {
	for _, p := range a.Permissions {
		if p == perm || p == "*" {
			return true
		}
	}
	return false
}

// HasAllPermissions checks if agent has all required permissions
func (a *AgentPermissions) HasAllPermissions(required []string) bool {
	for _, req := range required {
		if !a.HasPermission(req) {
			return false
		}
	}
	return true
}

// Manager manages skills with caching and async loading
type Manager struct {
	mu        sync.RWMutex
	skills    map[string]*Skill
	skillDirs []string
	loaded    bool
	loadOnce  sync.Once
	loadErr   error
}

// NewManager creates a new skill manager
func NewManager() *Manager {
	return &Manager{
		skills:    make(map[string]*Skill),
		skillDirs: defaultSkillDirs(),
	}
}

// defaultSkillDirs returns the default skill directories
// Priority (later overrides earlier): ~/.ycode/skills/, ~/.claude/skills/, skills/, .ycode/skills/, .claude/skills/
func defaultSkillDirs() []string {
	var dirs []string

	// Home directory based paths
	home, err := os.UserHomeDir()
	if err == nil {
		// ~/.ycode/skills/ (primary user skills)
		dirs = append(dirs, filepath.Join(home, ".ycode", "skills"))
		// ~/.claude/skills/
		dirs = append(dirs, filepath.Join(home, ".claude", "skills"))
	}

	// Current directory based paths
	cwd, err := os.Getwd()
	if err == nil {
		// skills/ (project built-in skills)
		dirs = append(dirs, filepath.Join(cwd, "skills"))
		// .ycode/skills/ (project-specific skills)
		dirs = append(dirs, filepath.Join(cwd, ".ycode", "skills"))
		// .claude/skills/ (Claude Code compatible project skills)
		dirs = append(dirs, filepath.Join(cwd, ".claude", "skills"))
	}

	return dirs
}

// dirs returns all configured skill directories
func (m *Manager) dirs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, len(m.skillDirs))
	copy(result, m.skillDirs)
	return result
}

// AddSkillDir adds a directory to search for skills
func (m *Manager) AddSkillDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Reset load state when directories change
	m.loaded = false
	m.loadOnce = sync.Once{}
	m.skillDirs = append(m.skillDirs, dir)
}

// LoadAll loads all skills from configured directories with caching
func (m *Manager) LoadAll() error {
	m.loadOnce.Do(func() {
		m.loadErr = m.loadAllSkills()
		m.loaded = true
	})
	return m.loadErr
}

// loadAllSkills performs the actual skill loading
func (m *Manager) loadAllSkills() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger.Debug("skill", "Loading skills from %d directories", len(m.skillDirs))

	for _, dir := range m.skillDirs {
		if err := m.loadFromDir(dir); err != nil {
			// Log warning but continue loading other directories
			if skillErr, ok := err.(*SkillError); ok {
				if skillErr.Code != ErrCodeSkillDirNotFound {
					logger.Warn("skill", "Failed to load from directory %s: %v", dir, err)
				}
			}
		}
	}

	logger.Info("skill", "Loaded %d skills", len(m.skills))
	return nil
}

func (m *Manager) loadFromDir(dir string) error {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return NewSkillError(ErrCodeSkillDirNotFound, "skill directory not found", dir, err)
	}

	// Find all SKILL.md files using glob pattern
	pattern := filepath.Join(dir, "**", "SKILL.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return NewSkillError(ErrCodeSkillLoadFailed, "failed to glob for skills", dir, err)
	}

	// Also check direct children
	entries, err := os.ReadDir(dir)
	if err != nil {
		return NewSkillError(ErrCodeSkillLoadFailed, "failed to read directory", dir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir := filepath.Join(dir, entry.Name())
		skillFile := filepath.Join(skillDir, "SKILL.md")

		// Check if SKILL.md exists
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			continue
		}

		// Avoid duplicates from glob
		alreadyFound := false
		for _, match := range matches {
			if match == skillFile {
				alreadyFound = true
				break
			}
		}
		if !alreadyFound {
			matches = append(matches, skillFile)
		}
	}

	// Load each skill
	for _, skillFile := range matches {
		skillDir := filepath.Dir(skillFile)
		skill, err := m.loadSkill(skillDir, skillFile)
		if err != nil {
			logger.Warn("skill", "Failed to load skill from %s: %v", skillDir, err)
			continue
		}

		m.skills[skill.Name] = skill
		logger.Debug("skill", "Loaded skill: %s (commands: %v, triggers: %v)", skill.Name, skill.Commands, skill.Triggers)
	}

	return nil
}

func (m *Manager) loadSkill(dir, file string) (*Skill, error) {
	// Read SKILL.md
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, NewSkillError(ErrCodeSkillLoadFailed, "failed to read skill file", file, err)
	}

	// Parse frontmatter
	skill, instructions, err := parseSkillFile(content)
	if err != nil {
		return nil, NewSkillError(ErrCodeSkillParseFailed, "failed to parse skill", file, err)
	}

	skill.Directory = dir
	skill.Instructions = instructions

	// Find supporting files
	files, err := findSupportingFiles(dir)
	if err == nil {
		skill.Files = files
	}

	return skill, nil
}

func parseSkillFile(content []byte) (*Skill, string, error) {
	lines := strings.Split(string(content), "\n")

	// Find frontmatter boundaries
	startIdx := -1
	endIdx := -1

	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if startIdx == -1 {
				startIdx = i
			} else {
				endIdx = i
				break
			}
		}
	}

	if startIdx == -1 || endIdx == -1 {
		return nil, "", fmt.Errorf("invalid SKILL.md format: missing frontmatter")
	}

	// Parse frontmatter
	frontmatter := strings.Join(lines[startIdx+1:endIdx], "\n")
	var skill Skill
	if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
		return nil, "", fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Get instructions (everything after frontmatter)
	instructions := strings.Join(lines[endIdx+1:], "\n")

	return &skill, instructions, nil
}

func findSupportingFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip SKILL.md and directories
		if info.IsDir() || filepath.Base(path) == "SKILL.md" {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		files = append(files, relPath)
		return nil
	})

	return files, err
}

// available returns skills that the agent has permission to use
func (m *Manager) available(agent *AgentPermissions) []*Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Skill
	for _, skill := range m.skills {
		// If no permissions required, skill is available
		if len(skill.Permissions) == 0 {
			result = append(result, skill)
			continue
		}

		// Check if agent has all required permissions
		if agent != nil && agent.HasAllPermissions(skill.Permissions) {
			result = append(result, skill)
		}
	}

	return result
}

// Available returns skills filtered by agent permissions
func (m *Manager) Available(agent *AgentPermissions) []*Skill {
	// Ensure skills are loaded
	if err := m.LoadAll(); err != nil {
		return nil
	}
	return m.available(agent)
}

// Get returns a skill by name
func (m *Manager) Get(name string) (*Skill, error) {
	// Ensure skills are loaded
	if err := m.LoadAll(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	skill, exists := m.skills[name]
	if !exists {
		logger.Debug("skill", "Skill not found: %s", name)
		return nil, NewSkillError(ErrCodeSkillNotFound, "skill not found", name, nil)
	}
	logger.Debug("skill", "Retrieved skill: %s", name)
	return skill, nil
}

// List returns all loaded skills
func (m *Manager) List() []*Skill {
	// Ensure skills are loaded
	if err := m.LoadAll(); err != nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	skills := make([]*Skill, 0, len(m.skills))
	for _, skill := range m.skills {
		skills = append(skills, skill)
	}
	return skills
}

// FindByTrigger finds skills that match a trigger keyword
func (m *Manager) FindByTrigger(trigger string) []*Skill {
	// Ensure skills are loaded
	if err := m.LoadAll(); err != nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var matched []*Skill
	trigger = strings.ToLower(trigger)

	for _, skill := range m.skills {
		for _, t := range skill.Triggers {
			if strings.ToLower(t) == trigger {
				matched = append(matched, skill)
				break
			}
		}
	}

	return matched
}

// FindByCommand finds a skill by slash command
func (m *Manager) FindByCommand(command string) (*Skill, bool) {
	// Ensure skills are loaded
	if err := m.LoadAll(); err != nil {
		return nil, false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	command = strings.ToLower(command)

	for _, skill := range m.skills {
		for _, cmd := range skill.Commands {
			if strings.ToLower(cmd) == command {
				logger.Debug("skill", "Found skill '%s' for command: %s", skill.Name, command)
				return skill, true
			}
		}
	}

	return nil, false
}

// GetInstructions returns the full instructions for a skill
func (s *Skill) GetInstructions() string {
	return s.Instructions
}

// FormatInstructions formats skill instructions with template variables
func (s *Skill) FormatInstructions(vars map[string]string) string {
	result := s.Instructions
	for key, value := range vars {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// GetSupportingFileContent reads a supporting file
func (s *Skill) GetSupportingFileContent(filename string) (string, error) {
	path := filepath.Join(s.Directory, filename)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", NewSkillError(ErrCodeSkillLoadFailed, "failed to read supporting file", path, err)
	}
	return string(content), nil
}

// IsRemote returns true if this is a remote skill (has URL)
func (s *Skill) IsRemote() bool {
	return s.URL != ""
}

// GetURL returns the remote skill URL (placeholder implementation)
func (s *Skill) GetURL() string {
	return s.URL
}

// Reload forces a reload of all skills
func (m *Manager) Reload() error {
	m.mu.Lock()
	m.skills = make(map[string]*Skill)
	m.loaded = false
	m.loadOnce = sync.Once{}
	m.mu.Unlock()

	return m.LoadAll()
}
