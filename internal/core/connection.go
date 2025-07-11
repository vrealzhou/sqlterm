package core

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

type Connection interface {
	Ping() error
	Execute(query string) (*QueryResult, error)
	ListTables() ([]string, error)
	DescribeTable(tableName string) (*TableInfo, error)
	Close() error
}

type connection struct {
	db     *sql.DB
	config *ConnectionConfig
}

func NewConnection(config *ConnectionConfig) (Connection, error) {
	var dsn string
	var driverName string

	switch config.DatabaseType {
	case MySQL:
		driverName = "mysql"
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
			config.Username, config.Password, config.Host, config.Port, config.Database)
	case PostgreSQL:
		driverName = "postgres"
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			config.Host, config.Port, config.Username, config.Password, config.Database)
	case SQLite:
		driverName = "sqlite3"
		dsn = config.Database
	default:
		return nil, fmt.Errorf("unsupported database type: %v", config.DatabaseType)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	conn := &connection{
		db:     db,
		config: config,
	}

	return conn, nil
}

func (c *connection) Ping() error {
	return c.db.Ping()
}

func (c *connection) Execute(query string) (*QueryResult, error) {
	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	return NewQueryResult(rows)
}

func (c *connection) ListTables() ([]string, error) {
	var query string
	switch c.config.DatabaseType {
	case MySQL:
		query = "SHOW TABLES"
	case PostgreSQL:
		query = "SELECT tablename FROM pg_tables WHERE schemaname = 'public'"
	case SQLite:
		query = "SELECT name FROM sqlite_master WHERE type='table'"
	default:
		return nil, fmt.Errorf("unsupported database type: %v", c.config.DatabaseType)
	}

	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}

func (c *connection) DescribeTable(tableName string) (*TableInfo, error) {
	var query string
	switch c.config.DatabaseType {
	case MySQL:
		query = fmt.Sprintf("DESCRIBE %s", tableName)
	case PostgreSQL:
		query = fmt.Sprintf(`
			SELECT column_name, data_type, is_nullable, column_default, ''
			FROM information_schema.columns
			WHERE table_name = '%s'
			ORDER BY ordinal_position`, tableName)
	case SQLite:
		query = fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	default:
		return nil, fmt.Errorf("unsupported database type: %v", c.config.DatabaseType)
	}

	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to describe table: %w", err)
	}
	defer rows.Close()

	tableInfo := &TableInfo{
		Name:    tableName,
		Columns: make([]ColumnInfo, 0),
	}

	for rows.Next() {
		var column ColumnInfo
		var nullable string
		var defaultVal interface{}

		switch c.config.DatabaseType {
		case MySQL:
			var key, extra string
			err = rows.Scan(&column.Name, &column.Type, &nullable, &key, &defaultVal, &extra)
			column.Key = key
			column.Extra = extra
		case PostgreSQL:
			var extra string
			err = rows.Scan(&column.Name, &column.Type, &nullable, &defaultVal, &extra)
			column.Extra = extra
		case SQLite:
			var cid int
			var pk int
			err = rows.Scan(&cid, &column.Name, &column.Type, &nullable, &defaultVal, &pk)
			if pk == 1 {
				column.Key = "PRI"
			}
		}

		if err != nil {
			return nil, fmt.Errorf("failed to scan column info: %w", err)
		}

		column.Nullable = (nullable == "YES" || nullable == "1")
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

	return tableInfo, nil
}

func (c *connection) Close() error {
	return c.db.Close()
}
