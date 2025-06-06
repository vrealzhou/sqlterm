use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use crate::Result;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Database {
    pub name: String,
    pub charset: Option<String>,
    pub collation: Option<String>,
    pub size: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Table {
    pub name: String,
    pub schema: Option<String>,
    pub table_type: TableType,
    pub row_count: Option<u64>,
    pub size: Option<u64>,
    pub comment: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum TableType {
    Table,
    View,
    MaterializedView,
    Temporary,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Column {
    pub name: String,
    pub data_type: String,
    pub nullable: bool,
    pub default_value: Option<String>,
    pub is_primary_key: bool,
    pub is_foreign_key: bool,
    pub is_unique: bool,
    pub is_auto_increment: bool,
    pub max_length: Option<usize>,
    pub precision: Option<u8>,
    pub scale: Option<u8>,
    pub comment: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Index {
    pub name: String,
    pub table_name: String,
    pub columns: Vec<String>,
    pub is_unique: bool,
    pub is_primary: bool,
    pub index_type: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ForeignKey {
    pub name: String,
    pub table_name: String,
    pub column_name: String,
    pub referenced_table: String,
    pub referenced_column: String,
    pub on_delete: Option<String>,
    pub on_update: Option<String>,
}

#[async_trait]
pub trait SchemaInspector: Send + Sync {
    /// List all databases
    async fn list_databases(&self) -> Result<Vec<Database>>;
    
    /// List all tables in a database
    async fn list_tables(&self, database: Option<&str>) -> Result<Vec<Table>>;
    
    /// Get detailed information about a table
    async fn describe_table(&self, table_name: &str, schema: Option<&str>) -> Result<Vec<Column>>;
    
    /// List all indexes for a table
    async fn list_indexes(&self, table_name: &str, schema: Option<&str>) -> Result<Vec<Index>>;
    
    /// List all foreign keys for a table
    async fn list_foreign_keys(&self, table_name: &str, schema: Option<&str>) -> Result<Vec<ForeignKey>>;
    
    /// Get table row count
    async fn get_table_row_count(&self, table_name: &str, schema: Option<&str>) -> Result<u64>;
    
    /// Check if table exists
    async fn table_exists(&self, table_name: &str, schema: Option<&str>) -> Result<bool>;
}
