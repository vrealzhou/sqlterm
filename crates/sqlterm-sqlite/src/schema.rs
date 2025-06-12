use async_trait::async_trait;
use sqlx::{Sqlite, Pool, Row};
use sqlterm_core::{
    SchemaInspector, Database, Table, Column, Index, ForeignKey, TableType,
    Result, SqlTermError
};

pub struct SqliteSchemaInspector {
    pool: Pool<Sqlite>,
}

impl SqliteSchemaInspector {
    pub fn new(pool: Pool<Sqlite>) -> Self {
        Self { pool }
    }
}

#[async_trait]
impl SchemaInspector for SqliteSchemaInspector {
    async fn list_databases(&self) -> Result<Vec<Database>> {
        // SQLite doesn't have multiple databases in the same connection
        // Return the current database (file)
        Ok(vec![Database {
            name: "main".to_string(),
            charset: None,
            collation: None,
            size: None,
        }])
    }

    async fn list_tables(&self, _database: Option<&str>) -> Result<Vec<Table>> {
        let query = r#"
            SELECT
                name,
                type
            FROM sqlite_master
            WHERE type IN ('table', 'view')
                AND name NOT LIKE 'sqlite_%'
            ORDER BY name
        "#;

        let rows = sqlx::query(query)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let tables = rows
            .iter()
            .map(|row| {
                let name: String = row.get("name");
                let type_str: String = row.get("type");

                let table_type = match type_str.as_str() {
                    "view" => TableType::View,
                    _ => TableType::Table,
                };

                Table {
                    name,
                    schema: None, // SQLite doesn't have schemas in the same way
                    table_type,
                    row_count: None,
                    size: None,
                    comment: None,
                }
            })
            .collect();

        Ok(tables)
    }

    async fn describe_table(&self, table_name: &str, _schema: Option<&str>) -> Result<Vec<Column>> {
        let query = format!("PRAGMA table_info({})", table_name);

        let rows = sqlx::query(&query)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let columns = rows
            .iter()
            .map(|row| {
                let name: String = row.get("name");
                let data_type: String = row.get("type");
                let not_null: i32 = row.get("notnull");
                let default_value: Option<String> = row.get("dflt_value");
                let pk: i32 = row.get("pk");

                Column {
                    name,
                    data_type,
                    nullable: not_null == 0,
                    default_value,
                    is_primary_key: pk > 0,
                    is_foreign_key: false, // Would need additional query
                    is_unique: false, // Would need additional query
                    is_auto_increment: false, // SQLite uses AUTOINCREMENT keyword
                    max_length: None,
                    precision: None,
                    scale: None,
                    comment: None,
                }
            })
            .collect();

        Ok(columns)
    }

    async fn list_indexes(&self, table_name: &str, _schema: Option<&str>) -> Result<Vec<Index>> {
        let query = format!("PRAGMA index_list({})", table_name);

        let rows = sqlx::query(&query)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let mut indexes = Vec::new();

        for row in rows {
            let name: String = row.get("name");
            let unique: i32 = row.get("unique");

            // Get index columns
            let info_query = format!("PRAGMA index_info({})", name);
            let info_rows = sqlx::query(&info_query)
                .fetch_all(&self.pool)
                .await
                .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

            let columns: Vec<String> = info_rows
                .iter()
                .map(|info_row| {
                    let col_name: String = info_row.get("name");
                    col_name
                })
                .collect();

            indexes.push(Index {
                name,
                table_name: table_name.to_string(),
                columns,
                is_unique: unique == 1,
                is_primary: false, // Would need to check if this is the primary key index
                index_type: "btree".to_string(), // SQLite default
            });
        }

        Ok(indexes)
    }

    async fn list_foreign_keys(&self, table_name: &str, _schema: Option<&str>) -> Result<Vec<ForeignKey>> {
        let query = format!("PRAGMA foreign_key_list({})", table_name);

        let rows = sqlx::query(&query)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let foreign_keys = rows
            .iter()
            .map(|row| {
                let id: i32 = row.get("id");
                let table: String = row.get("table");
                let from: String = row.get("from");
                let to: String = row.get("to");
                let on_update: String = row.get("on_update");
                let on_delete: String = row.get("on_delete");

                ForeignKey {
                    name: format!("fk_{}_{}", table_name, id),
                    table_name: table_name.to_string(),
                    column_name: from,
                    referenced_table: table,
                    referenced_column: to,
                    on_delete: Some(on_delete),
                    on_update: Some(on_update),
                }
            })
            .collect();

        Ok(foreign_keys)
    }

    async fn get_table_row_count(&self, table_name: &str, _schema: Option<&str>) -> Result<u64> {
        let query = format!("SELECT COUNT(*) FROM {}", table_name);

        let row = sqlx::query(&query)
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let count: i64 = row.get(0);
        Ok(count as u64)
    }

    async fn table_exists(&self, table_name: &str, _schema: Option<&str>) -> Result<bool> {
        let query = r#"
            SELECT COUNT(*)
            FROM sqlite_master
            WHERE type='table' AND name = ?
        "#;

        let row = sqlx::query(query)
            .bind(table_name)
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let count: i64 = row.get(0);
        Ok(count > 0)
    }
}
