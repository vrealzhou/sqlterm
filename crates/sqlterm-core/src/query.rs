use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use crate::{Result, QueryResult};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Query {
    pub sql: String,
    pub parameters: Vec<QueryParameter>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum QueryParameter {
    String(String),
    Integer(i64),
    Float(f64),
    Boolean(bool),
    Null,
}

#[derive(Debug, Clone)]
pub struct QueryExecution {
    pub query: Query,
    pub execution_time: std::time::Duration,
    pub rows_affected: Option<u64>,
}

#[async_trait]
pub trait QueryExecutor: Send + Sync {
    /// Execute a query and return results
    async fn execute_query(&self, query: &Query) -> Result<QueryResult>;
    
    /// Execute a query without returning results (for INSERT, UPDATE, DELETE)
    async fn execute_non_query(&self, query: &Query) -> Result<QueryExecution>;
    
    /// Prepare a statement for repeated execution
    async fn prepare_statement(&self, sql: &str) -> Result<Box<dyn PreparedStatement>>;
    
    /// Begin a transaction
    async fn begin_transaction(&self) -> Result<Box<dyn Transaction>>;
}

#[async_trait]
pub trait PreparedStatement: Send + Sync {
    /// Execute the prepared statement with parameters
    async fn execute(&self, parameters: &[QueryParameter]) -> Result<QueryResult>;
    
    /// Execute the prepared statement without returning results
    async fn execute_non_query(&self, parameters: &[QueryParameter]) -> Result<QueryExecution>;
}

#[async_trait]
pub trait Transaction: Send + Sync {
    /// Commit the transaction
    async fn commit(self: Box<Self>) -> Result<()>;
    
    /// Rollback the transaction
    async fn rollback(self: Box<Self>) -> Result<()>;
    
    /// Execute a query within the transaction
    async fn execute_query(&self, query: &Query) -> Result<QueryResult>;
    
    /// Execute a non-query within the transaction
    async fn execute_non_query(&self, query: &Query) -> Result<QueryExecution>;
}
