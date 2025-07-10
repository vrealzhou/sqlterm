package core

import (
	"fmt"
	"strings"
)

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
	case "sqlite":
		return SQLite, nil
	default:
		return 0, fmt.Errorf("unsupported database type: %s. Supported types: mysql, postgres, sqlite", s)
	}
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

type Value interface {
	String() string
	IsNull() bool
}

type StringValue struct {
	Value string
}

func (s StringValue) String() string {
	return s.Value
}

func (s StringValue) IsNull() bool {
	return false
}

type IntValue struct {
	Value int64
}

func (i IntValue) String() string {
	return fmt.Sprintf("%d", i.Value)
}

func (i IntValue) IsNull() bool {
	return false
}

type FloatValue struct {
	Value float64
}

func (f FloatValue) String() string {
	return fmt.Sprintf("%g", f.Value)
}

func (f FloatValue) IsNull() bool {
	return false
}

type BoolValue struct {
	Value bool
}

func (b BoolValue) String() string {
	return fmt.Sprintf("%t", b.Value)
}

func (b BoolValue) IsNull() bool {
	return false
}

type NullValue struct{}

func (n NullValue) String() string {
	return "NULL"
}

func (n NullValue) IsNull() bool {
	return true
}

type QueryResult struct {
	Columns []string
	Rows    [][]Value
}

type TableInfo struct {
	Name    string
	Columns []ColumnInfo
}

type ColumnInfo struct {
	Name     string
	Type     string
	Nullable bool
	Key      string
	Default  *string
	Extra    string
}