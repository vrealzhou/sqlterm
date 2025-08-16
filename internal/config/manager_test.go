package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestManager_LoadConfigFile(t *testing.T) {
	// Save original home to restore later
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	manager := NewManager()

	// Create a valid config file
	configContent := `
language: en_au
ai:
  provider: openrouter
  model: anthropic/claude-3.5-sonnet
  api_keys:
    openrouter: test-key
connections:
  test-db:
    database_type: mysql
    host: localhost
    port: 3306
    database: testdb
`

	configFile := filepath.Join(manager.configDir, "config.yaml")
	err := os.MkdirAll(filepath.Dir(configFile), 0755)
	if err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Override the LoadConfig to use the actual file instead of directory
	testConfigDir := filepath.Dir(configFile)
	manager.configDir = testConfigDir

	err = manager.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if manager.Config == nil {
		t.Fatal("Expected config to be loaded, got nil")
	}

	if manager.Config.Language != "en_au" {
		t.Errorf("Expected language 'en_au', got '%s'", manager.Config.Language)
	}

	if manager.Config.AI.Provider != ProviderOpenRouter {
		t.Errorf("Expected provider openrouter, got '%s'", manager.Config.AI.Provider)
	}

	if len(manager.Config.Connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(manager.Config.Connections))
	}

	testConn, exists := manager.Config.Connections["test-db"]
	if !exists {
		t.Error("Expected connection 'test-db' to exist")
	} else {
		if testConn.Name != "test-db" {
			t.Errorf("Expected connection name 'test-db', got '%s'", testConn.Name)
		}
		if testConn.DatabaseType != MySQL {
			t.Errorf("Expected database type MySQL, got %v", testConn.DatabaseType)
		}
	}
}

func TestManager_LoadConfig_NonExistentFile(t *testing.T) {
	// Save original home to restore later
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	manager := NewManager()

	// Try to load config when file doesn't exist
	err := manager.LoadConfig()
	if err == nil {
		t.Error("Expected error when loading non-existent config file, got none")
	}
}

func TestManager_LoadConfig_InvalidYAML(t *testing.T) {
	// Save original home to restore later
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	manager := NewManager()

	// Create an invalid YAML file
	invalidContent := `
language: en_au
ai:
  provider: openrouter
  invalid: [unclosed array
`

	configFile := filepath.Join(manager.configDir, "config.yaml")
	err := os.MkdirAll(filepath.Dir(configFile), 0755)
	if err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}
	err = os.WriteFile(configFile, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Override the LoadConfig to use the actual file instead of directory
	testConfigDir := filepath.Dir(configFile)
	manager.configDir = testConfigDir

	err = manager.LoadConfig()
	if err == nil {
		t.Error("Expected error when loading invalid YAML, got none")
	}
}

func TestManager_SaveConfig(t *testing.T) {
	// Save original home to restore later
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	manager := NewManager()
	manager.Config = DefaultConfig()

	// Set some values
	manager.Config.Language = "en_us"
	manager.Config.AI.Provider = ProviderLMStudio

	err := manager.SaveConfig()
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Check that file was created
	configFile := filepath.Join(manager.configDir, "config.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Errorf("Expected config file to be created at %s", configFile)
	}

	// Read back and verify content
	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read saved config file: %v", err)
	}

	// Basic checks that the content contains expected values
	contentStr := string(content)
	if !containsString(contentStr, "language: en_us") {
		t.Error("Expected saved config to contain language: en_us")
	}

	if !containsString(contentStr, "provider: lmstudio") {
		t.Error("Expected saved config to contain provider: lmstudio")
	}
}

func TestManager_LoadConnection(t *testing.T) {
	manager := NewManager()
	manager.Config = DefaultConfig()

	// Add test connections
	testConnections := map[string]ConnectionConfig{
		"test-mysql": {
			Name:         "test-mysql",
			DatabaseType: MySQL,
			Host:         "localhost",
			Port:         3306,
			Database:     "testdb",
			Username:     "user",
		},
		"test-sqlite": {
			Name:         "test-sqlite",
			DatabaseType: SQLite,
			Database:     "/tmp/test.db",
		},
	}

	manager.Config.Connections = testConnections

	// Test loading existing connection
	conn, err := manager.LoadConnection("test-mysql")
	if err != nil {
		t.Fatalf("LoadConnection failed: %v", err)
	}

	if conn.Name != "test-mysql" {
		t.Errorf("Expected connection name 'test-mysql', got '%s'", conn.Name)
	}

	if conn.DatabaseType != MySQL {
		t.Errorf("Expected database type MySQL, got %s", conn.DatabaseType)
	}

	if conn.Host != "localhost" {
		t.Errorf("Expected host 'localhost', got '%s'", conn.Host)
	}

	// Test loading non-existent connection
	_, err = manager.LoadConnection("non-existent")
	if err == nil {
		t.Error("Expected error when loading non-existent connection, got none")
	}
}

func TestManager_LoadConnection_NoConfig(t *testing.T) {
	manager := NewManager()
	// Don't set Config (leave it nil)

	_, err := manager.LoadConnection("test")
	if err == nil {
		t.Error("Expected error when loading connection with no config, got none")
	}

	expectedMsg := "config not loaded"
	if !containsString(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestManager_SaveConnection_NewConnection(t *testing.T) {
	// Save original home to restore later
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	manager := NewManager()
	// Don't set Config initially to test auto-initialization

	newConn := &ConnectionConfig{
		Name:         "new-connection",
		DatabaseType: PostgreSQL,
		Host:         "localhost",
		Port:         5432,
		Database:     "postgres",
		Username:     "postgres",
	}

	// Override configDir to point to a file in temp directory
	configFile := filepath.Join(tmpDir, "test-config.yaml")
	manager.configDir = configFile

	err := manager.SaveConnection(newConn)
	if err != nil {
		t.Fatalf("SaveConnection failed: %v", err)
	}

	// Check that config was initialized
	if manager.Config == nil {
		t.Error("Expected config to be initialized")
	}

	// Check that connection was added
	if len(manager.Config.Connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(manager.Config.Connections))
	}

	savedConn, exists := manager.Config.Connections[newConn.Name]
	if !exists {
		t.Fatalf("Expected connection '%s' to exist", newConn.Name)
	}

	if savedConn.Name != newConn.Name {
		t.Errorf("Expected connection name '%s', got '%s'", newConn.Name, savedConn.Name)
	}

	if savedConn.DatabaseType != newConn.DatabaseType {
		t.Errorf("Expected database type '%s', got '%s'", newConn.DatabaseType, savedConn.DatabaseType)
	}
}

func TestManager_SaveConnection_UpdateExisting(t *testing.T) {
	// Save original home to restore later
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	manager := NewManager()
	manager.Config = DefaultConfig()

	// Override configDir to point to a file in temp directory
	configFile := filepath.Join(tmpDir, "test-config.yaml")
	manager.configDir = configFile

	// Add initial connection
	initialConn := ConnectionConfig{
		Name:         "test-connection",
		DatabaseType: MySQL,
		Host:         "localhost",
		Port:         3306,
	}

	manager.Config.Connections = map[string]ConnectionConfig{
		"test-connection": initialConn,
	}

	// Update the connection
	updatedConn := &ConnectionConfig{
		Name:         "test-connection", // Same name
		DatabaseType: MySQL,
		Host:         "remote-host", // Different host
		Port:         3307,          // Different port
		Database:     "new-db",
		Username:     "new-user",
	}

	err := manager.SaveConnection(updatedConn)
	if err != nil {
		t.Fatalf("SaveConnection failed: %v", err)
	}

	// Check that we still have only one connection
	if len(manager.Config.Connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(manager.Config.Connections))
	}

	// Check that connection was updated
	savedConn, exists := manager.Config.Connections["test-connection"]
	if !exists {
		t.Fatal("Expected connection 'test-connection' to exist after update")
	}

	if savedConn.Host != "remote-host" {
		t.Errorf("Expected host 'remote-host', got '%s'", savedConn.Host)
	}

	if savedConn.Port != 3307 {
		t.Errorf("Expected port 3307, got %d", savedConn.Port)
	}

	if savedConn.Database != "new-db" {
		t.Errorf("Expected database 'new-db', got '%s'", savedConn.Database)
	}
}

func TestManager_ListConnections(t *testing.T) {
	manager := NewManager()

	// Test with nil config
	connections := manager.ListConnections()
	if connections != nil {
		t.Error("Expected nil connections when config is nil")
	}

	// Test with empty connections
	manager.Config = DefaultConfig()
	connections = manager.ListConnections()
	if len(connections) != 0 {
		t.Errorf("Expected 0 connections, got %d", len(connections))
	}

	// Test with multiple connections
	testConnections := map[string]ConnectionConfig{
		"conn1": {
			Name:         "conn1",
			DatabaseType: MySQL,
		},
		"conn2": {
			Name:         "conn2",
			DatabaseType: PostgreSQL,
		},
		"conn3": {
			Name:         "conn3",
			DatabaseType: SQLite,
		},
	}

	manager.Config.Connections = testConnections
	connections = manager.ListConnections()

	if len(connections) != 3 {
		t.Errorf("Expected 3 connections, got %d", len(connections))
	}

	// Check that all connections are returned
	for name, expected := range testConnections {
		conn, exists := connections[name]
		if !exists {
			t.Errorf("Expected connection '%s' to exist", name)
			continue
		}
		if conn.Name != expected.Name {
			t.Errorf("Expected connection name '%s', got '%s'", expected.Name, conn.Name)
		}
		if conn.DatabaseType != expected.DatabaseType {
			t.Errorf("Expected database type '%s', got '%s'", expected.DatabaseType, conn.DatabaseType)
		}
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDatabaseType_YAML_Marshalling(t *testing.T) {
	testCases := []struct {
		name     string
		dbType   DatabaseType
		expected string
	}{
		{
			name:     "MySQL",
			dbType:   MySQL,
			expected: "mysql",
		},
		{
			name:     "PostgreSQL",
			dbType:   PostgreSQL,
			expected: "postgres",
		},
		{
			name:     "SQLite",
			dbType:   SQLite,
			expected: "sqlite",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test marshalling
			yamlData, err := tc.dbType.MarshalYAML()
			if err != nil {
				t.Fatalf("MarshalYAML failed: %v", err)
			}

			yamlStr, ok := yamlData.(string)
			if !ok {
				t.Fatalf("Expected string from MarshalYAML, got %T", yamlData)
			}

			if yamlStr != tc.expected {
				t.Errorf("Expected marshalled value '%s', got '%s'", tc.expected, yamlStr)
			}

			// Test unmarshalling
			var dt DatabaseType
			node := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: tc.expected,
			}
			err = dt.UnmarshalYAML(node)
			if err != nil {
				t.Fatalf("UnmarshalYAML failed: %v", err)
			}

			if dt != tc.dbType {
				t.Errorf("Expected unmarshalled value %v, got %v", tc.dbType, dt)
			}
		})
	}
}

func TestDatabaseType_YAML_UnmarshallingVariants(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected DatabaseType
	}{
		{
			name:     "MySQL lowercase",
			input:    "mysql",
			expected: MySQL,
		},
		{
			name:     "MySQL uppercase",
			input:    "MYSQL",
			expected: MySQL,
		},
		{
			name:     "PostgreSQL as postgres",
			input:    "postgres",
			expected: PostgreSQL,
		},
		{
			name:     "PostgreSQL as postgresql",
			input:    "postgresql",
			expected: PostgreSQL,
		},
		{
			name:     "SQLite as sqlite",
			input:    "sqlite",
			expected: SQLite,
		},
		{
			name:     "SQLite as sqlite3",
			input:    "sqlite3",
			expected: SQLite,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var dt DatabaseType
			node := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: tc.input,
			}
			err := dt.UnmarshalYAML(node)
			if err != nil {
				t.Fatalf("UnmarshalYAML failed: %v", err)
			}

			if dt != tc.expected {
				t.Errorf("Expected unmarshalled value %v, got %v", tc.expected, dt)
			}
		})
	}
}

func TestDatabaseType_YAML_UnmarshallingErrors(t *testing.T) {
	testCases := []string{
		"invalid",
		"oracle",
		"mssql",
		"",
	}

	for _, input := range testCases {
		t.Run("Invalid_"+input, func(t *testing.T) {
			var dt DatabaseType
			node := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: input,
			}
			err := dt.UnmarshalYAML(node)
			if err == nil {
				t.Errorf("Expected error when unmarshalling invalid database type '%s', got none", input)
			}
		})
	}
}
