use async_trait::async_trait;
use sqlx::{PgPool, Pool, Postgres, Row, Column, TypeInfo};
use sqlx::types::chrono::{DateTime, NaiveDateTime, Utc};
use sqlterm_core::{
    ConnectionConfig, ConnectionInfo, DatabaseConnection, DatabaseType, Result, SqlTermError,
};

pub struct PostgresConnection {
    pool: Pool<Postgres>,
    config: ConnectionConfig,
    connected: bool,
}

impl PostgresConnection {
    fn build_connection_string(config: &ConnectionConfig) -> String {
        let mut url = if let Some(password) = &config.password {
            format!(
                "postgresql://{}:{}@{}:{}/{}",
                config.username,
                password,
                config.host,
                config.port,
                config.database
            )
        } else {
            format!(
                "postgresql://{}@{}:{}/{}",
                config.username,
                config.host,
                config.port,
                config.database
            )
        };
        
        if config.ssl {
            url.push_str("?sslmode=require");
        } else {
            url.push_str("?sslmode=disable");
        }
        
        url
    }
}

#[async_trait]
impl DatabaseConnection for PostgresConnection {
    async fn connect(config: &ConnectionConfig) -> Result<Box<dyn DatabaseConnection>> {
        if config.database_type != DatabaseType::PostgreSQL {
            return Err(SqlTermError::Configuration(
                "Invalid database type for PostgreSQL connection".to_string(),
            ));
        }

        let connection_string = Self::build_connection_string(config);
        
        let pool = PgPool::connect(&connection_string)
            .await
            .map_err(|e| SqlTermError::Connection(e.to_string()))?;

        Ok(Box::new(PostgresConnection {
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
        let row = sqlx::query("SELECT version() as version, current_database() as database, current_user as user")
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::Connection(e.to_string()))?;

        let server_version: String = row.get("version");
        let database_name: String = row.get("database");
        let username: String = row.get("user");

        Ok(ConnectionInfo {
            server_version,
            database_name,
            username,
            connection_id: None,
        })
    }

    async fn close(&mut self) -> Result<()> {
        self.pool.close().await;
        self.connected = false;
        Ok(())
    }

    fn database_type(&self) -> DatabaseType {
        DatabaseType::PostgreSQL
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
                                // Try different types - check integers first to avoid string conversion
                                if let Ok(val) = row.try_get::<Option<i32>, _>(col.name.as_str()) {
                                    match val {
                                        Some(v) => sqlterm_core::Value::Integer(v as i64),
                                        None => sqlterm_core::Value::Null,
                                    }
                                } else if let Ok(val) = row.try_get::<Option<i64>, _>(col.name.as_str()) {
                                    match val {
                                        Some(v) => sqlterm_core::Value::Integer(v),
                                        None => sqlterm_core::Value::Null,
                                    }
                                } else if let Ok(val) = row.try_get::<Option<i16>, _>(col.name.as_str()) {
                                    match val {
                                        Some(v) => sqlterm_core::Value::Integer(v as i64),
                                        None => sqlterm_core::Value::Null,
                                    }
                                } else if let Ok(val) = row.try_get::<Option<f64>, _>(col.name.as_str()) {
                                    match val {
                                        Some(v) => sqlterm_core::Value::Float(v),
                                        None => sqlterm_core::Value::Null,
                                    }
                                } else if let Ok(val) = row.try_get::<Option<f32>, _>(col.name.as_str()) {
                                    match val {
                                        Some(v) => sqlterm_core::Value::Float(v as f64),
                                        None => sqlterm_core::Value::Null,
                                    }
                                } else if let Ok(val) = row.try_get::<Option<bool>, _>(col.name.as_str()) {
                                    match val {
                                        Some(v) => sqlterm_core::Value::Boolean(v),
                                        None => sqlterm_core::Value::Null,
                                    }
                                } else if let Ok(val) = row.try_get::<Option<NaiveDateTime>, _>(col.name.as_str()) {
                                    match val {
                                        Some(v) => sqlterm_core::Value::String(v.format("%Y-%m-%d %H:%M:%S").to_string()),
                                        None => sqlterm_core::Value::Null,
                                    }
                                } else if let Ok(val) = row.try_get::<Option<DateTime<Utc>>, _>(col.name.as_str()) {
                                    match val {
                                        Some(v) => sqlterm_core::Value::String(v.format("%Y-%m-%d %H:%M:%S UTC").to_string()),
                                        None => sqlterm_core::Value::Null,
                                    }
                                } else {
                                    sqlterm_core::Value::String(format!("Unknown type: {}", col.data_type))
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
        let rows = sqlx::query(
            "SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename"
        )
        .fetch_all(&self.pool)
        .await
        .map_err(|e| SqlTermError::Query(e.to_string()))?;

        let tables = rows
            .iter()
            .map(|row| {
                let name: String = row.get("tablename");
                name
            })
            .collect();

        Ok(tables)
    }

    async fn get_table_details(&self, table_name: &str) -> Result<sqlterm_core::TableDetails> {
        // Get table info
        let table_info = sqlterm_core::Table {
            name: table_name.to_string(),
            schema: Some("public".to_string()),
            table_type: sqlterm_core::TableType::Table,
            row_count: None,
            size: None,
            comment: None,
        };

        // Get columns
        let column_rows = sqlx::query(
            r#"SELECT 
                column_name, 
                data_type, 
                is_nullable, 
                column_default
            FROM information_schema.columns 
            WHERE table_name = $1 AND table_schema = 'public'
            ORDER BY ordinal_position"#
        )
        .bind(table_name)
        .fetch_all(&self.pool)
        .await
        .map_err(|e| SqlTermError::Query(e.to_string()))?;

        let columns: Vec<sqlterm_core::Column> = column_rows
            .iter()
            .map(|row| {
                let name: String = row.get("column_name");
                let data_type: String = row.get("data_type");
                let is_nullable: String = row.get("is_nullable");
                let default_value: Option<String> = row.try_get("column_default").unwrap_or(None);

                let is_auto_increment = default_value.as_ref().map(|d| d.contains("nextval")).unwrap_or(false);
                sqlterm_core::Column {
                    name,
                    data_type,
                    nullable: is_nullable == "YES",
                    default_value,
                    is_primary_key: false, // We'd need to check constraint info separately
                    is_foreign_key: false, // We'd need to check foreign key info separately
                    is_unique: false, // We'd need to check constraint info separately
                    is_auto_increment,
                    max_length: None,
                    precision: None,
                    scale: None,
                    comment: None,
                }
            })
            .collect();

        // Get row count
        let count_row = sqlx::query(&format!("SELECT COUNT(*) as count FROM \"{}\"", table_name))
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

impl PostgresConnection {
    pub fn pool(&self) -> &Pool<Postgres> {
        &self.pool
    }
}
