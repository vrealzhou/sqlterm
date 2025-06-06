// SQLite query executor implementation
// TODO: Implement similar to MySQL but for SQLite

use async_trait::async_trait;
use sqlterm_core::{QueryExecutor, PreparedStatement, Transaction, Result, SqlTermError};

pub struct SqliteQueryExecutor;

#[async_trait]
impl QueryExecutor for SqliteQueryExecutor {
    async fn execute_query(&self, _query: &sqlterm_core::Query) -> Result<sqlterm_core::QueryResult> {
        Err(SqlTermError::QueryExecution("SQLite query executor not yet implemented".to_string()))
    }

    async fn execute_non_query(&self, _query: &sqlterm_core::Query) -> Result<sqlterm_core::QueryExecution> {
        Err(SqlTermError::QueryExecution("SQLite query executor not yet implemented".to_string()))
    }

    async fn prepare_statement(&self, _sql: &str) -> Result<Box<dyn PreparedStatement>> {
        Err(SqlTermError::QueryExecution("SQLite prepared statements not yet implemented".to_string()))
    }

    async fn begin_transaction(&self) -> Result<Box<dyn Transaction>> {
        Err(SqlTermError::QueryExecution("SQLite transactions not yet implemented".to_string()))
    }
}
