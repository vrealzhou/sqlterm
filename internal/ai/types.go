package ai

import (
	"context"
	"time"
)

// Provider represents different AI providers
type Provider string

const (
	ProviderOpenRouter Provider = "openrouter"
	ProviderOllama     Provider = "ollama"
	ProviderLMStudio   Provider = "lmstudio"
)

// Usage tracks token usage and costs
type Usage struct {
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	TotalTokens     int     `json:"total_tokens"`
	Cost            float64 `json:"cost"`
	RequestCount    int     `json:"request_count"`
	LastRequestTime time.Time `json:"last_request_time"`
}

// Config holds AI configuration
type Config struct {
	Provider      Provider          `yaml:"provider"`
	Model         string            `yaml:"model"`
	APIKeys       map[string]string `yaml:"api_keys"`
	BaseURLs      map[string]string `yaml:"base_urls"`
	DefaultModels map[string]string `yaml:"default_models"`
	Usage         Usage             `yaml:"usage"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// ModelInfo represents model information
type ModelInfo struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Provider    string  `json:"provider"`
	Pricing     *Pricing `json:"pricing,omitempty"`
}

// Pricing represents model pricing information
type Pricing struct {
	InputCostPerToken  float64 `json:"input_cost_per_token"`
	OutputCostPerToken float64 `json:"output_cost_per_token"`
}

// PromptEntry represents a single prompt exchange with the AI
type PromptEntry struct {
	Timestamp     time.Time `json:"timestamp"`
	UserMessage   string    `json:"user_message"`
	SystemPrompt  string    `json:"system_prompt"`
	Provider      Provider  `json:"provider"`
	Model         string    `json:"model"`
	InputTokens   int       `json:"input_tokens"`
	OutputTokens  int       `json:"output_tokens"`
	Cost          float64   `json:"cost"`
}

// PromptHistory holds the history of AI prompts
type PromptHistory struct {
	Entries []PromptEntry `json:"entries"`
	MaxSize int           `json:"max_size"`
}

// TableInfo represents detailed table information for AI context
type TableInfo struct {
	Name         string            `json:"name"`
	Columns      []ColumnInfo      `json:"columns"`
	Relationships []Relationship   `json:"relationships"`
	RecentlyUsed bool              `json:"recently_used"`
	Relevance    float64           `json:"relevance"`
}

// ColumnInfo represents column details
type ColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Key      string `json:"key"` // PRI, UNI, MUL, etc.
}

// Relationship represents foreign key relationships
type Relationship struct {
	FromTable  string `json:"from_table"`
	FromColumn string `json:"from_column"`
	ToTable    string `json:"to_table"`
	ToColumn   string `json:"to_column"`
}

// SchemaContext holds optimized schema information for AI
type SchemaContext struct {
	RelevantTables []TableInfo `json:"relevant_tables"`
	TotalTables    int         `json:"total_tables"`
	LimitReason    string      `json:"limit_reason"`
}

// Client interface for AI providers
type Client interface {
	Chat(ctx context.Context, request ChatRequest) (*ChatResponse, error)
	ListModels(ctx context.Context) ([]ModelInfo, error)
	GetModelInfo(ctx context.Context, modelID string) (*ModelInfo, error)
	Close() error
}