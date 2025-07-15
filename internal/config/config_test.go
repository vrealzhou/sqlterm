package config

import (
	"os"
	"path/filepath"
	"testing"

	"sqlterm/internal/i18n"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if config == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	
	if config.Language != "en_au" {
		t.Errorf("Expected default language 'en_au', got '%s'", config.Language)
	}
	
	if config.AI.Provider != ProviderOpenRouter {
		t.Errorf("Expected default provider OpenRouter, got %v", config.AI.Provider)
	}
	
	if config.AI.Model != "anthropic/claude-3.5-sonnet" {
		t.Errorf("Expected default model 'anthropic/claude-3.5-sonnet', got '%s'", config.AI.Model)
	}
	
	if config.AI.APIKeys == nil {
		t.Error("Expected APIKeys to be initialized")
	}
	
	if config.AI.BaseURLs == nil {
		t.Error("Expected BaseURLs to be initialized")
	}
}

func TestConfig_SetProvider(t *testing.T) {
	config := DefaultConfig()
	
	testCases := []struct {
		name     string
		provider Provider
		model    string
		expected string
	}{
		{
			name:     "OpenRouter provider",
			provider: ProviderOpenRouter,
			model:    "anthropic/claude-3-haiku",
			expected: "anthropic/claude-3-haiku",
		},
		{
			name:     "Ollama provider",
			provider: ProviderOllama,
			model:    "llama2:7b",
			expected: "llama2:7b",
		},
		{
			name:     "LMStudio provider",
			provider: ProviderLMStudio,
			model:    "local-model",
			expected: "local-model",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config.SetProvider(tc.provider, tc.model)
			
			if config.AI.Provider != tc.provider {
				t.Errorf("Expected provider %v, got %v", tc.provider, config.AI.Provider)
			}
			
			if config.AI.Model != tc.expected {
				t.Errorf("Expected model '%s', got '%s'", tc.expected, config.AI.Model)
			}
		})
	}
}

func TestConfig_APIKeys(t *testing.T) {
	config := DefaultConfig()
	
	testCases := []struct {
		name     string
		provider Provider
		apiKey   string
	}{
		{
			name:     "OpenRouter API key",
			provider: ProviderOpenRouter,
			apiKey:   "sk-or-v1-test-key",
		},
		{
			name:     "Empty API key",
			provider: ProviderOllama,
			apiKey:   "",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config.SetAPIKey(tc.provider, tc.apiKey)
			
			retrievedKey := config.GetAPIKey(tc.provider)
			if retrievedKey != tc.apiKey {
				t.Errorf("Expected API key '%s', got '%s'", tc.apiKey, retrievedKey)
			}
		})
	}
}

func TestConfig_BaseURLs(t *testing.T) {
	config := DefaultConfig()
	
	testCases := []struct {
		name     string
		provider Provider
		baseURL  string
	}{
		{
			name:     "Ollama base URL",
			provider: ProviderOllama,
			baseURL:  "http://localhost:11434",
		},
		{
			name:     "LMStudio base URL",
			provider: ProviderLMStudio,
			baseURL:  "http://localhost:1234",
		},
		{
			name:     "Empty base URL",
			provider: ProviderOpenRouter,
			baseURL:  "",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config.SetBaseURL(tc.provider, tc.baseURL)
			
			retrievedURL := config.GetBaseURL(tc.provider)
			if retrievedURL != tc.baseURL {
				t.Errorf("Expected base URL '%s', got '%s'", tc.baseURL, retrievedURL)
			}
		})
	}
}

func TestConfig_GetDefaultModel(t *testing.T) {
	config := DefaultConfig()
	
	testCases := []struct {
		name     string
		provider Provider
		expected string
	}{
		{
			name:     "OpenRouter default model",
			provider: ProviderOpenRouter,
			expected: "anthropic/claude-3.5-sonnet",
		},
		{
			name:     "Ollama default model",
			provider: ProviderOllama,
			expected: "llama3.2",
		},
		{
			name:     "LMStudio default model",
			provider: ProviderLMStudio,
			expected: "lmstudio-community/Meta-Llama-3-8B-Instruct-GGUF",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defaultModel := config.GetDefaultModel(tc.provider)
			if defaultModel != tc.expected {
				t.Errorf("Expected default model '%s', got '%s'", tc.expected, defaultModel)
			}
		})
	}
}

func TestConfig_FormatProviderInfo(t *testing.T) {
	config := DefaultConfig()
	config.SetProvider(ProviderOpenRouter, "anthropic/claude-3.5-sonnet")
	config.SetAPIKey(ProviderOpenRouter, "sk-or-v1-test-key-1234")
	
	info := config.FormatProviderInfo()
	
	if info == "" {
		t.Error("FormatProviderInfo() returned empty string")
	}
	
	// Check that provider and model are included
	if !contains(info, "openrouter") {
		t.Error("Provider info should contain provider name")
	}
	
	if !contains(info, "anthropic/claude-3.5-sonnet") {
		t.Error("Provider info should contain model name")
	}
	
	// FormatProviderInfo should only return provider/model format, not API key
	expected := "openrouter/anthropic/claude-3.5-sonnet"
	if info != expected {
		t.Errorf("Expected FormatProviderInfo() to return '%s', got '%s'", expected, info)
	}
	
	// API key should not be included in the output
	if contains(info, "sk-or-v1-test-key-1234") {
		t.Error("API key should not be included in provider info")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	
	// Create i18n manager for testing
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create i18n manager: %v", err)
	}
	
	// Create test config
	originalConfig := DefaultConfig()
	originalConfig.SetProvider(ProviderOllama, "llama2:13b")
	originalConfig.SetAPIKey(ProviderOpenRouter, "test-api-key")
	originalConfig.SetBaseURL(ProviderOllama, "http://localhost:11434")
	originalConfig.Language = "zh_cn"
	
	// Save config
	err = SaveConfig(originalConfig, tmpDir, i18nMgr)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}
	
	// Check that config file was created
	configPath := filepath.Join(tmpDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}
	
	// Load config
	loadedI18nMgr, loadedConfig, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	if loadedConfig == nil {
		t.Fatal("Loaded config is nil")
	}
	
	if loadedI18nMgr == nil {
		t.Fatal("Loaded i18n manager is nil")
	}
	
	// Verify loaded config matches original
	if loadedConfig.Language != originalConfig.Language {
		t.Errorf("Expected language '%s', got '%s'", originalConfig.Language, loadedConfig.Language)
	}
	
	if loadedConfig.AI.Provider != originalConfig.AI.Provider {
		t.Errorf("Expected provider %v, got %v", originalConfig.AI.Provider, loadedConfig.AI.Provider)
	}
	
	if loadedConfig.AI.Model != originalConfig.AI.Model {
		t.Errorf("Expected model '%s', got '%s'", originalConfig.AI.Model, loadedConfig.AI.Model)
	}
	
	if loadedConfig.GetAPIKey(ProviderOpenRouter) != originalConfig.GetAPIKey(ProviderOpenRouter) {
		t.Error("API keys don't match after save/load")
	}
	
	if loadedConfig.GetBaseURL(ProviderOllama) != originalConfig.GetBaseURL(ProviderOllama) {
		t.Error("Base URLs don't match after save/load")
	}
}

func TestLoadConfig_Migration(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	
	// Create legacy ai.yaml file
	legacyConfigPath := filepath.Join(tmpDir, "ai.yaml")
	legacyContent := `language: zh_cn
ai:
  provider: ollama
  model: llama2:7b
  api_keys:
    openrouter: old-api-key
  base_urls:
    ollama: http://localhost:11434
`
	
	err := os.WriteFile(legacyConfigPath, []byte(legacyContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create legacy config: %v", err)
	}
	
	// Load config (should trigger migration)
	i18nMgr, config, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config with migration: %v", err)
	}
	
	if config == nil {
		t.Fatal("Migrated config is nil")
	}
	
	if i18nMgr == nil {
		t.Fatal("i18n manager is nil after migration")
	}
	
	// Verify migration worked
	if config.Language != "zh_cn" {
		t.Errorf("Expected language 'zh_cn', got '%s'", config.Language)
	}
	
	if config.AI.Provider != ProviderOllama {
		t.Errorf("Expected provider Ollama, got %v", config.AI.Provider)
	}
	
	if config.AI.Model != "llama2:7b" {
		t.Errorf("Expected model 'llama2:7b', got '%s'", config.AI.Model)
	}
	
	// Check that new config.yaml exists
	newConfigPath := filepath.Join(tmpDir, "config.yaml")
	if _, err := os.Stat(newConfigPath); os.IsNotExist(err) {
		t.Error("New config.yaml file was not created during migration")
	}
	
	// Check that old ai.yaml was removed
	if _, err := os.Stat(legacyConfigPath); !os.IsNotExist(err) {
		t.Error("Legacy ai.yaml file was not removed after migration")
	}
}

func TestNewManager(t *testing.T) {
	manager := NewManager()
	
	if manager == nil {
		t.Fatal("NewManager() returned nil")
	}
	
	if manager.configDir == "" {
		t.Error("Manager configDir should not be empty")
	}
	
	// Test that the config directory path is reasonable
	if !filepath.IsAbs(manager.configDir) {
		t.Error("Config directory should be absolute path")
	}
}

func TestManager_GetConfigDir(t *testing.T) {
	manager := NewManager()
	
	configDir := manager.GetConfigDir()
	
	if configDir == "" {
		t.Error("GetConfigDir() returned empty string")
	}
	
	if !filepath.IsAbs(configDir) {
		t.Error("Config directory should be absolute path")
	}
	
	// Should contain "sqlterm" in the path
	if !contains(configDir, "sqlterm") {
		t.Error("Config directory should contain 'sqlterm'")
	}
}

// Helper function for string containment check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    s[:len(substr)] == substr || 
		    s[len(s)-len(substr):] == substr ||
		    containsAtPosition(s, substr))
}

func containsAtPosition(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}