package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vrealzhou/sqlterm/internal/config"
	"github.com/vrealzhou/sqlterm/internal/database"
	"github.com/vrealzhou/sqlterm/internal/i18n"
	"github.com/vrealzhou/sqlterm/internal/session"
	"github.com/vrealzhou/sqlterm/internal/utils"
)

// Manager manages AI clients and configuration
type Manager struct {
	configMgr       *config.Manager     // Config manager for AI configuration
	sessionMgr      *session.Manager    // Session manager for session handling
	client          Client              // AI client interface
	promptHistory   *PromptHistory      // Prompt history tracking
	recentTables    []string            // Session memory for recently mentioned tables
	maxTables       int                 // Maximum tables to include in context
	vectorStore     *VectorStore        // Vector database for semantic search
	conversationCtx *ConversationContext // Current conversation context
	i18nMgr         *i18n.Manager       // Internationalization manager
	usageStore      *UsageStore         // Usage tracking store
	currentSession  *session.Session    // Current active session
	idGen           *utils.IDGen        // ID generator
}

// NewManager creates a new AI manager with dependency injection
func NewManager(configMgr *config.Manager, sessionMgr *session.Manager, i18nMgr *i18n.Manager) (*Manager, error) {
	if configMgr == nil {
		return nil, fmt.Errorf("config manager cannot be nil")
	}
	if sessionMgr == nil {
		return nil, fmt.Errorf("session manager cannot be nil")
	}
	if i18nMgr == nil {
		return nil, fmt.Errorf("i18n manager cannot be nil")
	}

	idGen, err := utils.NewIDGen()
	if err != nil {
		return nil, fmt.Errorf("failed to create ID generator: %w", err)
	}

	manager := &Manager{
		configMgr: configMgr,
		sessionMgr: sessionMgr,
		i18nMgr:   i18nMgr,
		promptHistory: &PromptHistory{
			Entries: make([]PromptEntry, 0),
		},
		recentTables: make([]string, 0),
		maxTables:    10,
		idGen:       idGen,
	}

	// Initialize client with current config
	if err := manager.initializeClient(); err != nil {
		return nil, fmt.Errorf("failed to initialize AI client: %w", err)
	}

	return manager, nil
}

// NewManagerWithValidation creates a new AI manager and validates the configuration
func NewManagerWithValidation(configMgr *config.Manager, sessionMgr *session.Manager, i18nMgr *i18n.Manager) (*Manager, error) {
	manager, err := NewManager(configMgr, sessionMgr, i18nMgr)
	if err != nil {
		return nil, err
	}

	// Validate AI configuration
	if !manager.IsConfigured() {
		return nil, fmt.Errorf("AI configuration is incomplete")
	}

	return manager, nil
}

// GetConfig returns the current AI configuration
func (m *Manager) GetConfig() *config.Config {
	return m.configMgr.Config
}

// IsConfigured checks if the AI manager is properly configured
func (m *Manager) IsConfigured() bool {
	config := m.GetConfig()
	if config == nil || config.AI.Provider == "" || config.AI.Model == "" {
		return false
	}

	// Check if API key is required and present
	switch string(config.AI.Provider) {
	case "openrouter":
		return config.AI.GetAPIKey("openrouter") != ""
	case "ollama", "lmstudio":
		return config.AI.GetBaseURL(config.AI.Provider) != ""
	default:
		return false
	}
}

// initializeClient initializes the AI client based on current configuration
func (m *Manager) initializeClient() error {
	config := m.GetConfig()
	if config == nil {
		return fmt.Errorf("no configuration available")
	}

	switch string(config.AI.Provider) {
	case "openrouter":
		apiKey := config.AI.GetAPIKey("openrouter")
		if apiKey == "" {
			return fmt.Errorf("OpenRouter API key not configured")
		}
		client := NewOpenRouterClient(apiKey)
		m.client = client

	case "ollama":
		baseURL := config.AI.GetBaseURL("ollama")
		client := NewOllamaClient(baseURL)
		m.client = client

	case "lmstudio":
		baseURL := config.AI.GetBaseURL("lmstudio")
		client := NewLMStudioClient(baseURL, m.i18nMgr)
		m.client = client

	default:
		return fmt.Errorf(m.i18nMgr.Get("unsupported_provider"), config.AI.Provider)
	}

	return nil
}

// SetProvider changes the current AI provider and model
func (m *Manager) SetProvider(provider config.AIProvider, model string) error {
	config := m.GetConfig()
	config.AI.SetProvider(provider, model)
	
	// Re-initialize client with new provider
	if err := m.initializeClient(); err != nil {
		return fmt.Errorf("failed to initialize client with new provider: %w", err)
	}

	// Save configuration
	return m.configMgr.SaveConfig()
}

// SetAPIKey sets an API key for a provider and re-initializes client if it's the current provider
func (m *Manager) SetAPIKey(provider config.AIProvider, apiKey string) error {
	config := m.GetConfig()
	config.AI.SetAPIKey(provider, apiKey)

	// Re-initialize client if this is the current provider
	if provider == config.AI.Provider {
		if err := m.initializeClient(); err != nil {
			return fmt.Errorf("failed to re-initialize client with new API key: %w", err)
		}
	}

	// Save configuration
	return m.configMgr.SaveConfig()
}

// SetBaseURL sets a base URL for a provider and re-initializes client if it's the current provider
func (m *Manager) SetBaseURL(provider config.AIProvider, baseURL string) error {
	config := m.GetConfig()
	config.AI.SetBaseURL(provider, baseURL)

	// Re-initialize client if this is the current provider
	if provider == config.AI.Provider {
		if err := m.initializeClient(); err != nil {
			return fmt.Errorf("failed to re-initialize client with new base URL: %w", err)
		}
	}

	// Save configuration
	return m.configMgr.SaveConfig()
}

// GetModels returns available models for the current provider
func (m *Manager) GetModels(ctx context.Context) ([]ModelInfo, error) {
	if m.client == nil {
		return nil, fmt.Errorf("AI client not initialized")
	}

	models, err := m.client.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get models: %w", err)
	}

	return models, nil
}

// Chat sends a message to the AI and returns the response
func (m *Manager) Chat(ctx context.Context, userMessage string, conversationCtx *ConversationContext) (string, *Usage, error) {
	if m.client == nil {
		return "", nil, fmt.Errorf("AI client not initialized")
	}

	// Create messages for the conversation
	messages := m.buildMessages(userMessage, conversationCtx)

	// Create chat request
	request := ChatRequest{
		Model:    m.GetConfig().AI.Model,
		Messages: messages,
	}

	// Send to AI
	response, err := m.client.Chat(ctx, request)
	if err != nil {
		return "", nil, fmt.Errorf("AI request failed: %w", err)
	}

	// Extract response content
	if len(response.Choices) == 0 {
		return "", nil, fmt.Errorf("no response choices returned")
	}

	content := response.Choices[0].Message.Content

	// Create usage object
	usage := &Usage{
		InputTokens:  response.Usage.PromptTokens,
		OutputTokens: response.Usage.CompletionTokens,
		TotalTokens:  response.Usage.TotalTokens,
	}

	// Record usage if session is available
	if m.currentSession != nil && m.usageStore != nil {
		config := m.GetConfig()
		err := m.usageStore.RecordUsage(fmt.Sprintf("%d", m.currentSession.ID), config.AI.Provider, config.AI.Model,
			usage.InputTokens, usage.OutputTokens, usage.Cost, userMessage, content, m.buildSystemPrompt(conversationCtx))
		if err != nil {
			// Log warning but don't fail the request
			fmt.Printf("Warning: failed to record usage: %v\n", err)
		}
	}

	// Update vector store with query pattern if available
	if m.vectorStore != nil {
		recentTables, err := m.vectorStore.GetRecentTables(5)
		if err == nil {
			m.vectorStore.AddQueryPattern(userMessage, recentTables)
		}
	}

	return content, usage, nil
}

// Close closes the AI manager and cleans up resources
func (m *Manager) Close() error {
	if m.vectorStore != nil {
		m.vectorStore.Close()
	}
	return nil
}

// InitializeVectorStore sets up vector database for a database connection
func (m *Manager) InitializeVectorStore(connectionName string, connection database.Connection) error {
	if m.vectorStore != nil {
		m.vectorStore.Close()
	}

	// Get session directory from session manager
	sessionDir := m.sessionMgr.GetSessionDir(connectionName)
	
	vectorStore, err := NewVectorStore(sessionDir, connectionName, connection)
	if err != nil {
		return fmt.Errorf("failed to create vector store: %w", err)
	}

	m.vectorStore = vectorStore

	// Initialize usage store
	usageStore, err := NewUsageStore(vectorStore)
	if err != nil {
		return fmt.Errorf("failed to create usage store: %w", err)
	}

	m.usageStore = usageStore

	// Update table embeddings in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := m.vectorStore.UpdateTableEmbeddings(ctx); err != nil {
			fmt.Printf("Warning: failed to update table embeddings: %v\n", err)
		}
	}()

	return nil
}

// CloseVectorStore closes the vector store
func (m *Manager) CloseVectorStore() error {
	if m.vectorStore != nil {
		err := m.vectorStore.Close()
		m.vectorStore = nil
		return err
	}
	return nil
}

// SetCurrentSession sets the current active session
func (m *Manager) SetCurrentSession(session *session.Session) {
	m.currentSession = session
}

// GetCurrentSession returns the current active session
func (m *Manager) GetCurrentSession() *session.Session {
	return m.currentSession
}

// CreateSessionForConnection creates or gets an active session for a connection
func (m *Manager) CreateSessionForConnection(connectionName string) (*session.Session, error) {
	// Try to get existing active session first
	activeSession, err := m.sessionMgr.GetActiveSession(connectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to check for active session: %w", err)
	}

	if activeSession != nil {
		m.currentSession = activeSession
		return activeSession, nil
	}

	// Create new session
	newSession, err := m.sessionMgr.CreateSession(connectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	m.currentSession = newSession
	return newSession, nil
}

// RecordQuery records a query execution in the current session
func (m *Manager) RecordQuery(query string, result *database.QueryResult, executionTime time.Duration) error {
	if m.currentSession == nil {
		return fmt.Errorf("no active session")
	}

	_, err := m.sessionMgr.RecordQuery(m.currentSession.ID, query, result, executionTime)
	return err
}

// GetSessionHistory returns query history for the current session
func (m *Manager) GetSessionHistory(limit int) ([]*session.QueryRecord, error) {
	if m.currentSession == nil {
		return nil, fmt.Errorf("no active session")
	}

	return m.sessionMgr.GetQueryHistory(m.currentSession.ID, limit, 0)
}

// SearchHistory searches query history in the current session
func (m *Manager) SearchHistory(searchTerm string, limit int) ([]*session.QueryRecord, error) {
	if m.currentSession == nil {
		return nil, fmt.Errorf("no active session")
	}

	return m.sessionMgr.SearchQueryHistory(m.currentSession.ID, searchTerm, limit)
}

// Additional methods for backward compatibility

// GetUsageStore returns the usage store
func (m *Manager) GetUsageStore() *UsageStore {
	return m.usageStore
}

// GetSessionID returns the current session ID as string for compatibility
func (m *Manager) GetSessionID() string {
	if m.currentSession != nil {
		return fmt.Sprintf("%d", m.currentSession.ID)
	}
	return m.generateSessionID()
}

// ResetSessionID creates a new session if needed for compatibility
func (m *Manager) ResetSessionID() {
	// This method is for backward compatibility
	// In the new design, sessions are managed explicitly
}

// generateSessionID generates a session ID for compatibility
func (m *Manager) generateSessionID() string {
	return m.idGen.GenerateString()
}

// GetPromptHistory returns the prompt history
func (m *Manager) GetPromptHistory() []PromptEntry {
	return m.promptHistory.Entries
}

// buildMessages builds the message array for AI chat
func (m *Manager) buildMessages(userMessage string, conversationCtx *ConversationContext) []ChatMessage {
	messages := []ChatMessage{
		{Role: "system", Content: m.buildSystemPrompt(conversationCtx)},
	}

	// Add conversation context if available
	if conversationCtx != nil {
		// Add recent messages from conversation context
		for _, turn := range conversationCtx.ConversationHistory {
			messages = append(messages, ChatMessage{Role: "user", Content: turn.UserMessage})
			messages = append(messages, ChatMessage{Role: "assistant", Content: turn.AIResponse})
		}
	}

	// Add current user message
	messages = append(messages, ChatMessage{
		Role:    "user",
		Content: userMessage,
	})

	return messages
}

// buildSystemPrompt builds the system prompt with context
func (m *Manager) buildSystemPrompt(conversationCtx *ConversationContext) string {
	basePrompt := "You are an AI assistant that helps with SQL queries and database management."
	
	if conversationCtx != nil {
		// Add schema information
		if len(conversationCtx.LoadedTables) > 0 {
			basePrompt += "\n\nAvailable database schema:\n"
			for tableName, tableInfo := range conversationCtx.LoadedTables {
				basePrompt += fmt.Sprintf("\nTable: %s\n", tableName)
				for _, col := range tableInfo.Columns {
					basePrompt += fmt.Sprintf("- %s (%s)\n", col.Name, col.Type)
				}
			}
		}

		// Add context about recent tables
		if len(conversationCtx.RelatedTables) > 0 {
			basePrompt += fmt.Sprintf("\n\nRecently accessed tables: %s", strings.Join(conversationCtx.RelatedTables, ", "))
		}
	}

	return basePrompt
}

// Helper methods for semantic search and table management

// SearchSimilarTables searches for similar tables using vector search
func (m *Manager) SearchSimilarTables(ctx context.Context, query string, limit int) ([]VectorSearchResult, error) {
	if m.vectorStore == nil {
		return nil, fmt.Errorf("vector store not initialized")
	}

	return m.vectorStore.SearchSimilarTables(ctx, query, limit)
}

// GetRecentTables returns recently accessed tables
func (m *Manager) GetRecentTables(limit int) ([]string, error) {
	if m.vectorStore == nil {
		return nil, fmt.Errorf("vector store not initialized")
	}

	return m.vectorStore.GetRecentTables(limit)
}

// ParseModelString parses a model string to extract provider and model
func ParseModelString(modelStr string) (config.AIProvider, string, error) {
	// Check for empty string
	if strings.TrimSpace(modelStr) == "" {
		return "", "", fmt.Errorf("model string cannot be empty")
	}
	
	// Check for specific provider prefixes
	if strings.Contains(modelStr, "gpt-") || strings.Contains(modelStr, "claude-") || strings.Contains(modelStr, "anthropic/") {
		return config.ProviderOpenRouter, modelStr, nil
	}

	// Check for Ollama-style models (usually single names or with colons)
	if strings.Contains(modelStr, ":") || (!strings.Contains(modelStr, "/") && !strings.Contains(modelStr, "-")) {
		return config.ProviderOllama, modelStr, nil
	}

	// Default to LMStudio for other formats
	return config.ProviderLMStudio, modelStr, nil
}