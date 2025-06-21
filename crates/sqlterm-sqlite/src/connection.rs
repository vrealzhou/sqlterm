use async_trait::async_trait;
use sqlx::{Pool, Row, Sqlite, SqlitePool, Column, TypeInfo};
use sqlterm_core::{
    ConnectionConfig, ConnectionInfo, DatabaseConnection, DatabaseType, Result, SqlTermError,
};

pub struct SqliteConnection {
    pool: Pool<Sqlite>,
    config: ConnectionConfig,
    connected: bool,
}

impl SqliteConnection {
    fn build_connection_string(config: &ConnectionConfig) -> String {
        // For SQLite, the database field contains the file path
        format!("sqlite:{}", config.database)
    }
}

#[async_trait]
impl DatabaseConnection for SqliteConnection {
    async fn connect(config: &ConnectionConfig) -> Result<Box<dyn DatabaseConnection>> {
        if config.database_type != DatabaseType::SQLite {
            return Err(SqlTermError::Configuration(
                "Invalid database type for SQLite connection".to_string(),
            ));
        }

        let connection_string = Self::build_connection_string(config);
        
        let pool = SqlitePool::connect(&connection_string)
            .await
            .map_err(|e| SqlTermError::Connection(e.to_string()))?;

        Ok(Box::new(SqliteConnection {
            pool,
            config: config.clone(),
            connected: true,
        }))
    }

    async fn ping(&self) -> Result<()> {
        sqlx::query("SELECT 1")
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::Connection(e.to_string()))?;
        Ok(())
    }

    async fn get_connection_info(&self) -> Result<ConnectionInfo> {
        let row = sqlx::query("SELECT sqlite_version() as version")
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::Connection(e.to_string()))?;

        let server_version: String = row.get("version");

        Ok(ConnectionInfo {
            server_version: format!("SQLite {}", server_version),
            database_name: self.config.database.clone(),
            username: "sqlite".to_string(),
            connection_id: None,
        })
    }

    async fn close(&mut self) -> Result<()> {
        self.pool.close().await;
        self.connected = false;
        Ok(())
    }

    fn database_type(&self) -> DatabaseType {
        DatabaseType::SQLite
    }

    fn is_connected(&self) -> bool {
        self.connected && !self.pool.is_closed()
    }

    async fn execute_query(&self, sql: &str) -> Result<sqlterm_core::QueryResult> {
        let start_time = std::time::Instant::now();
        
        let rows = sqlx::query(sql)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::Query(e.to_string()))?;

        let execution_time = start_time.elapsed();
        
        if rows.is_empty() {
            return Ok(sqlterm_core::QueryResult {
                columns: vec![],
                rows: vec![],
                total_rows: 0,
                execution_time,
                is_truncated: false,
                truncated_at: None,
            });
        }

        // Extract column information from the first row
        let first_row = &rows[0];
        let columns: Vec<sqlterm_core::ColumnInfo> = first_row
            .columns()
            .iter()
            .map(|col| sqlterm_core::ColumnInfo {
                name: col.name().to_string(),
                data_type: col.type_info().name().to_string(),
                nullable: true,
                max_length: None,
                precision: None,
                scale: None,
            })
            .collect();

        // Convert rows to our format
        let result_rows: Vec<sqlterm_core::Row> = rows
            .iter()
            .map(|row| {
                let values: Vec<sqlterm_core::Value> = columns
                    .iter()
                    .map(|col| {
                        // Extract value based on column type
                        match row.try_get::<Option<String>, _>(col.name.as_str()) {
                            Ok(Some(val)) => sqlterm_core::Value::String(val),
                            Ok(None) => sqlterm_core::Value::Null,
                            Err(_) => {
                                // Try different types
                                if let Ok(val) = row.try_get::<Option<i64>, _>(col.name.as_str()) {
                                    match val {
                                        Some(v) => sqlterm_core::Value::Integer(v),
                                        None => sqlterm_core::Value::Null,
                                    }
                                } else if let Ok(val) = row.try_get::<Option<f64>, _>(col.name.as_str()) {
                                    match val {
                                        Some(v) => sqlterm_core::Value::Float(v),
                                        None => sqlterm_core::Value::Null,
                                    }
                                } else if let Ok(val) = row.try_get::<Option<bool>, _>(col.name.as_str()) {
                                    match val {
                                        Some(v) => sqlterm_core::Value::Boolean(v),
                                        None => sqlterm_core::Value::Null,
                                    }
                                } else {
                                    sqlterm_core::Value::String("Unknown type".to_string())
                                }
                            }
                        }
                    })
                    .collect();

                sqlterm_core::Row { values }
            })
            .collect();

        let total_rows = result_rows.len();
        Ok(sqlterm_core::QueryResult {
            columns,
            rows: result_rows,
            total_rows,
            execution_time,
            is_truncated: false,
            truncated_at: None,
        })
    }

    async fn list_tables(&self) -> Result<Vec<String>> {
        let rows = sqlx::query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::Query(e.to_string()))?;

        let tables = rows
            .iter()
            .map(|row| {
                let name: String = row.get("name");
                name
            })
            .collect();

        Ok(tables)
    }

    async fn get_table_details(&self, table_name: &str) -> Result<sqlterm_core::TableDetails> {
        // Get table info
        let table_info = sqlterm_core::Table {
            name: table_name.to_string(),
            schema: None,
            table_type: sqlterm_core::TableType::Table,
            row_count: None,
            size: None,
            comment: None,
        };

        // Get columns
        let column_rows = sqlx::query(&format!("PRAGMA table_info({})", table_name))
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::Query(e.to_string()))?;

        let columns: Vec<sqlterm_core::Column> = column_rows
            .iter()
            .map(|row| {
                let name: String = row.get("name");
                let data_type: String = row.get("type");
                let not_null: i32 = row.get("notnull");
                let default_value: Option<String> = row.try_get("dflt_value").unwrap_or(None);
                let pk: i32 = row.get("pk");

                sqlterm_core::Column {
                    name,
                    data_type,
                    nullable: not_null == 0,
                    default_value,
                    is_primary_key: pk != 0,
                    is_foreign_key: false, // We'd need to check foreign key info separately
                    is_unique: false, // We'd need to check index info separately
                    is_auto_increment: false, // SQLite doesn't have auto_increment in the same way
                    max_length: None,
                    precision: None,
                    scale: None,
                    comment: None,
                }
            })
            .collect();

        // Get row count
        let count_row = sqlx::query(&format!("SELECT COUNT(*) as count FROM {}", table_name))
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::Query(e.to_string()))?;
        let row_count: i64 = count_row.get("count");

        let statistics = sqlterm_core::TableStatistics {
            row_count: row_count as u64,
            size_bytes: None,
            last_updated: None,
            auto_increment_value: None,
        };

        Ok(sqlterm_core::TableDetails {
            table: table_info,
            columns,
            indexes: vec![], // TODO: Implement index detection
            foreign_keys: vec![], // TODO: Implement foreign key detection
            statistics,
        })
    }
}

impl SqliteConnection {
    pub fn pool(&self) -> &Pool<Sqlite> {
        &self.pool
    }
}
