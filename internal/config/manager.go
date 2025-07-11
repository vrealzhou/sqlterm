package config

import (
	"fmt"
	"os"
	"path/filepath"

	"sqlterm/internal/core"

	"gopkg.in/yaml.v3"
)

type Manager struct {
	configDir string
}

func NewManager() *Manager {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("failed to get user home directory: %v", err))
	}

	configDir := filepath.Join(home, ".config", "sqlterm")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		panic(fmt.Sprintf("failed to create config directory: %v", err))
	}

	return &Manager{
		configDir: configDir,
	}
}

func (m *Manager) GetConfigDir() string {
	return m.configDir
}

func (m *Manager) SaveConnection(config *core.ConnectionConfig) error {
	connectionsDir := filepath.Join(m.configDir, "connections")
	if err := os.MkdirAll(connectionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create connections directory: %w", err)
	}

	filename := fmt.Sprintf("%s.yaml", config.Name)
	filepath := filepath.Join(connectionsDir, filename)

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (m *Manager) LoadConnection(name string) (*core.ConnectionConfig, error) {
	filename := fmt.Sprintf("%s.yaml", name)
	filepath := filepath.Join(m.configDir, "connections", filename)

	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config core.ConnectionConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func (m *Manager) ListConnections() ([]*core.ConnectionConfig, error) {
	connectionsDir := filepath.Join(m.configDir, "connections")

	entries, err := os.ReadDir(connectionsDir)
	if os.IsNotExist(err) {
		return []*core.ConnectionConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read connections directory: %w", err)
	}

	var connections []*core.ConnectionConfig
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		name := entry.Name()[:len(entry.Name())-5] // Remove .yaml extension
		config, err := m.LoadConnection(name)
		if err != nil {
			continue // Skip corrupted files
		}

		connections = append(connections, config)
	}

	return connections, nil
}

func (m *Manager) DeleteConnection(name string) error {
	filename := fmt.Sprintf("%s.yaml", name)
	filepath := filepath.Join(m.configDir, "connections", filename)

	if err := os.Remove(filepath); err != nil {
		return fmt.Errorf("failed to delete config file: %w", err)
	}

	
return nil
}
