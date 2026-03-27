package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	LLM         LLMConfig         `mapstructure:"llm"`
	UI          UIConfig          `mapstructure:"ui"`
	Tools       ToolsConfig       `mapstructure:"tools"`
	Permissions PermissionsConfig `mapstructure:"permissions"`
	Agent       AgentConfig       `mapstructure:"agent"`
	Logging     LoggingConfig     `mapstructure:"logging"`
	MCP         MCPConfig         `mapstructure:"mcp"`
	LSP         LSPConfig         `mapstructure:"lsp"`
	Plugins     PluginsConfig     `mapstructure:"plugins"`
}

type LLMConfig struct {
	Provider  string      `mapstructure:"provider"`
	APIKey    string      `mapstructure:"api_key"`
	Model     string      `mapstructure:"model"`
	MaxTokens interface{} `mapstructure:"max_tokens"` // Supports both int and string (e.g., "128k", "4k")
	BaseURL   string      `mapstructure:"base_url"`
}

// GetMaxTokens returns the max_tokens as an integer, parsing human-readable formats
func (c *LLMConfig) GetMaxTokens() int {
	switch v := c.MaxTokens.(type) {
	case int:
		return v
	case string:
		if tokens, err := ParseTokenSize(v); err == nil {
			return tokens
		}
	}
	// Default fallback
	return 4096
}

type UIConfig struct {
	Theme           string `mapstructure:"theme"`
	Streaming       bool   `mapstructure:"streaming"`
	ShowLineNumbers bool   `mapstructure:"show_line_numbers"`
}

type ToolsConfig struct {
	Bash  BashConfig  `mapstructure:"bash"`
	Files FilesConfig `mapstructure:"files"`
}

type BashConfig struct {
	Timeout        string   `mapstructure:"timeout"`
	DeniedCommands []string `mapstructure:"denied_commands"`
}

type FilesConfig struct {
	MaxSize  string `mapstructure:"max_size"`
	MaxLines int    `mapstructure:"max_lines"`
	Encoding string `mapstructure:"encoding"`
}

type PermissionsConfig struct {
	Mode        string   `mapstructure:"mode"`
	AlwaysAllow []string `mapstructure:"always_allow"`
	AlwaysDeny  []string `mapstructure:"always_deny"`
}

type AgentConfig struct {
	MaxSteps         int              `mapstructure:"max_steps"`
	AutoCompact      bool             `mapstructure:"auto_compact"`
	CompactThreshold float64          `mapstructure:"compact_threshold"`
	MultiAgent       MultiAgentConfig `mapstructure:"multi_agent"`
}

type MultiAgentConfig struct {
	Enabled   bool `mapstructure:"enabled"`
	MaxAgents int  `mapstructure:"max_agents"`
}

type LoggingConfig struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

// PluginsConfig represents plugin configuration
type PluginsConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Directory string `mapstructure:"directory"`
	HotReload bool   `mapstructure:"hot_reload"` // Enable hot-reload for plugins
}

// MCPConfig represents MCP server configuration
type MCPConfig struct {
	Servers []MCPServer `mapstructure:"servers"`
}

// MCPServer represents a single MCP server
type MCPServer struct {
	Name    string   `mapstructure:"name"`
	URL     string   `mapstructure:"url"`     // For HTTP-based MCP servers (legacy)
	Command string   `mapstructure:"command"` // For stdio-based MCP servers (standard)
	Args    []string `mapstructure:"args"`    // Arguments for stdio-based servers
	Enabled bool     `mapstructure:"enabled"`
}

// LSPConfig represents LSP server configuration
type LSPConfig struct {
	Servers []LSPServer `mapstructure:"servers"`
}

// LSPServer represents a single LSP server
type LSPServer struct {
	Name    string   `mapstructure:"name"`
	Command string   `mapstructure:"command"`
	Args    []string `mapstructure:"args"`
	Enabled bool     `mapstructure:"enabled"`
}

// Load loads configuration from files and environment
func Load() (*Config, error) {
	// Set defaults
	setDefaults()

	// Load default config from embedded configs directory
	// This is the fallback configuration
	viper.SetConfigType("yaml")

	// Try to load from configs/default.yaml (development mode)
	// if _, err := os.Stat("configs/default.yaml"); err == nil {
	// 	viper.SetConfigFile("configs/default.yaml")
	// 	if err := viper.ReadInConfig(); err == nil {
	// 		// Successfully loaded default config
	// 	}
	// }

	// Load global config from user home directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalConfigDir := filepath.Join(homeDir, ".ycode")
		configFile := filepath.Join(globalConfigDir, "config.yaml")

		// Check if global config exists
		if _, err := os.Stat(configFile); err == nil {
			viper.SetConfigFile(configFile)
			if err := viper.MergeInConfig(); err == nil {
				// Successfully merged global config
			}
		}
	}

	// Load project config from current directory
	projectConfigFile := ".ycode/project.yaml"
	if _, err := os.Stat(projectConfigFile); err == nil {
		viper.SetConfigFile(projectConfigFile)
		if err := viper.MergeInConfig(); err == nil {
			// Successfully merged project config
		}
	}

	// Environment variables
	viper.SetEnvPrefix("YCODE")
	viper.AutomaticEnv()

	// Unmarshal
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	// Apply environment variable overrides
	applyEnvOverrides(&config)

	return &config, nil
}

func setDefaults() {
	// LLM defaults
	viper.SetDefault("llm.provider", "anthropic")
	viper.SetDefault("llm.max_tokens", 4096)

	// UI defaults
	viper.SetDefault("ui.theme", "auto")
	viper.SetDefault("ui.streaming", true)
	viper.SetDefault("ui.show_line_numbers", true)

	// Tools defaults
	viper.SetDefault("tools.bash.timeout", "30s")
	viper.SetDefault("tools.files.max_size", "10MB")
	viper.SetDefault("tools.files.max_lines", 1000)
	viper.SetDefault("tools.files.encoding", "utf-8")

	// Permissions defaults
	viper.SetDefault("permissions.mode", "confirm")

	// Agent defaults
	viper.SetDefault("agent.max_steps", 10)
	viper.SetDefault("agent.auto_compact", true)
	viper.SetDefault("agent.compact_threshold", 0.8)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
}

func applyEnvOverrides(config *Config) {
	// API key from environment
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		config.LLM.APIKey = apiKey
	}

	// Model from environment
	if model := os.Getenv("YCODE_MODEL"); model != "" {
		config.LLM.Model = model
	}
}
