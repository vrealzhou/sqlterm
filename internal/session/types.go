package session

import (
	"time"

	"github.com/vrealzhou/sqlterm/internal/database"
)

// Session represents a database session
type Session struct {
	ID             int64     `db:"id"`
	ConnectionName string    `db:"connection_name"`
	CreatedAt      time.Time `db:"created_at"`
	LastAccessedAt time.Time `db:"last_accessed_at"`
	Status         string    `db:"status"` // active, closed, archived
}

// QueryRecord represents a query execution record
type QueryRecord struct {
	ID              int64     `db:"id"`
	SessionID       int64     `db:"session_id"`
	Query           string    `db:"query"`
	ExecutedAt      time.Time `db:"executed_at"`
	ExecutionTimeMs int64     `db:"execution_time_ms"`
	RowsAffected    int64     `db:"rows_affected"`
	Success         bool      `db:"success"`
	ErrorMessage    *string   `db:"error_message"`
	ResultFilePath  *string   `db:"result_file_path"`
}

// SessionStats represents session statistics
type SessionStats struct {
	SessionID            int64     `db:"session_id"`
	TotalQueries         int64     `db:"total_queries"`
	SuccessfulQueries    int64     `db:"successful_queries"`
	TotalExecutionTimeMs int64     `db:"total_execution_time_ms"`
	LastQueryAt          *time.Time `db:"last_query_at"`
}

// UserPreference represents a user preference setting
type UserPreference struct {
	Key       string    `db:"key"`
	Value     string    `db:"value"`
	UpdatedAt time.Time `db:"updated_at"`
}

// ExportOptions contains options for exporting data
type ExportOptions struct {
	Format      string // csv, json, markdown
	Compression bool
	IncludeHeaders bool
	DateRange   *DateRange
}

// DateRange represents a time range for filtering
type DateRange struct {
	Start time.Time
	End   time.Time
}

// QueryResultSaveOptions contains options for saving query results
type QueryResultSaveOptions struct {
	Format          string // csv, markdown, json
	IncludeMetadata bool
	Compress        bool
}

// SessionManagerInterface defines the interface for session management
type SessionManagerInterface interface {
	// Session lifecycle
	CreateSession(connectionName string) (*Session, error)
	GetSession(sessionID int64) (*Session, error)
	GetActiveSession(connectionName string) (*Session, error)
	UpdateSessionAccess(sessionID int64) error
	CloseSession(sessionID int64) error
	ArchiveSession(sessionID int64) error
	
	// Query history
	RecordQuery(sessionID int64, query string, result *database.QueryResult, executionTime time.Duration) (*QueryRecord, error)
	GetQueryHistory(sessionID int64, limit int, offset int) ([]*QueryRecord, error)
	GetQueryHistoryByConnection(connectionName string, limit int, offset int) ([]*QueryRecord, error)
	SearchQueryHistory(sessionID int64, searchTerm string, limit int) ([]*QueryRecord, error)
	
	// Statistics
	GetSessionStats(sessionID int64) (*SessionStats, error)
	GetConnectionStats(connectionName string) (*SessionStats, error)
	UpdateSessionStats(sessionID int64) error
	
	// File operations
	SaveQueryResult(sessionID int64, queryID int64, result *database.QueryResult, options QueryResultSaveOptions) (string, error)
	LoadQueryResult(filePath string) (*database.QueryResult, error)
	ExportSessionData(sessionID int64, options ExportOptions) (string, error)
	ExportConnectionData(connectionName string, options ExportOptions) (string, error)
	
	// Preferences
	SetPreference(key, value string) error
	GetPreference(key string) (string, error)
	GetAllPreferences() (map[string]string, error)
	
	// Directory management
	GetSessionDir(connectionName string) string
	EnsureSessionDir(connectionName string) error
	GetResultsDir(connectionName string) string
	GetExportsDir(connectionName string) string
	GetTempDir(connectionName string) string
	
	// Cleanup
	CleanupOldSessions(retentionDays int) error
	CleanupResultFiles(retentionDays int) error
	CleanupTempFiles() error
	
	// Utilities
	DisplayMarkdown(markdown string) error
	ViewMarkdown(filePath string) error
	
	// Database operations (internal)
	initDatabase() error
	migrate() error
	close() error
}