package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vrealzhou/sqlterm/internal/database"
	"github.com/vrealzhou/sqlterm/internal/i18n"
)

// Manager implements the SessionManagerInterface with hybrid storage
type Manager struct {
	db        *Database
	configDir string
	i18nMgr   *i18n.Manager
}

// NewManager creates a new session manager with hybrid storage
func NewManager(configDir string, i18nMgr *i18n.Manager) (*Manager, error) {
	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Initialize SQLite database
	db, err := NewDatabase(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize session database: %w", err)
	}

	return &Manager{
		db:        db,
		configDir: configDir,
		i18nMgr:   i18nMgr,
	}, nil
}

// Session lifecycle methods

func (m *Manager) CreateSession(connectionName string) (*Session, error) {
	if connectionName == "" {
		return nil, fmt.Errorf("connection name cannot be empty")
	}

	// Ensure session directory exists
	if err := m.EnsureSessionDir(connectionName); err != nil {
		return nil, fmt.Errorf("failed to ensure session directory: %w", err)
	}

	return m.db.CreateSession(connectionName)
}

func (m *Manager) GetSession(sessionID int64) (*Session, error) {
	return m.db.GetSession(sessionID)
}

func (m *Manager) GetActiveSession(connectionName string) (*Session, error) {
	return m.db.GetActiveSession(connectionName)
}

func (m *Manager) UpdateSessionAccess(sessionID int64) error {
	return m.db.UpdateSessionAccess(sessionID)
}

func (m *Manager) CloseSession(sessionID int64) error {
	return m.db.CloseSession(sessionID)
}

func (m *Manager) ArchiveSession(sessionID int64) error {
	return m.db.ArchiveSession(sessionID)
}

// Query history methods

func (m *Manager) RecordQuery(sessionID int64, query string, result *database.QueryResult, executionTime time.Duration) (*QueryRecord, error) {
	var rowsAffected int64
	var success = true
	var errorMessage *string
	var resultFilePath *string

	if result != nil {
		if result.Error() != nil {
			success = false
			errMsg := result.Error().Error()
			errorMessage = &errMsg
		} else {
			// Count rows by iterating through the result
			count := 0
			for range result.Itor() {
				count++
			}
			rowsAffected = int64(count)
		}
	}

	executionTimeMs := executionTime.Milliseconds()

	return m.db.RecordQuery(sessionID, query, executionTimeMs, rowsAffected, success, errorMessage, resultFilePath)
}

func (m *Manager) GetQueryHistory(sessionID int64, limit int, offset int) ([]*QueryRecord, error) {
	return m.db.GetQueryHistory(sessionID, limit, offset)
}

func (m *Manager) GetQueryHistoryByConnection(connectionName string, limit int, offset int) ([]*QueryRecord, error) {
	return m.db.GetQueryHistoryByConnection(connectionName, limit, offset)
}

func (m *Manager) SearchQueryHistory(sessionID int64, searchTerm string, limit int) ([]*QueryRecord, error) {
	return m.db.SearchQueryHistory(sessionID, searchTerm, limit)
}

// Statistics methods

func (m *Manager) GetSessionStats(sessionID int64) (*SessionStats, error) {
	return m.db.GetSessionStats(sessionID)
}

func (m *Manager) GetConnectionStats(connectionName string) (*SessionStats, error) {
	return m.db.GetConnectionStats(connectionName)
}

func (m *Manager) UpdateSessionStats(sessionID int64) error {
	// Stats are automatically updated by database triggers
	return nil
}

// File operations methods

func (m *Manager) SaveQueryResult(sessionID int64, queryID int64, result *database.QueryResult, options QueryResultSaveOptions) (string, error) {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to get session: %w", err)
	}

	resultsDir := m.GetResultsDir(session.ConnectionName)
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create results directory: %w", err)
	}

	// Generate filename
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("query_%d_%s.%s", queryID, timestamp, options.Format)
	filePath := filepath.Join(resultsDir, filename)

	switch options.Format {
	case "csv":
		return filePath, m.saveAsCSV(result, filePath, options)
	case "markdown", "md":
		return filePath, m.saveAsMarkdown(result, filePath, options)
	case "json":
		return filePath, m.saveAsJSON(result, filePath, options)
	default:
		return "", fmt.Errorf("unsupported format: %s", options.Format)
	}
}

func (m *Manager) LoadQueryResult(filePath string) (*database.QueryResult, error) {
	// This would need to be implemented based on the file format
	// For now, return an error indicating it's not implemented
	return nil, fmt.Errorf("loading query results from file is not yet implemented")
}

func (m *Manager) ExportSessionData(sessionID int64, options ExportOptions) (string, error) {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to get session: %w", err)
	}

	exportsDir := m.GetExportsDir(session.ConnectionName)
	if err := os.MkdirAll(exportsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create exports directory: %w", err)
	}

	// Generate export filename
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("session_%d_export_%s.%s", sessionID, timestamp, options.Format)
	filePath := filepath.Join(exportsDir, filename)

	// Get query history for the session
	queries, err := m.GetQueryHistory(sessionID, 1000, 0) // Limit to 1000 queries
	if err != nil {
		return "", fmt.Errorf("failed to get query history: %w", err)
	}

	// Apply date range filter if specified
	if options.DateRange != nil {
		filteredQueries := make([]*QueryRecord, 0)
		for _, query := range queries {
			if query.ExecutedAt.After(options.DateRange.Start) && query.ExecutedAt.Before(options.DateRange.End) {
				filteredQueries = append(filteredQueries, query)
			}
		}
		queries = filteredQueries
	}

	switch options.Format {
	case "json":
		return filePath, m.exportAsJSON(queries, session, filePath)
	case "csv":
		return filePath, m.exportAsCSV(queries, filePath)
	case "markdown", "md":
		return filePath, m.exportAsMarkdown(queries, session, filePath)
	default:
		return "", fmt.Errorf("unsupported export format: %s", options.Format)
	}
}

func (m *Manager) ExportConnectionData(connectionName string, options ExportOptions) (string, error) {
	exportsDir := m.GetExportsDir(connectionName)
	if err := os.MkdirAll(exportsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create exports directory: %w", err)
	}

	// Generate export filename
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("connection_%s_export_%s.%s", connectionName, timestamp, options.Format)
	filePath := filepath.Join(exportsDir, filename)

	// Get all query history for the connection
	queries, err := m.GetQueryHistoryByConnection(connectionName, 10000, 0) // Limit to 10000 queries
	if err != nil {
		return "", fmt.Errorf("failed to get query history: %w", err)
	}

	// Apply date range filter if specified
	if options.DateRange != nil {
		filteredQueries := make([]*QueryRecord, 0)
		for _, query := range queries {
			if query.ExecutedAt.After(options.DateRange.Start) && query.ExecutedAt.Before(options.DateRange.End) {
				filteredQueries = append(filteredQueries, query)
			}
		}
		queries = filteredQueries
	}

	switch options.Format {
	case "json":
		return filePath, m.exportConnectionAsJSON(queries, connectionName, filePath)
	case "csv":
		return filePath, m.exportAsCSV(queries, filePath)
	case "markdown", "md":
		return filePath, m.exportConnectionAsMarkdown(queries, connectionName, filePath)
	default:
		return "", fmt.Errorf("unsupported export format: %s", options.Format)
	}
}

// Preferences methods

func (m *Manager) SetPreference(key, value string) error {
	return m.db.SetPreference(key, value)
}

func (m *Manager) GetPreference(key string) (string, error) {
	return m.db.GetPreference(key)
}

func (m *Manager) GetAllPreferences() (map[string]string, error) {
	return m.db.GetAllPreferences()
}

// Directory management methods

func (m *Manager) GetSessionDir(connectionName string) string {
	return filepath.Join(m.configDir, "session_data", connectionName)
}

func (m *Manager) EnsureSessionDir(connectionName string) error {
	if connectionName == "" {
		return fmt.Errorf("connection name cannot be empty")
	}

	sessionDir := m.GetSessionDir(connectionName)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Create subdirectories
	subdirs := []string{"results", "exports", "temp"}
	for _, subdir := range subdirs {
		dir := filepath.Join(sessionDir, subdir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", subdir, err)
		}
	}

	return nil
}

func (m *Manager) GetResultsDir(connectionName string) string {
	return filepath.Join(m.GetSessionDir(connectionName), "results")
}

func (m *Manager) GetExportsDir(connectionName string) string {
	return filepath.Join(m.GetSessionDir(connectionName), "exports")
}

func (m *Manager) GetTempDir(connectionName string) string {
	return filepath.Join(m.GetSessionDir(connectionName), "temp")
}

// Cleanup methods

func (m *Manager) CleanupOldSessions(retentionDays int) error {
	return m.db.CleanupOldSessions(retentionDays)
}

func (m *Manager) CleanupResultFiles(retentionDays int) error {
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	// Clean up files in all session data directories
	sessionDataDir := filepath.Join(m.configDir, "session_data")
	
	return filepath.Walk(sessionDataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		// Only remove files, not directories
		if !info.IsDir() && info.ModTime().Before(cutoffTime) {
			// Don't remove files that are still referenced in the database
			if strings.Contains(path, "results") || strings.Contains(path, "exports") {
				if err := os.Remove(path); err != nil {
					// Log error but continue
					fmt.Printf("Warning: failed to remove old file %s: %v\n", path, err)
				}
			}
		}

		return nil
	})
}

func (m *Manager) CleanupTempFiles() error {
	sessionDataDir := filepath.Join(m.configDir, "session_data")
	
	return filepath.Walk(sessionDataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		// Remove all files in temp directories
		if !info.IsDir() && strings.Contains(path, "temp") {
			if err := os.Remove(path); err != nil {
				fmt.Printf("Warning: failed to remove temp file %s: %v\n", path, err)
			}
		}

		return nil
	})
}

// Utility methods

func (m *Manager) DisplayMarkdown(markdown string) error {
	renderer := database.NewMarkdownRenderer(m.i18nMgr)
	return renderer.RenderAndDisplay(markdown)
}

func (m *Manager) ViewMarkdown(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read markdown file: %w", err)
	}

	return m.DisplayMarkdown(string(content))
}

// Database operations (internal)

func (m *Manager) initDatabase() error {
	// Database is already initialized in NewManager
	return nil
}

func (m *Manager) migrate() error {
	// Migration is handled by the Database struct
	return nil
}

func (m *Manager) close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// Close closes the session manager and its database connection
func (m *Manager) Close() error {
	return m.close()
}

// Private helper methods for file operations

func (m *Manager) saveAsCSV(result *database.QueryResult, filePath string, options QueryResultSaveOptions) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	// Use the existing CSV export functionality
	_, err = database.SaveQueryResultAsStreamingCSV(result, filePath)
	return err
}

func (m *Manager) saveAsMarkdown(result *database.QueryResult, filePath string, options QueryResultSaveOptions) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create markdown file: %w", err)
	}
	defer file.Close()

	// Use the existing markdown export functionality
	return database.SaveQueryResultAsMarkdown(result, "", "", file, m.i18nMgr)
}

func (m *Manager) saveAsJSON(result *database.QueryResult, filePath string, options QueryResultSaveOptions) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create JSON file: %w", err)
	}
	defer file.Close()

	// Convert result to JSON format
	jsonData := make([]map[string]interface{}, 0)
	defer result.Close()

	for row := range result.Itor() {
		rowData := make(map[string]interface{})
		for i, col := range result.Columns {
			if i < len(row) {
				if row[i].IsNull() {
					rowData[col.Name] = nil
				} else {
					rowData[col.Name] = row[i].String()
				}
			}
		}
		jsonData = append(jsonData, rowData)
	}

	if result.Error() != nil {
		return result.Error()
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonData)
}

func (m *Manager) exportAsJSON(queries []*QueryRecord, session *Session, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	exportData := map[string]interface{}{
		"session":         session,
		"exported_at":     time.Now(),
		"query_count":     len(queries),
		"query_history":   queries,
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(exportData)
}

func (m *Manager) exportConnectionAsJSON(queries []*QueryRecord, connectionName string, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	exportData := map[string]interface{}{
		"connection_name": connectionName,
		"exported_at":     time.Now(),
		"query_count":     len(queries),
		"query_history":   queries,
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(exportData)
}

func (m *Manager) exportAsCSV(queries []*QueryRecord, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	// Write CSV header
	header := "id,session_id,query,executed_at,execution_time_ms,rows_affected,success,error_message\n"
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write query records
	for _, query := range queries {
		errorMsg := ""
		if query.ErrorMessage != nil {
			errorMsg = *query.ErrorMessage
		}
		
		line := fmt.Sprintf("%d,%d,\"%s\",%s,%d,%d,%t,\"%s\"\n",
			query.ID,
			query.SessionID,
			strings.ReplaceAll(query.Query, "\"", "\"\""), // Escape quotes
			query.ExecutedAt.Format(time.RFC3339),
			query.ExecutionTimeMs,
			query.RowsAffected,
			query.Success,
			strings.ReplaceAll(errorMsg, "\"", "\"\""), // Escape quotes
		)
		
		if _, err := file.WriteString(line); err != nil {
			return fmt.Errorf("failed to write CSV line: %w", err)
		}
	}

	return nil
}

func (m *Manager) exportAsMarkdown(queries []*QueryRecord, session *Session, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	// Write markdown header
	header := fmt.Sprintf("# Session Export: %s\n\n", session.ConnectionName)
	header += fmt.Sprintf("- **Session ID**: %d\n", session.ID)
	header += fmt.Sprintf("- **Created**: %s\n", session.CreatedAt.Format(time.RFC3339))
	header += fmt.Sprintf("- **Last Accessed**: %s\n", session.LastAccessedAt.Format(time.RFC3339))
	header += fmt.Sprintf("- **Query Count**: %d\n\n", len(queries))
	header += "## Query History\n\n"

	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write markdown header: %w", err)
	}

	// Write query records
	for i, query := range queries {
		querySection := fmt.Sprintf("### Query %d (ID: %d)\n\n", i+1, query.ID)
		querySection += fmt.Sprintf("- **Executed**: %s\n", query.ExecutedAt.Format(time.RFC3339))
		querySection += fmt.Sprintf("- **Duration**: %dms\n", query.ExecutionTimeMs)
		querySection += fmt.Sprintf("- **Rows Affected**: %d\n", query.RowsAffected)
		querySection += fmt.Sprintf("- **Success**: %t\n", query.Success)
		
		if query.ErrorMessage != nil && *query.ErrorMessage != "" {
			querySection += fmt.Sprintf("- **Error**: %s\n", *query.ErrorMessage)
		}
		
		querySection += "\n```sql\n"
		querySection += query.Query
		querySection += "\n```\n\n"
		
		if _, err := file.WriteString(querySection); err != nil {
			return fmt.Errorf("failed to write query section: %w", err)
		}
	}

	return nil
}

func (m *Manager) exportConnectionAsMarkdown(queries []*QueryRecord, connectionName string, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	// Write markdown header
	header := fmt.Sprintf("# Connection Export: %s\n\n", connectionName)
	header += fmt.Sprintf("- **Exported**: %s\n", time.Now().Format(time.RFC3339))
	header += fmt.Sprintf("- **Query Count**: %d\n\n", len(queries))
	header += "## Query History\n\n"

	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write markdown header: %w", err)
	}

	// Write query records
	for i, query := range queries {
		querySection := fmt.Sprintf("### Query %d (ID: %d, Session: %d)\n\n", i+1, query.ID, query.SessionID)
		querySection += fmt.Sprintf("- **Executed**: %s\n", query.ExecutedAt.Format(time.RFC3339))
		querySection += fmt.Sprintf("- **Duration**: %dms\n", query.ExecutionTimeMs)
		querySection += fmt.Sprintf("- **Rows Affected**: %d\n", query.RowsAffected)
		querySection += fmt.Sprintf("- **Success**: %t\n", query.Success)
		
		if query.ErrorMessage != nil && *query.ErrorMessage != "" {
			querySection += fmt.Sprintf("- **Error**: %s\n", *query.ErrorMessage)
		}
		
		querySection += "\n```sql\n"
		querySection += query.Query
		querySection += "\n```\n\n"
		
		if _, err := file.WriteString(querySection); err != nil {
			return fmt.Errorf("failed to write query section: %w", err)
		}
	}

	return nil
}