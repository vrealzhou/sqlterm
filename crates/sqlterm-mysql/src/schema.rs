use async_trait::async_trait;
use sqlx::{MySql, Pool, Row};
use sqlterm_core::{
    SchemaInspector, Database, Table, Column, Index, ForeignKey, TableType,
    Result, SqlTermError
};

pub struct MySqlSchemaInspector {
    pool: Pool<MySql>,
}

impl MySqlSchemaInspector {
    pub fn new(pool: Pool<MySql>) -> Self {
        Self { pool }
    }
}

#[async_trait]
impl SchemaInspector for MySqlSchemaInspector {
    async fn list_databases(&self) -> Result<Vec<Database>> {
        let rows = sqlx::query("SHOW DATABASES")
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

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
        let query = if let Some(db) = database {
            format!("SHOW TABLES FROM `{}`", db)
        } else {
            "SHOW TABLES".to_string()
        };

        let rows = sqlx::query(&query)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let tables = rows
            .iter()
            .map(|row| {
                let name: String = row.get(0);
                Table {
                    name,
                    schema: database.map(|s| s.to_string()),
                    table_type: TableType::Table, // Simplified - would need more complex query to determine type
                    row_count: None,
                    size: None,
                    comment: None,
                }
            })
            .collect();

        Ok(tables)
    }

    async fn describe_table(&self, table_name: &str, schema: Option<&str>) -> Result<Vec<Column>> {
        let query = if let Some(db) = schema {
            format!("DESCRIBE `{}`.`{}`", db, table_name)
        } else {
            format!("DESCRIBE `{}`", table_name)
        };

        let rows = sqlx::query(&query)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let columns = rows
            .iter()
            .map(|row| {
                let name: String = row.get("Field");
                let data_type: String = row.get("Type");
                let nullable: String = row.get("Null");
                let key: String = row.get("Key");
                let default: Option<String> = row.get("Default");
                let extra: String = row.get("Extra");

                Column {
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
        let query = if let Some(db) = schema {
            format!("SHOW INDEX FROM `{}`.`{}`", db, table_name)
        } else {
            format!("SHOW INDEX FROM `{}`", table_name)
        };

        let rows = sqlx::query(&query)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        // Group by index name and collect columns
        let mut indexes = std::collections::HashMap::new();
        
        for row in rows {
            let index_name: String = row.get("Key_name");
            let column_name: String = row.get("Column_name");
            let non_unique: i32 = row.get("Non_unique");
            let index_type: String = row.get("Index_type");

            let index = indexes.entry(index_name.clone()).or_insert(Index {
                name: index_name.clone(),
                table_name: table_name.to_string(),
                columns: Vec::new(),
                is_unique: non_unique == 0,
                is_primary: index_name == "PRIMARY",
                index_type,
            });

            index.columns.push(column_name);
        }

        Ok(indexes.into_values().collect())
    }

    async fn list_foreign_keys(&self, _table_name: &str, _schema: Option<&str>) -> Result<Vec<ForeignKey>> {
        // This would require querying INFORMATION_SCHEMA
        // For now, return empty list
        Ok(Vec::new())
    }

    async fn get_table_row_count(&self, table_name: &str, schema: Option<&str>) -> Result<u64> {
        let query = if let Some(db) = schema {
            format!("SELECT COUNT(*) FROM `{}`.`{}`", db, table_name)
        } else {
            format!("SELECT COUNT(*) FROM `{}`", table_name)
        };

        let row = sqlx::query(&query)
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let count: i64 = row.get(0);
        Ok(count as u64)
    }

    async fn table_exists(&self, table_name: &str, schema: Option<&str>) -> Result<bool> {
        let query = if let Some(db) = schema {
            format!(
                "SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = '{}' AND TABLE_NAME = '{}'",
                db, table_name
            )
        } else {
            format!(
                "SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = '{}'",
                table_name
            )
        };

        let row = sqlx::query(&query)
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let count: i64 = row.get(0);
        Ok(count > 0)
    }
}
