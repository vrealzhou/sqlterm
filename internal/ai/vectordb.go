package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"sqlterm/internal/core"

	_ "github.com/mattn/go-sqlite3"
)

// VectorStore manages vector embeddings for database schema information
type VectorStore struct {
	db             *sql.DB
	connection     core.Connection
	configDir      string
	connectionName string
}

// TableEmbedding represents a table with its vector embeddings
type TableEmbedding struct {
	ID           int       `json:"id"`
	TableName    string    `json:"table_name"`
	Description  string    `json:"description"`
	Columns      []string  `json:"columns"`
	ColumnTypes  []string  `json:"column_types"`
	SampleData   string    `json:"sample_data"`
	Embedding    []float64 `json:"embedding"`
	LastUpdated  time.Time `json:"last_updated"`
	AccessCount  int       `json:"access_count"`
	LastAccessed time.Time `json:"last_accessed"`
}

// QueryPattern represents learned query patterns
type QueryPattern struct {
	ID          int       `json:"id"`
	QueryText   string    `json:"query_text"`
	Tables      []string  `json:"tables"`
	Embedding   []float64 `json:"embedding"`
	SuccessRate float64   `json:"success_rate"`
	UseCount    int       `json:"use_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// VectorSearchResult represents a search result with similarity score
type VectorSearchResult struct {
	Table      TableEmbedding `json:"table"`
	Similarity float64        `json:"similarity"`
	Reason     string         `json:"reason"`
}

// NewVectorStore creates a new vector store for a database connection
func NewVectorStore(configDir, connectionName string, connection core.Connection) (*VectorStore, error) {
	// Create session directory for this connection
	sessionDir := fmt.Sprintf("%s/sessions/%s", configDir, connectionName)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// New vector database path in session folder
	dbPath := fmt.Sprintf("%s/vectors.db", sessionDir)

	// Check for legacy vector database and migrate if exists
	if err := migrateLegacyVectorDB(configDir, connectionName, dbPath); err != nil {
		fmt.Printf("Warning: failed to migrate legacy vector database: %v\n", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open vector database: %w", err)
	}

	store := &VectorStore{
		db:             db,
		connection:     connection,
		configDir:      configDir,
		connectionName: connectionName,
	}

	if err := store.initializeSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize vector store schema: %w", err)
	}

	return store, nil
}

// migrateLegacyVectorDB moves old vector databases to new session folder structure
func migrateLegacyVectorDB(configDir, connectionName, newPath string) error {
	// Old vector database path
	oldPath := fmt.Sprintf("%s/vectors_%s.db", configDir, connectionName)

	// Check if old database exists
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return nil // No old database to migrate
	}

	// Check if new database already exists
	if _, err := os.Stat(newPath); err == nil {
		// New database exists, remove old one
		return os.Remove(oldPath)
	}

	// Move old database to new location
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("failed to move vector database from %s to %s: %w", oldPath, newPath, err)
	}

	fmt.Printf("📦 Migrated vector database for %s to session folder\n", connectionName)
	return nil
}

// initializeSchema creates the necessary tables for vector storage
func (vs *VectorStore) initializeSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS table_embeddings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			table_name TEXT UNIQUE NOT NULL,
			description TEXT,
			columns TEXT, -- JSON array of column names
			column_types TEXT, -- JSON array of column types
			sample_data TEXT,
			embedding TEXT, -- JSON array of float64 values
			last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
			access_count INTEGER DEFAULT 0,
			last_accessed DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS query_patterns (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query_text TEXT NOT NULL,
			tables TEXT, -- JSON array of table names used
			embedding TEXT, -- JSON array of float64 values
			success_rate REAL DEFAULT 1.0,
			use_count INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE INDEX IF NOT EXISTS idx_table_name ON table_embeddings(table_name)`,
		`CREATE INDEX IF NOT EXISTS idx_last_accessed ON table_embeddings(last_accessed DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_access_count ON table_embeddings(access_count DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_query_patterns_updated ON query_patterns(updated_at DESC)`,
	}

	for _, query := range queries {
		if _, err := vs.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

	return nil
}

// UpdateTableEmbeddings refreshes embeddings for all tables
func (vs *VectorStore) UpdateTableEmbeddings(ctx context.Context) error {
	tables, err := vs.connection.ListTables()
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}

	for _, tableName := range tables {
		if err := vs.updateTableEmbedding(ctx, tableName); err != nil {
			// Log error but continue with other tables
			fmt.Printf("Warning: failed to update embedding for table %s: %v\n", tableName, err)
		}
	}

	return nil
}

// updateTableEmbedding creates or updates embedding for a single table
func (vs *VectorStore) updateTableEmbedding(ctx context.Context, tableName string) error {
	// Get table schema information
	tableInfo, err := vs.connection.DescribeTable(tableName)
	if err != nil {
		return fmt.Errorf("failed to describe table %s: %w", tableName, err)
	}

	// Build description for embedding
	var descParts []string
	descParts = append(descParts, fmt.Sprintf("Table: %s", tableName))

	var columns []string
	var columnTypes []string
	for _, col := range tableInfo.Columns {
		columns = append(columns, col.Name)
		columnTypes = append(columnTypes, col.Type)
		descParts = append(descParts, fmt.Sprintf("Column: %s (%s)", col.Name, col.Type))
	}

	// Add foreign key relationships
	for _, fk := range tableInfo.ForeignKeys {
		descParts = append(descParts, fmt.Sprintf("Foreign key: %s references %s.%s",
			fk.Column, fk.ReferencedTable, fk.ReferencedColumn))
	}

	description := strings.Join(descParts, ". ")

	// Get sample data (first few rows)
	sampleData, err := vs.getSampleData(tableName)
	if err != nil {
		// Don't fail if we can't get sample data
		sampleData = ""
	}

	// Generate embedding (placeholder - will implement actual embedding generation)
	embedding := vs.generateEmbedding(description)

	// Store in database
	columnsJSON, _ := json.Marshal(columns)
	columnTypesJSON, _ := json.Marshal(columnTypes)
	embeddingJSON, _ := json.Marshal(embedding)

	query := `INSERT OR REPLACE INTO table_embeddings
		(table_name, description, columns, column_types, sample_data, embedding, last_updated)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err = vs.db.Exec(query, tableName, description, string(columnsJSON),
		string(columnTypesJSON), sampleData, string(embeddingJSON), time.Now())

	return err
}

// getSampleData retrieves a few sample rows from the table
func (vs *VectorStore) getSampleData(tableName string) (string, error) {
	query := fmt.Sprintf("SELECT * FROM %s LIMIT 3", tableName)
	result, err := vs.connection.Execute(query)
	if err != nil {
		return "", err
	}
	defer result.Close()

	var samples []string
	count := 0
	for row := range result.Itor() {
		if count >= 3 {
			break
		}

		var rowStrings []string
		for _, val := range row {
			rowStrings = append(rowStrings, val.String())
		}
		samples = append(samples, strings.Join(rowStrings, ", "))
		count++
	}

	if result.Error() != nil {
		return "", result.Error()
	}

	return strings.Join(samples, "; "), nil
}

// generateEmbedding creates a simple embedding for text (placeholder implementation)
// In a real implementation, this would use an embedding model like OpenAI's text-embedding-ada-002
func (vs *VectorStore) generateEmbedding(text string) []float64 {
	// This is a very simple bag-of-words style embedding for demonstration
	// In production, you'd use a proper embedding model

	words := strings.Fields(strings.ToLower(text))
	wordFreq := make(map[string]int)

	for _, word := range words {
		wordFreq[word]++
	}

	// Create a simple 384-dimensional vector (common size for sentence transformers)
	embedding := make([]float64, 384)

	// Use hash-based approach to map words to dimensions
	for word, freq := range wordFreq {
		hash := vs.simpleHash(word)
		for i := range 5 { // Use multiple dimensions per word
			idx := (hash + i) % 384
			embedding[idx] += float64(freq) / float64(len(words))
		}
	}

	// Normalize the vector
	return vs.normalizeVector(embedding)
}

// simpleHash creates a simple hash for string mapping
func (vs *VectorStore) simpleHash(s string) int {
	hash := 0
	for _, c := range s {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

// normalizeVector normalizes a vector to unit length
func (vs *VectorStore) normalizeVector(vec []float64) []float64 {
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)

	if norm == 0 {
		return vec
	}

	normalized := make([]float64, len(vec))
	for i, v := range vec {
		normalized[i] = v / norm
	}

	return normalized
}

// cosineSimilarity calculates cosine similarity between two vectors
func (vs *VectorStore) cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// SearchSimilarTables finds tables most similar to a query
func (vs *VectorStore) SearchSimilarTables(ctx context.Context, queryText string, limit int) ([]VectorSearchResult, error) {
	queryEmbedding := vs.generateEmbedding(queryText)

	query := `SELECT table_name, description, columns, column_types, sample_data, embedding, access_count, last_accessed
		FROM table_embeddings ORDER BY access_count DESC`

	rows, err := vs.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query embeddings: %w", err)
	}
	defer rows.Close()

	var results []VectorSearchResult

	for rows.Next() {
		var te TableEmbedding
		var embeddingJSON string
		var columnsJSON, columnTypesJSON string
		var lastAccessed sql.NullTime

		err := rows.Scan(&te.TableName, &te.Description, &columnsJSON, &columnTypesJSON,
			&te.SampleData, &embeddingJSON, &te.AccessCount, &lastAccessed)
		if err != nil {
			continue
		}

		if lastAccessed.Valid {
			te.LastAccessed = lastAccessed.Time
		}

		// Parse JSON data
		json.Unmarshal([]byte(columnsJSON), &te.Columns)
		json.Unmarshal([]byte(columnTypesJSON), &te.ColumnTypes)
		json.Unmarshal([]byte(embeddingJSON), &te.Embedding)

		// Calculate similarity
		similarity := vs.cosineSimilarity(queryEmbedding, te.Embedding)

		// Determine reason for inclusion
		reason := vs.determineRelevanceReason(queryText, te, similarity)

		results = append(results, VectorSearchResult{
			Table:      te,
			Similarity: similarity,
			Reason:     reason,
		})
	}

	// Sort by similarity (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Apply limit
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// determineRelevanceReason explains why a table was included in results
func (vs *VectorStore) determineRelevanceReason(queryText string, table TableEmbedding, similarity float64) string {
	queryLower := strings.ToLower(queryText)

	// Check for exact table name match
	if strings.Contains(queryLower, strings.ToLower(table.TableName)) {
		return "table name mentioned in query"
	}

	// Check for column name matches
	for _, col := range table.Columns {
		if strings.Contains(queryLower, strings.ToLower(col)) {
			return fmt.Sprintf("column '%s' mentioned in query", col)
		}
	}

	// Check access frequency
	if table.AccessCount > 10 {
		return "frequently accessed table"
	}

	// Check recent usage
	if time.Since(table.LastAccessed) < 24*time.Hour {
		return "recently used table"
	}

	// Semantic similarity
	if similarity > 0.7 {
		return "high semantic similarity"
	} else if similarity > 0.5 {
		return "moderate semantic similarity"
	}

	return "low semantic similarity"
}

// RecordTableAccess updates access statistics for tables
func (vs *VectorStore) RecordTableAccess(tableNames []string) error {
	for _, tableName := range tableNames {
		query := `UPDATE table_embeddings
			SET access_count = access_count + 1, last_accessed = ?
			WHERE table_name = ?`

		_, err := vs.db.Exec(query, time.Now(), tableName)
		if err != nil {
			return fmt.Errorf("failed to update access count for table %s: %w", tableName, err)
		}
	}
	return nil
}

// AddQueryPattern stores a successful query pattern for learning
func (vs *VectorStore) AddQueryPattern(queryText string, usedTables []string) error {
	embedding := vs.generateEmbedding(queryText)
	embeddingJSON, _ := json.Marshal(embedding)
	tablesJSON, _ := json.Marshal(usedTables)

	query := `INSERT INTO query_patterns (query_text, tables, embedding, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`

	now := time.Now()
	_, err := vs.db.Exec(query, queryText, string(tablesJSON), string(embeddingJSON), now, now)

	return err
}

// GetRecentTables returns recently accessed tables
func (vs *VectorStore) GetRecentTables(limit int) ([]string, error) {
	query := `SELECT table_name FROM table_embeddings
		WHERE last_accessed IS NOT NULL
		ORDER BY last_accessed DESC LIMIT ?`

	rows, err := vs.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}

// FindRelatedTables discovers tables related to the given tables via foreign key relationships
func (vs *VectorStore) FindRelatedTables(baseTableNames []string) (map[string][]string, error) {
	relationships := make(map[string][]string)

	for _, tableName := range baseTableNames {
		// Get table schema to analyze foreign keys
		tableInfo, err := vs.connection.DescribeTable(tableName)
		if err != nil {
			continue // Skip tables we can't describe
		}

		var relatedTables []string

		// Find tables this table references (via foreign keys)
		for _, fk := range tableInfo.ForeignKeys {
			if !vs.contains(relatedTables, fk.ReferencedTable) {
				relatedTables = append(relatedTables, fk.ReferencedTable)
			}
		}

		// Find tables that reference this table (reverse foreign keys)
		reverseFKTables, err := vs.findTablesReferencingTable(tableName)
		if err == nil {
			for _, refTable := range reverseFKTables {
				if !vs.contains(relatedTables, refTable) && !vs.contains(baseTableNames, refTable) {
					relatedTables = append(relatedTables, refTable)
				}
			}
		}

		// Find tables with similar naming patterns
		similarTables := vs.findSimilarlyNamedTables(tableName, baseTableNames)
		for _, simTable := range similarTables {
			if !vs.contains(relatedTables, simTable) {
				relatedTables = append(relatedTables, simTable)
			}
		}

		if len(relatedTables) > 0 {
			relationships[tableName] = relatedTables
		}
	}

	return relationships, nil
}

// findTablesReferencingTable finds tables that have foreign keys pointing to the given table
func (vs *VectorStore) findTablesReferencingTable(targetTableName string) ([]string, error) {
	var referencingTables []string

	// Get all tables to check their foreign keys
	allTables, err := vs.connection.ListTables()
	if err != nil {
		return nil, err
	}

	for _, tableName := range allTables {
		if tableName == targetTableName {
			continue // Skip the target table itself
		}

		tableInfo, err := vs.connection.DescribeTable(tableName)
		if err != nil {
			continue // Skip tables we can't describe
		}

		// Check if this table has foreign keys pointing to target table
		for _, fk := range tableInfo.ForeignKeys {
			if fk.ReferencedTable == targetTableName {
				referencingTables = append(referencingTables, tableName)
				break // Found one FK, that's enough
			}
		}
	}

	return referencingTables, nil
}

// findSimilarlyNamedTables finds tables with similar naming patterns
func (vs *VectorStore) findSimilarlyNamedTables(targetTableName string, excludeTables []string) []string {
	var similarTables []string

	allTables, err := vs.connection.ListTables()
	if err != nil {
		return similarTables
	}

	// Get base name patterns
	targetPrefix := vs.extractTablePrefix(targetTableName)
	targetBase := vs.extractTableBaseWord(targetTableName)

	for _, tableName := range allTables {
		if tableName == targetTableName || vs.contains(excludeTables, tableName) {
			continue
		}

		// Check for similar prefixes
		if targetPrefix != "" && vs.extractTablePrefix(tableName) == targetPrefix {
			similarTables = append(similarTables, tableName)
			continue
		}

		// Check for similar base words
		if targetBase != "" && vs.extractTableBaseWord(tableName) == targetBase {
			similarTables = append(similarTables, tableName)
			continue
		}

		// Check for common junction table patterns (e.g., user_roles, order_items)
		if vs.isJunctionTable(tableName, targetTableName) {
			similarTables = append(similarTables, tableName)
		}
	}

	return similarTables
}

// extractTablePrefix extracts prefix from table names (e.g., "user_" from "user_profiles")
func (vs *VectorStore) extractTablePrefix(tableName string) string {
	parts := strings.Split(tableName, "_")
	if len(parts) > 1 {
		return parts[0]
	}
	return ""
}

// extractTableBaseWord extracts the main concept from table name
func (vs *VectorStore) extractTableBaseWord(tableName string) string {
	// Remove common suffixes
	name := strings.ToLower(tableName)
	suffixes := []string{"s", "es", "ies", "_table", "_data", "_info"}
	
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			name = strings.TrimSuffix(name, suffix)
			break
		}
	}

	// Extract base word before underscore
	parts := strings.Split(name, "_")
	if len(parts) > 0 {
		return parts[0]
	}

	return name
}

// isJunctionTable checks if a table might be a junction/bridge table between two entities
func (vs *VectorStore) isJunctionTable(candidateTable, targetTable string) bool {
	candidate := strings.ToLower(candidateTable)
	target := strings.ToLower(targetTable)

	// Remove plural suffixes for better matching
	targetBase := vs.extractTableBaseWord(target)
	if targetBase == "" {
		targetBase = target
	}

	// Check if candidate table contains the target table name
	if strings.Contains(candidate, targetBase) || strings.Contains(candidate, target) {
		// Look for junction table patterns
		junctionPatterns := []string{"_", "2", "to", "has", "belongs"}
		for _, pattern := range junctionPatterns {
			if strings.Contains(candidate, pattern) {
				return true
			}
		}
	}

	return false
}

// GetTableRelationshipMap builds a comprehensive map of table relationships
func (vs *VectorStore) GetTableRelationshipMap() (map[string][]string, error) {
	allTables, err := vs.connection.ListTables()
	if err != nil {
		return nil, err
	}

	relationshipMap := make(map[string][]string)

	// Build relationships for all tables
	for _, tableName := range allTables {
		var relatedTables []string

		tableInfo, err := vs.connection.DescribeTable(tableName)
		if err != nil {
			continue
		}

		// Add directly referenced tables
		for _, fk := range tableInfo.ForeignKeys {
			if !vs.contains(relatedTables, fk.ReferencedTable) {
				relatedTables = append(relatedTables, fk.ReferencedTable)
			}
		}

		// Add tables that reference this table
		referencingTables, err := vs.findTablesReferencingTable(tableName)
		if err == nil {
			for _, refTable := range referencingTables {
				if !vs.contains(relatedTables, refTable) {
					relatedTables = append(relatedTables, refTable)
				}
			}
		}

		if len(relatedTables) > 0 {
			relationshipMap[tableName] = relatedTables
		}
	}

	return relationshipMap, nil
}

// SearchRelatedTablesForQuery finds tables related to a specific query context
func (vs *VectorStore) SearchRelatedTablesForQuery(ctx context.Context, initialTables []string, maxRelated int) ([]string, error) {
	var allRelatedTables []string
	processed := make(map[string]bool)

	// Mark initial tables as processed
	for _, table := range initialTables {
		processed[table] = true
	}

	// Find relationships for initial tables
	relationships, err := vs.FindRelatedTables(initialTables)
	if err != nil {
		return nil, err
	}

	// Collect all related tables with scoring
	type scoredTable struct {
		name  string
		score int
	}

	tableScores := make(map[string]int)

	for sourceTable, relatedTables := range relationships {
		for _, relatedTable := range relatedTables {
			if !processed[relatedTable] {
				// Score based on relationship type and frequency
				score := 1
				
				// Higher score for tables referenced by multiple source tables
				if existing, exists := tableScores[relatedTable]; exists {
					score = existing + 2 // Bonus for multiple references
				}

				tableScores[relatedTable] = score
			}
		}
	}

	// Sort by score and select top related tables
	var scoredTables []scoredTable
	for table, score := range tableScores {
		scoredTables = append(scoredTables, scoredTable{table, score})
	}

	sort.Slice(scoredTables, func(i, j int) bool {
		return scoredTables[i].score > scoredTables[j].score
	})

	// Limit results
	limit := maxRelated
	if len(scoredTables) < limit {
		limit = len(scoredTables)
	}

	for i := 0; i < limit; i++ {
		allRelatedTables = append(allRelatedTables, scoredTables[i].name)
	}

	return allRelatedTables, nil
}

// contains checks if a slice contains a string (helper method)
func (vs *VectorStore) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Close closes the vector store database connection
func (vs *VectorStore) Close() error {
	if vs.db != nil {
		return vs.db.Close()
	}
	return nil
}
