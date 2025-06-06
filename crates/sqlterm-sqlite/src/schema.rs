// SQLite schema inspector implementation
// TODO: Implement similar to MySQL but for SQLite

use async_trait::async_trait;
use sqlterm_core::{SchemaInspector, Database, Table, Column, Index, ForeignKey, Result, SqlTermError};

pub struct SqliteSchemaInspector;

#[async_trait]
impl SchemaInspector for SqliteSchemaInspector {
    async fn list_databases(&self) -> Result<Vec<Database>> {
        Err(SqlTermError::SchemaInspection("SQLite schema inspector not yet implemented".to_string()))
    }

    async fn list_tables(&self, _database: Option<&str>) -> Result<Vec<Table>> {
        Err(SqlTermError::SchemaInspection("SQLite schema inspector not yet implemented".to_string()))
    }

    async fn describe_table(&self, _table_name: &str, _schema: Option<&str>) -> Result<Vec<Column>> {
        Err(SqlTermError::SchemaInspection("SQLite schema inspector not yet implemented".to_string()))
    }

    async fn list_indexes(&self, _table_name: &str, _schema: Option<&str>) -> Result<Vec<Index>> {
        Err(SqlTermError::SchemaInspection("SQLite schema inspector not yet implemented".to_string()))
    }

    async fn list_foreign_keys(&self, _table_name: &str, _schema: Option<&str>) -> Result<Vec<ForeignKey>> {
        Err(SqlTermError::SchemaInspection("SQLite schema inspector not yet implemented".to_string()))
    }

    async fn get_table_row_count(&self, _table_name: &str, _schema: Option<&str>) -> Result<u64> {
        Err(SqlTermError::SchemaInspection("SQLite schema inspector not yet implemented".to_string()))
    }

    async fn table_exists(&self, _table_name: &str, _schema: Option<&str>) -> Result<bool> {
        Err(SqlTermError::SchemaInspection("SQLite schema inspector not yet implemented".to_string()))
    }
}
