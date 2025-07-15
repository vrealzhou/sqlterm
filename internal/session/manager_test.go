package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()

	manager := NewManager(tmpDir)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.configDir != tmpDir {
		t.Errorf("Expected configDir to be '%s', got '%s'", tmpDir, manager.configDir)
	}
}

func TestManager_GetSessionDir(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	testCases := []struct {
		name           string
		connectionName string
		expectedSuffix string
	}{
		{
			name:           "Simple connection name",
			connectionName: "test-db",
			expectedSuffix: filepath.Join("sessions", "test-db"),
		},
		{
			name:           "Connection with spaces",
			connectionName: "my database",
			expectedSuffix: filepath.Join("sessions", "my database"),
		},
		{
			name:           "Connection with special chars",
			connectionName: "db-prod_2024",
			expectedSuffix: filepath.Join("sessions", "db-prod_2024"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sessionDir := manager.GetSessionDir(tc.connectionName)

			if !filepath.IsAbs(sessionDir) {
				t.Error("Session directory should be absolute path")
			}

			expectedPath := filepath.Join(tmpDir, tc.expectedSuffix)
			if sessionDir != expectedPath {
				t.Errorf("Expected session dir '%s', got '%s'", expectedPath, sessionDir)
			}
		})
	}
}

func TestManager_EnsureSessionDir(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	connectionName := "test-connection"

	// Initially, session directory should not exist
	sessionDir := manager.GetSessionDir(connectionName)
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Error("Session directory should not exist initially")
	}

	// EnsureSessionDir should create it
	err := manager.EnsureSessionDir(connectionName)
	if err != nil {
		t.Fatalf("EnsureSessionDir failed: %v", err)
	}

	// Now it should exist
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		t.Error("Session directory should exist after EnsureSessionDir")
	}

	// Should create results subdirectory
	resultsDir := filepath.Join(sessionDir, "results")
	if _, err := os.Stat(resultsDir); os.IsNotExist(err) {
		t.Error("Results directory should be created")
	}

	// Should create session.yaml config file
	sessionConfigPath := filepath.Join(sessionDir, "session.yaml")
	if _, err := os.Stat(sessionConfigPath); os.IsNotExist(err) {
		t.Error("Session config file should be created")
	}

	// Calling EnsureSessionDir again should not fail
	err = manager.EnsureSessionDir(connectionName)
	if err != nil {
		t.Errorf("EnsureSessionDir should not fail when directory already exists: %v", err)
	}
}

func TestManager_EnsureSessionDir_CreatesConfig(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	connectionName := "test-db"

	err := manager.EnsureSessionDir(connectionName)
	if err != nil {
		t.Fatalf("EnsureSessionDir failed: %v", err)
	}

	// Read the session config
	config, err := manager.getSessionConfig(connectionName)
	if err != nil {
		t.Fatalf("Failed to get session config: %v", err)
	}

	if config == nil {
		t.Fatal("Session config should not be nil")
	}

	// Check default values
	if config.CleanupRetentionDays != 30 {
		t.Errorf("Expected default retention days to be 30, got %d", config.CleanupRetentionDays)
	}
}

func TestManager_CleanupOldFiles(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	connectionName := "test-cleanup"

	// Create session directory
	err := manager.EnsureSessionDir(connectionName)
	if err != nil {
		t.Fatalf("EnsureSessionDir failed: %v", err)
	}

	sessionDir := manager.GetSessionDir(connectionName)
	resultsDir := filepath.Join(sessionDir, "results")

	// Create some test files with different ages
	now := time.Now()

	// Create old file (should be deleted)
	oldFile := filepath.Join(resultsDir, "old_result.md")
	err = os.WriteFile(oldFile, []byte("old content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create old file: %v", err)
	}

	// Make it old by changing access time
	oldTime := now.Add(-35 * 24 * time.Hour) // 35 days ago
	err = os.Chtimes(oldFile, oldTime, oldTime)
	if err != nil {
		t.Fatalf("Failed to change file time: %v", err)
	}

	// Create recent file (should be kept)
	recentFile := filepath.Join(resultsDir, "recent_result.md")
	err = os.WriteFile(recentFile, []byte("recent content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create recent file: %v", err)
	}

	// Run cleanup with 30 days retention
	err = manager.CleanupOldFiles(connectionName, 30)
	if err != nil {
		t.Fatalf("CleanupOldFiles failed: %v", err)
	}

	// Old file should be deleted
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Old file should be deleted after cleanup")
	}

	// Recent file should still exist
	if _, err := os.Stat(recentFile); os.IsNotExist(err) {
		t.Error("Recent file should not be deleted")
	}
}

func TestManager_getSessionConfig(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	connectionName := "test-config"

	// Initially, getting config should create default config
	config, err := manager.getSessionConfig(connectionName)
	if err != nil {
		t.Fatalf("Failed to get session config: %v", err)
	}

	if config == nil {
		t.Fatal("Session config should not be nil")
	}

	// Check default values
	if config.CleanupRetentionDays != 30 {
		t.Errorf("Expected default retention days to be 30, got %d", config.CleanupRetentionDays)
	}

	// Getting config again should return the same values
	config2, err := manager.getSessionConfig(connectionName)
	if err != nil {
		t.Fatalf("Failed to get session config second time: %v", err)
	}

	if config2.CleanupRetentionDays != config.CleanupRetentionDays {
		t.Error("Config values should be consistent between calls")
	}
}

func TestManager_ErrorHandling(t *testing.T) {
	// Test with invalid directory
	invalidDir := "/nonexistent/path/that/should/not/exist"
	manager := NewManager(invalidDir)

	// EnsureSessionDir should handle permission errors gracefully
	err := manager.EnsureSessionDir("test")
	if err == nil {
		t.Error("Expected error when creating session in invalid directory")
	}

	// Test with empty connection name
	tmpDir := t.TempDir()
	validManager := NewManager(tmpDir)

	err = validManager.EnsureSessionDir("")
	if err == nil {
		t.Error("Expected error for empty connection name")
	}
}

func TestManager_SessionDirIsolation(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// Create multiple sessions
	connections := []string{"db1", "db2", "db3"}

	for _, conn := range connections {
		err := manager.EnsureSessionDir(conn)
		if err != nil {
			t.Fatalf("Failed to create session for %s: %v", conn, err)
		}
	}

	// Each should have its own directory
	for _, conn := range connections {
		sessionDir := manager.GetSessionDir(conn)

		if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
			t.Errorf("Session directory for %s should exist", conn)
		}

		// Check that each has its own config
		config, err := manager.getSessionConfig(conn)
		if err != nil {
			t.Errorf("Failed to get config for %s: %v", conn, err)
		}

		if config == nil {
			t.Errorf("Config for %s should not be nil", conn)
		}
	}

	// Directories should be different
	dir1 := manager.GetSessionDir("db1")
	dir2 := manager.GetSessionDir("db2")

	if dir1 == dir2 {
		t.Error("Different connections should have different session directories")
	}
}

func TestManager_ConfigPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	connectionName := "test-persistence"

	// Create session and modify config
	err := manager.EnsureSessionDir(connectionName)
	if err != nil {
		t.Fatalf("EnsureSessionDir failed: %v", err)
	}

	// Get initial config
	config1, err := manager.getSessionConfig(connectionName)
	if err != nil {
		t.Fatalf("Failed to get initial config: %v", err)
	}

	// Create new manager instance (simulating restart)
	manager2 := NewManager(tmpDir)

	// Config should be the same
	config2, err := manager2.getSessionConfig(connectionName)
	if err != nil {
		t.Fatalf("Failed to get config with new manager: %v", err)
	}

	if config1.CleanupRetentionDays != config2.CleanupRetentionDays {
		t.Error("Config should persist between manager instances")
	}
}

// Benchmark tests
func BenchmarkManager_GetSessionDir(b *testing.B) {
	tmpDir := b.TempDir()
	manager := NewManager(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetSessionDir("test-connection")
	}
}

func BenchmarkManager_EnsureSessionDir(b *testing.B) {
	tmpDir := b.TempDir()
	manager := NewManager(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		connectionName := "test-connection"
		manager.EnsureSessionDir(connectionName)
	}
}

func BenchmarkManager_getSessionConfig(b *testing.B) {
	tmpDir := b.TempDir()
	manager := NewManager(tmpDir)

	// Pre-create session
	manager.EnsureSessionDir("bench-connection")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.getSessionConfig("bench-connection")
	}
}
