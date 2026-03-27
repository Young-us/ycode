package plugin

import "context"

// Plugin represents a ycode plugin that can hook into various lifecycle events.
// All plugins must implement Initialize() and Shutdown() methods.
type Plugin interface {
	// Name returns the unique identifier for this plugin.
	Name() string

	// Version returns the plugin version string.
	Version() string

	// Description returns a human-readable description of the plugin.
	Description() string

	// Initialize is called when the plugin is loaded.
	// The plugin can perform setup operations here.
	// Return an error to prevent the plugin from loading.
	Initialize(ctx context.Context) error

	// Shutdown is called when the plugin is being unloaded.
	// The plugin should perform cleanup operations here.
	Shutdown(ctx context.Context) error
}

// PluginInfo contains metadata about a loaded plugin.
type PluginInfo struct {
	Name        string
	Version     string
	Description string
	Enabled     bool
}

// HookFunc is the signature for hook handler functions.
// Hooks receive a context and arbitrary arguments, and can return modified arguments or an error.
type HookFunc func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error)

// Hook represents a registered hook with its metadata.
type Hook struct {
	Name     string
	Plugin   string
	Handler  HookFunc
	Priority int // Lower values execute first
}
