package ai

import (
	"context"
	"sqlterm/internal/config"
	"sqlterm/internal/core"
	"time"
)

// Usage tracks token usage and costs
type Usage struct {
	InputTokens     int       `json:"input_tokens"`
	OutputTokens    int       `json:"output_tokens"`
	TotalTokens     int       `json:"total_tokens"`
	Cost            float64   `json:"cost"`
	RequestCount    int       `json:"request_count"`
	LastRequestTime time.Time `json:"last_request_time"`
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
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Provider    string   `json:"provider"`
	Pricing     *Pricing `json:"pricing,omitempty"`
}

// Pricing represents model pricing information
type Pricing struct {
	InputCostPerToken  float64 `json:"input_cost_per_token"`
	OutputCostPerToken float64 `json:"output_cost_per_token"`
}

// PromptEntry represents a single prompt exchange with the AI
type PromptEntry struct {
	Timestamp    time.Time       `json:"timestamp"`
	UserMessage  string          `json:"user_message"`
	SystemPrompt string          `json:"system_prompt"`
	AIResponse   string          `json:"ai_response"`
	Provider     config.Provider `json:"provider"`
	Model        string          `json:"model"`
	InputTokens  int             `json:"input_tokens"`
	OutputTokens int             `json:"output_tokens"`
	Cost         float64         `json:"cost"`
}

// PromptHistory holds the history of AI prompts
type PromptHistory struct {
	Entries []PromptEntry `json:"entries"`
	MaxSize int           `json:"max_size"`
}

// TableInfo represents detailed table information for AI context
type TableInfo struct {
	Name          string         `json:"name"`
	Columns       []ColumnInfo   `json:"columns"`
	Relationships []Relationship `json:"relationships"`
	RecentlyUsed  bool           `json:"recently_used"`
	Relevance     float64        `json:"relevance"`
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

// ConversationPhase represents different phases of multi-turn conversation
type ConversationPhase int

const (
	PhaseDiscovery      ConversationPhase = iota // Initial table discovery phase
	PhaseSchemaAnalysis                          // Detailed schema analysis phase
	PhaseSQLGeneration                           // Final SQL generation phase
)

// String returns the string representation of conversation phase
func (p ConversationPhase) String() string {
	switch p {
	case PhaseDiscovery:
		return "discovery"
	case PhaseSchemaAnalysis:
		return "schema_analysis"
	case PhaseSQLGeneration:
		return "sql_generation"
	default:
		return "unknown"
	}
}

// ConversationTurn represents a single turn in the conversation
type ConversationTurn struct {
	UserMessage   string            `json:"user_message"`
	SystemPrompt  string            `json:"system_prompt"`
	AIResponse    string            `json:"ai_response"`
	RequestedInfo []string          `json:"requested_info"` // Tables or info requested by AI
	Phase         ConversationPhase `json:"phase"`
	Timestamp     time.Time         `json:"timestamp"`
}

// ConversationContext maintains state across multiple conversation turns
type ConversationContext struct {
	ID                  string                     `json:"id"`
	OriginalQuery       string                     `json:"original_query"`
	CurrentPhase        ConversationPhase          `json:"current_phase"`
	DiscoveredTables    []string                   `json:"discovered_tables"` // Tables found via vector search
	LoadedTables        map[string]*core.TableInfo `json:"loaded_tables"`     // Full table schemas loaded
	RequestedTables     []string                   `json:"requested_tables"`  // Tables specifically requested by AI
	RelatedTables       []string                   `json:"related_tables"`    // Tables found via relationships
	ConversationHistory []ConversationTurn         `json:"conversation_history"`
	CreatedAt           time.Time                  `json:"created_at"`
	UpdatedAt           time.Time                  `json:"updated_at"`
	IsComplete          bool                       `json:"is_complete"`
	GeneratedSQL        string                     `json:"generated_sql"` // Final SQL if generated
}

// NewConversationContext creates a new conversation context
func NewConversationContext(userQuery string) *ConversationContext {
	now := time.Now()
	return &ConversationContext{
		ID:                  generateConversationID(),
		OriginalQuery:       userQuery,
		CurrentPhase:        PhaseDiscovery,
		DiscoveredTables:    make([]string, 0),
		LoadedTables:        make(map[string]*core.TableInfo),
		RequestedTables:     make([]string, 0),
		RelatedTables:       make([]string, 0),
		ConversationHistory: make([]ConversationTurn, 0),
		CreatedAt:           now,
		UpdatedAt:           now,
		IsComplete:          false,
	}
}

// AddTurn adds a new turn to the conversation history
func (c *ConversationContext) AddTurn(turn ConversationTurn) {
	turn.Timestamp = time.Now()
	c.ConversationHistory = append(c.ConversationHistory, turn)
	c.UpdatedAt = time.Now()
}

// AdvancePhase moves the conversation to the next phase
func (c *ConversationContext) AdvancePhase() {
	switch c.CurrentPhase {
	case PhaseDiscovery:
		c.CurrentPhase = PhaseSchemaAnalysis
	case PhaseSchemaAnalysis:
		c.CurrentPhase = PhaseSQLGeneration
	case PhaseSQLGeneration:
		c.IsComplete = true
	}
	c.UpdatedAt = time.Now()
}

// GetRequestedTablesFromLastTurn extracts table names from AI's last response
func (c *ConversationContext) GetRequestedTablesFromLastTurn() []string {
	if len(c.ConversationHistory) == 0 {
		return []string{}
	}
	return c.ConversationHistory[len(c.ConversationHistory)-1].RequestedInfo
}

// HasTableLoaded checks if a table's full schema has been loaded
func (c *ConversationContext) HasTableLoaded(tableName string) bool {
	_, exists := c.LoadedTables[tableName]
	return exists
}

// AddLoadedTable adds a table's full schema to the context
func (c *ConversationContext) AddLoadedTable(tableName string, tableInfo *core.TableInfo) {
	c.LoadedTables[tableName] = tableInfo
	c.UpdatedAt = time.Now()
}

// Client interface for AI providers
type Client interface {
	Chat(ctx context.Context, request ChatRequest) (*ChatResponse, error)
	ListModels(ctx context.Context) ([]ModelInfo, error)
	GetModelInfo(ctx context.Context, modelID string) (*ModelInfo, error)
	Close() error
}

// generateConversationID creates a unique ID for conversations
func generateConversationID() string {
	return time.Now().Format("20060102_150405_") + randomString(6)
}

// randomString generates a random string of given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
