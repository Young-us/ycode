package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Young-us/ycode/internal/config"
	"github.com/Young-us/ycode/internal/logger"
)

// Manager manages plugin lifecycle and hook execution.
// It provides thread-safe operations for loading, unloading, and triggering plugins.
type Manager struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	hooks   map[string][]Hook
	config  config.PluginsConfig
	enabled bool

	// Hot-reload support
	watchEnabled bool
	watchers     map[string]*PluginWatcher
	watchMu      sync.Mutex
	watchStop    chan struct{}
}

// NewManager creates a new plugin manager with the given configuration.
func NewManager(cfg config.PluginsConfig) *Manager {
	return &Manager{
		plugins:      make(map[string]Plugin),
		hooks:        make(map[string][]Hook),
		config:       cfg,
		enabled:      cfg.Enabled,
		watchEnabled: cfg.HotReload,
		watchers:     make(map[string]*PluginWatcher),
		watchStop:    make(chan struct{}),
	}
}

// RegisterHook registers a hook handler for a specific hook name.
// Plugins should call this during Initialize() to register their hooks.
// Lower priority values execute first.
func (m *Manager) RegisterHook(hookName string, pluginName string, handler HookFunc, priority int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !isValidHook(hookName) {
		return fmt.Errorf("invalid hook name: %s", hookName)
	}

	hook := Hook{
		Name:     hookName,
		Plugin:   pluginName,
		Handler:  handler,
		Priority: priority,
	}

	m.hooks[hookName] = append(m.hooks[hookName], hook)

	// Sort hooks by priority
	sort.Slice(m.hooks[hookName], func(i, j int) bool {
		return m.hooks[hookName][i].Priority < m.hooks[hookName][j].Priority
	})

	logger.Plugin("Registered hook %s for plugin %s (priority: %d)", hookName, pluginName, priority)
	return nil
}

// UnregisterHooks removes all hooks registered by a plugin.
func (m *Manager) UnregisterHooks(pluginName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for hookName, hooks := range m.hooks {
		filtered := make([]Hook, 0, len(hooks))
		for _, h := range hooks {
			if h.Plugin != pluginName {
				filtered = append(filtered, h)
			}
		}
		m.hooks[hookName] = filtered
	}
}

// Trigger executes all registered handlers for a hook synchronously.
// Handlers are called in priority order. If a handler returns an error,
// the error is logged but execution continues (errors don't stop the chain).
// Args are passed through each handler and can be modified.
func (m *Manager) Trigger(ctx context.Context, hookName string, args map[string]interface{}) (map[string]interface{}, error) {
	if !m.enabled {
		return args, nil
	}

	m.mu.RLock()
	hooks := make([]Hook, len(m.hooks[hookName]))
	copy(hooks, m.hooks[hookName])
	m.mu.RUnlock()

	if len(hooks) == 0 {
		return args, nil
	}

	currentArgs := args
	for _, hook := range hooks {
		select {
		case <-ctx.Done():
			return currentArgs, ctx.Err()
		default:
		}

		result, err := hook.Handler(ctx, currentArgs)
		if err != nil {
			// Log error but DON'T return it - plugins should not break the main program
			// Just continue with current args
			logger.PluginError("Hook %s failed for plugin %s: %v", hookName, hook.Plugin, err)
			continue
		}

		// Update args with the result from this handler
		if result != nil {
			currentArgs = result
		}
	}

	return currentArgs, nil
}

// TriggerAsync executes all registered handlers for a hook asynchronously.
// This is for notification-only hooks where we don't need to wait for results.
// The hook handlers run in background goroutines.
func (m *Manager) TriggerAsync(ctx context.Context, hookName string, args map[string]interface{}) {
	if !m.enabled {
		return
	}

	m.mu.RLock()
	hooks := make([]Hook, len(m.hooks[hookName]))
	copy(hooks, m.hooks[hookName])
	m.mu.RUnlock()

	if len(hooks) == 0 {
		return
	}

	// Execute each hook in a separate goroutine
	for _, hook := range hooks {
		go func(h Hook) {
			// Create a child context with timeout to prevent hanging
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := h.Handler(ctx, args)
			if err != nil {
				logger.PluginError("Async hook %s failed for plugin %s: %v", hookName, h.Plugin, err)
			}
		}(hook)
	}
}

// Load loads a single plugin by its implementation.
// The plugin's Initialize() method is called during loading.
func (m *Manager) Load(ctx context.Context, p Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := p.Name()
	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("plugin already loaded: %s", name)
	}

	logger.Plugin("Loading plugin: %s v%s", name, p.Version())

	// Initialize the plugin
	if err := p.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %w", name, err)
	}

	m.plugins[name] = p
	logger.Plugin("Plugin loaded successfully: %s", name)

	return nil
}

// LoadFromConfig loads plugins based on configuration.
// It discovers plugins from multiple directories in order:
// 1. plugins/ - System built-in plugins (shipped with ycode)
// 2. ~/.ycode/plugins/ - Global user plugins
// 3. .ycode/plugins/ - Project-specific plugins
// 4. [directory] - Custom configured directory (if specified)
func (m *Manager) LoadFromConfig(ctx context.Context) error {
	if !m.enabled {
		logger.Plugin("Plugin system disabled by configuration")
		return nil
	}

	// Collect all plugin directories
	dirs := []string{}

	// 1. System built-in plugins (plugins/ in working directory)
	systemDir := "plugins"
	if info, err := os.Stat(systemDir); err == nil && info.IsDir() {
		dirs = append(dirs, systemDir)
	}

	// 2. Global user plugins (~/.ycode/plugins/)
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalDir := filepath.Join(homeDir, ".ycode", "plugins")
		dirs = append(dirs, globalDir)
	}

	// 3. Project-specific plugins (.ycode/plugins/ in working directory)
	projectDir := filepath.Join(".ycode", "plugins")
	if info, err := os.Stat(projectDir); err == nil && info.IsDir() {
		dirs = append(dirs, projectDir)
	}

	// 4. Custom configured directory (highest priority)
	if m.config.Directory != "" {
		// Resolve ~ to home directory
		configDir := m.config.Directory
		if strings.HasPrefix(configDir, "~/") && homeDir != "" {
			configDir = filepath.Join(homeDir, configDir[2:])
		}
		dirs = append(dirs, configDir)
		logger.Plugin("Custom plugin directory: %s", configDir)
	}

	// Load plugins from all directories
	return m.LoadFromDirectories(ctx, dirs)
}

// LoadFromDirectories loads plugins from multiple directories.
// Later directories can override plugins from earlier directories.
func (m *Manager) LoadFromDirectories(ctx context.Context, dirs []string) error {
	loadedCount := 0

	for _, dir := range dirs {
		// Create directory if it doesn't exist (only for global/project dirs)
		if dir != "plugins" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				logger.Plugin("Warning: failed to create directory %s: %v", dir, err)
				continue
			}
		}

		// Check if directory exists
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			logger.Plugin("Directory not found: %s (skipping)", dir)
			continue
		}

		logger.Plugin("Scanning for plugins in: %s", dir)

		// Load plugins from this directory
		count, err := m.loadFromSingleDirectory(ctx, dir)
		if err != nil {
			logger.Plugin("Warning: error loading from %s: %v", dir, err)
			continue
		}
		loadedCount += count
	}

	logger.Plugin("Total plugins loaded: %d", loadedCount)
	return nil
}

// loadFromSingleDirectory discovers and loads plugins from a single directory.
// Supports Go native plugins (.so on Linux, .dylib on macOS, .dll on Windows)
// and script-based plugins (.py, .lua, .js).
// Returns the number of plugins loaded.
func (m *Manager) loadFromSingleDirectory(ctx context.Context, dir string) (int, error) {
	logger.Plugin("Scanning for plugins in: %s", dir)

	// Find all plugin files
	pluginFiles := make(map[string][]string) // extension -> files

	// Go native plugins
	for _, pattern := range []string{"*.so", "*.dylib", "*.dll"} {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			logger.Plugin("Warning: error scanning for %s: %v", pattern, err)
			continue
		}
		if len(matches) > 0 {
			pluginFiles["native"] = append(pluginFiles["native"], matches...)
		}
	}

	// Script-based plugins
	for _, pattern := range []string{"*.py", "*.lua", "*.js"} {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			logger.Plugin("Warning: error scanning for %s: %v", pattern, err)
			continue
		}
		if len(matches) > 0 {
			ext := filepath.Ext(pattern)
			pluginFiles[ext] = append(pluginFiles[ext], matches...)
		}
	}

	totalFiles := 0
	for _, files := range pluginFiles {
		totalFiles += len(files)
	}

	if totalFiles == 0 {
		logger.Plugin("No plugin files found in %s", dir)
		return 0, nil
	}

	logger.Plugin("Found %d plugin file(s) in %s", totalFiles, dir)

	loadedCount := 0

	// Load native Go plugins
	for _, file := range pluginFiles["native"] {
		if err := m.loadNativePlugin(ctx, file); err != nil {
			logger.Plugin("Failed to load native plugin %s: %v", filepath.Base(file), err)
		} else {
			loadedCount++
		}
	}

	// Load script plugins
	for ext, files := range pluginFiles {
		if ext == "native" {
			continue
		}
		for _, file := range files {
			if err := m.loadScriptPlugin(ctx, file, ext); err != nil {
				logger.Plugin("Failed to load script plugin %s: %v", filepath.Base(file), err)
			} else {
				loadedCount++
			}
		}
	}

	return loadedCount, nil
}

// LoadFromDirectory is kept for backward compatibility.
// It loads plugins from a single directory.
func (m *Manager) LoadFromDirectory(ctx context.Context, dir string) error {
	_, err := m.loadFromSingleDirectory(ctx, dir)
	return err
}

// loadNativePlugin loads a Go native plugin from .so/.dylib/.dll file.
// The plugin must export a "NewPlugin" function that returns a Plugin interface.
func (m *Manager) loadNativePlugin(ctx context.Context, path string) error {
	// Check if Go plugin package is available on this platform
	// Note: Go's plugin package doesn't work well on Windows
	// We'll skip native plugins on Windows and suggest script plugins instead

	logger.Plugin("Loading native plugin: %s", filepath.Base(path))

	// For now, log that native plugins require compilation with matching Go version
	// In a real implementation, you would use plugin.Open() here
	logger.Plugin("Native plugin loading requires:")
	logger.Plugin("  1. Plugin compiled with same Go version")
	logger.Plugin("  2. Plugin exports 'NewPlugin' function")
	logger.Plugin("  3. Platform support (Linux/macOS preferred)")

	return fmt.Errorf("native plugin loading not fully implemented - use script plugins instead")
}

// loadScriptPlugin loads a script-based plugin (.py, .lua, .js).
// Script plugins must implement the plugin interface via JSON-RPC or similar.
func (m *Manager) loadScriptPlugin(ctx context.Context, path string, ext string) error {
	logger.Plugin("Loading script plugin: %s", filepath.Base(path))

	// Read plugin metadata from the script file
	metadata, err := m.parsePluginMetadata(path)
	if err != nil {
		return fmt.Errorf("failed to parse plugin metadata: %w", err)
	}

	logger.Plugin("Plugin metadata: name=%s, version=%s, hooks=%d",
		metadata.Name, metadata.Version, len(metadata.Hooks))

	// Create a script plugin wrapper
	plugin := &ScriptPlugin{
		name:        metadata.Name,
		version:     metadata.Version,
		description: metadata.Description,
		scriptPath:  path,
		scriptType:  ext,
		manager:     m,
		hooks:       metadata.Hooks,
	}

	// Initialize the plugin
	if err := plugin.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize script plugin: %w", err)
	}

	// Register the plugin
	m.mu.Lock()
	m.plugins[plugin.Name()] = plugin
	m.mu.Unlock()

	logger.Plugin("Script plugin loaded successfully: %s", plugin.Name())
	return nil
}

// PluginMetadata represents metadata parsed from a plugin file.
type PluginMetadata struct {
	Name        string
	Version     string
	Description string
	Hooks       []HookConfig
	Priority    int
}

// HookConfig represents a hook configuration for a plugin.
type HookConfig struct {
	Name     string
	Priority int
}

// parsePluginMetadata extracts metadata from a plugin file.
// For script plugins, metadata is expected in comments at the top of the file.
func (m *Manager) parsePluginMetadata(path string) (*PluginMetadata, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	metadata := &PluginMetadata{
		Name:        strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Version:     "1.0.0",
		Description: "Script plugin",
		Hooks:       []HookConfig{},
		Priority:    100,
	}

	// Temporary storage for hooks (we'll create HookConfigs after parsing priority)
	var hookNames []string

	// Parse metadata from comments
	for i, line := range lines {
		if i > 30 { // Check first 30 lines
			break
		}

		line = strings.TrimSpace(line)

		// Handle different comment styles
		var comment string
		if strings.HasPrefix(line, "#") {
			comment = strings.TrimPrefix(line, "#")
		} else if strings.HasPrefix(line, "//") {
			comment = strings.TrimPrefix(line, "//")
		} else if strings.HasPrefix(line, "--") {
			comment = strings.TrimPrefix(line, "--")
		} else if strings.HasPrefix(line, "*") {
			// Handle multi-line comment style: * name: xxx
			comment = strings.TrimPrefix(line, "*")
		} else {
			continue
		}

		comment = strings.TrimSpace(comment)
		lowerComment := strings.ToLower(comment)

		// Parse metadata fields
		if strings.HasPrefix(lowerComment, "name:") {
			metadata.Name = strings.TrimSpace(strings.TrimPrefix(comment, "name:"))
		} else if strings.HasPrefix(lowerComment, "version:") {
			metadata.Version = strings.TrimSpace(strings.TrimPrefix(comment, "version:"))
		} else if strings.HasPrefix(lowerComment, "description:") {
			metadata.Description = strings.TrimSpace(strings.TrimPrefix(comment, "description:"))
		} else if strings.HasPrefix(lowerComment, "priority:") {
			priorityStr := strings.TrimSpace(strings.TrimPrefix(lowerComment, "priority:"))
			if priority, err := strconv.Atoi(priorityStr); err == nil {
				metadata.Priority = priority
			}
		} else if strings.HasPrefix(lowerComment, "hooks:") {
			// Parse hooks: hook1, hook2, hook3 (store names, create HookConfigs later)
			hooksStr := strings.TrimSpace(strings.TrimPrefix(comment, "hooks:"))
			for _, name := range strings.Split(hooksStr, ",") {
				name = strings.TrimSpace(name)
				if name != "" {
					hookNames = append(hookNames, name)
				}
			}
		}
	}

	// Now create HookConfigs with the correct priority
	for _, name := range hookNames {
		if isValidHook(name) {
			metadata.Hooks = append(metadata.Hooks, HookConfig{
				Name:     name,
				Priority: metadata.Priority,
			})
		} else {
			logger.Plugin("Warning: invalid hook name '%s' in %s", name, filepath.Base(path))
		}
	}

	// Default to on_tool_execute if no hooks specified
	if len(metadata.Hooks) == 0 {
		metadata.Hooks = append(metadata.Hooks, HookConfig{
			Name:     "on_tool_execute",
			Priority: metadata.Priority,
		})
	}

	return metadata, nil
}

// ScriptPlugin wraps a script-based plugin.
type ScriptPlugin struct {
	name        string
	version     string
	description string
	scriptPath  string
	scriptType  string
	manager     *Manager
	hooks       []HookConfig
	cachedCmd   string // Cached command for script execution
}

func (p *ScriptPlugin) Name() string        { return p.name }
func (p *ScriptPlugin) Version() string     { return p.version }
func (p *ScriptPlugin) Description() string { return p.description }

func (p *ScriptPlugin) Initialize(ctx context.Context) error {
	logger.Plugin("Initializing script plugin: %s", p.name)

	// Cache the command for script execution (avoid repeated lookups)
	switch p.scriptType {
	case ".py":
		p.cachedCmd = findPythonCommand()
	case ".lua":
		p.cachedCmd = "lua"
	case ".js":
		p.cachedCmd = "node"
	}

	// Register all hooks specified in metadata
	for _, hookConfig := range p.hooks {
		hookName := hookConfig.Name
		priority := hookConfig.Priority

		// Capture hook name for closure
		hookNameCopy := hookName

		handler := func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
			return p.executeScript(ctx, hookNameCopy, args)
		}

		if err := p.manager.RegisterHook(hookName, p.name, handler, priority); err != nil {
			return fmt.Errorf("failed to register hook %s: %w", hookName, err)
		}

		logger.Plugin("Registered hook: %s (priority: %d)", hookName, priority)
	}

	return nil
}

func (p *ScriptPlugin) Shutdown(ctx context.Context) error {
	logger.Plugin("Shutting down script plugin: %s", p.name)
	p.manager.UnregisterHooks(p.name)
	return nil
}

// findPythonCommand finds the best Python command for the current system.
// On Windows, it tries "py", "python", "python3" in order.
// On other systems, it tries "python3", "python" in order.
func findPythonCommand() string {
	// List of Python commands to try
	var commands []string

	// On Windows, prefer "py" launcher first
	if isWindows() {
		commands = []string{"py", "python", "python3"}
	} else {
		commands = []string{"python3", "python"}
	}

	for _, cmd := range commands {
		if _, err := exec.LookPath(cmd); err == nil {
			return cmd
		}
	}

	// Default fallback
	if isWindows() {
		return "py"
	}
	return "python3"
}

// isWindows returns true if running on Windows
func isWindows() bool {
	return strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") ||
		strings.HasSuffix(strings.ToLower(os.Getenv("ComSpec")), ".exe")
}

// executeScript executes the script plugin with given arguments.
func (p *ScriptPlugin) executeScript(ctx context.Context, hookName string, args map[string]interface{}) (map[string]interface{}, error) {
	// Use cached command
	if p.cachedCmd == "" {
		return args, fmt.Errorf("no cached command for script type: %s", p.scriptType)
	}

	cmd := exec.CommandContext(ctx, p.cachedCmd, p.scriptPath)

	// Pass arguments and hook name via environment variables
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return args, fmt.Errorf("failed to marshal args: %w", err)
	}

	cmd.Env = append(os.Environ(),
		"PLUGIN_ARGS="+string(argsJSON),
		"PLUGIN_HOOK="+hookName,
		"PLUGIN_NAME="+p.name,
	)

	// Execute script
	output, err := cmd.CombinedOutput()
	if err != nil {
		return args, fmt.Errorf("script execution failed: %w\nOutput: %s", err, string(output))
	}

	// Parse result
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		// If output is not JSON, return original args
		logger.Plugin("Script output is not JSON, returning original args")
		return args, nil
	}

	return result, nil
}

// Unload unloads a plugin by name.
// The plugin's Shutdown() method is called and all its hooks are removed.
func (m *Manager) Unload(ctx context.Context, name string) error {
	m.mu.Lock()
	p, exists := m.plugins[name]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("plugin not found: %s", name)
	}
	delete(m.plugins, name)
	m.mu.Unlock()

	logger.Plugin("Unloading plugin: %s", name)

	// Unregister all hooks for this plugin
	m.UnregisterHooks(name)

	// Shutdown the plugin
	if err := p.Shutdown(ctx); err != nil {
		logger.Plugin("Warning: plugin %s shutdown error: %v", name, err)
		return fmt.Errorf("failed to shutdown plugin %s: %w", name, err)
	}

	logger.Plugin("Plugin unloaded: %s", name)
	return nil
}

// Get returns a plugin by name.
func (m *Manager) Get(name string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, exists := m.plugins[name]
	return p, exists
}

// List returns information about all loaded plugins.
func (m *Manager) List() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]PluginInfo, 0, len(m.plugins))
	for _, p := range m.plugins {
		infos = append(infos, PluginInfo{
			Name:        p.Name(),
			Version:     p.Version(),
			Description: p.Description(),
			Enabled:     true,
		})
	}

	// Sort by name for consistent output
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos
}

// ListHooks returns information about all registered hooks.
func (m *Manager) ListHooks() map[string][]Hook {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string][]Hook, len(m.hooks))
	for name, hooks := range m.hooks {
		hookCopy := make([]Hook, len(hooks))
		copy(hookCopy, hooks)
		result[name] = hookCopy
	}
	return result
}

// Shutdown gracefully shuts down all plugins.
// Plugins are unloaded in reverse registration order.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	// Collect all plugin names
	names := make([]string, 0, len(m.plugins))
	for name := range m.plugins {
		names = append(names, name)
	}
	m.mu.Unlock()

	logger.Plugin("Shutting down %d plugin(s)", len(names))

	var lastErr error
	for _, name := range names {
		if err := m.Unload(ctx, name); err != nil {
			lastErr = err
			logger.Plugin("Error unloading plugin %s: %v", name, err)
		}
	}

	m.mu.Lock()
	m.enabled = false
	m.mu.Unlock()

	logger.Plugin("Plugin system shutdown complete")
	return lastErr
}

// Enabled returns whether the plugin system is enabled.
func (m *Manager) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// SetEnabled enables or disables the plugin system.
// When disabled, Trigger() calls are no-ops.
func (m *Manager) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
	logger.Plugin("Plugin system enabled: %v", enabled)
}

// Count returns the number of loaded plugins.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.plugins)
}

// HookCount returns the number of registered hooks for a specific hook name.
// If hookName is empty, returns total hook count across all hook names.
func (m *Manager) HookCount(hookName string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if hookName != "" {
		return len(m.hooks[hookName])
	}

	total := 0
	for _, hooks := range m.hooks {
		total += len(hooks)
	}
	return total
}

// isValidHook checks if a hook name is valid.
func isValidHook(name string) bool {
	for _, h := range AllHooks() {
		if h == name {
			return true
		}
	}
	return false
}

// PluginWatcher watches a plugin file for changes
type PluginWatcher struct {
	path       string
	lastMod    time.Time
	manager    *Manager
	stopChan   chan struct{}
	reloadChan chan string
}

// StartHotReload starts watching plugin directories for changes
func (m *Manager) StartHotReload(ctx context.Context, dirs []string) error {
	if !m.watchEnabled {
		return nil
	}

	m.watchMu.Lock()
	defer m.watchMu.Unlock()

	logger.Plugin("Starting hot-reload for plugin directories")

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		watcher := &PluginWatcher{
			path:       dir,
			manager:    m,
			stopChan:   make(chan struct{}),
			reloadChan: make(chan string, 10),
		}

		m.watchers[dir] = watcher
		go watcher.watch(ctx)
	}

	return nil
}

// StopHotReload stops all plugin watchers
func (m *Manager) StopHotReload() {
	m.watchMu.Lock()
	defer m.watchMu.Unlock()

	close(m.watchStop)

	for dir, watcher := range m.watchers {
		close(watcher.stopChan)
		delete(m.watchers, dir)
	}

	logger.Plugin("Hot-reload stopped")
}

// watch watches a directory for plugin file changes
func (w *PluginWatcher) watch(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	logger.Plugin("Watching directory: %s", w.path)

	for {
		select {
		case <-w.stopChan:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.checkForChanges(ctx)
		case path := <-w.reloadChan:
			w.reloadPlugin(ctx, path)
		}
	}
}

// checkForChanges scans for modified plugin files
func (w *PluginWatcher) checkForChanges(ctx context.Context) {
	// Check for plugin files
	patterns := []string{"*.py", "*.lua", "*.js", "*.so", "*.dylib", "*.dll"}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(w.path, pattern))
		if err != nil {
			continue
		}

		for _, file := range matches {
			info, err := os.Stat(file)
			if err != nil {
				continue
			}

			// Check if file was modified after last check
			if info.ModTime().After(w.lastMod) {
				if w.lastMod.IsZero() {
					// First scan, just record the time
					w.lastMod = info.ModTime()
					continue
				}

				w.lastMod = info.ModTime()
				logger.Plugin("Plugin file changed: %s", filepath.Base(file))

				// Queue for reload
				select {
				case w.reloadChan <- file:
				default:
					// Channel full, skip
				}
			}
		}
	}
}

// reloadPlugin reloads a plugin after file change
func (w *PluginWatcher) reloadPlugin(ctx context.Context, path string) {
	logger.Plugin("Reloading plugin: %s", filepath.Base(path))

	// Determine plugin name from file
	ext := filepath.Ext(path)
	name := strings.TrimSuffix(filepath.Base(path), ext)

	// Unload existing plugin if loaded
	if _, exists := w.manager.Get(name); exists {
		if err := w.manager.Unload(ctx, name); err != nil {
			logger.Plugin("Error unloading plugin %s: %v", name, err)
			return
		}
	}

	// Reload the plugin
	if err := w.manager.loadScriptPlugin(ctx, path, ext); err != nil {
		logger.Plugin("Error reloading plugin %s: %v", name, err)
		return
	}

	logger.Plugin("Plugin reloaded successfully: %s", name)
}

// ReloadPlugin manually reloads a plugin by name
func (m *Manager) ReloadPlugin(ctx context.Context, name string) error {
	m.mu.RLock()
	p, exists := m.plugins[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// Get script path if it's a script plugin
	if sp, ok := p.(*ScriptPlugin); ok {
		return m.reloadPluginFromPath(ctx, sp.scriptPath, sp.scriptType)
	}

	return fmt.Errorf("only script plugins support hot reload")
}

// reloadPluginFromPath reloads a plugin from its path
func (m *Manager) reloadPluginFromPath(ctx context.Context, path, ext string) error {
	name := strings.TrimSuffix(filepath.Base(path), ext)

	// Unload existing
	if _, exists := m.Get(name); exists {
		if err := m.Unload(ctx, name); err != nil {
			return err
		}
	}

	// Reload
	return m.loadScriptPlugin(ctx, path, ext)
}

// GetWatcherStatus returns the status of plugin watchers
func (m *Manager) GetWatcherStatus() map[string]bool {
	m.watchMu.Lock()
	defer m.watchMu.Unlock()

	status := make(map[string]bool)
	for dir := range m.watchers {
		status[dir] = true
	}
	return status
}
