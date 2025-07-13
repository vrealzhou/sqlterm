package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = "ai.yaml"

// DefaultConfig returns a default AI configuration
func DefaultConfig() *Config {
	return &Config{
		Provider: ProviderOpenRouter,
		Model:    "anthropic/claude-3.5-sonnet",
		APIKeys:  make(map[string]string),
		BaseURLs: map[string]string{
			string(ProviderOllama):   "http://localhost:11434",
			string(ProviderLMStudio): "http://localhost:1234",
		},
		DefaultModels: map[string]string{
			string(ProviderOpenRouter): "anthropic/claude-3.5-sonnet",
			string(ProviderOllama):     "llama3.2",
			string(ProviderLMStudio):   "lmstudio-community/Meta-Llama-3-8B-Instruct-GGUF",
		},
		Usage: Usage{
			RequestCount: 0,
			Cost:         0.0,
		},
	}
}

// LoadConfig loads AI configuration from file
func LoadConfig(configDir string) (*Config, error) {
	configPath := filepath.Join(configDir, DefaultConfigFile)
	
	// Create default config if file doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := DefaultConfig()
		if err := SaveConfig(config, configDir); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Ensure maps are initialized
	if config.APIKeys == nil {
		config.APIKeys = make(map[string]string)
	}
	if config.BaseURLs == nil {
		config.BaseURLs = make(map[string]string)
	}
	if config.DefaultModels == nil {
		config.DefaultModels = make(map[string]string)
	}

	return &config, nil
}

// SaveConfig saves AI configuration to file
func SaveConfig(config *Config, configDir string) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, DefaultConfigFile)
	
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// UpdateUsage updates usage statistics and saves config
func (c *Config) UpdateUsage(inputTokens, outputTokens int, cost float64, configDir string) error {
	c.Usage.InputTokens += inputTokens
	c.Usage.OutputTokens += outputTokens
	c.Usage.TotalTokens += inputTokens + outputTokens
	c.Usage.Cost += cost
	c.Usage.RequestCount++
	c.Usage.LastRequestTime = time.Now()

	return SaveConfig(c, configDir)
}

// SetProvider sets the current provider and model
func (c *Config) SetProvider(provider Provider, model string) {
	c.Provider = provider
	c.Model = model
}

// SetAPIKey sets an API key for a provider
func (c *Config) SetAPIKey(provider Provider, apiKey string) {
	if c.APIKeys == nil {
		c.APIKeys = make(map[string]string)
	}
	c.APIKeys[string(provider)] = apiKey
}

// GetAPIKey gets an API key for a provider
func (c *Config) GetAPIKey(provider Provider) string {
	if c.APIKeys == nil {
		return ""
	}
	return c.APIKeys[string(provider)]
}

// SetBaseURL sets a base URL for a provider
func (c *Config) SetBaseURL(provider Provider, baseURL string) {
	if c.BaseURLs == nil {
		c.BaseURLs = make(map[string]string)
	}
	c.BaseURLs[string(provider)] = baseURL
}

// GetBaseURL gets a base URL for a provider
func (c *Config) GetBaseURL(provider Provider) string {
	if c.BaseURLs == nil {
		return ""
	}
	return c.BaseURLs[string(provider)]
}

// GetDefaultModel gets the default model for a provider
func (c *Config) GetDefaultModel(provider Provider) string {
	if c.DefaultModels == nil {
		return ""
	}
	return c.DefaultModels[string(provider)]
}

// FormatUsageStats returns formatted usage statistics
func (c *Config) FormatUsageStats() string {
	if c.Usage.RequestCount == 0 {
		return "No requests made yet"
	}

	return fmt.Sprintf("Requests: %d | Tokens: %d in/%d out/%d total | Cost: $%.4f",
		c.Usage.RequestCount,
		c.Usage.InputTokens,
		c.Usage.OutputTokens,
		c.Usage.TotalTokens,
		c.Usage.Cost,
	)
}

// FormatProviderInfo returns formatted provider and model information
func (c *Config) FormatProviderInfo() string {
	return fmt.Sprintf("%s/%s", c.Provider, c.Model)
}