// PostgreSQL query executor implementation
// TODO: Implement similar to MySQL but for PostgreSQL

use async_trait::async_trait;
use sqlterm_core::{QueryExecutor, PreparedStatement, Transaction, Result, SqlTermError};

pub struct PostgresQueryExecutor;

#[async_trait]
impl QueryExecutor for PostgresQueryExecutor {
    async fn execute_query(&self, _query: &sqlterm_core::Query) -> Result<sqlterm_core::QueryResult> {
        Err(SqlTermError::QueryExecution("PostgreSQL query executor not yet implemented".to_string()))
    }

    async fn execute_non_query(&self, _query: &sqlterm_core::Query) -> Result<sqlterm_core::QueryExecution> {
        Err(SqlTermError::QueryExecution("PostgreSQL query executor not yet implemented".to_string()))
    }

    async fn prepare_statement(&self, _sql: &str) -> Result<Box<dyn PreparedStatement>> {
        Err(SqlTermError::QueryExecution("PostgreSQL prepared statements not yet implemented".to_string()))
    }

    async fn begin_transaction(&self) -> Result<Box<dyn Transaction>> {
        Err(SqlTermError::QueryExecution("PostgreSQL transactions not yet implemented".to_string()))
    }
}
