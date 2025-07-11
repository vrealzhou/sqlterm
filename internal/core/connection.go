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

	// Get primary keys
	primaryKeys, err := c.getPrimaryKeys(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get primary keys: %w", err)
	}
	tableInfo.PrimaryKeys = primaryKeys

	// Get constraints
	constraints, err := c.getConstraints(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get constraints: %w", err)
	}
	tableInfo.Constraints = constraints

	// Get foreign keys
	foreignKeys, err := c.getForeignKeys(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get foreign keys: %w", err)
	}
	tableInfo.ForeignKeys = foreignKeys

	return tableInfo, nil
}

func (c *connection) getPrimaryKeys(tableName string) ([]string, error) {
	var query string
	switch c.config.DatabaseType {
	case MySQL:
		query = fmt.Sprintf(`
			SELECT COLUMN_NAME 
			FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE 
			WHERE TABLE_SCHEMA = DATABASE() 
			AND TABLE_NAME = '%s' 
			AND CONSTRAINT_NAME = 'PRIMARY'
			ORDER BY ORDINAL_POSITION`, tableName)
	case PostgreSQL:
		query = fmt.Sprintf(`
			SELECT a.attname
			FROM pg_index i
			JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
			WHERE i.indrelid = '%s'::regclass AND i.indisprimary
			ORDER BY a.attnum`, tableName)
	case SQLite:
		query = fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	default:
		return nil, fmt.Errorf("unsupported database type: %v", c.config.DatabaseType)
	}

	rows, err := c.db.Query(query)
	if err != nil {
		return []string{}, nil // Return empty slice if query fails
	}
	defer rows.Close()

	var primaryKeys []string
	for rows.Next() {
		if c.config.DatabaseType == SQLite {
			var cid int
			var name, dataType string
			var nullable interface{}
			var defaultVal interface{}
			var pk int
			err = rows.Scan(&cid, &name, &dataType, &nullable, &defaultVal, &pk)
			if err != nil {
				continue
			}
			if pk == 1 {
				primaryKeys = append(primaryKeys, name)
			}
		} else {
			var columnName string
			err = rows.Scan(&columnName)
			if err != nil {
				continue
			}
			primaryKeys = append(primaryKeys, columnName)
		}
	}

	return primaryKeys, nil
}

func (c *connection) getConstraints(tableName string) ([]ConstraintInfo, error) {
	var query string
	switch c.config.DatabaseType {
	case MySQL:
		query = fmt.Sprintf(`
			SELECT CONSTRAINT_NAME, CONSTRAINT_TYPE, COLUMN_NAME, CHECK_CLAUSE
			FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
			LEFT JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu ON tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME
			LEFT JOIN INFORMATION_SCHEMA.CHECK_CONSTRAINTS cc ON tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
			WHERE tc.TABLE_SCHEMA = DATABASE() AND tc.TABLE_NAME = '%s'
			AND tc.CONSTRAINT_TYPE IN ('UNIQUE', 'CHECK')`, tableName)
	case PostgreSQL:
		query = fmt.Sprintf(`
			SELECT tc.constraint_name, tc.constraint_type, kcu.column_name, cc.check_clause
			FROM information_schema.table_constraints tc
			LEFT JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
			LEFT JOIN information_schema.check_constraints cc ON tc.constraint_name = cc.constraint_name
			WHERE tc.table_name = '%s'
			AND tc.constraint_type IN ('UNIQUE', 'CHECK')`, tableName)
	case SQLite:
		return []ConstraintInfo{}, nil // SQLite constraint info is limited
	default:
		return nil, fmt.Errorf("unsupported database type: %v", c.config.DatabaseType)
	}

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

func (c *connection) getForeignKeys(tableName string) ([]ForeignKeyInfo, error) {
	var query string
	switch c.config.DatabaseType {
	case MySQL:
		query = fmt.Sprintf(`
			SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME,
			       DELETE_RULE, UPDATE_RULE
			FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu
			JOIN INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS rc ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME
			WHERE kcu.TABLE_SCHEMA = DATABASE() AND kcu.TABLE_NAME = '%s'
			AND kcu.REFERENCED_TABLE_NAME IS NOT NULL`, tableName)
	case PostgreSQL:
		query = fmt.Sprintf(`
			SELECT tc.constraint_name, kcu.column_name, ccu.table_name, ccu.column_name,
			       rc.delete_rule, rc.update_rule
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
			JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name
			JOIN information_schema.referential_constraints rc ON tc.constraint_name = rc.constraint_name
			WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name = '%s'`, tableName)
	case SQLite:
		query = fmt.Sprintf("PRAGMA foreign_key_list(%s)", tableName)
	default:
		return nil, fmt.Errorf("unsupported database type: %v", c.config.DatabaseType)
	}

	rows, err := c.db.Query(query)
	if err != nil {
		return []ForeignKeyInfo{}, nil // Return empty slice if query fails
	}
	defer rows.Close()

	var foreignKeys []ForeignKeyInfo
	for rows.Next() {
		var fk ForeignKeyInfo
		if c.config.DatabaseType == SQLite {
			var id int
			var seq int
			err = rows.Scan(&id, &seq, &fk.ReferencedTable, &fk.Column, &fk.ReferencedColumn, &fk.OnUpdate, &fk.OnDelete, &fk.Name)
			if err != nil {
				continue
			}
			if fk.Name == "" {
				fk.Name = fmt.Sprintf("fk_%d", id)
			}
		} else {
			err = rows.Scan(&fk.Name, &fk.Column, &fk.ReferencedTable, &fk.ReferencedColumn, &fk.OnDelete, &fk.OnUpdate)
			if err != nil {
				continue
			}
		}
		foreignKeys = append(foreignKeys, fk)
	}

	return foreignKeys, nil
}

func (c *connection) Close() error {
	return c.db.Close()
}
