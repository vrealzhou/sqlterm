package session

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database handles SQLite operations for session management
type Database struct {
	db   *sql.DB
	path string
}

// NewDatabase creates a new session database
func NewDatabase(configDir string) (*Database, error) {
	dbPath := filepath.Join(configDir, "sessions.db")
	
	db, err := sql.Open("sqlite3", dbPath+"?_fk=1&_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open session database: %w", err)
	}

	// Set pragmas for better performance and reliability
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=1000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA temp_store=MEMORY",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma %s: %w", pragma, err)
		}
	}

	database := &Database{
		db:   db,
		path: dbPath,
	}

	if err := database.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return database, nil
}

// migrate creates or updates the database schema
func (d *Database) migrate() error {
	// Core schema without FTS5
	coreSchema := `
	-- Session metadata table
	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		connection_name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_accessed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		status TEXT DEFAULT 'active' CHECK (status IN ('active', 'closed', 'archived'))
	);

	-- Index for faster lookups
	CREATE INDEX IF NOT EXISTS idx_sessions_connection_status ON sessions(connection_name, status);
	CREATE INDEX IF NOT EXISTS idx_sessions_last_accessed ON sessions(last_accessed_at);

	-- Query history table
	CREATE TABLE IF NOT EXISTS query_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		query TEXT NOT NULL,
		executed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		execution_time_ms INTEGER DEFAULT 0,
		rows_affected INTEGER DEFAULT 0,
		success BOOLEAN DEFAULT TRUE,
		error_message TEXT,
		result_file_path TEXT
	);

	-- Indexes for query history
	CREATE INDEX IF NOT EXISTS idx_query_history_session_id ON query_history(session_id);
	CREATE INDEX IF NOT EXISTS idx_query_history_executed_at ON query_history(executed_at);
	CREATE INDEX IF NOT EXISTS idx_query_history_success ON query_history(success);

	-- Session statistics table
	CREATE TABLE IF NOT EXISTS session_stats (
		session_id INTEGER PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
		total_queries INTEGER DEFAULT 0,
		successful_queries INTEGER DEFAULT 0,
		total_execution_time_ms INTEGER DEFAULT 0,
		last_query_at DATETIME
	);

	-- User preferences table
	CREATE TABLE IF NOT EXISTS user_preferences (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Trigger to update session last_accessed_at
	CREATE TRIGGER IF NOT EXISTS update_session_access AFTER INSERT ON query_history BEGIN
		UPDATE sessions SET last_accessed_at = CURRENT_TIMESTAMP WHERE id = new.session_id;
	END;

	-- Trigger to update session stats
	CREATE TRIGGER IF NOT EXISTS update_session_stats AFTER INSERT ON query_history BEGIN
		INSERT OR REPLACE INTO session_stats (session_id, total_queries, successful_queries, total_execution_time_ms, last_query_at)
		VALUES (
			new.session_id,
			COALESCE((SELECT total_queries FROM session_stats WHERE session_id = new.session_id), 0) + 1,
			COALESCE((SELECT successful_queries FROM session_stats WHERE session_id = new.session_id), 0) + CASE WHEN new.success THEN 1 ELSE 0 END,
			COALESCE((SELECT total_execution_time_ms FROM session_stats WHERE session_id = new.session_id), 0) + new.execution_time_ms,
			new.executed_at
		);
	END;
	`

	// Execute core schema
	_, err := d.db.Exec(coreSchema)
	if err != nil {
		return err
	}

	// Try to enable FTS5 features (optional)
	d.migrateFTS5()

	return nil
}

// migrateFTS5 attempts to create FTS5 features if available
func (d *Database) migrateFTS5() {
	fts5Schema := `
	-- Full-text search for query history
	CREATE VIRTUAL TABLE IF NOT EXISTS query_search USING fts5(
		query, error_message, 
		content='query_history', 
		content_rowid='id'
	);

	-- Triggers to maintain FTS table
	CREATE TRIGGER IF NOT EXISTS query_search_insert AFTER INSERT ON query_history BEGIN
		INSERT INTO query_search(rowid, query, error_message) 
		VALUES (new.id, new.query, new.error_message);
	END;

	CREATE TRIGGER IF NOT EXISTS query_search_delete AFTER DELETE ON query_history BEGIN
		INSERT INTO query_search(query_search, rowid, query, error_message) 
		VALUES('delete', old.id, old.query, old.error_message);
	END;

	CREATE TRIGGER IF NOT EXISTS query_search_update AFTER UPDATE ON query_history BEGIN
		INSERT INTO query_search(query_search, rowid, query, error_message) 
		VALUES('delete', old.id, old.query, old.error_message);
		INSERT INTO query_search(rowid, query, error_message) 
		VALUES (new.id, new.query, new.error_message);
	END;
	`

	// Try to execute FTS5 schema, but don't fail if it's not available
	_, err := d.db.Exec(fts5Schema)
	if err != nil {
		// FTS5 not available, continue without it
		// In a production system, you might want to log this
		_ = err
	}
}

// CreateSession creates a new session
func (d *Database) CreateSession(connectionName string) (*Session, error) {
	query := `
		INSERT INTO sessions (connection_name, created_at, last_accessed_at, status) 
		VALUES (?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'active')
	`
	
	result, err := d.db.Exec(query, connectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get session ID: %w", err)
	}

	return d.GetSession(id)
}

// GetSession retrieves a session by ID
func (d *Database) GetSession(sessionID int64) (*Session, error) {
	query := `
		SELECT id, connection_name, created_at, last_accessed_at, status 
		FROM sessions WHERE id = ?
	`
	
	session := &Session{}
	err := d.db.QueryRow(query, sessionID).Scan(
		&session.ID,
		&session.ConnectionName,
		&session.CreatedAt,
		&session.LastAccessedAt,
		&session.Status,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session %d not found", sessionID)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// GetActiveSession retrieves the active session for a connection
func (d *Database) GetActiveSession(connectionName string) (*Session, error) {
	query := `
		SELECT id, connection_name, created_at, last_accessed_at, status 
		FROM sessions 
		WHERE connection_name = ? AND status = 'active' 
		ORDER BY last_accessed_at DESC 
		LIMIT 1
	`
	
	session := &Session{}
	err := d.db.QueryRow(query, connectionName).Scan(
		&session.ID,
		&session.ConnectionName,
		&session.CreatedAt,
		&session.LastAccessedAt,
		&session.Status,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No active session found
		}
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}

	return session, nil
}

// UpdateSessionAccess updates the last accessed time for a session
func (d *Database) UpdateSessionAccess(sessionID int64) error {
	query := `UPDATE sessions SET last_accessed_at = CURRENT_TIMESTAMP WHERE id = ?`
	
	_, err := d.db.Exec(query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session access: %w", err)
	}

	return nil
}

// CloseSession marks a session as closed
func (d *Database) CloseSession(sessionID int64) error {
	query := `UPDATE sessions SET status = 'closed' WHERE id = ?`
	
	_, err := d.db.Exec(query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to close session: %w", err)
	}

	return nil
}

// ArchiveSession marks a session as archived
func (d *Database) ArchiveSession(sessionID int64) error {
	query := `UPDATE sessions SET status = 'archived' WHERE id = ?`
	
	_, err := d.db.Exec(query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to archive session: %w", err)
	}

	return nil
}

// RecordQuery records a query execution
func (d *Database) RecordQuery(sessionID int64, query string, executionTimeMs int64, rowsAffected int64, success bool, errorMessage *string, resultFilePath *string) (*QueryRecord, error) {
	insertQuery := `
		INSERT INTO query_history (session_id, query, executed_at, execution_time_ms, rows_affected, success, error_message, result_file_path)
		VALUES (?, ?, CURRENT_TIMESTAMP, ?, ?, ?, ?, ?)
	`
	
	result, err := d.db.Exec(insertQuery, sessionID, query, executionTimeMs, rowsAffected, success, errorMessage, resultFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to record query: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get query record ID: %w", err)
	}

	return d.GetQueryRecord(id)
}

// GetQueryRecord retrieves a query record by ID
func (d *Database) GetQueryRecord(queryID int64) (*QueryRecord, error) {
	query := `
		SELECT id, session_id, query, executed_at, execution_time_ms, rows_affected, success, error_message, result_file_path
		FROM query_history WHERE id = ?
	`
	
	record := &QueryRecord{}
	err := d.db.QueryRow(query, queryID).Scan(
		&record.ID,
		&record.SessionID,
		&record.Query,
		&record.ExecutedAt,
		&record.ExecutionTimeMs,
		&record.RowsAffected,
		&record.Success,
		&record.ErrorMessage,
		&record.ResultFilePath,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("query record %d not found", queryID)
		}
		return nil, fmt.Errorf("failed to get query record: %w", err)
	}

	return record, nil
}

// GetQueryHistory retrieves query history for a session
func (d *Database) GetQueryHistory(sessionID int64, limit, offset int) ([]*QueryRecord, error) {
	query := `
		SELECT id, session_id, query, executed_at, execution_time_ms, rows_affected, success, error_message, result_file_path
		FROM query_history 
		WHERE session_id = ? 
		ORDER BY executed_at DESC 
		LIMIT ? OFFSET ?
	`
	
	rows, err := d.db.Query(query, sessionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get query history: %w", err)
	}
	defer rows.Close()

	var records []*QueryRecord
	for rows.Next() {
		record := &QueryRecord{}
		err := rows.Scan(
			&record.ID,
			&record.SessionID,
			&record.Query,
			&record.ExecutedAt,
			&record.ExecutionTimeMs,
			&record.RowsAffected,
			&record.Success,
			&record.ErrorMessage,
			&record.ResultFilePath,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan query record: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}

// GetQueryHistoryByConnection retrieves query history for all sessions of a connection
func (d *Database) GetQueryHistoryByConnection(connectionName string, limit, offset int) ([]*QueryRecord, error) {
	query := `
		SELECT qh.id, qh.session_id, qh.query, qh.executed_at, qh.execution_time_ms, qh.rows_affected, qh.success, qh.error_message, qh.result_file_path
		FROM query_history qh
		JOIN sessions s ON qh.session_id = s.id
		WHERE s.connection_name = ? 
		ORDER BY qh.executed_at DESC 
		LIMIT ? OFFSET ?
	`
	
	rows, err := d.db.Query(query, connectionName, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get query history by connection: %w", err)
	}
	defer rows.Close()

	var records []*QueryRecord
	for rows.Next() {
		record := &QueryRecord{}
		err := rows.Scan(
			&record.ID,
			&record.SessionID,
			&record.Query,
			&record.ExecutedAt,
			&record.ExecutionTimeMs,
			&record.RowsAffected,
			&record.Success,
			&record.ErrorMessage,
			&record.ResultFilePath,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan query record: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}

// SearchQueryHistory searches query history using FTS or LIKE fallback
func (d *Database) SearchQueryHistory(sessionID int64, searchTerm string, limit int) ([]*QueryRecord, error) {
	// Try FTS5 first
	ftsQuery := `
		SELECT qh.id, qh.session_id, qh.query, qh.executed_at, qh.execution_time_ms, qh.rows_affected, qh.success, qh.error_message, qh.result_file_path
		FROM query_history qh
		JOIN query_search qs ON qh.id = qs.rowid
		WHERE qh.session_id = ? AND query_search MATCH ?
		ORDER BY qh.executed_at DESC 
		LIMIT ?
	`
	
	rows, err := d.db.Query(ftsQuery, sessionID, searchTerm, limit)
	
	// If FTS5 is not available, fallback to LIKE search
	if err != nil {
		likeQuery := `
			SELECT id, session_id, query, executed_at, execution_time_ms, rows_affected, success, error_message, result_file_path
			FROM query_history
			WHERE session_id = ? AND (query LIKE ? OR error_message LIKE ?)
			ORDER BY executed_at DESC 
			LIMIT ?
		`
		
		searchPattern := "%" + searchTerm + "%"
		rows, err = d.db.Query(likeQuery, sessionID, searchPattern, searchPattern, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to search query history: %w", err)
	}
	defer rows.Close()

	var records []*QueryRecord
	for rows.Next() {
		record := &QueryRecord{}
		err := rows.Scan(
			&record.ID,
			&record.SessionID,
			&record.Query,
			&record.ExecutedAt,
			&record.ExecutionTimeMs,
			&record.RowsAffected,
			&record.Success,
			&record.ErrorMessage,
			&record.ResultFilePath,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan query record: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}

// GetSessionStats retrieves statistics for a session
func (d *Database) GetSessionStats(sessionID int64) (*SessionStats, error) {
	query := `
		SELECT session_id, total_queries, successful_queries, total_execution_time_ms, last_query_at
		FROM session_stats WHERE session_id = ?
	`
	
	stats := &SessionStats{}
	err := d.db.QueryRow(query, sessionID).Scan(
		&stats.SessionID,
		&stats.TotalQueries,
		&stats.SuccessfulQueries,
		&stats.TotalExecutionTimeMs,
		&stats.LastQueryAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			// Return empty stats if none exist
			return &SessionStats{SessionID: sessionID}, nil
		}
		return nil, fmt.Errorf("failed to get session stats: %w", err)
	}

	return stats, nil
}

// GetConnectionStats retrieves aggregated statistics for all sessions of a connection
func (d *Database) GetConnectionStats(connectionName string) (*SessionStats, error) {
	query := `
		SELECT 
			0 as session_id,
			COALESCE(SUM(ss.total_queries), 0) as total_queries,
			COALESCE(SUM(ss.successful_queries), 0) as successful_queries,
			COALESCE(SUM(ss.total_execution_time_ms), 0) as total_execution_time_ms,
			MAX(ss.last_query_at) as last_query_at
		FROM session_stats ss
		JOIN sessions s ON ss.session_id = s.id
		WHERE s.connection_name = ?
	`
	
	stats := &SessionStats{}
	err := d.db.QueryRow(query, connectionName).Scan(
		&stats.SessionID,
		&stats.TotalQueries,
		&stats.SuccessfulQueries,
		&stats.TotalExecutionTimeMs,
		&stats.LastQueryAt,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get connection stats: %w", err)
	}

	return stats, nil
}

// SetPreference sets a user preference
func (d *Database) SetPreference(key, value string) error {
	query := `
		INSERT OR REPLACE INTO user_preferences (key, value, updated_at) 
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`
	
	_, err := d.db.Exec(query, key, value)
	if err != nil {
		return fmt.Errorf("failed to set preference: %w", err)
	}

	return nil
}

// GetPreference gets a user preference
func (d *Database) GetPreference(key string) (string, error) {
	query := `SELECT value FROM user_preferences WHERE key = ?`
	
	var value string
	err := d.db.QueryRow(query, key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("preference %s not found", key)
		}
		return "", fmt.Errorf("failed to get preference: %w", err)
	}

	return value, nil
}

// GetAllPreferences gets all user preferences
func (d *Database) GetAllPreferences() (map[string]string, error) {
	query := `SELECT key, value FROM user_preferences`
	
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get preferences: %w", err)
	}
	defer rows.Close()

	preferences := make(map[string]string)
	for rows.Next() {
		var key, value string
		err := rows.Scan(&key, &value)
		if err != nil {
			return nil, fmt.Errorf("failed to scan preference: %w", err)
		}
		preferences[key] = value
	}

	return preferences, nil
}

// CleanupOldSessions removes old sessions and their data
func (d *Database) CleanupOldSessions(retentionDays int) error {
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
	
	// First get the session IDs to be deleted for cascading cleanup
	sessionQuery := `
		SELECT id FROM sessions 
		WHERE status != 'active' AND last_accessed_at < ?
	`
	
	rows, err := d.db.Query(sessionQuery, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to get old sessions: %w", err)
	}
	defer rows.Close()

	var sessionIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan session ID: %w", err)
		}
		sessionIDs = append(sessionIDs, id)
	}

	// Delete the sessions (cascade will handle related records)
	deleteQuery := `
		DELETE FROM sessions 
		WHERE status != 'active' AND last_accessed_at < ?
	`
	
	result, err := d.db.Exec(deleteQuery, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to delete old sessions: %w", err)
	}

	deletedCount, _ := result.RowsAffected()
	if deletedCount > 0 {
		fmt.Printf("Cleaned up %d old sessions\n", deletedCount)
	}

	return nil
}

// Close closes the database connection
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// GetDatabasePath returns the database file path
func (d *Database) GetDatabasePath() string {
	return d.path
}