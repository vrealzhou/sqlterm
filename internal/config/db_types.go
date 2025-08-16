package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the main configuration with AI section
type Config struct {
	Language    string                      `yaml:"language"`
	Connections map[string]ConnectionConfig `yaml:"connections"`
	AI          AIConfig                    `yaml:"ai"`
}

type DatabaseType int

const (
	MySQL DatabaseType = iota
	PostgreSQL
	SQLite
)

func (dt DatabaseType) String() string {
	switch dt {
	case MySQL:
		return "mysql"
	case PostgreSQL:
		return "postgres"
	case SQLite:
		return "sqlite"
	default:
		return "unknown"
	}
}

func ParseDatabaseType(s string) (DatabaseType, error) {
	switch strings.ToLower(s) {
	case "mysql":
		return MySQL, nil
	case "postgres", "postgresql":
		return PostgreSQL, nil
	case "sqlite", "sqlite3":
		return SQLite, nil
	default:
		return 0, fmt.Errorf("unsupported database type: %s. Supported types: mysql, postgres, sqlite", s)
	}
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for DatabaseType
func (dt *DatabaseType) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	
	parsed, err := ParseDatabaseType(s)
	if err != nil {
		return err
	}
	
	*dt = parsed
	return nil
}

// MarshalYAML implements the yaml.Marshaler interface for DatabaseType
func (dt DatabaseType) MarshalYAML() (interface{}, error) {
	return dt.String(), nil
}

func GetDefaultPort(dbType DatabaseType) int {
	switch dbType {
	case MySQL:
		return 3306
	case PostgreSQL:
		return 5432
	case SQLite:
		return 0
	default:
		return 0
	}
}

type ConnectionConfig struct {
	Name         string       `yaml:"name"`
	DatabaseType DatabaseType `yaml:"database_type"`
	Host         string       `yaml:"host"`
	Port         int          `yaml:"port"`
	Database     string       `yaml:"database"`
	Username     string       `yaml:"username"`
	Password     string       `yaml:"password,omitempty"`
	SSL          bool         `yaml:"ssl"`
}
