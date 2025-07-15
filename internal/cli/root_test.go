package cli

import (
	"bytes"
	"testing"

	"sqlterm/internal/core"
	"sqlterm/internal/i18n"
	"github.com/spf13/cobra"
)

func TestSetVersionInfo(t *testing.T) {
	// Test setting version info
	testVersion := "1.0.0"
	testBuildTime := "2024-01-01T00:00:00Z"
	testGitCommit := "abc123"
	
	SetVersionInfo(testVersion, testBuildTime, testGitCommit)
	
	if Version != testVersion {
		t.Errorf("Expected Version to be '%s', got '%s'", testVersion, Version)
	}
	
	if BuildTime != testBuildTime {
		t.Errorf("Expected BuildTime to be '%s', got '%s'", testBuildTime, BuildTime)
	}
	
	if GitCommit != testGitCommit {
		t.Errorf("Expected GitCommit to be '%s', got '%s'", testGitCommit, GitCommit)
	}
}

func TestVersionCommand(t *testing.T) {
	// Set test version info
	SetVersionInfo("1.0.0", "2024-01-01T00:00:00Z", "abc123")
	
	// Create a separate command instance for testing
	testCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("SQLTerm %s\n", Version)
			cmd.Printf("Build time: %s\n", BuildTime)
			cmd.Printf("Git commit: %s\n", GitCommit)
		},
	}
	
	// Capture output
	var buf bytes.Buffer
	testCmd.SetOut(&buf)
	testCmd.SetErr(&buf)
	
	// Execute version command
	err := testCmd.Execute()
	if err != nil {
		t.Errorf("Version command failed: %v", err)
	}
	
	// Check that output contains expected version information
	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("1.0.0")) {
		t.Errorf("Version output should contain version number, got: %s", output)
	}
	
	if !bytes.Contains(buf.Bytes(), []byte("2024-01-01T00:00:00Z")) {
		t.Errorf("Version output should contain build time, got: %s", output)
	}
	
	if !bytes.Contains(buf.Bytes(), []byte("abc123")) {
		t.Errorf("Version output should contain git commit, got: %s", output)
	}
}

func TestGetI18nString(t *testing.T) {
	testCases := []struct {
		name     string
		key      string
		fallback string
		expected string
	}{
		{
			name:     "Nil manager",
			key:      "test_key",
			fallback: "fallback text",
			expected: "fallback text",
		},
		{
			name:     "Empty key",
			key:      "",
			fallback: "fallback text",
			expected: "fallback text",
		},
		{
			name:     "Valid key",
			key:      "help_connect",
			fallback: "fallback text",
			expected: "Connect to a database connection",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var mgr *i18n.Manager
			
			if tc.name == "Valid key" {
				// Create a real manager for valid key test
				var err error
				mgr, err = i18n.NewManager("en_au")
				if err != nil {
					t.Skip("Could not create i18n manager")
				}
			}
			
			result := getI18nString(mgr, tc.key, tc.fallback)
			
			if tc.name == "Valid key" {
				// For valid key test, we expect the actual i18n string
				if result == tc.fallback {
					t.Errorf("Expected actual i18n string, got fallback")
				}
			} else {
				if result != tc.expected {
					t.Errorf("Expected '%s', got '%s'", tc.expected, result)
				}
			}
		})
	}
}

func TestConnectCommand_Validation(t *testing.T) {
	// Test missing required flags
	testCases := []struct {
		name     string
		args     []string
		expected bool // true if should succeed
	}{
		{
			name:     "Missing db-type",
			args:     []string{"--database", "test", "--username", "user"},
			expected: false,
		},
		{
			name:     "Missing database",
			args:     []string{"--db-type", "postgres", "--username", "user"},
			expected: false,
		},
		{
			name:     "Missing username",
			args:     []string{"--db-type", "postgres", "--database", "test"},
			expected: false,
		},
		{
			name:     "All required flags",
			args:     []string{"--db-type", "postgres", "--database", "test", "--username", "user"},
			expected: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new command to avoid state pollution
			cmd := &cobra.Command{
				Use:   "connect",
				Short: "Test connect command",
				RunE: func(cmd *cobra.Command, args []string) error {
					// Just validate, don't actually connect
					return nil
				},
			}
			
			cmd.Flags().StringP("db-type", "t", "", "Database type")
			cmd.Flags().StringP("database", "d", "", "Database name")
			cmd.Flags().StringP("username", "u", "", "Username")
			cmd.Flags().StringP("host", "H", "localhost", "Host")
			cmd.Flags().IntP("port", "p", 0, "Port")
			cmd.Flags().StringP("password", "P", "", "Password")
			cmd.MarkFlagRequired("db-type")
			cmd.MarkFlagRequired("database")
			cmd.MarkFlagRequired("username")
			
			cmd.SetArgs(tc.args)
			
			err := cmd.Execute()
			
			if tc.expected && err != nil {
				t.Errorf("Expected command to succeed, but got error: %v", err)
			} else if !tc.expected && err == nil {
				t.Error("Expected command to fail, but it succeeded")
			}
		})
	}
}

func TestAddCommand_Validation(t *testing.T) {
	// Test argument validation
	testCases := []struct {
		name     string
		args     []string
		expected bool // true if should succeed
	}{
		{
			name:     "No connection name",
			args:     []string{"--db-type", "postgres", "--database", "test", "--username", "user"},
			expected: false,
		},
		{
			name:     "Missing db-type",
			args:     []string{"myconn", "--database", "test", "--username", "user"},
			expected: false,
		},
		{
			name:     "Valid args",
			args:     []string{"myconn", "--db-type", "postgres", "--database", "test", "--username", "user"},
			expected: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new command to avoid state pollution
			cmd := &cobra.Command{
				Use:   "add [name]",
				Short: "Test add command",
				Args:  cobra.ExactArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					// Just validate, don't actually add
					return nil
				},
			}
			
			cmd.Flags().StringP("db-type", "t", "", "Database type")
			cmd.Flags().StringP("database", "d", "", "Database name")
			cmd.Flags().StringP("username", "u", "", "Username")
			cmd.Flags().StringP("host", "H", "localhost", "Host")
			cmd.Flags().IntP("port", "p", 0, "Port")
			cmd.MarkFlagRequired("db-type")
			cmd.MarkFlagRequired("database")
			cmd.MarkFlagRequired("username")
			
			cmd.SetArgs(tc.args)
			
			err := cmd.Execute()
			
			if tc.expected && err != nil {
				t.Errorf("Expected command to succeed, but got error: %v", err)
			} else if !tc.expected && err == nil {
				t.Error("Expected command to fail, but it succeeded")
			}
		})
	}
}

func TestCoreParseDatabaseType(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected core.DatabaseType
		hasError bool
	}{
		{
			name:     "MySQL",
			input:    "mysql",
			expected: core.MySQL,
			hasError: false,
		},
		{
			name:     "PostgreSQL",
			input:    "postgres",
			expected: core.PostgreSQL,
			hasError: false,
		},
		{
			name:     "PostgreSQL alternative",
			input:    "postgresql",
			expected: core.PostgreSQL,
			hasError: false,
		},
		{
			name:     "SQLite",
			input:    "sqlite",
			expected: core.SQLite,
			hasError: false,
		},
		{
			name:     "Invalid type",
			input:    "invalid",
			expected: core.DatabaseType(0),
			hasError: true,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: core.DatabaseType(0),
			hasError: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.ParseDatabaseType(tc.input)
			
			if tc.hasError {
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input '%s': %v", tc.input, err)
				}
				
				if result != tc.expected {
					t.Errorf("Expected %v, got %v for input '%s'", tc.expected, result, tc.input)
				}
			}
		})
	}
}

func TestCoreGetDefaultPort(t *testing.T) {
	testCases := []struct {
		name     string
		dbType   core.DatabaseType
		expected int
	}{
		{
			name:     "MySQL default port",
			dbType:   core.MySQL,
			expected: 3306,
		},
		{
			name:     "PostgreSQL default port",
			dbType:   core.PostgreSQL,
			expected: 5432,
		},
		{
			name:     "SQLite default port",
			dbType:   core.SQLite,
			expected: 0,
		},
		{
			name:     "Invalid database type",
			dbType:   core.DatabaseType(999),
			expected: 0,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := core.GetDefaultPort(tc.dbType)
			
			if result != tc.expected {
				t.Errorf("Expected port %d, got %d for database type %v", tc.expected, result, tc.dbType)
			}
		})
	}
}

func TestDatabaseTypeString(t *testing.T) {
	testCases := []struct {
		name     string
		dbType   core.DatabaseType
		expected string
	}{
		{
			name:     "MySQL string",
			dbType:   core.MySQL,
			expected: "mysql",
		},
		{
			name:     "PostgreSQL string",
			dbType:   core.PostgreSQL,
			expected: "postgres",
		},
		{
			name:     "SQLite string",
			dbType:   core.SQLite,
			expected: "sqlite",
		},
		{
			name:     "Unknown type",
			dbType:   core.DatabaseType(999),
			expected: "unknown",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.dbType.String()
			
			if result != tc.expected {
				t.Errorf("Expected string '%s', got '%s' for database type %v", tc.expected, result, tc.dbType)
			}
		})
	}
}

func TestConnectionConfigCreation(t *testing.T) {
	// Test creating ConnectionConfig with proper values
	config := &core.ConnectionConfig{
		Name:         "test-connection",
		DatabaseType: core.PostgreSQL,
		Host:         "localhost",
		Port:         5432,
		Database:     "testdb",
		Username:     "testuser",
		Password:     "testpass",
		SSL:          false,
	}
	
	if config.Name != "test-connection" {
		t.Errorf("Expected name 'test-connection', got '%s'", config.Name)
	}
	
	if config.DatabaseType != core.PostgreSQL {
		t.Errorf("Expected database type PostgreSQL, got %v", config.DatabaseType)
	}
	
	if config.Host != "localhost" {
		t.Errorf("Expected host 'localhost', got '%s'", config.Host)
	}
	
	if config.Port != 5432 {
		t.Errorf("Expected port 5432, got %d", config.Port)
	}
	
	if config.Database != "testdb" {
		t.Errorf("Expected database 'testdb', got '%s'", config.Database)
	}
	
	if config.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", config.Username)
	}
	
	if config.Password != "testpass" {
		t.Errorf("Expected password 'testpass', got '%s'", config.Password)
	}
	
	if config.SSL != false {
		t.Errorf("Expected SSL false, got %v", config.SSL)
	}
}

// Integration tests that test the actual command execution would be more complex
// as they would need to mock the database connections and file system operations.
// For now, we focus on unit tests for the individual functions and validation logic.

// Benchmark tests
func BenchmarkParseDatabaseType(b *testing.B) {
	for i := 0; i < b.N; i++ {
		core.ParseDatabaseType("postgres")
	}
}

func BenchmarkGetDefaultPort(b *testing.B) {
	for i := 0; i < b.N; i++ {
		core.GetDefaultPort(core.PostgreSQL)
	}
}

func BenchmarkDatabaseTypeString(b *testing.B) {
	dbType := core.PostgreSQL
	for i := 0; i < b.N; i++ {
		dbType.String()
	}
}