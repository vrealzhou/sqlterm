package database

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/vrealzhou/sqlterm/internal/config"
)

// PostgreSQLConnection represents a PostgreSQL database connection
type PostgreSQLConnection struct {
	db     *sql.DB
	config *config.ConnectionConfig
}

// NewPostgreSQLConnection creates a new PostgreSQL connection
func NewPostgreSQLConnection(cfg *config.ConnectionConfig) (Connection, error) {
	if cfg.DatabaseType != config.PostgreSQL {
		return nil, fmt.Errorf("invalid database type for PostgreSQL connection: %v", cfg.DatabaseType)
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL database: %w", err)
	}

	return &PostgreSQLConnection{
		db:     db,
		config: cfg,
	}, nil
}

// Ping implements the Connection interface
func (c *PostgreSQLConnection) Ping() error {
	return c.db.Ping()
}

// Execute implements the Connection interface
func (c *PostgreSQLConnection) Execute(query string) (*QueryResult, error) {
	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute PostgreSQL query: %w", err)
	}

	return NewQueryResult(rows)
}

// ListTables implements the Connection interface
func (c *PostgreSQLConnection) ListTables() ([]string, error) {
	query := "SELECT tablename FROM pg_tables WHERE schemaname = 'public'"
	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list PostgreSQL tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan PostgreSQL table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}

// DescribeTable implements the Connection interface
func (c *PostgreSQLConnection) DescribeTable(tableName string) (*TableInfo, error) {
	query := fmt.Sprintf(`
		SELECT column_name, data_type, is_nullable, column_default, ''
		FROM information_schema.columns
		WHERE table_name = '%s'
		ORDER BY ordinal_position`, tableName)

	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to describe PostgreSQL table: %w", err)
	}
	defer rows.Close()

	tableInfo := &TableInfo{
		Name:    tableName,
		Columns: make([]ColumnInfo, 0),
	}

	for rows.Next() {
		var column ColumnInfo
		var nullable string
		var defaultVal any
		var extra string

		err = rows.Scan(&column.Name, &column.Type, &nullable, &defaultVal, &extra)
		if err != nil {
			return nil, fmt.Errorf("failed to scan PostgreSQL column info: %w", err)
		}

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
		return nil, fmt.Errorf("failed to get PostgreSQL primary keys: %w", err)
	}
	tableInfo.PrimaryKeys = primaryKeys

	// Get constraints
	constraints, err := c.getConstraints(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get PostgreSQL constraints: %w", err)
	}
	tableInfo.Constraints = constraints

	// Get foreign keys
	foreignKeys, err := c.getForeignKeys(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get PostgreSQL foreign keys: %w", err)
	}
	tableInfo.ForeignKeys = foreignKeys

	return tableInfo, nil
}

// getPrimaryKeys retrieves primary keys for PostgreSQL
func (c *PostgreSQLConnection) getPrimaryKeys(tableName string) ([]string, error) {
	query := fmt.Sprintf(`
		SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		WHERE i.indrelid = '%s'::regclass AND i.indisprimary
		ORDER BY a.attnum`, tableName)

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

// getConstraints retrieves constraints for PostgreSQL
func (c *PostgreSQLConnection) getConstraints(tableName string) ([]ConstraintInfo, error) {
	query := fmt.Sprintf(`
		SELECT tc.constraint_name, tc.constraint_type, kcu.column_name, cc.check_clause
		FROM information_schema.table_constraints tc
		LEFT JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
		LEFT JOIN information_schema.check_constraints cc ON tc.constraint_name = cc.constraint_name
		WHERE tc.table_name = '%s'
		AND tc.constraint_type IN ('UNIQUE', 'CHECK')`, tableName)

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

// getForeignKeys retrieves foreign keys for PostgreSQL
func (c *PostgreSQLConnection) getForeignKeys(tableName string) ([]ForeignKeyInfo, error) {
	query := fmt.Sprintf(`
		SELECT tc.constraint_name, kcu.column_name, ccu.table_name, ccu.column_name,
		       rc.delete_rule, rc.update_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name
		JOIN information_schema.referential_constraints rc ON tc.constraint_name = rc.constraint_name
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name = '%s'`, tableName)

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
func (c *PostgreSQLConnection) Close() error {
	return c.db.Close()
}
