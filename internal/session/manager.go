package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Manager struct {
	configDir string
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
	return os.MkdirAll(sessionDir, 0755)
}

func (m *Manager) ViewMarkdownWithGlow(filePath string) error {
	// Check if glow is installed
	if _, err := exec.LookPath("glow"); err != nil {
		return fmt.Errorf("glow is not installed. Install with: go install github.com/charmbracelet/glow@latest")
	}

	// Run glow with the markdown file
	cmd := exec.Command("glow", "-p", filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run glow: %w", err)
	}

	return nil
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