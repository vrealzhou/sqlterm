use async_trait::async_trait;
use sqlx::{Sqlite, Pool, Row, Column, TypeInfo};
use sqlterm_core::{
    Query, QueryExecutor, QueryResult, QueryExecution, PreparedStatement, Transaction,
    Result, SqlTermError, Value, ColumnInfo
};
use std::time::Instant;

pub struct SqliteQueryExecutor {
    pool: Pool<Sqlite>,
}

impl SqliteQueryExecutor {
    pub fn new(pool: Pool<Sqlite>) -> Self {
        Self { pool }
    }
}

#[async_trait]
impl QueryExecutor for SqliteQueryExecutor {
    async fn execute_query(&self, query: &Query) -> Result<QueryResult> {
        let start = Instant::now();

        let rows = sqlx::query(&query.sql)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::QueryExecution(e.to_string()))?;

        let execution_time = start.elapsed();

        if rows.is_empty() {
            return Ok(QueryResult::new(
                vec![],
                vec![],
                execution_time,
            ));
        }

        // Extract column information from the first row
        let columns = rows[0]
            .columns()
            .iter()
            .map(|col| ColumnInfo {
                name: col.name().to_string(),
                data_type: col.type_info().name().to_string(),
                nullable: true, // SQLite is flexible with types
                max_length: None,
                precision: None,
                scale: None,
            })
            .collect();

        // Convert rows to our format
        let result_rows = rows
            .iter()
            .map(|row| {
                let values = (0..row.len())
                    .map(|i| {
                        // SQLite type conversion - SQLite is very flexible with types
                        if let Ok(val) = row.try_get::<String, _>(i) {
                            Value::String(val)
                        } else if let Ok(val) = row.try_get::<i64, _>(i) {
                            Value::Integer(val)
                        } else if let Ok(val) = row.try_get::<f64, _>(i) {
                            Value::Float(val)
                        } else if let Ok(val) = row.try_get::<bool, _>(i) {
                            Value::Boolean(val)
                        } else {
                            Value::Null
                        }
                    })
                    .collect();

                sqlterm_core::Row { values }
            })
            .collect();

        Ok(QueryResult::new(
            columns,
            result_rows,
            execution_time,
        ))
    }

    async fn execute_non_query(&self, query: &Query) -> Result<QueryExecution> {
        let start = Instant::now();

        let result = sqlx::query(&query.sql)
            .execute(&self.pool)
            .await
            .map_err(|e| SqlTermError::QueryExecution(e.to_string()))?;

        let execution_time = start.elapsed();

        Ok(QueryExecution {
            query: query.clone(),
            execution_time,
            rows_affected: Some(result.rows_affected()),
        })
    }

    async fn prepare_statement(&self, _sql: &str) -> Result<Box<dyn PreparedStatement>> {
        // TODO: Implement prepared statements
        Err(SqlTermError::QueryExecution("Prepared statements not yet implemented".to_string()))
    }

    async fn begin_transaction(&self) -> Result<Box<dyn Transaction>> {
        // TODO: Implement transactions
        Err(SqlTermError::QueryExecution("Transactions not yet implemented".to_string()))
    }
}
