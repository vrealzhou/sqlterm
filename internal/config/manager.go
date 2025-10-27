package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Manager struct {
	configDir   string
	Config      *Config
	connections []*ConnectionConfig
}

func NewManager() *Manager {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("failed to get user home directory: %v", err))
	}

	configDir := filepath.Join(home, ".config", "sqlterm")
	if err := os.MkdirAll(filepath.Dir(configDir), 0755); err != nil {
		panic(fmt.Sprintf("failed to create config directory: %v", err))
	}

	return &Manager{
		configDir: configDir,
	}
}

func (m *Manager) GetConfigDir() string {
	return m.configDir
}

func (m *Manager) LoadConfig() error {
	f, err := os.Open(filepath.Join(m.configDir, "config.yaml"))
	if err != nil {
		return fmt.Errorf("failed on opening config file %s: %w", m.configDir, err)
	}
	defer f.Close()

	// Parse YAML while preserving key order
	var yamlNode yaml.Node
	if err := yaml.NewDecoder(f).Decode(&yamlNode); err != nil {
		return fmt.Errorf("failed on parsing config file %s: %w", m.configDir, err)
	}

	// Also parse normally for the config struct
	f.Seek(0, 0) // Reset file position
	config := &Config{}
	if err := yaml.NewDecoder(f).Decode(config); err != nil {
		return fmt.Errorf("failed on parsing config file %s: %w", m.configDir, err)
	}

	// Extract connection names in order from YAML structure
	connectionOrder := m.extractConnectionOrder(&yamlNode)

	// Build connections slice in the correct order
	m.connections = make([]*ConnectionConfig, 0, len(config.Connections))
	for _, name := range connectionOrder {
		if conn, exists := config.Connections[name]; exists {
			conn.Name = name
			m.connections = append(m.connections, &conn)
		}
	}

	m.Config = config
	return nil
}

// extractConnectionOrder extracts connection names in the order they appear in the YAML file
func (m *Manager) extractConnectionOrder(node *yaml.Node) []string {
	var connectionOrder []string

	// Navigate to the root document node
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}

	// Look for the "connections" key in the mapping
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == "connections" && i+1 < len(node.Content) {
				connectionsNode := node.Content[i+1]

				// Extract connection names in order
				if connectionsNode.Kind == yaml.MappingNode {
					for j := 0; j < len(connectionsNode.Content); j += 2 {
						connectionName := connectionsNode.Content[j].Value
						connectionOrder = append(connectionOrder, connectionName)
					}
				}
				break
			}
		}
	}

	return connectionOrder
}

func (m *Manager) SaveConfig() error {
	data, err := yaml.Marshal(m.Config)
	if err != nil {
		return fmt.Errorf("failed on marshalling config data: %w", err)
	}

	// Ensure the config directory exists
	if err := os.MkdirAll(m.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configFile := filepath.Join(m.configDir, "config.yaml")
	f, err := os.Create(configFile)
	if err != nil {
		return fmt.Errorf("failed on opening config file %s: %w", configFile, err)
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func (m *Manager) LoadConnection(name string) (*ConnectionConfig, error) {
	if m.Config == nil {
		return nil, fmt.Errorf("config not loaded")
	}

	conn, exists := m.Config.Connections[name]
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", name)
	}

	return &conn, nil
}

func (m *Manager) SaveConnection(conn *ConnectionConfig) error {
	if m.Config == nil {
		m.Config = DefaultConfig()
	}

	// Ensure connections map is initialized
	if m.Config.Connections == nil {
		m.Config.Connections = make(map[string]ConnectionConfig)
	}

	// Add or update the connection
	m.Config.Connections[conn.Name] = *conn
	return m.SaveConfig()
}

// DeleteConnection removes the connection with the provided name from the config.
func (m *Manager) DeleteConnection(name string) error {
	if name == "" {
		return fmt.Errorf("connection name is required")
	}
	if m.Config == nil || m.Config.Connections == nil {
		return nil
	}

	if _, exists := m.Config.Connections[name]; !exists {
		return nil
	}

	delete(m.Config.Connections, name)
	for i, conn := range m.connections {
		if conn != nil && conn.Name == name {
			m.connections = append(m.connections[:i], m.connections[i+1:]...)
			break
		}
	}

	return m.SaveConfig()
}

func (m *Manager) ListConnections() []*ConnectionConfig {
	if m.Config == nil {
		return nil
	}
	return m.connections
}
