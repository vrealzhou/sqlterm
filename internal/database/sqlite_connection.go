package database

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/vrealzhou/sqlterm/internal/config"
)

// SQLiteConnection represents a SQLite database connection
type SQLiteConnection struct {
	db     *sql.DB
	config *config.ConnectionConfig
}

// NewSQLiteConnection creates a new SQLite connection
func NewSQLiteConnection(cfg *config.ConnectionConfig) (Connection, error) {
	if cfg.DatabaseType != config.SQLite {
		return nil, fmt.Errorf("invalid database type for SQLite connection: %v", cfg.DatabaseType)
	}

	db, err := sql.Open("sqlite3", cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	return &SQLiteConnection{
		db:     db,
		config: cfg,
	}, nil
}

// Ping implements the Connection interface
func (c *SQLiteConnection) Ping() error {
	return c.db.Ping()
}

// Execute implements the Connection interface
func (c *SQLiteConnection) Execute(query string) (*QueryResult, error) {
	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute SQLite query: %w", err)
	}

	return NewQueryResult(rows)
}

// ListTables implements the Connection interface
func (c *SQLiteConnection) ListTables() ([]string, error) {
	query := "SELECT name FROM sqlite_master WHERE type='table'"
	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list SQLite tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan SQLite table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}

// DescribeTable implements the Connection interface
func (c *SQLiteConnection) DescribeTable(tableName string) (*TableInfo, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to describe SQLite table: %w", err)
	}
	defer rows.Close()

	tableInfo := &TableInfo{
		Name:    tableName,
		Columns: make([]ColumnInfo, 0),
	}

	for rows.Next() {
		var column ColumnInfo
		var cid int
		var pk int
		var nullable string
		var defaultVal any

		err = rows.Scan(&cid, &column.Name, &column.Type, &nullable, &defaultVal, &pk)
		if err != nil {
			return nil, fmt.Errorf("failed to scan SQLite column info: %w", err)
		}

		if pk == 1 {
			column.Key = "PRI"
		}
		column.Nullable = (nullable == "1")
		if defaultVal != nil {
			var defaultStr string
			switch v := defaultVal.(type) {
			case []byte:
				defaultStr = string(v)
			case string:
				defaultStr = v
			default:
				defaultStr = fmt.Sprintf("%v", v)
			}
			column.Default = &defaultStr
		}

		tableInfo.Columns = append(tableInfo.Columns, column)
	}

	// Get primary keys
	primaryKeys, err := c.getPrimaryKeys(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get SQLite primary keys: %w", err)
	}
	tableInfo.PrimaryKeys = primaryKeys

	// Get constraints (SQLite has limited constraint info)
	constraints := []ConstraintInfo{} // SQLite constraint info is limited
	tableInfo.Constraints = constraints

	// Get foreign keys
	foreignKeys, err := c.getForeignKeys(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get SQLite foreign keys: %w", err)
	}
	tableInfo.ForeignKeys = foreignKeys

	return tableInfo, nil
}

// getPrimaryKeys retrieves primary keys for SQLite
func (c *SQLiteConnection) getPrimaryKeys(tableName string) ([]string, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	rows, err := c.db.Query(query)
	if err != nil {
		return []string{}, nil // Return empty slice if query fails
	}
	defer rows.Close()

	var primaryKeys []string
	for rows.Next() {
		var cid int
		var name, dataType string
		var nullable any
		var defaultVal any
		var pk int
		err = rows.Scan(&cid, &name, &dataType, &nullable, &defaultVal, &pk)
		if err != nil {
			continue
		}
		if pk == 1 {
			primaryKeys = append(primaryKeys, name)
		}
	}

	return primaryKeys, nil
}

// getForeignKeys retrieves foreign keys for SQLite
func (c *SQLiteConnection) getForeignKeys(tableName string) ([]ForeignKeyInfo, error) {
	query := fmt.Sprintf("PRAGMA foreign_key_list(%s)", tableName)
	rows, err := c.db.Query(query)
	if err != nil {
		return []ForeignKeyInfo{}, nil // Return empty slice if query fails
	}
	defer rows.Close()

	var foreignKeys []ForeignKeyInfo
	for rows.Next() {
		var fk ForeignKeyInfo
		var id int
		var seq int
		err = rows.Scan(&id, &seq, &fk.ReferencedTable, &fk.Column, &fk.ReferencedColumn, &fk.OnUpdate, &fk.OnDelete, &fk.Name)
		if err != nil {
			continue
		}
		if fk.Name == "" {
			fk.Name = fmt.Sprintf("fk_%d", id)
		}
		foreignKeys = append(foreignKeys, fk)
	}

	return foreignKeys, nil
}

// Close implements the Connection interface
func (c *SQLiteConnection) Close() error {
	return c.db.Close()
}
