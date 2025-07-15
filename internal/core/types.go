package core

import (
	"database/sql"
	"fmt"
	"iter"
	"strings"
	"time"
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
	case "sqlite", "sqlite3":
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
	Null  bool
}

func (s StringValue) String() string {
	if s.Null {
		return ""
	}
	return s.Value
}

func (s StringValue) IsNull() bool {
	return s.Null
}

type IntValue struct {
	Value int64
	Null  bool
}

func (i IntValue) String() string {
	if i.Null {
		return ""
	}
	return fmt.Sprintf("%d", i.Value)
}

func (i IntValue) IsNull() bool {
	return i.Null
}

type FloatValue struct {
	Value float64
	Null  bool
}

func (f FloatValue) String() string {
	if f.Null {
		return ""
	}
	return fmt.Sprintf("%g", f.Value)
}

func (f FloatValue) IsNull() bool {
	return f.Null
}

type BoolValue struct {
	Value bool
	Null  bool
}

func (b BoolValue) String() string {
	if b.Null {
		return ""
	}
	return fmt.Sprintf("%t", b.Value)
}

func (b BoolValue) IsNull() bool {
	return b.Null
}

type NullValue struct{}

func (n NullValue) String() string {
	return ""
}

func (n NullValue) IsNull() bool {
	return true
}

type Column struct {
	Name string
	Type string
}

type QueryResult struct {
	Columns []Column
	rows    *sql.Rows
	err     error
}

func (r *QueryResult) ColumnNames() []string {
	names := make([]string, len(r.Columns))
	for i, col := range r.Columns {
		names[i] = col.Name
	}
	return names
}

func NewQueryResult(rows *sql.Rows) (*QueryResult, error) {
	columnNames, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	columns := make([]Column, len(columnNames))
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("failed to get column types: %w", err)
	}
	for i, tp := range columnTypes {
		columns[i] = Column{
			Name: columnNames[i],
			Type: tp.Name(),
		}
	}

	return &QueryResult{
		Columns: columns,
		rows:    rows,
	}, nil
}

func (r *QueryResult) Close() error {
	return r.rows.Close()
}

func assambleRow(columns []Column, rows *sql.Rows) ([]Value, error) {
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
			case time.Time:
				// Format datetime/timestamp to "2006-01-02 15:04:05-0700"
				row[i] = StringValue{Value: v.Format("2006-01-02 15:04:05-0700")}
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
	Name        string
	Columns     []ColumnInfo
	PrimaryKeys []string
	Constraints []ConstraintInfo
	ForeignKeys []ForeignKeyInfo
}

type ConstraintInfo struct {
	Name   string
	Type   string
	Column string
	Check  string
}

type ForeignKeyInfo struct {
	Name             string
	Column           string
	ReferencedTable  string
	ReferencedColumn string
	OnDelete         string
	OnUpdate         string
}

type ColumnInfo struct {
	Name     string
	Type     string
	Nullable bool
	Key      string
	Default  *string
	Extra    string
}
