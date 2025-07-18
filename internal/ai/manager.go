package ai

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"sqlterm/internal/config"
	"sqlterm/internal/core"
	"sqlterm/internal/i18n"
	"sqlterm/internal/utils"
)

// Manager manages AI clients and configuration
type Manager struct {
	config          *config.Config
	configDir       string
	client          Client
	promptHistory   *PromptHistory
	recentTables    []string             // Session memory for recently mentioned tables
	maxTables       int                  // Maximum tables to include in context
	vectorStore     *VectorStore         // Vector database for semantic search
	conversationCtx *ConversationContext // Current conversation context
	i18nMgr         *i18n.Manager        // Internationalization manager
	usageStore      *UsageStore          // Usage tracking store
	sessionID       string               // Current session ID for usage tracking
	idGen           *utils.IDGen
}

// NewManager creates a new AI manager
func NewManager(configDir string) (*Manager, error) {
	i18nMgr, config, err := config.LoadConfig(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load AI config: %w", err)
	}

	idGen, err := utils.NewIDGen()
	if err != nil {
		return nil, err
	}

	manager := &Manager{
		config:    config,
		configDir: configDir,
		promptHistory: &PromptHistory{
			Entries: make([]PromptEntry, 0),
			MaxSize: 100, // Keep last 100 prompts
		},
		recentTables: make([]string, 0),
		maxTables:    15, // Limit context to 15 most relevant tables
		i18nMgr:      i18nMgr,
		idGen:        idGen,
	}
	manager.sessionID = manager.generateSessionID()

	// Try to initialize client, but don't fail if it's not possible
	// This allows the manager to be created even if API keys aren't configured yet
	_ = manager.initializeClient()

	return manager, nil
}

// NewManagerWithValidation creates a new AI manager and requires valid client initialization
func NewManagerWithValidation(configDir string) (*Manager, error) {
	i18nMgr, config, err := config.LoadConfig(configDir)
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
		recentTables: make([]string, 0),
		maxTables:    15, // Limit context to 15 most relevant tables
		i18nMgr:      i18nMgr,
	}
	manager.sessionID = manager.generateSessionID()

	// Initialize client - this will fail if configuration is invalid
	if err := manager.initializeClient(); err != nil {
		return nil, fmt.Errorf("failed to initialize AI client: %w", err)
	}

	return manager, nil
}

// initializeClient initializes the appropriate client based on current provider
func (m *Manager) initializeClient() error {
	switch m.config.AI.Provider {
	case config.ProviderOpenRouter:
		apiKey := m.config.GetAPIKey(config.ProviderOpenRouter)
		if apiKey == "" {
			return errors.New(m.i18nMgr.Get("openrouter_api_key_not_configured"))
		}
		m.client = NewOpenRouterClient(apiKey)
	case config.ProviderOllama:
		baseURL := m.config.GetBaseURL(config.ProviderOllama)
		m.client = NewOllamaClient(baseURL)
	case config.ProviderLMStudio:
		baseURL := m.config.GetBaseURL(config.ProviderLMStudio)
		m.client = NewLMStudioClient(baseURL, m.i18nMgr)
	default:
		return fmt.Errorf(m.i18nMgr.Get("unsupported_provider"), m.config.AI.Provider)
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

// UpdateLanguage updates the i18n manager when language changes
func (m *Manager) UpdateLanguage(language string) error {
	if m.i18nMgr != nil {
		m.i18nMgr.SetLanguage(language)
	}
	return nil
}

// Chat sends a chat message and returns the response
func (m *Manager) Chat(ctx context.Context, message string, systemPrompt string) (string, error) {
	if !m.IsConfigured() {
		return "", errors.New(m.i18nMgr.Get("ai_client_not_configured"))
	}

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: message},
	}

	request := ChatRequest{
		Model:       m.config.AI.Model,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   4000,
	}

	response, err := m.client.Chat(ctx, request)
	if err != nil {
		return "", fmt.Errorf(m.i18nMgr.Get("chat_request_failed"), err)
	}

	if len(response.Choices) == 0 {
		return "", errors.New(m.i18nMgr.Get("no_response_choices_returned"))
	}

	// Calculate cost and update usage
	cost := m.calculateCost(response.Usage.PromptTokens, response.Usage.CompletionTokens)

	// Add to prompt history
	aiResponse := response.Choices[0].Message.Content
	m.addToPromptHistory(message, systemPrompt, aiResponse, response.Usage.PromptTokens, response.Usage.CompletionTokens, cost)

	return aiResponse, nil
}

// calculateCost calculates the cost based on token usage and current model
func (m *Manager) calculateCost(inputTokens, outputTokens int) float64 {
	// Only calculate cost for OpenRouter (others are free/local)
	if m.config.AI.Provider != config.ProviderOpenRouter {
		return 0.0
	}

	// Default pricing for popular models (per 1M tokens)
	pricing := map[string]*Pricing{
		"anthropic/claude-3.5-sonnet": {
			InputCostPerToken:  3.0 / 1000000,  // $3 per 1M input tokens
			OutputCostPerToken: 15.0 / 1000000, // $15 per 1M output tokens
		},
		"anthropic/claude-3-haiku": {
			InputCostPerToken:  0.25 / 1000000, // $0.25 per 1M input tokens
			OutputCostPerToken: 1.25 / 1000000, // $1.25 per 1M output tokens
		},
		"openai/gpt-4o": {
			InputCostPerToken:  5.0 / 1000000,  // $5 per 1M input tokens
			OutputCostPerToken: 15.0 / 1000000, // $15 per 1M output tokens
		},
		"openai/gpt-4o-mini": {
			InputCostPerToken:  0.15 / 1000000, // $0.15 per 1M input tokens
			OutputCostPerToken: 0.6 / 1000000,  // $0.6 per 1M output tokens
		},
	}

	modelPricing, exists := pricing[m.config.AI.Model]
	if !exists {
		// Default pricing if model not found
		return float64(inputTokens)*0.001/1000 + float64(outputTokens)*0.003/1000
	}

	return float64(inputTokens)*modelPricing.InputCostPerToken + float64(outputTokens)*modelPricing.OutputCostPerToken
}

// ListModels returns available models for the current provider
func (m *Manager) ListModels(ctx context.Context) ([]ModelInfo, error) {
	if !m.IsConfigured() {
		return nil, errors.New(m.i18nMgr.Get("ai_client_not_configured"))
	}
	return m.client.ListModels(ctx)
}

// SetProvider changes the current provider and model
func (m *Manager) SetProvider(provider config.Provider, model string) error {
	m.config.SetProvider(provider, model)

	// Try to initialize client, but don't fail if credentials aren't ready yet
	_ = m.initializeClient()

	return config.SaveConfig(m.config, m.configDir, m.i18nMgr)
}

// SetAPIKey sets an API key for a provider
func (m *Manager) SetAPIKey(provider config.Provider, apiKey string) error {
	m.config.SetAPIKey(provider, apiKey)

	// Re-initialize client if this is the current provider
	if provider == m.config.AI.Provider {
		_ = m.initializeClient()
	}

	return config.SaveConfig(m.config, m.configDir, m.i18nMgr)
}

// SetBaseURL sets a base URL for a provider
func (m *Manager) SetBaseURL(provider config.Provider, baseURL string) error {
	m.config.SetBaseURL(provider, baseURL)

	// Re-initialize client if this is the current provider
	if provider == m.config.AI.Provider {
		_ = m.initializeClient()
	}

	return config.SaveConfig(m.config, m.configDir, m.i18nMgr)
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *config.Config {
	return m.config
}

// SetLanguage updates the language configuration
func (m *Manager) SetLanguage(language string) error {
	m.config.SetLanguage(language)
	return config.SaveConfig(m.config, m.configDir, m.i18nMgr)
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

// ParseModelString parses a model string and determines the provider
func ParseModelString(modelStr string) (config.Provider, string, error) {
	if modelStr == "" {
		return "", "", fmt.Errorf("model string cannot be empty")
	}

	// If it contains a slash, it's an OpenRouter model
	if strings.Contains(modelStr, "/") {
		return config.ProviderOpenRouter, modelStr, nil
	}

	// If it contains a colon, it's an Ollama model
	if strings.Contains(modelStr, ":") {
		return config.ProviderOllama, modelStr, nil
	}

	// Otherwise, it's an LMStudio model
	return config.ProviderLMStudio, modelStr, nil
}

// FormatPrice formats a price for display
func FormatPrice(price float64) string {
	if price <= 0 {
		return "Free"
	}
	if price < 0.01 {
		return fmt.Sprintf("$%.6f", price)
	}
	return fmt.Sprintf("$%.2f", price)
}

// ParseFloat safely parses a float from string
func ParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// addToPromptHistory adds a prompt entry to the history
func (m *Manager) addToPromptHistory(userMessage, systemPrompt, aiResponse string, inputTokens, outputTokens int, cost float64) {
	entry := PromptEntry{
		Timestamp:    time.Now(),
		UserMessage:  userMessage,
		SystemPrompt: systemPrompt,
		AIResponse:   aiResponse,
		Provider:     m.config.AI.Provider,
		Model:        m.config.AI.Model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         cost,
	}

	m.promptHistory.Entries = append(m.promptHistory.Entries, entry)

	// Keep only the last MaxSize entries
	if len(m.promptHistory.Entries) > m.promptHistory.MaxSize {
		m.promptHistory.Entries = m.promptHistory.Entries[len(m.promptHistory.Entries)-m.promptHistory.MaxSize:]
	}

	// Record usage statistics in the database
	if m.usageStore != nil {
		err := m.usageStore.RecordUsage(m.sessionID, m.config.AI.Provider, m.config.AI.Model,
			inputTokens, outputTokens, cost, userMessage, aiResponse, systemPrompt)
		if err != nil {
			fmt.Printf(m.i18nMgr.Get("failed_record_usage_warning"), err)
		}
	}

	// Learn from successful queries for vector store
	if m.vectorStore != nil {
		m.learnFromQuery(userMessage)
	}
}

// learnFromQuery extracts table usage patterns for machine learning
func (m *Manager) learnFromQuery(userMessage string) {
	// Extract table names that were likely used in the response
	// This is a simplified implementation - in practice, you'd parse the AI response
	// to extract actual SQL queries and table usage

	go func() {
		// Get recent tables as proxy for what was likely used
		recentTables, err := m.vectorStore.GetRecentTables(5)
		if err == nil && len(recentTables) > 0 {
			m.vectorStore.AddQueryPattern(userMessage, recentTables)
		}
	}()
}

// GetPromptHistory returns the prompt history
func (m *Manager) GetPromptHistory() []PromptEntry {
	if m.promptHistory == nil {
		return []PromptEntry{}
	}
	return m.promptHistory.Entries
}

// InitializeVectorStore sets up vector database for a database connection
func (m *Manager) InitializeVectorStore(connectionName string, connection core.Connection) error {
	if m.vectorStore != nil {
		m.vectorStore.Close()
	}

	vectorStore, err := NewVectorStore(m.configDir, connectionName, connection)
	if err != nil {
		return fmt.Errorf("failed to initialize vector store: %w", err)
	}

	m.vectorStore = vectorStore

	// Initialize usage store with the vector store
	usageStore, err := NewUsageStore(vectorStore)
	if err != nil {
		return fmt.Errorf("failed to initialize usage store: %w", err)
	}
	m.usageStore = usageStore

	// Update embeddings in background
	go func() {
		ctx := context.Background()
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

// extractTableNames extracts table names mentioned in user query
func (m *Manager) extractTableNames(userQuery string, allTables []string) []string {
	var mentioned []string
	queryLower := strings.ToLower(userQuery)

	for _, table := range allTables {
		tableLower := strings.ToLower(table)
		// Look for table name as word boundary
		pattern := `\b` + regexp.QuoteMeta(tableLower) + `\b`
		if matched, _ := regexp.MatchString(pattern, queryLower); matched {
			mentioned = append(mentioned, table)
		}
	}

	return mentioned
}

// findRelatedTables finds tables related via foreign keys (simplified version)
func (m *Manager) findRelatedTables(tables []string, allTables []string) []string {
	// For now, use simple heuristics - look for tables with similar prefixes
	// In a full implementation, this would query the database for actual FK relationships
	var related []string

	for _, table := range tables {
		tablePrefix := m.getTablePrefix(table)
		for _, candidate := range allTables {
			if m.getTablePrefix(candidate) == tablePrefix && !m.contains(tables, candidate) {
				related = append(related, candidate)
			}
		}
	}

	return related
}

// getTablePrefix extracts common prefixes from table names
func (m *Manager) getTablePrefix(tableName string) string {
	// Look for common patterns like user_, order_, product_, etc.
	parts := strings.Split(tableName, "_")
	if len(parts) > 1 {
		return parts[0]
	}

	// Look for camelCase patterns
	re := regexp.MustCompile(`^[A-Z][a-z]+`)
	if match := re.FindString(tableName); match != "" {
		return strings.ToLower(match)
	}

	return ""
}

// contains checks if slice contains string
func (m *Manager) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// addToRecentTables adds tables to session memory
func (m *Manager) addToRecentTables(tables []string) {
	for _, table := range tables {
		// Remove if already exists to move to front
		for i, existing := range m.recentTables {
			if existing == table {
				m.recentTables = append(m.recentTables[:i], m.recentTables[i+1:]...)
				break
			}
		}
		// Add to front
		m.recentTables = append([]string{table}, m.recentTables...)
	}

	// Keep only last 10 recent tables
	if len(m.recentTables) > 10 {
		m.recentTables = m.recentTables[:10]
	}
}

// GenerateVectorSystemPrompt creates system prompt using vector similarity search
func (m *Manager) GenerateVectorSystemPrompt(userQuery string, allTables []string) string {
	var prompt strings.Builder

	prompt.WriteString("You are an AI assistant helping with SQL queries and database operations. ")

	if len(allTables) == 0 {
		prompt.WriteString("No database connection available.\n\n")
		return m.addGuidelines(&prompt)
	}

	// Use vector search if available, otherwise fall back to simple method
	if m.vectorStore != nil {
		return m.generateVectorBasedPrompt(userQuery, allTables)
	}

	// Fallback to the original smart prompt
	return m.GenerateSmartSystemPrompt(userQuery, allTables)
}

// generateVectorBasedPrompt uses vector similarity for context selection
func (m *Manager) generateVectorBasedPrompt(userQuery string, allTables []string) string {
	var prompt strings.Builder

	prompt.WriteString("You are an AI assistant helping with SQL queries and database operations. ")
	prompt.WriteString(fmt.Sprintf("You have access to a database with %d total tables. ", len(allTables)))

	ctx := context.Background()
	results, err := m.vectorStore.SearchSimilarTables(ctx, userQuery, m.maxTables)
	if err != nil {
		// Fallback to simple method on error
		fmt.Printf("Warning: vector search failed, falling back to simple method: %v\n", err)
		return m.GenerateSmartSystemPrompt(userQuery, allTables)
	}

	if len(results) > 0 {
		prompt.WriteString("Most relevant tables for this query:\n\n")

		for i, result := range results {
			table := result.Table
			prompt.WriteString(fmt.Sprintf("%d. **%s** (similarity: %.2f) - %s\n",
				i+1, table.TableName, result.Similarity, result.Reason))

			// Add column information for top results
			if i < 5 && len(table.Columns) > 0 {
				prompt.WriteString("   Columns: ")
				var colDescs []string
				for j, col := range table.Columns {
					if j < len(table.ColumnTypes) {
						colDescs = append(colDescs, fmt.Sprintf("%s (%s)", col, table.ColumnTypes[j]))
					} else {
						colDescs = append(colDescs, col)
					}
				}
				prompt.WriteString(strings.Join(colDescs, ", "))
				prompt.WriteString("\n")
			}

			// Add sample data for very relevant tables
			if result.Similarity > 0.8 && table.SampleData != "" {
				prompt.WriteString(fmt.Sprintf("   Sample data: %s\n", table.SampleData))
			}
			prompt.WriteString("\n")
		}

		// Record table access for learning
		var accessedTables []string
		for _, result := range results {
			accessedTables = append(accessedTables, result.Table.TableName)
		}
		m.vectorStore.RecordTableAccess(accessedTables)

		if len(allTables) > len(results) {
			prompt.WriteString(fmt.Sprintf("(%d additional tables available but not shown for brevity)\n\n",
				len(allTables)-len(results)))
		}
	} else {
		prompt.WriteString("No highly relevant tables found for this query. ")
		if len(allTables) <= 10 {
			prompt.WriteString("Available tables:\n")
			for _, table := range allTables {
				prompt.WriteString(fmt.Sprintf("- %s\n", table))
			}
		} else {
			prompt.WriteString("Use the /tables command to see all available tables.\n")
		}
		prompt.WriteString("\n")
	}

	return m.addGuidelines(&prompt)
}

// GenerateSmartSystemPrompt creates optimized system prompt with relevant context (fallback method)
func (m *Manager) GenerateSmartSystemPrompt(userQuery string, allTables []string) string {
	var prompt strings.Builder

	prompt.WriteString("You are an AI assistant helping with SQL queries and database operations. ")

	if len(allTables) == 0 {
		prompt.WriteString("No database connection available.\n\n")
		return m.addGuidelines(&prompt)
	}

	// Extract relevant tables using smart context
	explicitTables := m.extractTableNames(userQuery, allTables)
	relatedTables := m.findRelatedTables(explicitTables, allTables)

	// Combine and prioritize tables
	relevantTables := make(map[string]float64)

	// Explicit mentions get highest priority
	for _, table := range explicitTables {
		relevantTables[table] = 3.0
	}

	// Related tables get medium priority
	for _, table := range relatedTables {
		if _, exists := relevantTables[table]; !exists {
			relevantTables[table] = 2.0
		}
	}

	// Recent tables get low priority
	for _, table := range m.recentTables {
		if _, exists := relevantTables[table]; !exists && m.contains(allTables, table) {
			relevantTables[table] = 1.0
		}
	}

	// If we have few relevant tables, add some common ones
	if len(relevantTables) < 5 {
		for _, table := range allTables {
			if len(relevantTables) >= m.maxTables {
				break
			}
			if _, exists := relevantTables[table]; !exists {
				// Prioritize tables with common names
				if m.isCommonTableName(table) {
					relevantTables[table] = 0.5
				}
			}
		}
	}

	// Sort tables by relevance
	type tableScore struct {
		name  string
		score float64
	}

	var sortedTables []tableScore
	for table, score := range relevantTables {
		sortedTables = append(sortedTables, tableScore{table, score})
	}

	sort.Slice(sortedTables, func(i, j int) bool {
		return sortedTables[i].score > sortedTables[j].score
	})

	// Limit to maxTables
	if len(sortedTables) > m.maxTables {
		sortedTables = sortedTables[:m.maxTables]
	}

	// Add tables mentioned in this query to recent memory
	m.addToRecentTables(explicitTables)

	// Generate context
	if len(sortedTables) > 0 {
		prompt.WriteString(fmt.Sprintf("You have access to a database with %d total tables. ", len(allTables)))
		prompt.WriteString("Most relevant tables for this query:\n\n")

		for _, ts := range sortedTables {
			priority := ""
			switch {
			case ts.score >= 3.0:
				priority = " (mentioned in query)"
			case ts.score >= 2.0:
				priority = " (related table)"
			case ts.score >= 1.0:
				priority = " (recently used)"
			default:
				priority = " (common table)"
			}
			prompt.WriteString(fmt.Sprintf("- %s%s\n", ts.name, priority))
		}

		if len(allTables) > len(sortedTables) {
			prompt.WriteString(fmt.Sprintf("\n(%d additional tables available but not shown for brevity)\n", len(allTables)-len(sortedTables)))
		}
		prompt.WriteString("\n")
	} else {
		prompt.WriteString(fmt.Sprintf("You have access to a database with %d tables. ", len(allTables)))
		if len(allTables) <= 10 {
			prompt.WriteString("Available tables:\n")
			for _, table := range allTables {
				prompt.WriteString(fmt.Sprintf("- %s\n", table))
			}
		} else {
			prompt.WriteString("Use the /tables command to see all available tables.\n")
		}
		prompt.WriteString("\n")
	}

	return m.addGuidelines(&prompt)
}

// isCommonTableName checks if table name suggests common database entities
func (m *Manager) isCommonTableName(tableName string) bool {
	common := []string{"user", "order", "product", "customer", "item", "account", "payment", "transaction", "log", "event"}
	tableLower := strings.ToLower(tableName)

	for _, pattern := range common {
		if strings.Contains(tableLower, pattern) {
			return true
		}
	}

	return false
}

// addGuidelines adds the standard AI guidelines to the prompt
func (m *Manager) addGuidelines(prompt *strings.Builder) string {
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

// StartConversation begins a new multi-turn conversation
func (m *Manager) StartConversation(userQuery string) *ConversationContext {
	m.conversationCtx = NewConversationContext(userQuery)
	return m.conversationCtx
}

// GetCurrentConversation returns the current conversation context
func (m *Manager) GetCurrentConversation() *ConversationContext {
	return m.conversationCtx
}

// ClearConversation clears the current conversation context
func (m *Manager) ClearConversation() {
	m.conversationCtx = nil
}

// ChatWithConversation handles chat with conversation context
func (m *Manager) ChatWithConversation(ctx context.Context, userMessage string, allTables []string) (string, error) {
	if !m.IsConfigured() {
		return "", errors.New(m.i18nMgr.Get("ai_client_not_configured"))
	}

	// Start new conversation if none exists
	if m.conversationCtx == nil {
		m.conversationCtx = NewConversationContext(userMessage)
	}

	// Generate system prompt based on conversation phase
	systemPrompt, err := m.generateConversationalPrompt(m.conversationCtx, allTables)
	if err != nil {
		return "", fmt.Errorf("failed to generate prompt: %w", err)
	}

	// Send chat request
	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}

	request := ChatRequest{
		Model:       m.config.AI.Model,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   4000,
	}

	response, err := m.client.Chat(ctx, request)
	if err != nil {
		return "", fmt.Errorf(m.i18nMgr.Get("chat_request_failed"), err)
	}

	if len(response.Choices) == 0 {
		return "", errors.New(m.i18nMgr.Get("no_response_choices_returned"))
	}

	aiResponse := response.Choices[0].Message.Content

	// Parse AI response for requested tables/actions
	requestedInfo := m.parseAIResponse(aiResponse, m.conversationCtx.CurrentPhase)

	// Add turn to conversation history
	turn := ConversationTurn{
		UserMessage:   userMessage,
		SystemPrompt:  systemPrompt,
		AIResponse:    aiResponse,
		RequestedInfo: requestedInfo,
		Phase:         m.conversationCtx.CurrentPhase,
	}
	m.conversationCtx.AddTurn(turn)

	// Process AI's requests and advance conversation if needed
	initialPhase := m.conversationCtx.CurrentPhase
	err = m.processConversationTurn(requestedInfo, allTables)
	if err != nil {
		fmt.Printf(m.i18nMgr.Get("conversation_turn_warning"), err)
	}

	// Calculate cost and update usage
	cost := m.calculateCost(response.Usage.PromptTokens, response.Usage.CompletionTokens)

	// Add to prompt history
	m.addToPromptHistory(userMessage, systemPrompt, aiResponse, response.Usage.PromptTokens, response.Usage.CompletionTokens, cost)

	// If schemas were loaded, automatically continue the conversation
	if len(requestedInfo) > 0 {
		// Discovery phase: advance to schema analysis
		if m.conversationCtx.CurrentPhase != initialPhase && m.conversationCtx.CurrentPhase == PhaseSchemaAnalysis {
			fmt.Printf("📋 Schemas loaded for %v. Analyzing...\n", requestedInfo)
			followUpMessage := "Please analyze the provided table schemas and generate the SQL query for my original request."

			// Make follow-up call with schema information
			followUpResponse, err := m.ChatWithConversation(ctx, followUpMessage, allTables)
			if err != nil {
				fmt.Printf(m.i18nMgr.Get("conversation_continue_warning"), err)
				return aiResponse, nil
			}
			return followUpResponse, nil
		}

		// Schema analysis phase: continue with additional schema requests
		if m.conversationCtx.CurrentPhase == PhaseSchemaAnalysis {
			fmt.Printf("📋 Additional schemas loaded for %v. Continuing analysis...\n", requestedInfo)
			followUpMessage := "Please continue your analysis with the newly provided table schemas."

			// Make follow-up call with additional schema information
			followUpResponse, err := m.ChatWithConversation(ctx, followUpMessage, allTables)
			if err != nil {
				fmt.Printf(m.i18nMgr.Get("conversation_continue_additional_warning"), err)
				return aiResponse, nil
			}
			return followUpResponse, nil
		}
	}

	return aiResponse, nil
}

// generateConversationalPrompt creates phase-specific prompts
func (m *Manager) generateConversationalPrompt(convCtx *ConversationContext, allTables []string) (string, error) {
	switch convCtx.CurrentPhase {
	case PhaseDiscovery:
		return m.generateDiscoveryPrompt(convCtx, allTables), nil
	case PhaseSchemaAnalysis:
		return m.generateSchemaAnalysisPrompt(convCtx), nil
	case PhaseSQLGeneration:
		return m.generateSQLGenerationPrompt(convCtx), nil
	default:
		return "", fmt.Errorf("unknown conversation phase: %v", convCtx.CurrentPhase)
	}
}

// generateDiscoveryPrompt creates prompt for table discovery phase
func (m *Manager) generateDiscoveryPrompt(convCtx *ConversationContext, allTables []string) string {
	var prompt strings.Builder

	prompt.WriteString("You are an AI assistant helping with SQL queries and database operations. ")
	prompt.WriteString(fmt.Sprintf("The user wants to: %s\n\n", convCtx.OriginalQuery))

	// Use vector search to find most relevant tables
	if m.vectorStore != nil && len(allTables) > 0 {
		ctx := context.Background()
		results, err := m.vectorStore.SearchSimilarTables(ctx, convCtx.OriginalQuery, 10)
		if err == nil && len(results) > 0 {
			prompt.WriteString(fmt.Sprintf("Database has %d tables total. Most relevant tables for this query:\n\n", len(allTables)))
			for i, result := range results {
				prompt.WriteString(fmt.Sprintf("%d. **%s** (relevance: %.2f) - %s\n",
					i+1, result.Table.TableName, result.Similarity, result.Reason))
			}

			// Store discovered tables in context
			for _, result := range results {
				convCtx.DiscoveredTables = append(convCtx.DiscoveredTables, result.Table.TableName)
			}
		} else {
			// Fallback to simple list
			prompt.WriteString(fmt.Sprintf("Available tables (%d total):\n", len(allTables)))
			for i, table := range allTables {
				if i >= 15 { // Limit to first 15
					prompt.WriteString(fmt.Sprintf("... and %d more tables\n", len(allTables)-15))
					break
				}
				prompt.WriteString(fmt.Sprintf("- %s\n", table))
			}
		}
	} else {
		prompt.WriteString("No database connection available.\n")
	}

	prompt.WriteString("\nYour task:\n")
	prompt.WriteString("1. Analyze the user's request and identify which tables you need detailed schema information for\n")
	prompt.WriteString("2. Respond with: 'I need detailed schema for: [table1], [table2], [table3]' to request specific table structures\n")
	prompt.WriteString("3. Be selective - only request tables that are directly relevant to the query\n")
	prompt.WriteString("4. If you can answer with the information already provided, do so\n\n")

	prompt.WriteString("Important: If you need table schemas, use EXACTLY this format:\n")
	prompt.WriteString("'I need detailed schema for: table1, table2, table3'\n")

	return prompt.String()
}

// generateSchemaAnalysisPrompt creates prompt for schema analysis phase
func (m *Manager) generateSchemaAnalysisPrompt(convCtx *ConversationContext) string {
	var prompt strings.Builder

	prompt.WriteString("You are an AI assistant helping with SQL queries. ")
	prompt.WriteString(fmt.Sprintf("The user wants to: %s\n\n", convCtx.OriginalQuery))

	prompt.WriteString("You have requested detailed schema information. Here are the table structures:\n\n")

	// Include loaded table schemas
	for tableName, tableInfo := range convCtx.LoadedTables {
		prompt.WriteString(fmt.Sprintf("## Table: %s\n", tableName))
		prompt.WriteString("Columns:\n")
		for _, col := range tableInfo.Columns {
			nullable := "NOT NULL"
			if col.Nullable {
				nullable = "NULL"
			}
			key := ""
			if col.Key != "" {
				key = fmt.Sprintf(" [%s]", col.Key)
			}
			prompt.WriteString(fmt.Sprintf("- %s (%s) %s%s\n", col.Name, col.Type, nullable, key))
		}

		// Include foreign key relationships
		if len(tableInfo.ForeignKeys) > 0 {
			prompt.WriteString("Foreign Keys:\n")
			for _, fk := range tableInfo.ForeignKeys {
				prompt.WriteString(fmt.Sprintf("- %s → %s.%s\n", fk.Column, fk.ReferencedTable, fk.ReferencedColumn))
			}
		}
		prompt.WriteString("\n")
	}

	// Add information about available related tables
	m.addRelatedTableSuggestions(&prompt, convCtx)

	prompt.WriteString("Your task:\n")
	prompt.WriteString("1. Analyze the provided schemas and relationships\n")
	prompt.WriteString("2. If you need information about related tables (via foreign keys), request them using: 'I need schema for related tables: [table1], [table2]'\n")
	prompt.WriteString("3. If you have sufficient information, generate the SQL query\n")
	prompt.WriteString("4. Include explanations for complex queries\n\n")

	prompt.WriteString("Use ```sql blocks for any SQL queries you generate.\n")

	return prompt.String()
}

// generateSQLGenerationPrompt creates prompt for final SQL generation
func (m *Manager) generateSQLGenerationPrompt(convCtx *ConversationContext) string {
	var prompt strings.Builder

	prompt.WriteString("You are an AI assistant specialized in SQL query generation. ")
	prompt.WriteString(fmt.Sprintf("The user wants to: %s\n\n", convCtx.OriginalQuery))

	prompt.WriteString("You have complete schema information for the following tables:\n\n")

	// Include all loaded table information
	for tableName, tableInfo := range convCtx.LoadedTables {
		prompt.WriteString(fmt.Sprintf("## %s\n", tableName))
		for _, col := range tableInfo.Columns {
			nullable := "NOT NULL"
			if col.Nullable {
				nullable = "NULL"
			}
			prompt.WriteString(fmt.Sprintf("- %s (%s) %s\n", col.Name, col.Type, nullable))
		}

		if len(tableInfo.ForeignKeys) > 0 {
			prompt.WriteString("Relationships:\n")
			for _, fk := range tableInfo.ForeignKeys {
				prompt.WriteString(fmt.Sprintf("- %s → %s.%s\n", fk.Column, fk.ReferencedTable, fk.ReferencedColumn))
			}
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString("Generate the complete SQL query to fulfill the user's request.\n")
	prompt.WriteString("Include:\n")
	prompt.WriteString("- Proper JOINs based on foreign key relationships\n")
	prompt.WriteString("- Appropriate WHERE clauses and conditions\n")
	prompt.WriteString("- Comments explaining complex parts\n")
	prompt.WriteString("- Performance optimization suggestions if relevant\n\n")

	prompt.WriteString("Use ```sql blocks for your query.\n")

	return prompt.String()
}

// parseAIResponse extracts requested information from AI response
func (m *Manager) parseAIResponse(response string, phase ConversationPhase) []string {
	var requested []string

	switch phase {
	case PhaseDiscovery, PhaseSchemaAnalysis:
		// Look for table requests in various formats
		patterns := []string{
			`I need detailed schema for:\s*([^.]+)`,
			`I need schema for related tables:\s*([^.]+)`,
			`Please provide schema for:\s*([^.]+)`,
			`Need table structure for:\s*([^.]+)`,
		}

		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			if matches := re.FindStringSubmatch(response); len(matches) > 1 {
				// Parse comma-separated table names
				tableNames := strings.Split(matches[1], ",")
				for _, name := range tableNames {
					name = strings.TrimSpace(name)
					if name != "" {
						requested = append(requested, name)
					}
				}
				break
			}
		}
	}

	return requested
}

// processConversationTurn handles the AI's requests and advances conversation
func (m *Manager) processConversationTurn(requestedInfo []string, allTables []string) error {
	if len(requestedInfo) == 0 {
		// No specific requests, advance phase if appropriate
		if m.conversationCtx.CurrentPhase == PhaseDiscovery && len(m.conversationCtx.LoadedTables) > 0 {
			m.conversationCtx.AdvancePhase()
		}
		return nil
	}

	// Process table schema requests
	if m.vectorStore != nil && m.vectorStore.connection != nil {
		for _, tableName := range requestedInfo {
			// Verify table exists
			if !m.contains(allTables, tableName) {
				continue
			}

			// Skip if already loaded
			if m.conversationCtx.HasTableLoaded(tableName) {
				continue
			}

			// Load table schema
			tableInfo, err := m.vectorStore.connection.DescribeTable(tableName)
			if err != nil {
				fmt.Printf("Warning: failed to describe table %s: %v\n", tableName, err)
				continue
			}

			// Add to conversation context
			m.conversationCtx.AddLoadedTable(tableName, tableInfo)
			m.conversationCtx.RequestedTables = append(m.conversationCtx.RequestedTables, tableName)

			// Find related tables via foreign keys
			for _, fk := range tableInfo.ForeignKeys {
				if !m.contains(m.conversationCtx.RelatedTables, fk.ReferencedTable) {
					m.conversationCtx.RelatedTables = append(m.conversationCtx.RelatedTables, fk.ReferencedTable)
				}
			}
		}

		// Advance phase if we have loaded tables
		if len(m.conversationCtx.LoadedTables) > 0 && m.conversationCtx.CurrentPhase == PhaseDiscovery {
			m.conversationCtx.AdvancePhase()
		}
	}

	return nil
}

// addRelatedTableSuggestions adds information about available related tables to prompt
func (m *Manager) addRelatedTableSuggestions(prompt *strings.Builder, convCtx *ConversationContext) {
	if m.vectorStore == nil {
		return
	}

	// Get current loaded table names
	var loadedTableNames []string
	for tableName := range convCtx.LoadedTables {
		loadedTableNames = append(loadedTableNames, tableName)
	}

	if len(loadedTableNames) == 0 {
		return
	}

	// Find related tables using enhanced relationship discovery
	ctx := context.Background()
	relatedTables, err := m.vectorStore.SearchRelatedTablesForQuery(ctx, loadedTableNames, 5)
	if err != nil || len(relatedTables) == 0 {
		return
	}

	// Also get detailed relationship mapping
	relationships, err := m.vectorStore.FindRelatedTables(loadedTableNames)
	if err == nil && len(relationships) > 0 {
		prompt.WriteString("## Available Related Tables\n\n")
		prompt.WriteString("The following tables are related to your loaded tables and might be useful:\n\n")

		// Show direct relationships first
		for sourceTable, related := range relationships {
			if len(related) > 0 {
				prompt.WriteString(fmt.Sprintf("**%s** is related to:\n", sourceTable))
				for _, relTable := range related {
					// Check if this is a foreign key relationship
					if tableInfo, exists := convCtx.LoadedTables[sourceTable]; exists {
						for _, fk := range tableInfo.ForeignKeys {
							if fk.ReferencedTable == relTable {
								prompt.WriteString(fmt.Sprintf("- %s (via foreign key %s)\n", relTable, fk.Column))
								goto nextTable
							}
						}
					}
					prompt.WriteString(fmt.Sprintf("- %s (similar naming pattern)\n", relTable))
				nextTable:
				}
				prompt.WriteString("\n")
			}
		}

		// Add additional suggestions from similarity search
		additionalTables := make(map[string]bool)
		for _, table := range relatedTables {
			alreadyShown := false
			for _, related := range relationships {
				if m.contains(related, table) {
					alreadyShown = true
					break
				}
			}
			if !alreadyShown && !convCtx.HasTableLoaded(table) {
				additionalTables[table] = true
			}
		}

		if len(additionalTables) > 0 {
			prompt.WriteString("**Additional potentially relevant tables:**\n")
			for table := range additionalTables {
				prompt.WriteString(fmt.Sprintf("- %s\n", table))
			}
			prompt.WriteString("\n")
		}

		prompt.WriteString("💡 You can request any of these tables by saying: 'I need schema for related tables: table1, table2'\n\n")
	}
}

// GetUsageStore returns the usage store for accessing usage statistics
func (m *Manager) GetUsageStore() *UsageStore {
	return m.usageStore
}

// GetSessionID returns the current session ID
func (m *Manager) GetSessionID() string {
	return m.sessionID
}

func (m *Manager) generateSessionID() string {
	return fmt.Sprintf("session_%s", m.idGen.GenerateString())
}

// NewSession starts a new session with a new session ID
func (m *Manager) NewSession() {
	m.sessionID = m.generateSessionID()
}
