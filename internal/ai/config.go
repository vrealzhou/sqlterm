package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = "config.yaml"

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Language: "en_au",
		AI: AIConfig{
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
		},
	}
}

// LoadConfig loads AI configuration from file
func LoadConfig(configDir string) (*Config, error) {
	configPath := filepath.Join(configDir, DefaultConfigFile)
	legacyConfigPath := filepath.Join(configDir, "ai.yaml")
	
	// Handle migration from ai.yaml to config.yaml
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Check if legacy ai.yaml exists
		if _, err := os.Stat(legacyConfigPath); err == nil {
			// Migrate from ai.yaml to config.yaml
			if err := os.Rename(legacyConfigPath, configPath); err != nil {
				return nil, fmt.Errorf(i18nMgr.Get("failed_to_migrate_config"), err)
			}
			fmt.Printf("📦 Migrated configuration from ai.yaml to config.yaml\n")
		} else {
			// Create default config if neither file exists
			config := DefaultConfig()
			if err := SaveConfig(config, configDir, i18nMgr); err != nil {
				return nil, fmt.Errorf(i18nMgr.Get("failed_to_create_default_config"), err)
			}
			return config, nil
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf(i18nMgr.Get("failed_to_read_config_file"), err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf(i18nMgr.Get("failed_to_parse_config_file"), err)
	}

	// Ensure maps are initialized
	if config.AI.APIKeys == nil {
		config.AI.APIKeys = make(map[string]string)
	}
	if config.AI.BaseURLs == nil {
		config.AI.BaseURLs = make(map[string]string)
	}
	if config.AI.DefaultModels == nil {
		config.AI.DefaultModels = make(map[string]string)
	}
	
	// Set default language if not specified
	if config.Language == "" {
		config.Language = "en_au"
	}

	return &config, nil
}

// SaveConfig saves AI configuration to file
func SaveConfig(config *Config, configDir string, i18nMgr *i18n.Manager) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf(i18nMgr.Get("failed_to_create_config_dir"), err)
	}

	configPath := filepath.Join(configDir, DefaultConfigFile)
	
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf(i18nMgr.Get("failed_to_marshal_config"), err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf(i18nMgr.Get("failed_to_write_config_file"), err)
	}

	return nil
}

// UpdateUsage updates usage statistics and saves config
func (c *Config) UpdateUsage(inputTokens, outputTokens int, cost float64, configDir string) error {
	c.AI.Usage.InputTokens += inputTokens
	c.AI.Usage.OutputTokens += outputTokens
	c.AI.Usage.TotalTokens += inputTokens + outputTokens
	c.AI.Usage.Cost += cost
	c.AI.Usage.RequestCount++
	c.AI.Usage.LastRequestTime = time.Now()

	return SaveConfig(c, configDir)
}

// SetProvider sets the current provider and model
func (c *Config) SetProvider(provider Provider, model string) {
	c.AI.Provider = provider
	c.AI.Model = model
}

// SetAPIKey sets an API key for a provider
func (c *Config) SetAPIKey(provider Provider, apiKey string) {
	if c.AI.APIKeys == nil {
		c.AI.APIKeys = make(map[string]string)
	}
	c.AI.APIKeys[string(provider)] = apiKey
}

// GetAPIKey gets an API key for a provider
func (c *Config) GetAPIKey(provider Provider) string {
	if c.AI.APIKeys == nil {
		return ""
	}
	return c.AI.APIKeys[string(provider)]
}

// SetBaseURL sets a base URL for a provider
func (c *Config) SetBaseURL(provider Provider, baseURL string) {
	if c.AI.BaseURLs == nil {
		c.AI.BaseURLs = make(map[string]string)
	}
	c.AI.BaseURLs[string(provider)] = baseURL
}

// GetBaseURL gets a base URL for a provider
func (c *Config) GetBaseURL(provider Provider) string {
	if c.AI.BaseURLs == nil {
		return ""
	}
	return c.AI.BaseURLs[string(provider)]
}

// GetDefaultModel gets the default model for a provider
func (c *Config) GetDefaultModel(provider Provider) string {
	if c.AI.DefaultModels == nil {
		return ""
	}
	return c.AI.DefaultModels[string(provider)]
}

// SetLanguage sets the language for the configuration
func (c *Config) SetLanguage(language string) {
	c.Language = language
}

// FormatUsageStats returns formatted usage statistics
func (c *Config) FormatUsageStats() string {
	if c.AI.Usage.RequestCount == 0 {
		return "No requests made yet"
	}

	return fmt.Sprintf("Requests: %d | Tokens: %d in/%d out/%d total | Cost: $%.4f",
		c.AI.Usage.RequestCount,
		c.AI.Usage.InputTokens,
		c.AI.Usage.OutputTokens,
		c.AI.Usage.TotalTokens,
		c.AI.Usage.Cost,
	)
}

// FormatProviderInfo returns formatted provider and model information
func (c *Config) FormatProviderInfo() string {
	return fmt.Sprintf("%s/%s", c.AI.Provider, c.AI.Model)
}