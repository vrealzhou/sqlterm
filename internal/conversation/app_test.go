package conversation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sqlterm/internal/ai"
	"sqlterm/internal/config"
	"sqlterm/internal/core"
	"sqlterm/internal/i18n"
	"sqlterm/internal/session"
)

// Mock implementations for testing

type mockConnection struct {
	connected bool
	tables    []string
	dbType    core.DatabaseType
	name      string
}

func (m *mockConnection) Connect() error {
	m.connected = true
	return nil
}

func (m *mockConnection) Close() error {
	m.connected = false
	return nil
}

func (m *mockConnection) IsConnected() bool {
	return m.connected
}

func (m *mockConnection) Execute(query string) (*core.QueryResult, error) {
	// Mock query execution - return minimal QueryResult
	return &core.QueryResult{
		Columns: []core.Column{
			{Name: "id", Type: "INTEGER"},
			{Name: "name", Type: "VARCHAR"},
		},
	}, nil
}

func (m *mockConnection) Ping() error {
	return nil
}

func (m *mockConnection) ListTables() ([]string, error) {
	return m.tables, nil
}

func (m *mockConnection) DescribeTable(tableName string) (*core.TableInfo, error) {
	return &core.TableInfo{
		Name: tableName,
		Columns: []core.ColumnInfo{
			{Name: "id", Type: "INTEGER", Key: "PRI"},
			{Name: "name", Type: "VARCHAR(255)", Nullable: true},
		},
	}, nil
}

func (m *mockConnection) GetDatabaseType() core.DatabaseType {
	return m.dbType
}

func (m *mockConnection) GetConnectionName() string {
	return m.name
}

func createTestApp(t *testing.T) *App {
	tmpDir := t.TempDir()
	
	configMgr := config.NewManager()
	
	// Initialize i18n manager for testing
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create i18n manager: %v", err)
	}
	
	sessionMgr := session.NewManager(tmpDir, i18nMgr)
	
	// Create a mock AI manager
	aiManager, err := ai.NewManager(tmpDir)
	if err != nil {
		// It's okay if AI manager fails to initialize for tests
		aiManager = nil
	}
	
	// Create i18n manager
	i18nMgr, i18nErr := i18n.NewManager("en_au")
	if i18nErr != nil {
		t.Fatalf("Failed to create i18n manager: %v", i18nErr)
	}
	
	app := &App{
		configMgr:  configMgr,
		sessionMgr: sessionMgr,
		aiManager:  aiManager,
		i18nMgr:    i18nMgr,
	}
	
	return app
}

func TestNewApp(t *testing.T) {
	app, err := NewApp()
	
	if err != nil {
		t.Fatalf("NewApp() failed: %v", err)
	}
	
	if app == nil {
		t.Fatal("NewApp() returned nil")
	}
	
	if app.configMgr == nil {
		t.Error("App should have configMgr")
	}
	
	if app.sessionMgr == nil {
		t.Error("App should have sessionMgr")
	}
	
	if app.i18nMgr == nil {
		t.Error("App should have i18nMgr")
	}
	
	// AI manager might be nil if configuration is not set up
	// This is acceptable in tests
}

func TestApp_SetConnection(t *testing.T) {
	app := createTestApp(t)
	
	// Disable AI manager to avoid vector database initialization issues
	app.aiManager = nil
	
	mockConn := &mockConnection{
		dbType: core.PostgreSQL,
		name:   "test-db",
		tables: []string{"users", "posts"},
	}
	
	config := &core.ConnectionConfig{
		Name:     "test-db",
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		Username: "testuser",
	}
	
	app.SetConnection(mockConn, config)
	
	if app.connection == nil {
		t.Error("Connection should be set")
	}
	
	if app.config != config {
		t.Error("Config should be set")
	}
}

func TestApp_ClearConnection(t *testing.T) {
	app := createTestApp(t)
	
	// Disable AI manager to avoid vector database initialization issues
	app.aiManager = nil
	
	mockConn := &mockConnection{
		dbType: core.PostgreSQL,
		name:   "test-db",
		connected: true,
	}
	
	config := &core.ConnectionConfig{
		Name: "test-db",
	}
	
	app.SetConnection(mockConn, config)
	
	err := app.ClearConnection()
	if err != nil {
		t.Errorf("ClearConnection() failed: %v", err)
	}
	
	if app.connection != nil {
		t.Error("Connection should be cleared")
	}
	
	if app.config != nil {
		t.Error("Config should be cleared")
	}
}

func TestApp_processCommand(t *testing.T) {
	app := createTestApp(t)
	
	testCases := []struct {
		name        string
		command     string
		expectError bool
	}{
		{
			name:        "Help command",
			command:     "/help",
			expectError: false,
		},
		{
			name:        "Status command", 
			command:     "/status",
			expectError: false,
		},
		{
			name:        "List connections command",
			command:     "/list-connections",
			expectError: false,
		},
		{
			name:        "Invalid command",
			command:     "/invalid-command",
			expectError: false, // processCommand prints message but doesn't return error
		},
		{
			name:        "Empty command",
			command:     "/",
			expectError: false, // processCommand prints message but doesn't return error
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := app.processCommand(tc.command)
			
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for command '%s', but got none", tc.command)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for command '%s': %v", tc.command, err)
				}
			}
		})
	}
}

func TestApp_parseQueries(t *testing.T) {
	app := createTestApp(t)
	
	testCases := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "Single query",
			content:  "SELECT * FROM users;",
			expected: []string{"SELECT * FROM users; "},
		},
		{
			name:     "Multiple queries",
			content:  "SELECT * FROM users; SELECT * FROM posts;",
			expected: []string{"SELECT * FROM users; SELECT * FROM posts; "},
		},
		{
			name:     "Query with comments",
			content:  "-- Get all users\nSELECT * FROM users;\n-- Get all posts\nSELECT * FROM posts;",
			expected: []string{"SELECT * FROM users; ", "SELECT * FROM posts; "},
		},
		{
			name:     "Empty content",
			content:  "",
			expected: []string{},
		},
		{
			name:     "Only comments",
			content:  "-- This is a comment\n-- Another comment",
			expected: []string{},
		},
		{
			name:     "Query without semicolon",
			content:  "SELECT * FROM users",
			expected: []string{"SELECT * FROM users"},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := app.parseQueries(tc.content)
			
			if len(result) != len(tc.expected) {
				t.Errorf("Expected %d queries, got %d", len(tc.expected), len(result))
				return
			}
			
			for i, expected := range tc.expected {
				if strings.TrimSpace(result[i]) != strings.TrimSpace(expected) {
					t.Errorf("Query %d: expected '%s', got '%s'", i, expected, result[i])
				}
			}
		})
	}
}

func TestApp_truncateQuery(t *testing.T) {
	app := createTestApp(t)
	
	testCases := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "Short query",
			query:    "SELECT * FROM users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "Long query",
			query:    strings.Repeat("SELECT * FROM users WHERE id = 1 AND ", 10) + "name = 'test'",
			expected: "SELECT * FROM users WHERE id = 1 AND SELECT * F...",
		},
		{
			name:     "Empty query",
			query:    "",
			expected: "",
		},
		{
			name:     "Exactly 100 characters",
			query:    strings.Repeat("a", 100),
			expected: strings.Repeat("a", 47) + "...",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := app.truncateQuery(tc.query)
			
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
			
			// Ensure result is never longer than 100 characters
			if len(result) > 100 {
				t.Errorf("Truncated query should not exceed 100 characters, got %d", len(result))
			}
		})
	}
}

func TestApp_generateTableMarkdown(t *testing.T) {
	app := createTestApp(t)
	
	tableInfo := &core.TableInfo{
		Name: "users",
		Columns: []core.ColumnInfo{
			{
				Name:     "id",
				Type:     "INTEGER",
				Key:      "PRI",
				Nullable: false,
			},
			{
				Name:     "name",
				Type:     "VARCHAR(255)",
				Nullable: true,
			},
			{
				Name:     "email",
				Type:     "VARCHAR(255)",
				Key:      "UNI",
				Nullable: false,
			},
		},
	}
	
	markdown := app.generateTableMarkdown(tableInfo)
	
	// Check that markdown contains expected elements
	if !strings.Contains(markdown, "# 📊 Table: users") {
		t.Error("Markdown should contain table name as header")
	}
	
	if !strings.Contains(markdown, "| Column | Type | Nullable | Key | Default |") {
		t.Error("Markdown should contain table header")
	}
	
	if !strings.Contains(markdown, "| **id** | `INTEGER` | ❌ NOT NULL | 🔑 PRI |") {
		t.Error("Markdown should contain primary key column info")
	}
	
	if !strings.Contains(markdown, "| **name** | `VARCHAR(255)` | ✅ NULL |") {
		t.Error("Markdown should contain nullable column info")
	}
	
	if !strings.Contains(markdown, "| **email** | `VARCHAR(255)` | ❌ NOT NULL | 🔑 UNI |") {
		t.Error("Markdown should contain unique column info")
	}
}

func TestApp_handleConnect_WithArgs(t *testing.T) {
	app := createTestApp(t)
	
	// Create a test connection configuration
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "connections.yaml")
	
	configContent := `connections:
  - name: "test-db"
    type: "postgresql"
    host: "localhost"
    port: 5432
    database: "testdb"
    username: "testuser"
    password: "testpass"
`
	
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}
	
	// Test with non-existent connection
	err = app.handleConnect([]string{"nonexistent"})
	if err == nil {
		t.Error("Expected error for non-existent connection")
	}
	
	// Test with empty args (should trigger interactive mode, but we can't test that easily)
	// We'll test that it doesn't panic
	err = app.handleConnect([]string{})
	// Interactive mode should not return an error immediately
}

func TestApp_handleListTables_NoConnection(t *testing.T) {
	app := createTestApp(t)
	
	err := app.handleListTables()
	if err != nil {
		t.Errorf("handleListTables() should not return error without connection, got: %v", err)
	}
}

func TestApp_handleListTables_WithConnection(t *testing.T) {
	app := createTestApp(t)
	
	// Disable AI manager to avoid vector database initialization issues
	app.aiManager = nil
	
	mockConn := &mockConnection{
		tables: []string{"users", "posts", "comments"},
		connected: true,
		dbType: core.PostgreSQL,
		name:   "test-db",
	}
	
	config := &core.ConnectionConfig{
		Name: "test-db",
	}
	
	app.SetConnection(mockConn, config)
	
	err := app.handleListTables()
	if err != nil {
		t.Errorf("handleListTables() failed: %v", err)
	}
}

func TestApp_handleDescribeTable(t *testing.T) {
	app := createTestApp(t)
	
	// Test without connection
	err := app.handleDescribeTable([]string{"users"})
	if err != nil {
		t.Errorf("handleDescribeTable() should not return error without connection, got: %v", err)
	}
	
	// Test with connection
	// Disable AI manager to avoid vector database initialization issues
	app.aiManager = nil
	
	mockConn := &mockConnection{
		tables: []string{"users", "posts"},
		connected: true,
		dbType: core.PostgreSQL,
		name:   "test-db",
	}
	
	config := &core.ConnectionConfig{
		Name: "test-db",
	}
	
	app.SetConnection(mockConn, config)
	
	// Test with valid table
	err = app.handleDescribeTable([]string{"users"})
	if err != nil {
		t.Errorf("handleDescribeTable() failed: %v", err)
	}
	
	// Test with no args
	err = app.handleDescribeTable([]string{})
	if err != nil {
		t.Errorf("handleDescribeTable() should not return error with no args, got: %v", err)
	}
}

// Benchmark tests
func BenchmarkApp_parseQueries(b *testing.B) {
	app := createTestApp(&testing.T{})
	
	content := "SELECT * FROM users; SELECT * FROM posts; SELECT COUNT(*) FROM comments;"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.parseQueries(content)
	}
}

func BenchmarkApp_truncateQuery(b *testing.B) {
	app := createTestApp(&testing.T{})
	
	query := strings.Repeat("SELECT * FROM users WHERE id = 1 AND ", 10) + "name = 'test'"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.truncateQuery(query)
	}
}

func BenchmarkApp_generateTableMarkdown(b *testing.B) {
	app := createTestApp(&testing.T{})
	
	tableInfo := &core.TableInfo{
		Name: "users",
		Columns: []core.ColumnInfo{
			{Name: "id", Type: "INTEGER", Key: "PRI"},
			{Name: "name", Type: "VARCHAR(255)", Nullable: true},
			{Name: "email", Type: "VARCHAR(255)", Key: "UNI"},
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.generateTableMarkdown(tableInfo)
	}
}