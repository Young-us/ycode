package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Young-us/ycode/internal/command"
)

// AutoCompleteType represents the type of autocomplete
type AutoCompleteType int

const (
	AutoCompleteNone AutoCompleteType = iota
	AutoCompleteCommand    // /command completion
	AutoCompleteFilePath   // file path completion
	AutoCompleteMixed      // mixed commands and files (when input is just "/")
)

// AutoCompleteState holds the current autocomplete state
type AutoCompleteState struct {
	Active        bool
	Type          AutoCompleteType
	Suggestions   []SuggestionItem
	SelectedIndex int
	StartPos      int // cursor position when autocomplete started
	Prefix        string
	PathPrefix    string // For file paths: the directory part
}

// SuggestionItem represents a single suggestion with metadata
type SuggestionItem struct {
	Text        string
	Display     string // What to display in the list
	Description string
	Type        string // "command", "file", "directory"
	IsDirectory bool
}

// AutoCompleter handles autocomplete functionality
type AutoCompleter struct {
	cmdManager *command.CommandManager
	workingDir string
	state      AutoCompleteState
}

// NewAutoCompleter creates a new autocomplete handler
func NewAutoCompleter(cmdManager *command.CommandManager, workingDir string) *AutoCompleter {
	return &AutoCompleter{
		cmdManager: cmdManager,
		workingDir: workingDir,
	}
}

// GetState returns the current autocomplete state
func (a *AutoCompleter) GetState() *AutoCompleteState {
	return &a.state
}

// ShouldTrigger checks if autocomplete should auto-trigger based on input
// This is called on every keystroke to auto-show suggestions
func (a *AutoCompleter) ShouldTrigger(input string, cursor int) bool {
	// Clean control characters (null bytes from Windows CMD after Alt+Tab)
	input = cleanInput(input)
	// Adjust cursor if it exceeds cleaned input length
	if cursor > len(input) {
		cursor = len(input)
	}

	if cursor == 0 {
		return false
	}

	// Always trigger when input starts with /
	if len(input) > 0 && input[0] == '/' {
		return true
	}

	// Trigger if current word looks like a path
	if a.isPathLikeInput(input, cursor) {
		return true
	}

	return false
}

// Trigger triggers autocomplete based on current input
func (a *AutoCompleter) Trigger(input string, cursor int) bool {
	// Clean control characters (null bytes from Windows CMD after Alt+Tab)
	input = cleanInput(input)
	// Adjust cursor if it exceeds cleaned input length
	if cursor > len(input) {
		cursor = len(input)
	}

	if cursor == 0 {
		return false
	}

	// Handle "/" - show both commands and current directory
	if input[0] == '/' {
		return a.triggerMixedCompletion(input, cursor)
	}

	// Handle file path completion
	if a.isPathLikeInput(input, cursor) {
		return a.triggerFilePathCompletion(input, cursor)
	}

	return false
}

// cleanInput removes control characters (null bytes) from input
func cleanInput(input string) string {
	return strings.Map(func(r rune) rune {
		if r == 0 || (r < 32 && r != '\t' && r != '\n' && r != '\r') {
			return -1 // Remove control characters including null bytes
		}
		return r
	}, input)
}

// triggerMixedCompletion handles "/" input - shows commands and current directory
func (a *AutoCompleter) triggerMixedCompletion(input string, cursor int) bool {
	var suggestions []SuggestionItem

	// Get the prefix after /
	prefix := ""
	if cursor > 1 {
		prefix = input[1:cursor]
	}

	// If prefix contains path separator, it's a path, not command
	if strings.Contains(prefix, "/") || strings.Contains(prefix, "\\") {
		return a.triggerFilePathCompletion(input, cursor)
	}

	// Get matching commands - Text does NOT include "/" since we're replacing from position 1
	commands := a.cmdManager.List()
	for _, cmd := range commands {
		if prefix == "" || strings.HasPrefix(strings.ToLower(cmd.Name), strings.ToLower(prefix)) {
			suggestions = append(suggestions, SuggestionItem{
				Text:        cmd.Name, // No "/" prefix - it will be preserved from input
				Display:     cmd.Name,
				Description: cmd.Description,
				Type:        "command",
			})
		}
	}

	// Also show files from current directory (for paths starting with /)
	// This allows / to work like absolute path from working dir
	fileSuggestions := a.getFileSuggestions(a.workingDir, prefix, 0)
	suggestions = append(suggestions, fileSuggestions...)

	if len(suggestions) == 0 {
		return false
	}

	// Sort: commands first, then files
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].Type == "command" && suggestions[j].Type != "command" {
			return true
		}
		if suggestions[i].Type != "command" && suggestions[j].Type == "command" {
			return false
		}
		return suggestions[i].Text < suggestions[j].Text
	})

	a.state = AutoCompleteState{
		Active:        true,
		Type:          AutoCompleteMixed,
		Suggestions:   suggestions,
		SelectedIndex: 0,
		StartPos:      1, // Start after the "/"
		Prefix:        prefix,
	}

	return true
}

// triggerFilePathCompletion handles file path completion with multi-level support
func (a *AutoCompleter) triggerFilePathCompletion(input string, cursor int) bool {
	// Find the start of the current path
	pathStart := a.findPathStart(input, cursor)
	if pathStart == -1 {
		pathStart = 0
	}

	// Extract the path being typed
	pathInput := input[pathStart:cursor]

	// Store the original input prefix (before the path) for reconstruction
	inputPrefix := input[:pathStart]

	// Handle "/" at the start - treat as working directory path
	// But we need to track this for proper replacement
	isSlashPath := len(pathInput) > 0 && pathInput[0] == '/' && !filepath.IsAbs(pathInput)

	// Find where the directory part ends and partial name begins
	var dir, partial string
	var dirPrefix string // The directory part in the original input format

	// Normalize separators
	pathInputNormalized := filepath.FromSlash(pathInput)

	// Expand ~ to home directory for directory lookup
	if strings.HasPrefix(pathInputNormalized, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		if pathInputNormalized == "~" {
			dir = homeDir
			partial = ""
			dirPrefix = "~/"
		} else if strings.Contains(pathInputNormalized, string(filepath.Separator)) {
			rest := strings.TrimPrefix(pathInputNormalized, "~"+string(filepath.Separator))
			parts := strings.Split(rest, string(filepath.Separator))
			if len(parts) > 0 {
				partial = parts[len(parts)-1]
				dir = filepath.Join(homeDir, strings.Join(parts[:len(parts)-1], string(filepath.Separator)))
				dirPrefix = "~/" + strings.Join(parts[:len(parts)-1], "/") + "/"
			} else {
				dir = homeDir
				partial = ""
				dirPrefix = "~/"
			}
		} else {
			dir = homeDir
			partial = strings.TrimPrefix(pathInputNormalized, "~")
			dirPrefix = "~/"
		}
	} else if strings.Contains(pathInputNormalized, string(filepath.Separator)) {
		// Path contains separators - split into dir and partial
		lastSep := strings.LastIndex(pathInput, "/")
		if lastSep == -1 {
			lastSep = strings.LastIndex(pathInput, "\\")
		}

		if lastSep >= 0 {
			dirPrefix = pathInput[:lastSep+1] // Keep the separator
			partial = pathInput[lastSep+1:]
		} else {
			dirPrefix = ""
			partial = pathInput
		}

		// Get actual directory for reading
		if isSlashPath {
			// "/" prefix means working directory
			dir = filepath.Join(a.workingDir, filepath.FromSlash(pathInput[:lastSep]))
			// Remove leading "/" from dirPrefix since StartPos=1 will preserve it
			dirPrefix = dirPrefix[1:]
		} else {
			dir = filepath.FromSlash(pathInput[:lastSep])
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(a.workingDir, dir)
			}
		}

		// Handle case where path ends with separator (e.g., "dir/")
		if strings.HasSuffix(pathInput, "/") || strings.HasSuffix(pathInput, "\\") {
			dir = filepath.Join(a.workingDir, filepath.FromSlash(pathInput))
			if isSlashPath {
				dir = a.workingDir + string(filepath.Separator) + filepath.FromSlash(pathInput[1:])
				// Remove leading "/" from dirPrefix since StartPos=1 will preserve it
				dirPrefix = pathInput[1:]
			} else {
				dirPrefix = pathInput
			}
			partial = ""
		}
	} else {
		// No separator - could be relative to working directory
		if pathStart > 0 {
			prevChar := input[pathStart-1]
			if prevChar == ' ' || prevChar == '"' || prevChar == '\'' {
				dir = a.workingDir
				partial = pathInput
				dirPrefix = ""
			} else {
				return false
			}
		} else {
			dir = a.workingDir
			partial = pathInput
			dirPrefix = ""
		}
	}

	// Get suggestions with full path prefix
	suggestions := a.getFileSuggestionsWithPrefix(dir, partial, dirPrefix)

	if len(suggestions) == 0 {
		return false
	}

	// For "/" paths, StartPos should be after the "/" so it's preserved
	startPos := pathStart
	if isSlashPath && pathStart == 0 && len(input) > 0 && input[0] == '/' {
		startPos = 1 // Preserve the "/" at the beginning
	}

	a.state = AutoCompleteState{
		Active:        true,
		Type:          AutoCompleteFilePath,
		Suggestions:   suggestions,
		SelectedIndex: 0,
		StartPos:      startPos,
		Prefix:        partial,
		PathPrefix:    dir,
	}

	_ = inputPrefix // Used for context
	return true
}

// getFileSuggestionsWithPrefix gets file/directory suggestions with path prefix preserved
func (a *AutoCompleter) getFileSuggestionsWithPrefix(dir, partial, dirPrefix string) []SuggestionItem {
	var suggestions []SuggestionItem

	entries, err := os.ReadDir(dir)
	if err != nil {
		return suggestions
	}

	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden files unless user is typing a hidden file prefix
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(partial, ".") {
			continue
		}

		if partial == "" || strings.HasPrefix(strings.ToLower(name), strings.ToLower(partial)) {
			isDir := entry.IsDir()
			display := name
			if isDir {
				display += "/" // Always use forward slash for display
			}

			// Text includes the directory prefix so the full path is preserved
			text := dirPrefix + name
			if isDir {
				text += "/"
			}

			suggestions = append(suggestions, SuggestionItem{
				Text:        text,
				Display:     display,
				Description: a.getFileDescription(entry),
				Type:        a.getFileType(entry),
				IsDirectory: isDir,
			})
		}
	}

	// Sort directories first, then files
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].IsDirectory && !suggestions[j].IsDirectory {
			return true
		}
		if !suggestions[i].IsDirectory && suggestions[j].IsDirectory {
			return false
		}
		return suggestions[i].Text < suggestions[j].Text
	})

	return suggestions
}

// getFileSuggestions gets file/directory suggestions for a given directory
func (a *AutoCompleter) getFileSuggestions(dir, partial string, pathStart int) []SuggestionItem {
	var suggestions []SuggestionItem

	entries, err := os.ReadDir(dir)
	if err != nil {
		return suggestions
	}

	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden files unless user is typing a hidden file prefix
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(partial, ".") {
			continue
		}

		if partial == "" || strings.HasPrefix(strings.ToLower(name), strings.ToLower(partial)) {
			isDir := entry.IsDir()
			display := name
			text := name
			if isDir {
				display += "/" // Always use forward slash for display
				text += "/"
			}

			suggestions = append(suggestions, SuggestionItem{
				Text:        text,
				Display:     display,
				Description: a.getFileDescription(entry),
				Type:        a.getFileType(entry),
				IsDirectory: isDir,
			})
		}
	}

	// Sort directories first, then files
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].IsDirectory && !suggestions[j].IsDirectory {
			return true
		}
		if !suggestions[i].IsDirectory && suggestions[j].IsDirectory {
			return false
		}
		return suggestions[i].Text < suggestions[j].Text
	})

	return suggestions
}

// getFileDescription returns a description for a file entry
func (a *AutoCompleter) getFileDescription(entry os.DirEntry) string {
	if entry.IsDir() {
		return "directory"
	}
	info, err := entry.Info()
	if err != nil {
		return "file"
	}
	// Format file size
	size := info.Size()
	if size < 1024 {
		return "file"
	} else if size < 1024*1024 {
		return "file"
	} else {
		return "file"
	}
}

// getFileType returns the type string for an entry
func (a *AutoCompleter) getFileType(entry os.DirEntry) string {
	if entry.IsDir() {
		return "directory"
	}
	return "file"
}

// isPathLikeInput checks if the current input looks like a path
func (a *AutoCompleter) isPathLikeInput(input string, cursor int) bool {
	if cursor == 0 || len(input) == 0 {
		return false
	}

	// Ensure cursor is within bounds
	if cursor > len(input) {
		cursor = len(input)
	}

	// Find current word
	start := cursor - 1
	for start >= 0 && input[start] != ' ' && input[start] != '"' && input[start] != '\'' {
		start--
	}
	start++

	if start >= cursor {
		return false
	}

	word := input[start:cursor]

	// Check for path indicators
	if len(word) == 0 {
		return false
	}

	// Starts with path indicators
	if word[0] == '.' || word[0] == '/' || word[0] == '~' || word[0] == '\\' {
		return true
	}

	// Windows drive letter
	if len(word) >= 2 && word[1] == ':' {
		return true
	}

	// Contains path separators
	if strings.Contains(word, "/") || strings.Contains(word, "\\") {
		return true
	}

	return false
}

// findPathStart finds the start position of a path in the input
func (a *AutoCompleter) findPathStart(input string, cursor int) int {
	start := cursor - 1
	for start >= 0 {
		c := input[start]
		if c == ' ' || c == '"' || c == '\'' || c == '`' || c == '(' || c == '[' || c == '{' {
			start++
			break
		}
		start--
	}

	if start < 0 {
		start = 0
	}

	return start
}

// Update updates the autocomplete suggestions based on input changes
// Returns true if suggestions were updated
func (a *AutoCompleter) Update(input string, cursor int) bool {
	if !a.state.Active {
		return false
	}

	// Re-trigger to get updated suggestions
	return a.Trigger(input, cursor)
}

// Navigate moves the selection in the suggestions list
func (a *AutoCompleter) Navigate(direction int) {
	if !a.state.Active || len(a.state.Suggestions) == 0 {
		return
	}

	a.state.SelectedIndex += direction
	if a.state.SelectedIndex < 0 {
		a.state.SelectedIndex = len(a.state.Suggestions) - 1
	}
	if a.state.SelectedIndex >= len(a.state.Suggestions) {
		a.state.SelectedIndex = 0
	}
}

// Accept applies the selected suggestion to the input
func (a *AutoCompleter) Accept(input string, cursor int) (string, int) {
	if !a.state.Active || len(a.state.Suggestions) == 0 {
		return input, cursor
	}

	selected := a.state.Suggestions[a.state.SelectedIndex]

	// Clean control characters from input (null bytes from Windows CMD)
	// This must match what Trigger does, otherwise StartPos will be wrong
	originalInput := input
	input = cleanInput(input)
	// Adjust cursor if it exceeds cleaned input length
	if cursor > len(input) {
		cursor = len(input)
	}

	// Calculate the new input
	before := input[:a.state.StartPos]
	after := input[cursor:]

	// For file paths, the Text already includes "/" for directories, so use it directly
	newText := selected.Text

	newInput := before + newText + after
	newCursor := a.state.StartPos + len(newText)

	// Add space after command completion
	if selected.Type == "command" {
		newInput = newInput[:newCursor] + " " + newInput[newCursor:]
		newCursor++
	}

	// Reset state
	a.state.Active = false
	a.state.Suggestions = nil

	_ = originalInput // for debugging if needed
	return newInput, newCursor
}

// Cancel cancels the current autocomplete
func (a *AutoCompleter) Cancel() {
	a.state.Active = false
	a.state.Suggestions = nil
}

// SetWorkingDir updates the working directory
func (a *AutoCompleter) SetWorkingDir(dir string) {
	a.workingDir = dir
}

// SuggestionsView renders the suggestions for display
type SuggestionsView struct {
	Items      []SuggestionItem
	Selected   int
	MaxVisible int
}

// NewSuggestionsView creates a new suggestions view
func NewSuggestionsView() *SuggestionsView {
	return &SuggestionsView{
		MaxVisible: 10,
	}
}

// SetItems sets the items to display
func (v *SuggestionsView) SetItems(items []SuggestionItem) {
	v.Items = items
	v.Selected = 0
}

// Navigate moves the selection
func (v *SuggestionsView) Navigate(direction int) {
	if len(v.Items) == 0 {
		return
	}

	v.Selected += direction
	if v.Selected < 0 {
		v.Selected = len(v.Items) - 1
	}
	if v.Selected >= len(v.Items) {
		v.Selected = 0
	}
}

// GetVisibleItems returns the visible items based on scroll
func (v *SuggestionsView) GetVisibleItems() []SuggestionItem {
	if len(v.Items) <= v.MaxVisible {
		return v.Items
	}

	start := 0
	if v.Selected >= v.MaxVisible {
		start = v.Selected - v.MaxVisible + 1
	}

	end := start + v.MaxVisible
	if end > len(v.Items) {
		end = len(v.Items)
	}

	return v.Items[start:end]
}