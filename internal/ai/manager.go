package ai

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"sqlterm/internal/core"
)

// Manager manages AI clients and configuration
type Manager struct {
	config        *Config
	configDir     string
	client        Client
	promptHistory *PromptHistory
	recentTables  []string     // Session memory for recently mentioned tables
	maxTables     int          // Maximum tables to include in context
	vectorStore   *VectorStore // Vector database for semantic search
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
		recentTables: make([]string, 0),
		maxTables:    15, // Limit context to 15 most relevant tables
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
		recentTables: make([]string, 0),
		maxTables:    15, // Limit context to 15 most relevant tables
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
