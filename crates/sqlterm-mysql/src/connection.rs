use async_trait::async_trait;
use sqlx::{MySql, MySqlPool, Pool, Row, Column, TypeInfo};
use sqlterm_core::{
    ConnectionConfig, ConnectionInfo, DatabaseConnection, DatabaseType, Result, SqlTermError,
    QueryExecutor, SchemaInspector, Query, QueryResult, QueryExecution, PreparedStatement,
    Transaction, Value, Table, TableType, Index, ForeignKey, Database,
};

pub struct MySqlConnection {
    pool: Pool<MySql>,
    config: ConnectionConfig,
    connected: bool,
}

impl MySqlConnection {
    fn build_connection_string(config: &ConnectionConfig) -> String {
        let mut url = if let Some(password) = &config.password {
            format!(
                "mysql://{}:{}@{}:{}/{}",
                config.username,
                password,
                config.host,
                config.port,
                config.database
            )
        } else {
            format!(
                "mysql://{}@{}:{}/{}",
                config.username,
                config.host,
                config.port,
                config.database
            )
        };
        
        if config.ssl {
            url.push_str("?ssl-mode=required");
        }
        
        url
    }
}

#[async_trait]
impl DatabaseConnection for MySqlConnection {
    async fn connect(config: &ConnectionConfig) -> Result<Box<dyn DatabaseConnection>> {
        if config.database_type != DatabaseType::MySQL {
            return Err(SqlTermError::Configuration(
                "Invalid database type for MySQL connection".to_string(),
            ));
        }

        let connection_string = Self::build_connection_string(config);
        
        let pool = MySqlPool::connect(&connection_string)
            .await
            .map_err(|e| SqlTermError::Connection(e.to_string()))?;

        Ok(Box::new(MySqlConnection {
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
        let row = sqlx::query("SELECT VERSION() as version, DATABASE() as database_name, USER() as username")
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::Connection(e.to_string()))?;

        let server_version: String = row.get("version");
        let database_name: String = row.get("database_name");
        let username: String = row.get("username");

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
        DatabaseType::MySQL
    }

    fn is_connected(&self) -> bool {
        self.connected && !self.pool.is_closed()
    }

    async fn execute_query(&self, sql: &str) -> Result<sqlterm_core::QueryResult> {
        let query = Query {
            sql: sql.to_string(),
            parameters: vec![],
        };
        QueryExecutor::execute_query(self, &query).await
    }

    async fn list_tables(&self) -> Result<Vec<String>> {
        let tables = SchemaInspector::list_tables(self, None).await?;
        Ok(tables.into_iter().map(|t| t.name).collect())
    }

    async fn get_table_details(&self, table_name: &str) -> Result<sqlterm_core::TableDetails> {
        SchemaInspector::get_table_details(self, table_name, None).await
    }
}

impl MySqlConnection {
    pub fn pool(&self) -> &Pool<MySql> {
        &self.pool
    }
}

#[async_trait]
impl QueryExecutor for MySqlConnection {
    async fn execute_query(&self, query: &Query) -> Result<QueryResult> {
        let start_time = std::time::Instant::now();
        
        let rows = sqlx::query(&query.sql)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::Query(e.to_string()))?;

        let execution_time = start_time.elapsed();
        
        if rows.is_empty() {
            return Ok(QueryResult {
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
                nullable: true, // We'll determine this from schema inspection if needed
                max_length: None,
                precision: None,
                scale: None,
            })
            .collect();

        // Convert rows to our format
        let result_rows: Vec<sqlterm_core::Row> = rows
            .iter()
            .map(|row| {
                let values: Vec<Value> = columns
                    .iter()
                    .map(|col| {
                        // Extract value based on column type - check integers first to avoid string conversion
                        if let Ok(val) = row.try_get::<Option<i32>, _>(col.name.as_str()) {
                            match val {
                                Some(v) => Value::Integer(v as i64),
                                None => Value::Null,
                            }
                        } else if let Ok(val) = row.try_get::<Option<i64>, _>(col.name.as_str()) {
                            match val {
                                Some(v) => Value::Integer(v),
                                None => Value::Null,
                            }
                        } else if let Ok(val) = row.try_get::<Option<i16>, _>(col.name.as_str()) {
                            match val {
                                Some(v) => Value::Integer(v as i64),
                                None => Value::Null,
                            }
                        } else if let Ok(val) = row.try_get::<Option<i8>, _>(col.name.as_str()) {
                            match val {
                                Some(v) => Value::Integer(v as i64),
                                None => Value::Null,
                            }
                        } else if let Ok(val) = row.try_get::<Option<u32>, _>(col.name.as_str()) {
                            match val {
                                Some(v) => Value::Integer(v as i64),
                                None => Value::Null,
                            }
                        } else if let Ok(val) = row.try_get::<Option<u64>, _>(col.name.as_str()) {
                            match val {
                                Some(v) => Value::Integer(v as i64),
                                None => Value::Null,
                            }
                        } else if let Ok(val) = row.try_get::<Option<f64>, _>(col.name.as_str()) {
                            match val {
                                Some(v) => Value::Float(v),
                                None => Value::Null,
                            }
                        } else if let Ok(val) = row.try_get::<Option<f32>, _>(col.name.as_str()) {
                            match val {
                                Some(v) => Value::Float(v as f64),
                                None => Value::Null,
                            }
                        } else if let Ok(val) = row.try_get::<Option<bool>, _>(col.name.as_str()) {
                            match val {
                                Some(v) => Value::Boolean(v),
                                None => Value::Null,
                            }
                        } else if let Ok(val) = row.try_get::<Option<chrono::NaiveDateTime>, _>(col.name.as_str()) {
                            match val {
                                Some(v) => Value::String(v.format("%Y-%m-%d %H:%M:%S").to_string()),
                                None => Value::Null,
                            }
                        } else if let Ok(val) = row.try_get::<Option<chrono::DateTime<chrono::Utc>>, _>(col.name.as_str()) {
                            match val {
                                Some(v) => Value::String(v.format("%Y-%m-%d %H:%M:%S UTC").to_string()),
                                None => Value::Null,
                            }
                        } else if let Ok(val) = row.try_get::<Option<String>, _>(col.name.as_str()) {
                            match val {
                                Some(v) => Value::String(v),
                                None => Value::Null,
                            }
                        } else {
                            Value::String(format!("Unknown type: {}", col.data_type))
                        }
                    })
                    .collect();

                sqlterm_core::Row { values }
            })
            .collect();

        let total_rows = result_rows.len();
        Ok(QueryResult {
            columns,
            rows: result_rows,
            total_rows,
            execution_time,
            is_truncated: false,
            truncated_at: None,
        })
    }

    async fn execute_non_query(&self, query: &Query) -> Result<QueryExecution> {
        let start_time = std::time::Instant::now();
        
        let result = sqlx::query(&query.sql)
            .execute(&self.pool)
            .await
            .map_err(|e| SqlTermError::Query(e.to_string()))?;

        let execution_time = start_time.elapsed();

        Ok(QueryExecution {
            query: query.clone(),
            execution_time,
            rows_affected: Some(result.rows_affected()),
        })
    }

    async fn prepare_statement(&self, _sql: &str) -> Result<Box<dyn PreparedStatement>> {
        // TODO: Implement prepared statements for MySQL
        Err(SqlTermError::NotImplemented("Prepared statements not yet implemented for MySQL".to_string()))
    }

    async fn begin_transaction(&self) -> Result<Box<dyn Transaction>> {
        // TODO: Implement transactions for MySQL
        Err(SqlTermError::NotImplemented("Transactions not yet implemented for MySQL".to_string()))
    }
}

#[async_trait]
impl SchemaInspector for MySqlConnection {
    async fn list_databases(&self) -> Result<Vec<Database>> {
        let rows = sqlx::query("SHOW DATABASES")
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::Query(e.to_string()))?;

        let databases = rows
            .iter()
            .map(|row| {
                let name: String = row.get(0);
                Database {
                    name,
                    charset: None,
                    collation: None,
                    size: None,
                }
            })
            .collect();

        Ok(databases)
    }

    async fn list_tables(&self, database: Option<&str>) -> Result<Vec<Table>> {
        let sql = if let Some(db) = database {
            format!("SHOW TABLES FROM `{}`", db)
        } else {
            "SHOW TABLES".to_string()
        };

        let rows = sqlx::query(&sql)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::Query(e.to_string()))?;

        let tables = rows
            .iter()
            .map(|row| {
                let name: String = row.get(0);
                Table {
                    name,
                    schema: database.map(|s| s.to_string()),
                    table_type: TableType::Table,
                    row_count: None,
                    size: None,
                    comment: None,
                }
            })
            .collect();

        Ok(tables)
    }

    async fn describe_table(&self, table_name: &str, schema: Option<&str>) -> Result<Vec<sqlterm_core::Column>> {
        let sql = if let Some(db) = schema {
            format!("DESCRIBE `{}`.`{}`", db, table_name)
        } else {
            format!("DESCRIBE `{}`", table_name)
        };

        let rows = sqlx::query(&sql)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::Query(e.to_string()))?;

        let columns = rows
            .iter()
            .map(|row| {
                let name: String = row.get("Field");
                let data_type: String = row.get("Type");
                let nullable: String = row.get("Null");
                let key: String = row.get("Key");
                let default: Option<String> = row.try_get("Default").unwrap_or(None);
                let extra: String = row.get("Extra");

                sqlterm_core::Column {
                    name,
                    data_type,
                    nullable: nullable == "YES",
                    default_value: default,
                    is_primary_key: key == "PRI",
                    is_foreign_key: key == "MUL",
                    is_unique: key == "UNI",
                    is_auto_increment: extra.contains("auto_increment"),
                    max_length: None,
                    precision: None,
                    scale: None,
                    comment: None,
                }
            })
            .collect();

        Ok(columns)
    }

    async fn list_indexes(&self, table_name: &str, schema: Option<&str>) -> Result<Vec<Index>> {
        let sql = if let Some(db) = schema {
            format!("SHOW INDEX FROM `{}`.`{}`", db, table_name)
        } else {
            format!("SHOW INDEX FROM `{}`", table_name)
        };

        let rows = sqlx::query(&sql)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::Query(e.to_string()))?;

        let mut indexes = std::collections::HashMap::new();

        for row in rows {
            let index_name: String = row.get("Key_name");
            let column_name: String = row.get("Column_name");
            let non_unique: i32 = row.get("Non_unique");
            let index_type: String = row.get("Index_type");

            indexes
                .entry(index_name.clone())
                .or_insert_with(|| Index {
                    name: index_name.clone(),
                    table_name: table_name.to_string(),
                    columns: vec![],
                    is_unique: non_unique == 0,
                    is_primary: index_name == "PRIMARY",
                    index_type,
                })
                .columns
                .push(column_name);
        }

        Ok(indexes.into_values().collect())
    }

    async fn list_foreign_keys(&self, table_name: &str, schema: Option<&str>) -> Result<Vec<ForeignKey>> {
        let database = schema.unwrap_or(&self.config.database);
        
        let sql = r#"
            SELECT 
                kcu.CONSTRAINT_NAME as name,
                kcu.TABLE_NAME as table_name,
                kcu.COLUMN_NAME as column_name,
                kcu.REFERENCED_TABLE_NAME as referenced_table,
                kcu.REFERENCED_COLUMN_NAME as referenced_column,
                rc.DELETE_RULE as on_delete,
                rc.UPDATE_RULE as on_update
            FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu
            LEFT JOIN INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS rc 
                ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME 
                AND kcu.TABLE_SCHEMA = rc.CONSTRAINT_SCHEMA
            WHERE kcu.TABLE_SCHEMA = ? AND kcu.TABLE_NAME = ? AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
        "#;

        let rows = sqlx::query(sql)
            .bind(database)
            .bind(table_name)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::Query(e.to_string()))?;

        let foreign_keys = rows
            .iter()
            .map(|row| {
                ForeignKey {
                    name: row.get("name"),
                    table_name: row.get("table_name"),
                    column_name: row.get("column_name"),
                    referenced_table: row.get("referenced_table"),
                    referenced_column: row.get("referenced_column"),
                    on_delete: row.try_get("on_delete").ok(),
                    on_update: row.try_get("on_update").ok(),
                }
            })
            .collect();

        Ok(foreign_keys)
    }

    async fn get_table_row_count(&self, table_name: &str, schema: Option<&str>) -> Result<u64> {
        let sql = if let Some(db) = schema {
            format!("SELECT COUNT(*) as count FROM `{}`.`{}`", db, table_name)
        } else {
            format!("SELECT COUNT(*) as count FROM `{}`", table_name)
        };

        let row = sqlx::query(&sql)
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::Query(e.to_string()))?;

        let count: i64 = row.get("count");
        Ok(count as u64)
    }

    async fn table_exists(&self, table_name: &str, schema: Option<&str>) -> Result<bool> {
        let sql = if let Some(db) = schema {
            format!(
                "SELECT COUNT(*) as count FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = '{}' AND TABLE_NAME = '{}'",
                db, table_name
            )
        } else {
            format!(
                "SELECT COUNT(*) as count FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '{}'",
                table_name
            )
        };

        let row = sqlx::query(&sql)
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::Query(e.to_string()))?;

        let count: i64 = row.get("count");
        Ok(count > 0)
    }
}
