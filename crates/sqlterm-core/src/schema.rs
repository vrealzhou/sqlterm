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

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TableDetails {
    pub table: Table,
    pub columns: Vec<Column>,
    pub indexes: Vec<Index>,
    pub foreign_keys: Vec<ForeignKey>,
    pub statistics: TableStatistics,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TableStatistics {
    pub row_count: u64,
    pub size_bytes: Option<u64>,
    pub last_updated: Option<String>,
    pub auto_increment_value: Option<u64>,
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

    /// Get comprehensive table details including columns, indexes, foreign keys, and statistics
    async fn get_table_details(&self, table_name: &str, schema: Option<&str>) -> Result<TableDetails> {
        let table_info = Table {
            name: table_name.to_string(),
            schema: schema.map(|s| s.to_string()),
            table_type: TableType::Table, // Default, could be enhanced
            row_count: None,
            size: None,
            comment: None,
        };

        let columns = self.describe_table(table_name, schema).await?;
        let indexes = self.list_indexes(table_name, schema).await?;
        let foreign_keys = self.list_foreign_keys(table_name, schema).await?;
        let row_count = self.get_table_row_count(table_name, schema).await?;

        let statistics = TableStatistics {
            row_count,
            size_bytes: None, // Could be implemented per database
            last_updated: None, // Could be implemented per database
            auto_increment_value: None, // Could be implemented per database
        };

        Ok(TableDetails {
            table: table_info,
            columns,
            indexes,
            foreign_keys,
            statistics,
        })
    }
}
