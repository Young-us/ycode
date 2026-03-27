package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func resetViper() {
	viper.Reset()
}

func TestLoadDefaultConfig(t *testing.T) {
	// Reset viper before test
	resetViper()

	// Save current directory
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Create temp directory for test
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	// Create configs directory and default.yaml
	configsDir := filepath.Join(tmpDir, "configs")
	os.MkdirAll(configsDir, 0755)

	defaultConfig := `
llm:
  provider: anthropic
  model: test-model
  max_tokens: "128k"
  base_url: https://test.api.com
`
	os.WriteFile(filepath.Join(configsDir, "default.yaml"), []byte(defaultConfig), 0644)

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify config values
	if cfg.LLM.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got '%s'", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "test-model" {
		t.Errorf("expected model 'test-model', got '%s'", cfg.LLM.Model)
	}
	if cfg.LLM.BaseURL != "https://test.api.com" {
		t.Errorf("expected base_url 'https://test.api.com', got '%s'", cfg.LLM.BaseURL)
	}

	// Test max_tokens parsing
	maxTokens := cfg.LLM.GetMaxTokens()
	if maxTokens != 128000 {
		t.Errorf("expected max_tokens 128000, got %d", maxTokens)
	}
}

func TestLoadWithProjectConfig(t *testing.T) {
	// Reset viper before test
	resetViper()

	// Save current directory
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Create temp directory for test
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	// Create configs directory and default.yaml
	configsDir := filepath.Join(tmpDir, "configs")
	os.MkdirAll(configsDir, 0755)

	defaultConfig := `
llm:
  provider: anthropic
  model: default-model
  max_tokens: "4k"
`
	os.WriteFile(filepath.Join(configsDir, "default.yaml"), []byte(defaultConfig), 0644)

	// Create project config that overrides model
	projectDir := filepath.Join(tmpDir, ".ycode")
	os.MkdirAll(projectDir, 0755)

	projectConfig := `
llm:
  model: project-model
  max_tokens: "128k"
`
	os.WriteFile(filepath.Join(projectDir, "project.yaml"), []byte(projectConfig), 0644)

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify project config overrides default
	if cfg.LLM.Model != "project-model" {
		t.Errorf("expected model 'project-model', got '%s'", cfg.LLM.Model)
	}

	// Verify max_tokens is from project config
	maxTokens := cfg.LLM.GetMaxTokens()
	if maxTokens != 128000 {
		t.Errorf("expected max_tokens 128000, got %d", maxTokens)
	}

	// Verify provider is from default config
	if cfg.LLM.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got '%s'", cfg.LLM.Provider)
	}
}

func TestLoadWithEnvOverride(t *testing.T) {
	// Reset viper before test
	resetViper()

	// Save current directory
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Create temp directory for test
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	// Create configs directory and default.yaml
	configsDir := filepath.Join(tmpDir, "configs")
	os.MkdirAll(configsDir, 0755)

	defaultConfig := `
llm:
  provider: anthropic
  model: default-model
  api_key: default-key
`
	os.WriteFile(filepath.Join(configsDir, "default.yaml"), []byte(defaultConfig), 0644)

	// Set environment variable
	os.Setenv("ANTHROPIC_API_KEY", "env-api-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify environment variable overrides config file
	if cfg.LLM.APIKey != "env-api-key" {
		t.Errorf("expected api_key 'env-api-key', got '%s'", cfg.LLM.APIKey)
	}
}
