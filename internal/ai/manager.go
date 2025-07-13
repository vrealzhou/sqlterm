package ai

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Manager manages AI clients and configuration
type Manager struct {
	config        *Config
	configDir     string
	client        Client
	promptHistory *PromptHistory
}

// NewManager creates a new AI manager
func NewManager(configDir string) (*Manager, error) {
	config, err := LoadConfig(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load AI config: %w", err)
	}

	manager := &Manager{
		config:    config,
		configDir: configDir,
		promptHistory: &PromptHistory{
			Entries: make([]PromptEntry, 0),
			MaxSize: 100, // Keep last 100 prompts
		},
	}

	// Try to initialize client, but don't fail if it's not possible
	// This allows the manager to be created even if API keys aren't configured yet
	_ = manager.initializeClient()

	return manager, nil
}

// NewManagerWithValidation creates a new AI manager and requires valid client initialization
func NewManagerWithValidation(configDir string) (*Manager, error) {
	config, err := LoadConfig(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load AI config: %w", err)
	}

	manager := &Manager{
		config:    config,
		configDir: configDir,
		promptHistory: &PromptHistory{
			Entries: make([]PromptEntry, 0),
			MaxSize: 100, // Keep last 100 prompts
		},
	}

	// Initialize client - this will fail if configuration is invalid
	if err := manager.initializeClient(); err != nil {
		return nil, fmt.Errorf("failed to initialize AI client: %w", err)
	}

	return manager, nil
}

// initializeClient initializes the appropriate client based on current provider
func (m *Manager) initializeClient() error {
	switch m.config.Provider {
	case ProviderOpenRouter:
		apiKey := m.config.GetAPIKey(ProviderOpenRouter)
		if apiKey == "" {
			return fmt.Errorf("OpenRouter API key not configured")
		}
		m.client = NewOpenRouterClient(apiKey)
	case ProviderOllama:
		baseURL := m.config.GetBaseURL(ProviderOllama)
		m.client = NewOllamaClient(baseURL)
	case ProviderLMStudio:
		baseURL := m.config.GetBaseURL(ProviderLMStudio)
		m.client = NewLMStudioClient(baseURL)
	default:
		return fmt.Errorf("unsupported provider: %s", m.config.Provider)
	}

	return nil
}

// IsConfigured checks if the AI manager is properly configured and ready to use
func (m *Manager) IsConfigured() bool {
	return m.client != nil
}

// EnsureConfigured reinitializes the client if needed
func (m *Manager) EnsureConfigured() error {
	if m.client == nil {
		return m.initializeClient()
	}
	return nil
}

// Chat sends a chat message and returns the response
func (m *Manager) Chat(ctx context.Context, message string, systemPrompt string) (string, error) {
	if !m.IsConfigured() {
		return "", fmt.Errorf("AI client not configured")
	}

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: message},
	}

	request := ChatRequest{
		Model:       m.config.Model,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   4000,
	}

	response, err := m.client.Chat(ctx, request)
	if err != nil {
		return "", fmt.Errorf("chat request failed: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	// Calculate cost and update usage
	cost := m.calculateCost(response.Usage.PromptTokens, response.Usage.CompletionTokens)
	if err := m.config.UpdateUsage(
		response.Usage.PromptTokens,
		response.Usage.CompletionTokens,
		cost,
		m.configDir,
	); err != nil {
		// Don't fail the request if usage update fails
		fmt.Printf("Warning: failed to update usage stats: %v\n", err)
	}

	// Add to prompt history
	m.addToPromptHistory(message, systemPrompt, response.Usage.PromptTokens, response.Usage.CompletionTokens, cost)

	return response.Choices[0].Message.Content, nil
}

// calculateCost calculates the cost based on token usage and current model
func (m *Manager) calculateCost(inputTokens, outputTokens int) float64 {
	// Only calculate cost for OpenRouter (others are free/local)
	if m.config.Provider != ProviderOpenRouter {
		return 0.0
	}

	// Default pricing for popular models (per 1M tokens)
	pricing := map[string]*Pricing{
		"anthropic/claude-3.5-sonnet": {
			InputCostPerToken:  3.0 / 1000000,   // $3 per 1M input tokens
			OutputCostPerToken: 15.0 / 1000000,  // $15 per 1M output tokens
		},
		"anthropic/claude-3-haiku": {
			InputCostPerToken:  0.25 / 1000000,  // $0.25 per 1M input tokens
			OutputCostPerToken: 1.25 / 1000000,  // $1.25 per 1M output tokens
		},
		"openai/gpt-4o": {
			InputCostPerToken:  5.0 / 1000000,   // $5 per 1M input tokens
			OutputCostPerToken: 15.0 / 1000000,  // $15 per 1M output tokens
		},
		"openai/gpt-4o-mini": {
			InputCostPerToken:  0.15 / 1000000,  // $0.15 per 1M input tokens
			OutputCostPerToken: 0.6 / 1000000,   // $0.6 per 1M output tokens
		},
	}

	modelPricing, exists := pricing[m.config.Model]
	if !exists {
		// Default pricing if model not found
		return float64(inputTokens)*0.001/1000 + float64(outputTokens)*0.003/1000
	}

	return float64(inputTokens)*modelPricing.InputCostPerToken + float64(outputTokens)*modelPricing.OutputCostPerToken
}

// ListModels returns available models for the current provider
func (m *Manager) ListModels(ctx context.Context) ([]ModelInfo, error) {
	if !m.IsConfigured() {
		return nil, fmt.Errorf("AI client not configured")
	}
	return m.client.ListModels(ctx)
}

// SetProvider changes the current provider and model
func (m *Manager) SetProvider(provider Provider, model string) error {
	m.config.SetProvider(provider, model)
	
	// Try to initialize client, but don't fail if credentials aren't ready yet
	_ = m.initializeClient()

	return SaveConfig(m.config, m.configDir)
}

// SetAPIKey sets an API key for a provider
func (m *Manager) SetAPIKey(provider Provider, apiKey string) error {
	m.config.SetAPIKey(provider, apiKey)
	
	// Re-initialize client if this is the current provider
	if provider == m.config.Provider {
		_ = m.initializeClient()
	}

	return SaveConfig(m.config, m.configDir)
}

// SetBaseURL sets a base URL for a provider
func (m *Manager) SetBaseURL(provider Provider, baseURL string) error {
	m.config.SetBaseURL(provider, baseURL)
	
	// Re-initialize client if this is the current provider
	if provider == m.config.Provider {
		_ = m.initializeClient()
	}

	return SaveConfig(m.config, m.configDir)
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *Config {
	return m.config
}

// GenerateSystemPrompt creates a system prompt with database context
func (m *Manager) GenerateSystemPrompt(tables []string, currentTable string) string {
	var prompt strings.Builder
	
	prompt.WriteString("You are an AI assistant helping with SQL queries and database operations. ")
	prompt.WriteString("You have access to a database with the following context:\n\n")
	
	if len(tables) > 0 {
		prompt.WriteString("Available tables:\n")
		for _, table := range tables {
			if table == currentTable {
				prompt.WriteString(fmt.Sprintf("- %s (currently described)\n", table))
			} else {
				prompt.WriteString(fmt.Sprintf("- %s\n", table))
			}
		}
		prompt.WriteString("\n")
	}
	
	prompt.WriteString("Guidelines:\n")
	prompt.WriteString("- Generate accurate SQL queries based on user requests\n")
	prompt.WriteString("- Explain your reasoning when helpful\n")
	prompt.WriteString("- Suggest optimizations when appropriate\n")
	prompt.WriteString("- Use proper SQL syntax and best practices\n")
	prompt.WriteString("- Ask for clarification if the request is ambiguous\n")
	prompt.WriteString("- Consider data types and constraints when generating queries\n\n")
	
	prompt.WriteString("When generating SQL:\n")
	prompt.WriteString("- Use ```sql code blocks for SQL queries\n")
	prompt.WriteString("- Include comments for complex queries\n")
	prompt.WriteString("- Consider performance implications\n")
	prompt.WriteString("- Validate against available tables and expected schema\n")
	
	return prompt.String()
}

// ParseModelString parses a model string in format "provider/model"
func ParseModelString(modelStr string) (Provider, string, error) {
	parts := strings.SplitN(modelStr, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid model format, expected 'provider/model'")
	}

	provider := Provider(parts[0])
	model := parts[1]

	switch provider {
	case ProviderOpenRouter, ProviderOllama, ProviderLMStudio:
		return provider, model, nil
	default:
		return "", "", fmt.Errorf("unsupported provider: %s", provider)
	}
}

// FormatPrice formats a price for display
func FormatPrice(price float64) string {
	if price == 0 {
		return "Free"
	}
	if price < 0.001 {
		return fmt.Sprintf("$%.6f", price)
	}
	return fmt.Sprintf("$%.4f", price)
}

// ParseFloat safely parses a float from string
func ParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// addToPromptHistory adds a prompt entry to the history
func (m *Manager) addToPromptHistory(userMessage, systemPrompt string, inputTokens, outputTokens int, cost float64) {
	entry := PromptEntry{
		Timestamp:    time.Now(),
		UserMessage:  userMessage,
		SystemPrompt: systemPrompt,
		Provider:     m.config.Provider,
		Model:        m.config.Model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         cost,
	}

	m.promptHistory.Entries = append(m.promptHistory.Entries, entry)

	// Keep only the last MaxSize entries
	if len(m.promptHistory.Entries) > m.promptHistory.MaxSize {
		m.promptHistory.Entries = m.promptHistory.Entries[len(m.promptHistory.Entries)-m.promptHistory.MaxSize:]
	}
}

// GetPromptHistory returns the prompt history
func (m *Manager) GetPromptHistory() []PromptEntry {
	if m.promptHistory == nil {
		return []PromptEntry{}
	}
	return m.promptHistory.Entries
}