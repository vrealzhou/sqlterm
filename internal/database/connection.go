package database

import (
	"fmt"

	"github.com/vrealzhou/sqlterm/internal/config"
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

// NewConnection creates a new database connection based on the config
func NewConnection(cfg *config.ConnectionConfig) (Connection, error) {
	switch cfg.DatabaseType {
	case config.MySQL:
		return NewMySQLConnection(cfg)
	case config.PostgreSQL:
		return NewPostgreSQLConnection(cfg)
	case config.SQLite:
		return NewSQLiteConnection(cfg)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.DatabaseType)
	}
}
