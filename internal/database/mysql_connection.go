package database

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/vrealzhou/sqlterm/internal/config"
)

// MySQLConnection represents a MySQL database connection
type MySQLConnection struct {
	db     *sql.DB
	config *config.ConnectionConfig
}

// NewMySQLConnection creates a new MySQL connection
func NewMySQLConnection(cfg *config.ConnectionConfig) (Connection, error) {
	if cfg.DatabaseType != config.MySQL {
		return nil, fmt.Errorf("invalid database type for MySQL connection: %v", cfg.DatabaseType)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL database: %w", err)
	}

	return &MySQLConnection{
		db:     db,
		config: cfg,
	}, nil
}

// Ping implements the Connection interface
func (c *MySQLConnection) Ping() error {
	return c.db.Ping()
}

// Execute implements the Connection interface
func (c *MySQLConnection) Execute(query string) (*QueryResult, error) {
	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute MySQL query: %w", err)
	}

	return NewQueryResult(rows)
}

// ListTables implements the Connection interface
func (c *MySQLConnection) ListTables() ([]string, error) {
	query := "SHOW TABLES"
	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list MySQL tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan MySQL table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}

// DescribeTable implements the Connection interface
func (c *MySQLConnection) DescribeTable(tableName string) (*TableInfo, error) {
	query := fmt.Sprintf("DESCRIBE %s", tableName)
	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to describe MySQL table: %w", err)
	}
	defer rows.Close()

	tableInfo := &TableInfo{
		Name:    tableName,
		Columns: make([]ColumnInfo, 0),
	}

	for rows.Next() {
		var column ColumnInfo
		var key, extra string
		var nullable string
		var defaultVal any

		err = rows.Scan(&column.Name, &column.Type, &nullable, &key, &defaultVal, &extra)
		if err != nil {
			return nil, fmt.Errorf("failed to scan MySQL column info: %w", err)
		}

		column.Key = key
		column.Extra = extra
		column.Nullable = (nullable == "YES")
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
		return nil, fmt.Errorf("failed to get MySQL primary keys: %w", err)
	}
	tableInfo.PrimaryKeys = primaryKeys

	// Get constraints
	constraints, err := c.getConstraints(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get MySQL constraints: %w", err)
	}
	tableInfo.Constraints = constraints

	// Get foreign keys
	foreignKeys, err := c.getForeignKeys(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get MySQL foreign keys: %w", err)
	}
	tableInfo.ForeignKeys = foreignKeys

	return tableInfo, nil
}

// getPrimaryKeys retrieves primary keys for MySQL
func (c *MySQLConnection) getPrimaryKeys(tableName string) ([]string, error) {
	query := fmt.Sprintf(`
		SELECT COLUMN_NAME
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = DATABASE()
		AND TABLE_NAME = '%s'
		AND CONSTRAINT_NAME = 'PRIMARY'
		ORDER BY ORDINAL_POSITION`, tableName)

	rows, err := c.db.Query(query)
	if err != nil {
		return []string{}, nil // Return empty slice if query fails
	}
	defer rows.Close()

	var primaryKeys []string
	for rows.Next() {
		var columnName string
		err = rows.Scan(&columnName)
		if err != nil {
			continue
		}
		primaryKeys = append(primaryKeys, columnName)
	}

	return primaryKeys, nil
}

// getConstraints retrieves constraints for MySQL
func (c *MySQLConnection) getConstraints(tableName string) ([]ConstraintInfo, error) {
	query := fmt.Sprintf(`
		SELECT CONSTRAINT_NAME, CONSTRAINT_TYPE, COLUMN_NAME, CHECK_CLAUSE
		FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
		LEFT JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu ON tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME
		LEFT JOIN INFORMATION_SCHEMA.CHECK_CONSTRAINTS cc ON tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
		WHERE tc.TABLE_SCHEMA = DATABASE() AND tc.TABLE_NAME = '%s'
		AND tc.CONSTRAINT_TYPE IN ('UNIQUE', 'CHECK')`, tableName)

	rows, err := c.db.Query(query)
	if err != nil {
		return []ConstraintInfo{}, nil // Return empty slice if query fails
	}
	defer rows.Close()

	var constraints []ConstraintInfo
	for rows.Next() {
		var constraint ConstraintInfo
		var checkClause sql.NullString
		err = rows.Scan(&constraint.Name, &constraint.Type, &constraint.Column, &checkClause)
		if err != nil {
			continue
		}
		if checkClause.Valid {
			constraint.Check = checkClause.String
		}
		constraints = append(constraints, constraint)
	}

	return constraints, nil
}

// getForeignKeys retrieves foreign keys for MySQL
func (c *MySQLConnection) getForeignKeys(tableName string) ([]ForeignKeyInfo, error) {
	query := fmt.Sprintf(`
		SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME,
		       DELETE_RULE, UPDATE_RULE
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu
		JOIN INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS rc ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME
		WHERE kcu.TABLE_SCHEMA = DATABASE() AND kcu.TABLE_NAME = '%s'
		AND kcu.REFERENCED_TABLE_NAME IS NOT NULL`, tableName)

	rows, err := c.db.Query(query)
	if err != nil {
		return []ForeignKeyInfo{}, nil // Return empty slice if query fails
	}
	defer rows.Close()

	var foreignKeys []ForeignKeyInfo
	for rows.Next() {
		var fk ForeignKeyInfo
		err = rows.Scan(&fk.Name, &fk.Column, &fk.ReferencedTable, &fk.ReferencedColumn, &fk.OnDelete, &fk.OnUpdate)
		if err != nil {
			continue
		}
		foreignKeys = append(foreignKeys, fk)
	}

	return foreignKeys, nil
}

// Close implements the Connection interface
func (c *MySQLConnection) Close() error {
	return c.db.Close()
}
