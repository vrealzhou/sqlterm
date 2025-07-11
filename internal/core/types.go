package core

import (
	"database/sql"
	"fmt"
	"iter"
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
	rows    *sql.Rows
	err     error
}

func NewQueryResult(rows *sql.Rows) (*QueryResult, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	return &QueryResult{
		Columns: columns,
		rows:    rows,
	}, nil
}

func (r *QueryResult) Close() error {
	return r.rows.Close()
}

func assambleRow(columns []string, rows *sql.Rows) ([]Value, error) {
	values := make([]any, len(columns))
	valuePtrs := make([]any, len(columns))

	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	row := make([]Value, len(columns))
	for i, val := range values {
		if val == nil {
			row[i] = NullValue{}
		} else {
			switch v := val.(type) {
			case string:
				row[i] = StringValue{Value: v}
			case []byte:
				row[i] = StringValue{Value: string(v)}
			case int64:
				row[i] = IntValue{Value: v}
			case float64:
				row[i] = FloatValue{Value: v}
			case bool:
				row[i] = BoolValue{Value: v}
			default:
				row[i] = StringValue{Value: fmt.Sprintf("%v", v)}
			}
		}
	}
	return row, nil
}

func (r *QueryResult) Itor() iter.Seq[[]Value] {
	return func(yield func([]Value) bool) {
		for r.rows.Next() {
			row, err := assambleRow(r.Columns, r.rows)
			if err != nil {
				r.err = err
				return
			}
			if !yield(row) {
				return
			}
		}
	}
}

func (r *QueryResult) Error() error {
	return r.err
}

type TableInfo struct {
	Name         string
	Columns      []ColumnInfo
	PrimaryKeys  []string
	Constraints  []ConstraintInfo
	ForeignKeys  []ForeignKeyInfo
}

type ConstraintInfo struct {
	Name   string
	Type   string
	Column string
	Check  string
}

type ForeignKeyInfo struct {
	Name           string
	Column         string
	ReferencedTable string
	ReferencedColumn string
	OnDelete       string
	OnUpdate       string
}

type ColumnInfo struct {
	Name     string
	Type     string
	Nullable bool
	Key      string
	Default  *string
	Extra    string
}
