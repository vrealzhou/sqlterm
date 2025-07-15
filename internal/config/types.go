package config

// Provider represents different AI providers
type Provider string

const (
	ProviderOpenRouter Provider = "openrouter"
	ProviderOllama     Provider = "ollama"
	ProviderLMStudio   Provider = "lmstudio"
)

// AIConfig holds AI-specific configuration
type AIConfig struct {
	Provider      Provider          `yaml:"provider"`
	Model         string            `yaml:"model"`
	APIKeys       map[string]string `yaml:"api_keys"`
	BaseURLs      map[string]string `yaml:"base_urls"`
	DefaultModels map[string]string `yaml:"default_models"`
}

// Config holds the main configuration with AI section
type Config struct {
	Language string   `yaml:"language"`
	AI       AIConfig `yaml:"ai"`
}
