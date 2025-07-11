package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sqlterm/internal/core"
	"gopkg.in/yaml.v3"
)

type Manager struct {
	configDir string
}

type SessionConfig struct {
	CleanupRetentionDays int `yaml:"cleanup_retention_days"`
}

func NewManager(configDir string) *Manager {
	return &Manager{
		configDir: configDir,
	}
}

func (m *Manager) GetSessionDir(connectionName string) string {
	return filepath.Join(m.configDir, "sessions", connectionName)
}

func (m *Manager) EnsureSessionDir(connectionName string) error {
	sessionDir := m.GetSessionDir(connectionName)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return err
	}

	// Create results directory
	resultsDir := filepath.Join(sessionDir, "results")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return err
	}

	// Ensure session config exists and perform cleanup
	if err := m.ensureSessionConfig(connectionName); err != nil {
		return err
	}

	// Perform automatic cleanup
	if err := m.performAutoCleanup(connectionName); err != nil {
		// Don't fail if cleanup fails, just log a warning
		fmt.Printf("Warning: cleanup failed for %s: %v\n", connectionName, err)
	}

	return nil
}

func (m *Manager) ViewMarkdown(filePath string) error {
	// Read the markdown file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read markdown file: %w", err)
	}

	return m.DisplayMarkdown(string(content))
}

func (m *Manager) DisplayMarkdown(markdown string) error {
	// Use the shared markdown renderer
	renderer := core.NewMarkdownRenderer()
	return renderer.RenderAndDisplay(markdown)
}

func (m *Manager) CleanupOldFiles(connectionName string, retentionDays int) error {
	sessionDir := m.GetSessionDir(connectionName)
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		return nil // No session directory exists
	}

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	return filepath.Walk(sessionDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove old file %s: %w", path, err)
			}
		}

		return nil
	})
}

func (m *Manager) getSessionConfigPath(connectionName string) string {
	return filepath.Join(m.GetSessionDir(connectionName), "session.yaml")
}

func (m *Manager) getSessionConfig(connectionName string) (*SessionConfig, error) {
	configPath := m.getSessionConfigPath(connectionName)

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Return default config
		return &SessionConfig{
			CleanupRetentionDays: 30, // Default to 30 days
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session config: %w", err)
	}

	var config SessionConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse session config: %w", err)
	}

	// Set default if not specified
	if config.CleanupRetentionDays <= 0 {
		config.CleanupRetentionDays = 30
	}

	return &config, nil
}

func (m *Manager) saveSessionConfig(connectionName string, config *SessionConfig) error {
	configPath := m.getSessionConfigPath(connectionName)

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal session config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write session config: %w", err)
	}

	return nil
}

func (m *Manager) ensureSessionConfig(connectionName string) error {
	configPath := m.getSessionConfigPath(connectionName)

	// Check if YAML config already exists
	if _, err := os.Stat(configPath); err == nil {
		return nil // Config already exists
	}

	// Check for old TOML config and migrate if exists
	oldTomlPath := filepath.Join(m.GetSessionDir(connectionName), "session.toml")
	if _, err := os.Stat(oldTomlPath); err == nil {
		if migrateErr := m.migrateTOMLToYAML(connectionName, oldTomlPath, configPath); migrateErr != nil {
			fmt.Printf("Warning: failed to migrate TOML config: %v\n", migrateErr)
		} else {
			fmt.Printf("📁 Migrated session.toml to session.yaml for %s\n", connectionName)
			return nil
		}
	}

	// Create default config
	defaultConfig := &SessionConfig{
		CleanupRetentionDays: 30,
	}

	if err := m.saveSessionConfig(connectionName, defaultConfig); err != nil {
		return err
	}

	fmt.Printf("📁 Created session.yaml for %s (cleanup_retention_days: %d)\n", connectionName, defaultConfig.CleanupRetentionDays)
	return nil
}

func (m *Manager) migrateTOMLToYAML(connectionName, oldPath, newPath string) error {
	// Read old TOML file (we'll do basic text parsing since we only care about cleanup_retention_days)
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return fmt.Errorf("failed to read TOML file: %w", err)
	}

	// Simple parsing for cleanup_retention_days
	config := &SessionConfig{
		CleanupRetentionDays: 30, // Default
	}

	// Basic parsing - look for cleanup_retention_days = value
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "cleanup_retention_days") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				if retentionDays, parseErr := strconv.Atoi(value); parseErr == nil && retentionDays > 0 {
					config.CleanupRetentionDays = retentionDays
				}
			}
		}
	}

	// Save as YAML
	if err := m.saveSessionConfig(connectionName, config); err != nil {
		return fmt.Errorf("failed to save migrated config: %w", err)
	}

	// Remove old TOML file
	if err := os.Remove(oldPath); err != nil {
		fmt.Printf("Warning: failed to remove old TOML file %s: %v\n", oldPath, err)
	}

	return nil
}

func (m *Manager) performAutoCleanup(connectionName string) error {
	config, err := m.getSessionConfig(connectionName)
	if err != nil {
		return fmt.Errorf("failed to get session config: %w", err)
	}

	// Only cleanup results directory, not the entire session
	resultsDir := filepath.Join(m.GetSessionDir(connectionName), "results")
	return m.cleanupDirectory(resultsDir, config.CleanupRetentionDays)
}

func (m *Manager) cleanupDirectory(dirPath string, retentionDays int) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return nil // Directory doesn't exist
	}

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only remove files, not directories
		if !info.IsDir() && info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove old file %s: %w", path, err)
			}
		}

		return nil
	})
}
