package config

import "fmt"

// AIProvider represents different AI providers
type AIProvider string

const (
	ProviderOpenRouter AIProvider = "openrouter"
	ProviderOllama     AIProvider = "ollama"
	ProviderLMStudio   AIProvider = "lmstudio"
)

// AIConfig holds AI-specific configuration
type AIConfig struct {
	Provider      AIProvider        `yaml:"provider"`
	Model         string            `yaml:"model"`
	APIKeys       map[string]string `yaml:"api_keys"`
	BaseURLs      map[string]string `yaml:"base_urls"`
	DefaultModels map[string]string `yaml:"default_models"`
}

// SetProvider sets the current provider and model
func (c *AIConfig) SetProvider(provider AIProvider, model string) {
	c.Provider = provider
	c.Model = model
}

// SetAPIKey sets an API key for a provider
func (c *AIConfig) SetAPIKey(provider AIProvider, apiKey string) {
	if c.APIKeys == nil {
		c.APIKeys = make(map[string]string)
	}
	c.APIKeys[string(provider)] = apiKey
}

// GetAPIKey gets an API key for a provider
func (c *AIConfig) GetAPIKey(provider AIProvider) string {
	if c.APIKeys == nil {
		return ""
	}
	return c.APIKeys[string(provider)]
}

// SetBaseURL sets a base URL for a provider
func (c *AIConfig) SetBaseURL(provider AIProvider, baseURL string) {
	if c.BaseURLs == nil {
		c.BaseURLs = make(map[string]string)
	}
	c.BaseURLs[string(provider)] = baseURL
}

// GetBaseURL gets a base URL for a provider
func (c *AIConfig) GetBaseURL(provider AIProvider) string {
	if c.BaseURLs == nil {
		return ""
	}
	return c.BaseURLs[string(provider)]
}

// GetDefaultModel gets the default model for a provider
func (c *AIConfig) GetDefaultModel(provider AIProvider) string {
	if c.DefaultModels == nil {
		return ""
	}
	return c.DefaultModels[string(provider)]
}

// FormatProviderInfo returns formatted provider and model information
func (c *AIConfig) FormatProviderInfo() string {
	return fmt.Sprintf("%s/%s", c.Provider, c.Model)
}
