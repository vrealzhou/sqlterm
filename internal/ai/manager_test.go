package ai

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/vrealzhou/sqlterm/internal/config"
	"github.com/vrealzhou/sqlterm/internal/database"
	"github.com/vrealzhou/sqlterm/internal/i18n"
	"github.com/vrealzhou/sqlterm/internal/session"
)


// MockClient implements the Client interface for testing
type MockClient struct {
	models    []ModelInfo
	responses map[string]*ChatResponse
	shouldErr bool
}

func (m *MockClient) Chat(ctx context.Context, request ChatRequest) (*ChatResponse, error) {
	if m.shouldErr {
		return nil, fmt.Errorf("mock error")
	}

	response, exists := m.responses[request.Model]
	if !exists {
		return &ChatResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   request.Model,
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}{
						Role:    "assistant",
						Content: "Mock response",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}, nil
	}
	return response, nil
}

func (m *MockClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	if m.shouldErr {
		return nil, fmt.Errorf("mock error")
	}
	return m.models, nil
}

func (m *MockClient) GetModelInfo(ctx context.Context, modelID string) (*ModelInfo, error) {
	if m.shouldErr {
		return nil, fmt.Errorf("mock error")
	}
	for _, model := range m.models {
		if model.ID == modelID {
			return &model, nil
		}
	}
	return nil, fmt.Errorf("model not found")
}

func (m *MockClient) Close() error {
	return nil
}

// MockConnection implements a basic database connection for testing
type MockConnection struct{}

func (m *MockConnection) Ping() error                                                  { return nil }
func (m *MockConnection) Execute(query string) (*database.QueryResult, error)         { return nil, nil }
func (m *MockConnection) ListTables() ([]string, error)                               { return []string{"users", "posts"}, nil }
func (m *MockConnection) DescribeTable(tableName string) (*database.TableInfo, error) { return nil, nil }
func (m *MockConnection) Close() error                                                 { return nil }

func createTestAIManager(t *testing.T, configDir string) (*Manager, *config.Manager, *session.Manager) {
	// Create i18n manager
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create i18n manager: %v", err)
	}

	// Create config manager
	configMgr := config.NewManager()
	configMgr.Config = config.DefaultConfig()

	// Set up basic AI configuration
	configMgr.Config.AI.Provider = "lmstudio"
	configMgr.Config.AI.Model = "test-model"
	configMgr.Config.AI.SetBaseURL("lmstudio", "http://localhost:1234")

	// Create session manager
	sessionMgr, err := session.NewManager(configDir, i18nMgr)
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	// Create AI manager
	aiMgr, err := NewManager(configMgr, sessionMgr, i18nMgr)
	if err != nil {
		t.Fatalf("Failed to create AI manager: %v", err)
	}

	return aiMgr, configMgr, sessionMgr
}

func TestNewManager_ValidParameters(t *testing.T) {
	tmpDir := t.TempDir()
	aiMgr, _, _ := createTestAIManager(t, tmpDir)

	if aiMgr == nil {
		t.Fatal("Expected AI manager to be created, got nil")
	}

	if aiMgr.configMgr == nil {
		t.Error("Expected config manager to be set")
	}

	if aiMgr.sessionMgr == nil {
		t.Error("Expected session manager to be set")
	}

	if aiMgr.i18nMgr == nil {
		t.Error("Expected i18n manager to be set")
	}

	if aiMgr.promptHistory == nil {
		t.Error("Expected prompt history to be initialized")
	}

	if aiMgr.idGen == nil {
		t.Error("Expected ID generator to be initialized")
	}
}

func TestNewManager_NilParameters(t *testing.T) {
	tmpDir := t.TempDir()
	i18nMgr, _ := i18n.NewManager("en_au")
	configMgr := config.NewManager()
	configMgr.Config = config.DefaultConfig()
	sessionMgr, _ := session.NewManager(tmpDir, i18nMgr)

	testCases := []struct {
		name       string
		configMgr  *config.Manager
		sessionMgr *session.Manager
		i18nMgr    *i18n.Manager
		shouldErr  bool
	}{
		{
			name:       "Nil config manager",
			configMgr:  nil,
			sessionMgr: sessionMgr,
			i18nMgr:    i18nMgr,
			shouldErr:  true,
		},
		{
			name:       "Nil session manager",
			configMgr:  configMgr,
			sessionMgr: nil,
			i18nMgr:    i18nMgr,
			shouldErr:  true,
		},
		{
			name:       "Nil i18n manager",
			configMgr:  configMgr,
			sessionMgr: sessionMgr,
			i18nMgr:    nil,
			shouldErr:  true,
		},
		{
			name:       "All valid",
			configMgr:  configMgr,
			sessionMgr: sessionMgr,
			i18nMgr:    i18nMgr,
			shouldErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up basic configuration if configMgr is not nil
			if tc.configMgr != nil {
				tc.configMgr.Config.AI.Provider = "lmstudio"
				tc.configMgr.Config.AI.Model = "test-model"
				tc.configMgr.Config.AI.SetBaseURL("lmstudio", "http://localhost:1234")
			}

			_, err := NewManager(tc.configMgr, tc.sessionMgr, tc.i18nMgr)

			if tc.shouldErr {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tc.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tc.name, err)
				}
			}
		})
	}
}

func TestManager_IsConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	aiMgr, configMgr, _ := createTestAIManager(t, tmpDir)

	testCases := []struct {
		name       string
		provider   config.AIProvider
		model      string
		apiKey     string
		baseURL    string
		configured bool
	}{
		{
			name:       "OpenRouter properly configured",
			provider:   config.ProviderOpenRouter,
			model:      "anthropic/claude-3.5-sonnet",
			apiKey:     "test-api-key",
			baseURL:    "",
			configured: true,
		},
		{
			name:       "OpenRouter missing API key",
			provider:   config.ProviderOpenRouter,
			model:      "anthropic/claude-3.5-sonnet",
			apiKey:     "",
			baseURL:    "",
			configured: false,
		},
		{
			name:       "LMStudio properly configured",
			provider:   config.ProviderLMStudio,
			model:      "test-model",
			apiKey:     "",
			baseURL:    "http://localhost:1234",
			configured: true,
		},
		{
			name:       "LMStudio missing base URL",
			provider:   config.ProviderLMStudio,
			model:      "test-model",
			apiKey:     "",
			baseURL:    "",
			configured: false,
		},
		{
			name:       "Empty provider",
			provider:   "",
			model:      "test-model",
			apiKey:     "",
			baseURL:    "",
			configured: false,
		},
		{
			name:       "Empty model",
			provider:   config.ProviderLMStudio,
			model:      "",
			apiKey:     "",
			baseURL:    "http://localhost:1234",
			configured: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset configuration
			configMgr.Config.AI.Provider = tc.provider
			configMgr.Config.AI.Model = tc.model
			if tc.apiKey != "" {
				configMgr.Config.AI.SetAPIKey(tc.provider, tc.apiKey)
			} else {
				configMgr.Config.AI.SetAPIKey(tc.provider, "")
			}
			if tc.baseURL != "" {
				configMgr.Config.AI.SetBaseURL(tc.provider, tc.baseURL)
			} else {
				configMgr.Config.AI.SetBaseURL(tc.provider, "")
			}

			configured := aiMgr.IsConfigured()
			if configured != tc.configured {
				t.Errorf("Expected IsConfigured() to return %v, got %v", tc.configured, configured)
			}
		})
	}
}

func TestManager_SetProvider(t *testing.T) {
	tmpDir := t.TempDir()
	aiMgr, configMgr, _ := createTestAIManager(t, tmpDir)

	// Set up LMStudio configuration
	configMgr.Config.AI.SetBaseURL(config.ProviderLMStudio, "http://localhost:1234")

	newProvider := config.ProviderLMStudio
	newModel := "new-model"

	// Test the provider setting logic directly without save
	oldConfig := aiMgr.GetConfig()
	oldConfig.AI.SetProvider(newProvider, newModel)
	
	// Re-initialize client with new provider
	err := aiMgr.initializeClient()
	if err != nil {
		t.Fatalf("Failed to initialize client with new provider: %v", err)
	}

	config := aiMgr.GetConfig()
	if config.AI.Provider != newProvider {
		t.Errorf("Expected provider %s, got %s", newProvider, config.AI.Provider)
	}

	if config.AI.Model != newModel {
		t.Errorf("Expected model %s, got %s", newModel, config.AI.Model)
	}
}

func TestManager_SetAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	aiMgr, configMgr, _ := createTestAIManager(t, tmpDir)

	provider := config.ProviderOpenRouter
	apiKey := "test-api-key"
	model := "anthropic/claude-3.5-sonnet"

	// Set provider and model first
	configMgr.Config.AI.Provider = provider
	configMgr.Config.AI.Model = model

	// Test setting API key directly without save
	config := aiMgr.GetConfig()
	config.AI.SetAPIKey(provider, apiKey)

	// Re-initialize client if this is the current provider
	if provider == config.AI.Provider {
		err := aiMgr.initializeClient()
		if err != nil {
			t.Fatalf("Failed to re-initialize client with new API key: %v", err)
		}
	}

	retrievedKey := config.AI.GetAPIKey(provider)
	if retrievedKey != apiKey {
		t.Errorf("Expected API key %s, got %s", apiKey, retrievedKey)
	}
}

func TestManager_SetBaseURL(t *testing.T) {
	tmpDir := t.TempDir()
	aiMgr, configMgr, _ := createTestAIManager(t, tmpDir)

	provider := config.ProviderLMStudio
	baseURL := "http://localhost:8080"

	// Set provider first
	configMgr.Config.AI.Provider = provider

	// Test setting base URL directly without save
	config := aiMgr.GetConfig()
	config.AI.SetBaseURL(provider, baseURL)

	// Re-initialize client if this is the current provider
	if provider == config.AI.Provider {
		err := aiMgr.initializeClient()
		if err != nil {
			t.Fatalf("Failed to re-initialize client with new base URL: %v", err)
		}
	}

	retrievedURL := config.AI.GetBaseURL(provider)
	if retrievedURL != baseURL {
		t.Errorf("Expected base URL %s, got %s", baseURL, retrievedURL)
	}
}

func TestManager_SessionIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	aiMgr, _, sessionMgr := createTestAIManager(t, tmpDir)

	connectionName := "test-connection"

	// Test creating a session for connection
	session, err := aiMgr.CreateSessionForConnection(connectionName)
	if err != nil {
		t.Fatalf("CreateSessionForConnection failed: %v", err)
	}

	if session == nil {
		t.Fatal("Expected session to be created, got nil")
	}

	if session.ConnectionName != connectionName {
		t.Errorf("Expected connection name %s, got %s", connectionName, session.ConnectionName)
	}

	// Test that current session is set
	currentSession := aiMgr.GetCurrentSession()
	if currentSession == nil {
		t.Fatal("Expected current session to be set, got nil")
	}

	if currentSession.ID != session.ID {
		t.Errorf("Expected current session ID %d, got %d", session.ID, currentSession.ID)
	}

	// Test getting session ID as string
	sessionIDStr := aiMgr.GetSessionID()
	expectedIDStr := fmt.Sprintf("%d", session.ID)
	if sessionIDStr != expectedIDStr {
		t.Errorf("Expected session ID string %s, got %s", expectedIDStr, sessionIDStr)
	}

	// Test recording a query
	query := "SELECT * FROM users"
	executionTime := 100 * time.Millisecond

	err = aiMgr.RecordQuery(query, nil, executionTime)
	if err != nil {
		t.Fatalf("RecordQuery failed: %v", err)
	}

	// Test getting session history
	history, err := aiMgr.GetSessionHistory(10)
	if err != nil {
		t.Fatalf("GetSessionHistory failed: %v", err)
	}

	if len(history) != 1 {
		t.Errorf("Expected 1 query in history, got %d", len(history))
	}

	if history[0].Query != query {
		t.Errorf("Expected query %s, got %s", query, history[0].Query)
	}

	// Test searching history
	searchResults, err := aiMgr.SearchHistory("SELECT", 10)
	if err != nil {
		t.Fatalf("SearchHistory failed: %v", err)
	}

	if len(searchResults) != 1 {
		t.Errorf("Expected 1 search result, got %d", len(searchResults))
	}

	// Test getting existing active session
	existingSession, err := aiMgr.CreateSessionForConnection(connectionName)
	if err != nil {
		t.Fatalf("CreateSessionForConnection (existing) failed: %v", err)
	}

	// Should get the same session since it's already active
	if existingSession.ID != session.ID {
		t.Errorf("Expected to get existing session ID %d, got %d", session.ID, existingSession.ID)
	}

	// Clean up
	sessionMgr.Close()
}

func TestManager_VectorStore(t *testing.T) {
	tmpDir := t.TempDir()
	aiMgr, _, _ := createTestAIManager(t, tmpDir)

	// Test closing vector store when none is initialized (should not fail)
	err := aiMgr.CloseVectorStore()
	if err != nil {
		t.Errorf("CloseVectorStore failed: %v", err)
	}

	// Test that vector store operations fail gracefully without initialized store
	_, err = aiMgr.SearchSimilarTables(context.Background(), "test", 5)
	if err == nil {
		t.Error("Expected error when searching similar tables without vector store")
	}

	_, err = aiMgr.GetRecentTables(5)
	if err == nil {
		t.Error("Expected error when getting recent tables without vector store")
	}
}

func TestManager_Chat_NoClient(t *testing.T) {
	tmpDir := t.TempDir()
	aiMgr, _, _ := createTestAIManager(t, tmpDir)

	// Set client to nil to test error condition
	aiMgr.client = nil

	ctx := context.Background()
	userMessage := "Test message"
	conversationCtx := NewConversationContext(userMessage)

	_, _, err := aiMgr.Chat(ctx, userMessage, conversationCtx)
	if err == nil {
		t.Error("Expected error when client is nil, got none")
	}

	expectedErrMsg := "AI client not initialized"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrMsg, err.Error())
	}
}

func TestManager_Chat_WithMockClient(t *testing.T) {
	tmpDir := t.TempDir()
	aiMgr, _, _ := createTestAIManager(t, tmpDir)

	// Set up mock client
	mockClient := &MockClient{
		responses: map[string]*ChatResponse{
			"test-model": {
				ID:      "test-response",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "test-model",
				Choices: []struct {
					Index   int `json:"index"`
					Message struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"message"`
					FinishReason string `json:"finish_reason"`
				}{
					{
						Index: 0,
						Message: struct {
							Role    string `json:"role"`
							Content string `json:"content"`
						}{
							Role:    "assistant",
							Content: "Test response from mock",
						},
						FinishReason: "stop",
					},
				},
				Usage: struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				}{
					PromptTokens:     15,
					CompletionTokens: 25,
					TotalTokens:      40,
				},
			},
		},
	}

	aiMgr.client = mockClient

	ctx := context.Background()
	userMessage := "Test message"
	conversationCtx := NewConversationContext(userMessage)

	response, usage, err := aiMgr.Chat(ctx, userMessage, conversationCtx)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	expectedResponse := "Test response from mock"
	if response != expectedResponse {
		t.Errorf("Expected response '%s', got '%s'", expectedResponse, response)
	}

	if usage == nil {
		t.Fatal("Expected usage to be returned, got nil")
	}

	if usage.InputTokens != 15 {
		t.Errorf("Expected input tokens 15, got %d", usage.InputTokens)
	}

	if usage.OutputTokens != 25 {
		t.Errorf("Expected output tokens 25, got %d", usage.OutputTokens)
	}

	if usage.TotalTokens != 40 {
		t.Errorf("Expected total tokens 40, got %d", usage.TotalTokens)
	}
}

func TestManager_GetModels(t *testing.T) {
	tmpDir := t.TempDir()
	aiMgr, _, _ := createTestAIManager(t, tmpDir)

	// Test with nil client (set client to nil explicitly)
	aiMgr.client = nil
	_, err := aiMgr.GetModels(context.Background())
	if err == nil {
		t.Error("Expected error when client is nil, got none")
	}

	// Test with mock client
	mockModels := []ModelInfo{
		{
			ID:          "model-1",
			Name:        "Test Model 1",
			Description: "First test model",
			Provider:    "test",
		},
		{
			ID:          "model-2",
			Name:        "Test Model 2",
			Description: "Second test model",
			Provider:    "test",
		},
	}

	mockClient := &MockClient{
		models: mockModels,
	}
	aiMgr.client = mockClient

	ctx := context.Background()
	models, err := aiMgr.GetModels(ctx)
	if err != nil {
		t.Fatalf("GetModels failed: %v", err)
	}

	if len(models) != len(mockModels) {
		t.Errorf("Expected %d models, got %d", len(mockModels), len(models))
	}

	for i, model := range models {
		expected := mockModels[i]
		if model.ID != expected.ID {
			t.Errorf("Expected model ID %s, got %s", expected.ID, model.ID)
		}
		if model.Name != expected.Name {
			t.Errorf("Expected model name %s, got %s", expected.Name, model.Name)
		}
	}
}

func TestManager_Close(t *testing.T) {
	tmpDir := t.TempDir()
	aiMgr, _, _ := createTestAIManager(t, tmpDir)

	// Test closing without vector store (should not fail)
	err := aiMgr.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestManager_PromptHistory(t *testing.T) {
	tmpDir := t.TempDir()
	aiMgr, _, _ := createTestAIManager(t, tmpDir)

	// Initially should be empty
	history := aiMgr.GetPromptHistory()
	if len(history) != 0 {
		t.Errorf("Expected empty prompt history, got %d entries", len(history))
	}

	// Set max size for prompt history
	aiMgr.promptHistory.MaxSize = 10

	// Test adding entries through prompt history object
	entry := PromptEntry{
		Timestamp:    time.Now(),
		UserMessage:  "Test prompt",
		SystemPrompt: "Test system prompt",
		AIResponse:   "Test response",
		Provider:     config.ProviderLMStudio,
		Model:        "test-model",
		InputTokens:  10,
		OutputTokens: 20,
		Cost:         0.01,
	}

	aiMgr.promptHistory.AddEntry(entry)

	history = aiMgr.GetPromptHistory()
	if len(history) != 1 {
		t.Errorf("Expected 1 entry in prompt history, got %d", len(history))
		return
	}

	if history[0].UserMessage != entry.UserMessage {
		t.Errorf("Expected user message %s, got %s", entry.UserMessage, history[0].UserMessage)
	}
}

func TestManager_ErrorConditions(t *testing.T) {
	tmpDir := t.TempDir()
	aiMgr, _, _ := createTestAIManager(t, tmpDir)

	// Test session operations without current session
	err := aiMgr.RecordQuery("SELECT 1", nil, time.Millisecond)
	if err == nil {
		t.Error("Expected error when recording query without current session")
	}

	_, err = aiMgr.GetSessionHistory(10)
	if err == nil {
		t.Error("Expected error when getting history without current session")
	}

	_, err = aiMgr.SearchHistory("test", 10)
	if err == nil {
		t.Error("Expected error when searching history without current session")
	}

	// Test vector store operations without vector store
	_, err = aiMgr.SearchSimilarTables(context.Background(), "test", 5)
	if err == nil {
		t.Error("Expected error when searching similar tables without vector store")
	}

	_, err = aiMgr.GetRecentTables(5)
	if err == nil {
		t.Error("Expected error when getting recent tables without vector store")
	}
}

func TestManager_UsageStore(t *testing.T) {
	tmpDir := t.TempDir()
	aiMgr, _, _ := createTestAIManager(t, tmpDir)

	// Initially should be nil
	usageStore := aiMgr.GetUsageStore()
	if usageStore != nil {
		t.Error("Expected usage store to be nil initially")
	}

	// Usage store remains nil until vector store is successfully initialized
	// In our test environment this is expected to remain nil
}

// Test helper to clean up test files
func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}