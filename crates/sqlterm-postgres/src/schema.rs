use async_trait::async_trait;
use sqlx::{Postgres, Pool, Row};
use sqlterm_core::{
    SchemaInspector, Database, Table, Column, Index, ForeignKey, TableType,
    Result, SqlTermError
};

pub struct PostgresSchemaInspector {
    pool: Pool<Postgres>,
}

impl PostgresSchemaInspector {
    pub fn new(pool: Pool<Postgres>) -> Self {
        Self { pool }
    }
}

#[async_trait]
impl SchemaInspector for PostgresSchemaInspector {
    async fn list_databases(&self) -> Result<Vec<Database>> {
        let rows = sqlx::query("SELECT datname FROM pg_database WHERE datistemplate = false")
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let databases = rows
            .iter()
            .map(|row| {
                let name: String = row.get("datname");
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

    async fn list_tables(&self, _database: Option<&str>) -> Result<Vec<Table>> {
        let query = r#"
            SELECT
                schemaname,
                tablename,
                'table' as table_type
            FROM pg_tables
            WHERE schemaname NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
            UNION ALL
            SELECT
                schemaname,
                viewname as tablename,
                'view' as table_type
            FROM pg_views
            WHERE schemaname NOT IN ('information_schema', 'pg_catalog')
            ORDER BY schemaname, tablename
        "#;

        let rows = sqlx::query(query)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let tables = rows
            .iter()
            .map(|row| {
                let schema: String = row.get("schemaname");
                let name: String = row.get("tablename");
                let table_type_str: String = row.get("table_type");

                let table_type = match table_type_str.as_str() {
                    "view" => TableType::View,
                    _ => TableType::Table,
                };

                Table {
                    name,
                    schema: Some(schema),
                    table_type,
                    row_count: None,
                    size: None,
                    comment: None,
                }
            })
            .collect();

        Ok(tables)
    }

    async fn describe_table(&self, table_name: &str, schema: Option<&str>) -> Result<Vec<Column>> {
        let schema_name = schema.unwrap_or("public");

        let query = r#"
            SELECT
                column_name,
                data_type,
                is_nullable,
                column_default,
                character_maximum_length,
                numeric_precision,
                numeric_scale
            FROM information_schema.columns
            WHERE table_schema = $1 AND table_name = $2
            ORDER BY ordinal_position
        "#;

        let rows = sqlx::query(query)
            .bind(schema_name)
            .bind(table_name)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let columns = rows
            .iter()
            .map(|row| {
                let name: String = row.get("column_name");
                let data_type: String = row.get("data_type");
                let is_nullable: String = row.get("is_nullable");
                let default_value: Option<String> = row.get("column_default");
                let max_length: Option<i32> = row.get("character_maximum_length");
                let precision: Option<i32> = row.get("numeric_precision");
                let scale: Option<i32> = row.get("numeric_scale");

                let is_auto_increment = default_value.as_ref().map_or(false, |d| d.contains("nextval"));

                Column {
                    name,
                    data_type,
                    nullable: is_nullable == "YES",
                    default_value,
                    is_primary_key: false, // Would need additional query to determine
                    is_foreign_key: false, // Would need additional query to determine
                    is_unique: false, // Would need additional query to determine
                    is_auto_increment,
                    max_length: max_length.map(|l| l as usize),
                    precision: precision.map(|p| p as u8),
                    scale: scale.map(|s| s as u8),
                    comment: None,
                }
            })
            .collect();

        Ok(columns)
    }

    async fn list_indexes(&self, table_name: &str, schema: Option<&str>) -> Result<Vec<Index>> {
        let schema_name = schema.unwrap_or("public");

        let query = r#"
            SELECT
                i.relname as index_name,
                t.relname as table_name,
                ix.indisunique as is_unique,
                ix.indisprimary as is_primary,
                array_agg(a.attname ORDER BY a.attnum) as column_names
            FROM pg_class t
            JOIN pg_index ix ON t.oid = ix.indrelid
            JOIN pg_class i ON i.oid = ix.indexrelid
            JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
            JOIN pg_namespace n ON n.oid = t.relnamespace
            WHERE n.nspname = $1 AND t.relname = $2
            GROUP BY i.relname, t.relname, ix.indisunique, ix.indisprimary
        "#;

        let rows = sqlx::query(query)
            .bind(schema_name)
            .bind(table_name)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let indexes = rows
            .iter()
            .map(|row| {
                let name: String = row.get("index_name");
                let table_name: String = row.get("table_name");
                let is_unique: bool = row.get("is_unique");
                let is_primary: bool = row.get("is_primary");
                let column_names: Vec<String> = row.get("column_names");

                Index {
                    name,
                    table_name,
                    columns: column_names,
                    is_unique,
                    is_primary,
                    index_type: "btree".to_string(), // PostgreSQL default
                }
            })
            .collect();

        Ok(indexes)
    }

    async fn list_foreign_keys(&self, table_name: &str, schema: Option<&str>) -> Result<Vec<ForeignKey>> {
        let schema_name = schema.unwrap_or("public");

        let query = r#"
            SELECT
                tc.constraint_name,
                tc.table_name,
                kcu.column_name,
                ccu.table_name AS foreign_table_name,
                ccu.column_name AS foreign_column_name,
                rc.update_rule,
                rc.delete_rule
            FROM information_schema.table_constraints AS tc
            JOIN information_schema.key_column_usage AS kcu
                ON tc.constraint_name = kcu.constraint_name
                AND tc.table_schema = kcu.table_schema
            JOIN information_schema.constraint_column_usage AS ccu
                ON ccu.constraint_name = tc.constraint_name
                AND ccu.table_schema = tc.table_schema
            JOIN information_schema.referential_constraints AS rc
                ON tc.constraint_name = rc.constraint_name
                AND tc.table_schema = rc.constraint_schema
            WHERE tc.constraint_type = 'FOREIGN KEY'
                AND tc.table_schema = $1
                AND tc.table_name = $2
        "#;

        let rows = sqlx::query(query)
            .bind(schema_name)
            .bind(table_name)
            .fetch_all(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let foreign_keys = rows
            .iter()
            .map(|row| {
                let name: String = row.get("constraint_name");
                let table_name: String = row.get("table_name");
                let column_name: String = row.get("column_name");
                let referenced_table: String = row.get("foreign_table_name");
                let referenced_column: String = row.get("foreign_column_name");
                let on_update: Option<String> = row.get("update_rule");
                let on_delete: Option<String> = row.get("delete_rule");

                ForeignKey {
                    name,
                    table_name,
                    column_name,
                    referenced_table,
                    referenced_column,
                    on_delete,
                    on_update,
                }
            })
            .collect();

        Ok(foreign_keys)
    }

    async fn get_table_row_count(&self, table_name: &str, schema: Option<&str>) -> Result<u64> {
        let schema_name = schema.unwrap_or("public");
        let query = format!("SELECT COUNT(*) FROM \"{}\".\"{}\"", schema_name, table_name);

        let row = sqlx::query(&query)
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let count: i64 = row.get(0);
        Ok(count as u64)
    }

    async fn table_exists(&self, table_name: &str, schema: Option<&str>) -> Result<bool> {
        let schema_name = schema.unwrap_or("public");

        let query = r#"
            SELECT COUNT(*)
            FROM information_schema.tables
            WHERE table_schema = $1 AND table_name = $2
        "#;

        let row = sqlx::query(query)
            .bind(schema_name)
            .bind(table_name)
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::SchemaInspection(e.to_string()))?;

        let count: i64 = row.get(0);
        Ok(count > 0)
    }
}
