package session

import (
	"strings"
	"testing"
	"time"

	"github.com/vrealzhou/sqlterm/internal/database"
	"github.com/vrealzhou/sqlterm/internal/i18n"
)

func createTestManager(tb testing.TB, configDir string) *Manager {
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		tb.Fatalf("Failed to create i18n manager: %v", err)
	}
	manager, err := NewManager(configDir, i18nMgr)
	if err != nil {
		tb.Fatalf("Failed to create session manager: %v", err)
	}
	return manager
}

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()

	manager := createTestManager(t, tmpDir)

	if manager == nil {
		t.Fatal("Expected manager to be created, got nil")
	}

	if manager.configDir != tmpDir {
		t.Errorf("Expected configDir to be %s, got %s", tmpDir, manager.configDir)
	}
}

func TestManager_CreateSession(t *testing.T) {
	tmpDir := t.TempDir()
	manager := createTestManager(t, tmpDir)

	connectionName := "test-connection"
	
	session, err := manager.CreateSession(connectionName)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session == nil {
		t.Fatal("Expected session to be created, got nil")
	}

	if session.ConnectionName != connectionName {
		t.Errorf("Expected connection name %s, got %s", connectionName, session.ConnectionName)
	}

	if session.Status != "active" {
		t.Errorf("Expected session status to be 'active', got %s", session.Status)
	}
}

func TestManager_GetSession(t *testing.T) {
	tmpDir := t.TempDir()
	manager := createTestManager(t, tmpDir)

	connectionName := "test-connection"
	
	// Create a session first
	createdSession, err := manager.CreateSession(connectionName)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Get the session by ID
	retrievedSession, err := manager.GetSession(createdSession.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if retrievedSession == nil {
		t.Fatal("Expected session to be retrieved, got nil")
	}

	if retrievedSession.ID != createdSession.ID {
		t.Errorf("Expected session ID %d, got %d", createdSession.ID, retrievedSession.ID)
	}

	if retrievedSession.ConnectionName != connectionName {
		t.Errorf("Expected connection name %s, got %s", connectionName, retrievedSession.ConnectionName)
	}
}

func TestManager_GetActiveSession(t *testing.T) {
	tmpDir := t.TempDir()
	manager := createTestManager(t, tmpDir)

	connectionName := "test-connection"
	
	// Check no active session initially
	activeSession, err := manager.GetActiveSession(connectionName)
	if err != nil {
		t.Fatalf("GetActiveSession failed: %v", err)
	}

	if activeSession != nil {
		t.Error("Expected no active session initially, got one")
	}

	// Create a session
	createdSession, err := manager.CreateSession(connectionName)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Check active session exists
	activeSession, err = manager.GetActiveSession(connectionName)
	if err != nil {
		t.Fatalf("GetActiveSession failed: %v", err)
	}

	if activeSession == nil {
		t.Fatal("Expected active session, got nil")
	}

	if activeSession.ID != createdSession.ID {
		t.Errorf("Expected active session ID %d, got %d", createdSession.ID, activeSession.ID)
	}
}

func TestManager_RecordQuery(t *testing.T) {
	tmpDir := t.TempDir()
	manager := createTestManager(t, tmpDir)

	connectionName := "test-connection"
	
	// Create a session
	session, err := manager.CreateSession(connectionName)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// For testing, we'll pass nil result since QueryResult needs proper initialization
	// The manager should handle nil results gracefully
	var result *database.QueryResult = nil

	query := "SELECT * FROM users"
	executionTime := 100 * time.Millisecond

	// Record the query
	queryRecord, err := manager.RecordQuery(session.ID, query, result, executionTime)
	if err != nil {
		t.Fatalf("RecordQuery failed: %v", err)
	}

	if queryRecord == nil {
		t.Fatal("Expected query record to be created, got nil")
	}

	if queryRecord.Query != query {
		t.Errorf("Expected query %s, got %s", query, queryRecord.Query)
	}

	expectedTimeMs := executionTime.Milliseconds()
	if queryRecord.ExecutionTimeMs != expectedTimeMs {
		t.Errorf("Expected execution time %d ms, got %d ms", expectedTimeMs, queryRecord.ExecutionTimeMs)
	}

	if queryRecord.RowsAffected != 0 {
		t.Errorf("Expected rows affected 0, got %d", queryRecord.RowsAffected)
	}
}

func TestManager_GetQueryHistory(t *testing.T) {
	tmpDir := t.TempDir()
	manager := createTestManager(t, tmpDir)

	connectionName := "test-connection"
	
	// Create a session
	session, err := manager.CreateSession(connectionName)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Record multiple queries
	queries := []string{
		"SELECT * FROM users",
		"SELECT * FROM posts",
		"SELECT COUNT(*) FROM comments",
	}

	for _, query := range queries {
		_, err := manager.RecordQuery(session.ID, query, nil, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("RecordQuery failed: %v", err)
		}
	}

	// Get query history
	history, err := manager.GetQueryHistory(session.ID, 10, 0)
	if err != nil {
		t.Fatalf("GetQueryHistory failed: %v", err)
	}

	if len(history) != len(queries) {
		t.Errorf("Expected %d queries in history, got %d", len(queries), len(history))
	}

	// Check that all queries are present (order may vary)
	queryMap := make(map[string]bool)
	for _, query := range queries {
		queryMap[query] = false
	}

	for _, record := range history {
		if _, exists := queryMap[record.Query]; exists {
			queryMap[record.Query] = true
		}
	}

	// Verify all queries were found
	for query, found := range queryMap {
		if !found {
			t.Errorf("Expected query %s not found in history", query)
		}
	}
}

func TestManager_SearchQueryHistory(t *testing.T) {
	tmpDir := t.TempDir()
	manager := createTestManager(t, tmpDir)

	connectionName := "test-connection"
	
	// Create a session
	session, err := manager.CreateSession(connectionName)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Record queries
	queries := []string{
		"SELECT * FROM users WHERE name = 'john'",
		"SELECT * FROM posts WHERE title LIKE '%test%'",
		"SELECT COUNT(*) FROM users",
		"UPDATE users SET active = 1",
	}

	for _, query := range queries {
		_, err := manager.RecordQuery(session.ID, query, nil, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("RecordQuery failed: %v", err)
		}
	}

	// Search for queries containing "users"
	results, err := manager.SearchQueryHistory(session.ID, "users", 10)
	if err != nil {
		t.Fatalf("SearchQueryHistory failed: %v", err)
	}

	// Should find 3 queries containing "users"
	expectedCount := 3
	if len(results) != expectedCount {
		t.Errorf("Expected %d search results, got %d", expectedCount, len(results))
	}

	// Verify all results contain "users"
	for _, record := range results {
		if !containsIgnoreCase(record.Query, "users") {
			t.Errorf("Search result should contain 'users': %s", record.Query)
		}
	}
}

func TestManager_GetSessionStats(t *testing.T) {
	tmpDir := t.TempDir()
	manager := createTestManager(t, tmpDir)

	connectionName := "test-connection"
	
	// Create a session
	session, err := manager.CreateSession(connectionName)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Record some queries
	for i := 0; i < 5; i++ {
		query := "SELECT * FROM test_table"
		_, err := manager.RecordQuery(session.ID, query, nil, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("RecordQuery failed: %v", err)
		}
	}

	// Get session stats
	stats, err := manager.GetSessionStats(session.ID)
	if err != nil {
		t.Fatalf("GetSessionStats failed: %v", err)
	}

	if stats == nil {
		t.Fatal("Expected session stats, got nil")
	}

	if stats.TotalQueries != 5 {
		t.Errorf("Expected 5 total queries, got %d", stats.TotalQueries)
	}
}

func TestManager_CleanupOldSessions(t *testing.T) {
	tmpDir := t.TempDir()
	manager := createTestManager(t, tmpDir)

	// Test cleanup functionality
	err := manager.CleanupOldSessions(30)
	if err != nil {
		t.Fatalf("CleanupOldSessions failed: %v", err)
	}
}

func TestManager_CleanupResultFiles(t *testing.T) {
	tmpDir := t.TempDir()
	manager := createTestManager(t, tmpDir)

	// Test cleanup functionality
	err := manager.CleanupResultFiles(30)
	if err != nil {
		t.Fatalf("CleanupResultFiles failed: %v", err)
	}
}

func TestManager_GetSessionDir(t *testing.T) {
	tmpDir := t.TempDir()
	manager := createTestManager(t, tmpDir)

	connectionName := "test-connection"
	
	sessionDir := manager.GetSessionDir(connectionName)
	if sessionDir == "" {
		t.Error("Expected non-empty session directory")
	}

	// Should contain the connection name
	if !containsIgnoreCase(sessionDir, connectionName) {
		t.Errorf("Expected session dir to contain connection name '%s', got '%s'", connectionName, sessionDir)
	}
}

func TestManager_Preferences(t *testing.T) {
	tmpDir := t.TempDir()
	manager := createTestManager(t, tmpDir)

	key := "test_key"
	value := "test_value"

	// Set preference
	err := manager.SetPreference(key, value)
	if err != nil {
		t.Fatalf("SetPreference failed: %v", err)
	}

	// Get preference
	retrievedValue, err := manager.GetPreference(key)
	if err != nil {
		t.Fatalf("GetPreference failed: %v", err)
	}

	if retrievedValue != value {
		t.Errorf("Expected preference value %s, got %s", value, retrievedValue)
	}

	// Get all preferences
	allPrefs, err := manager.GetAllPreferences()
	if err != nil {
		t.Fatalf("GetAllPreferences failed: %v", err)
	}

	if allPrefs[key] != value {
		t.Errorf("Expected preference %s to be %s in all preferences, got %s", key, value, allPrefs[key])
	}
}

// Helper function for string contains check (case insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    (len(s) > len(substr) && 
		     contains(strings.ToLower(s), strings.ToLower(substr))))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}