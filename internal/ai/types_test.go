package ai

import (
	"testing"
	"time"

	"sqlterm/internal/config"
)

func TestParseModelString(t *testing.T) {
	testCases := []struct {
		name             string
		modelStr         string
		expectedProvider config.Provider
		expectedModel    string
		hasError         bool
	}{
		{
			name:             "OpenRouter model",
			modelStr:         "anthropic/claude-3.5-sonnet",
			expectedProvider: config.ProviderOpenRouter,
			expectedModel:    "anthropic/claude-3.5-sonnet",
			hasError:         false,
		},
		{
			name:             "OpenRouter model with slash",
			modelStr:         "openai/gpt-4",
			expectedProvider: config.ProviderOpenRouter,
			expectedModel:    "openai/gpt-4",
			hasError:         false,
		},
		{
			name:             "Ollama model",
			modelStr:         "llama2:7b",
			expectedProvider: config.ProviderOllama,
			expectedModel:    "llama2:7b",
			hasError:         false,
		},
		{
			name:             "LM Studio model",
			modelStr:         "local-model",
			expectedProvider: config.ProviderLMStudio,
			expectedModel:    "local-model",
			hasError:         false,
		},
		{
			name:             "Empty string",
			modelStr:         "",
			expectedProvider: config.Provider(""),
			expectedModel:    "",
			hasError:         true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider, model, err := ParseModelString(tc.modelStr)

			if tc.hasError {
				if err == nil {
					t.Errorf("Expected error for model string '%s', but got none", tc.modelStr)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for model string '%s': %v", tc.modelStr, err)
				}

				if provider != tc.expectedProvider {
					t.Errorf("Expected provider %v, got %v for model '%s'", tc.expectedProvider, provider, tc.modelStr)
				}

				if model != tc.expectedModel {
					t.Errorf("Expected model '%s', got '%s' for input '%s'", tc.expectedModel, model, tc.modelStr)
				}
			}
		})
	}
}

func TestFormatPrice(t *testing.T) {
	testCases := []struct {
		name     string
		price    float64
		expected string
	}{
		{
			name:     "Zero price",
			price:    0.0,
			expected: "Free",
		},
		{
			name:     "Small price",
			price:    0.001234,
			expected: "$0.001234",
		},
		{
			name:     "Regular price",
			price:    1.50,
			expected: "$1.50",
		},
		{
			name:     "Large price",
			price:    100.0,
			expected: "$100.00",
		},
		{
			name:     "Negative price",
			price:    -1.0,
			expected: "Free",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := FormatPrice(tc.price)
			if result != tc.expected {
				t.Errorf("Expected FormatPrice(%.6f) to return '%s', got '%s'", tc.price, tc.expected, result)
			}
		})
	}
}

func TestParseFloat(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected float64
		hasError bool
	}{
		{
			name:     "Valid integer",
			input:    "42",
			expected: 42.0,
			hasError: false,
		},
		{
			name:     "Valid float",
			input:    "3.14159",
			expected: 3.14159,
			hasError: false,
		},
		{
			name:     "Zero",
			input:    "0",
			expected: 0.0,
			hasError: false,
		},
		{
			name:     "Negative number",
			input:    "-1.5",
			expected: -1.5,
			hasError: false,
		},
		{
			name:     "Invalid string",
			input:    "not-a-number",
			expected: 0.0,
			hasError: true,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: 0.0,
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseFloat(tc.input)

			if tc.hasError {
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input '%s': %v", tc.input, err)
				}

				if result != tc.expected {
					t.Errorf("Expected ParseFloat('%s') to return %.6f, got %.6f", tc.input, tc.expected, result)
				}
			}
		})
	}
}

func TestNewConversationContext(t *testing.T) {
	userQuery := "Show me all users"
	ctx := NewConversationContext(userQuery)

	if ctx == nil {
		t.Fatal("NewConversationContext returned nil")
	}

	if ctx.OriginalQuery != userQuery {
		t.Errorf("Expected OriginalQuery to be '%s', got '%s'", userQuery, ctx.OriginalQuery)
	}

	if ctx.CurrentPhase != PhaseDiscovery {
		t.Errorf("Expected CurrentPhase to be PhaseDiscovery, got %v", ctx.CurrentPhase)
	}

	if ctx.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	if time.Since(ctx.CreatedAt) > time.Minute {
		t.Error("CreatedAt should be recent")
	}

	if ctx.ConversationHistory == nil {
		t.Error("ConversationHistory should be initialized")
	}

	if len(ctx.ConversationHistory) != 0 {
		t.Error("ConversationHistory should be empty initially")
	}

	if ctx.DiscoveredTables == nil {
		t.Error("DiscoveredTables should be initialized")
	}

	if ctx.LoadedTables == nil {
		t.Error("LoadedTables should be initialized")
	}
}

func TestConversationContext_AddTurn(t *testing.T) {
	ctx := NewConversationContext("test query")

	turn := ConversationTurn{
		UserMessage: "test input",
		AIResponse:  "test response",
		Timestamp:   time.Now(),
		Phase:       PhaseDiscovery,
	}

	ctx.AddTurn(turn)

	if len(ctx.ConversationHistory) != 1 {
		t.Errorf("Expected 1 turn, got %d", len(ctx.ConversationHistory))
	}

	if ctx.ConversationHistory[0].UserMessage != turn.UserMessage {
		t.Error("Turn was not added correctly")
	}

	// Add another turn
	turn2 := ConversationTurn{
		UserMessage: "second input",
		AIResponse:  "second response",
		Timestamp:   time.Now(),
		Phase:       PhaseSchemaAnalysis,
	}

	ctx.AddTurn(turn2)

	if len(ctx.ConversationHistory) != 2 {
		t.Errorf("Expected 2 turns, got %d", len(ctx.ConversationHistory))
	}

	if ctx.ConversationHistory[1].UserMessage != turn2.UserMessage {
		t.Error("Second turn was not added correctly")
	}
}

func TestConversationContext_AdvancePhase(t *testing.T) {
	ctx := NewConversationContext("test query")

	// Should start at PhaseDiscovery
	if ctx.CurrentPhase != PhaseDiscovery {
		t.Errorf("Expected initial phase to be PhaseDiscovery, got %v", ctx.CurrentPhase)
	}

	// Advance to next phase
	ctx.AdvancePhase()
	if ctx.CurrentPhase != PhaseSchemaAnalysis {
		t.Errorf("Expected phase to advance to PhaseSchemaAnalysis, got %v", ctx.CurrentPhase)
	}

	// Advance to next phase
	ctx.AdvancePhase()
	if ctx.CurrentPhase != PhaseSQLGeneration {
		t.Errorf("Expected phase to advance to PhaseSQLGeneration, got %v", ctx.CurrentPhase)
	}

	// Advancing beyond final phase should mark as complete
	ctx.AdvancePhase()
	if !ctx.IsComplete {
		t.Error("Expected conversation to be marked as complete")
	}
}

func TestConversationPhase_String(t *testing.T) {
	testCases := []struct {
		name     string
		phase    ConversationPhase
		expected string
	}{
		{
			name:     "Discovery phase",
			phase:    PhaseDiscovery,
			expected: "Discovery",
		},
		{
			name:     "SchemaAnalysis phase",
			phase:    PhaseSchemaAnalysis,
			expected: "SchemaAnalysis",
		},
		{
			name:     "SQLGeneration phase",
			phase:    PhaseSQLGeneration,
			expected: "SQLGeneration",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.phase.String()
			if result != tc.expected {
				t.Errorf("Expected phase.String() to return '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestPromptEntry_Methods(t *testing.T) {
	now := time.Now()
	entry := PromptEntry{
		Timestamp:    now,
		UserMessage:  "test request",
		SystemPrompt: "test system prompt",
		AIResponse:   "test response",
		Provider:     config.ProviderOpenRouter,
		Model:        "claude-3.5-sonnet",
		InputTokens:  100,
		OutputTokens: 200,
		Cost:         0.05,
	}

	// Test that all fields are set correctly
	if entry.Timestamp != now {
		t.Error("Timestamp not set correctly")
	}

	if entry.UserMessage != "test request" {
		t.Error("UserMessage not set correctly")
	}

	if entry.SystemPrompt != "test system prompt" {
		t.Error("SystemPrompt not set correctly")
	}

	if entry.AIResponse != "test response" {
		t.Error("AIResponse not set correctly")
	}

	if entry.Provider != config.ProviderOpenRouter {
		t.Error("Provider not set correctly")
	}

	if entry.Model != "claude-3.5-sonnet" {
		t.Error("Model not set correctly")
	}

	if entry.InputTokens != 100 {
		t.Error("InputTokens not set correctly")
	}

	if entry.OutputTokens != 200 {
		t.Error("OutputTokens not set correctly")
	}

	if entry.Cost != 0.05 {
		t.Error("Cost not set correctly")
	}
}

func TestPromptHistory_AddEntry(t *testing.T) {
	history := &PromptHistory{
		Entries: make([]PromptEntry, 0),
		MaxSize: 3,
	}

	// Add first entry
	entry1 := PromptEntry{UserMessage: "request 1", Timestamp: time.Now()}
	history.AddEntry(entry1)

	if len(history.Entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(history.Entries))
	}

	// Add second entry
	entry2 := PromptEntry{UserMessage: "request 2", Timestamp: time.Now()}
	history.AddEntry(entry2)

	if len(history.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(history.Entries))
	}

	// Add third entry
	entry3 := PromptEntry{UserMessage: "request 3", Timestamp: time.Now()}
	history.AddEntry(entry3)

	if len(history.Entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(history.Entries))
	}

	// Add fourth entry (should remove oldest)
	entry4 := PromptEntry{UserMessage: "request 4", Timestamp: time.Now()}
	history.AddEntry(entry4)

	if len(history.Entries) != 3 {
		t.Errorf("Expected 3 entries after exceeding max size, got %d", len(history.Entries))
	}

	// First entry should be removed, last entry should be entry4
	if history.Entries[0].UserMessage != "request 2" {
		t.Error("Oldest entry should be removed")
	}

	if history.Entries[2].UserMessage != "request 4" {
		t.Error("Newest entry should be at the end")
	}
}

// Benchmark tests
func BenchmarkParseModelString(b *testing.B) {
	modelStr := "anthropic/claude-3.5-sonnet"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseModelString(modelStr)
	}
}

func BenchmarkFormatPrice(b *testing.B) {
	price := 1.23456
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatPrice(price)
	}
}
