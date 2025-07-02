use async_trait::async_trait;
use sqlx::{MySql, Pool, Row, Column, TypeInfo, ValueRef};
use sqlterm_core::{
    Query, QueryExecutor, QueryResult, QueryExecution, PreparedStatement, Transaction,
    Result, SqlTermError, Value, ColumnInfo
};
use std::time::Instant;

pub struct MySqlQueryExecutor {
    pool: Pool<MySql>,
}

impl MySqlQueryExecutor {
    pub fn new(pool: Pool<MySql>) -> Self {
        Self { pool }
    }
}

#[async_trait]
impl QueryExecutor for MySqlQueryExecutor {
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
                nullable: true, // MySQL doesn't provide this info easily
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
                        // Try to get the column type info
                        let column = &row.columns()[i];
                        let type_name = column.type_info().name();
                        
                        // Check if the column is null using the raw value interface
                        use sqlx::Row as _;
                        if let Ok(raw_value) = row.try_get_raw(i) {
                            if raw_value.is_null() {
                                return Value::Null;
                            }
                        }
                        
                        // MySQL type conversion - check by MySQL type name first
                        match type_name {
                            // MySQL integer types
                            "TINYINT" => {
                                if let Ok(val) = row.try_get::<i8, _>(i) {
                                    Value::Integer(val as i64)
                                } else {
                                    Value::Unknown(format!("Failed to parse {} as i8", type_name))
                                }
                            }
                            "SMALLINT" => {
                                if let Ok(val) = row.try_get::<i16, _>(i) {
                                    Value::Integer(val as i64)
                                } else {
                                    Value::Unknown(format!("Failed to parse {} as i16", type_name))
                                }
                            }
                            "MEDIUMINT" | "INT" | "INTEGER" => {
                                if let Ok(val) = row.try_get::<i32, _>(i) {
                                    Value::Integer(val as i64)
                                } else {
                                    Value::Unknown(format!("Failed to parse {} as i32", type_name))
                                }
                            }
                            "BIGINT" => {
                                if let Ok(val) = row.try_get::<i64, _>(i) {
                                    Value::Integer(val)
                                } else {
                                    Value::Unknown(format!("Failed to parse {} as i64", type_name))
                                }
                            }
                            // MySQL float types
                            "FLOAT" => {
                                if let Ok(val) = row.try_get::<f32, _>(i) {
                                    Value::Float(val as f64)
                                } else {
                                    Value::Unknown(format!("Failed to parse {} as f32", type_name))
                                }
                            }
                            "DOUBLE" => {
                                if let Ok(val) = row.try_get::<f64, _>(i) {
                                    Value::Float(val)
                                } else {
                                    Value::Unknown(format!("Failed to parse {} as f64", type_name))
                                }
                            }
                            // MySQL boolean type
                            "BOOLEAN" | "BOOL" => {
                                if let Ok(val) = row.try_get::<bool, _>(i) {
                                    Value::Boolean(val)
                                } else {
                                    Value::Unknown(format!("Failed to parse {} as bool", type_name))
                                }
                            }
                            // MySQL decimal/numeric types
                            "DECIMAL" | "NUMERIC" => {
                                // Try as f64 first, then as string for high precision
                                if let Ok(val) = row.try_get::<f64, _>(i) {
                                    Value::Float(val)
                                } else if let Ok(val) = row.try_get::<String, _>(i) {
                                    Value::String(val)
                                } else {
                                    Value::Unknown(format!("Failed to parse {} as f64 or String", type_name))
                                }
                            }
                            // MySQL text types
                            "VARCHAR" | "CHAR" | "TEXT" | "TINYTEXT" | "MEDIUMTEXT" | "LONGTEXT" | "JSON" => {
                                if let Ok(val) = row.try_get::<String, _>(i) {
                                    Value::String(val)
                                } else {
                                    Value::Unknown(format!("Failed to parse {} as String", type_name))
                                }
                            }
                            // MySQL date/time types
                            "DATETIME" | "TIMESTAMP" => {
                                if let Ok(val) = row.try_get::<String, _>(i) {
                                    Value::DateTime(val)
                                } else {
                                    Value::Unknown(format!("Failed to parse {} as String", type_name))
                                }
                            }
                            "DATE" => {
                                if let Ok(val) = row.try_get::<String, _>(i) {
                                    Value::Date(val)
                                } else {
                                    Value::Unknown(format!("Failed to parse {} as String", type_name))
                                }
                            }
                            "TIME" => {
                                if let Ok(val) = row.try_get::<String, _>(i) {
                                    Value::Time(val)
                                } else {
                                    Value::Unknown(format!("Failed to parse {} as String", type_name))
                                }
                            }
                            // Default fallback - try different types
                            _ => {
                                // Try common types in order
                                if let Ok(val) = row.try_get::<i32, _>(i) {
                                    Value::Integer(val as i64)
                                } else if let Ok(val) = row.try_get::<i64, _>(i) {
                                    Value::Integer(val)
                                } else if let Ok(val) = row.try_get::<i16, _>(i) {
                                    Value::Integer(val as i64)
                                } else if let Ok(val) = row.try_get::<i8, _>(i) {
                                    Value::Integer(val as i64)
                                } else if let Ok(val) = row.try_get::<u32, _>(i) {
                                    Value::Integer(val as i64)
                                } else if let Ok(val) = row.try_get::<u16, _>(i) {
                                    Value::Integer(val as i64)
                                } else if let Ok(val) = row.try_get::<u8, _>(i) {
                                    Value::Integer(val as i64)
                                } else if let Ok(val) = row.try_get::<f64, _>(i) {
                                    Value::Float(val)
                                } else if let Ok(val) = row.try_get::<f32, _>(i) {
                                    Value::Float(val as f64)
                                } else if let Ok(val) = row.try_get::<bool, _>(i) {
                                    Value::Boolean(val)
                                } else if let Ok(val) = row.try_get::<String, _>(i) {
                                    Value::String(val)
                                } else {
                                    Value::Unknown(format!("Unknown MySQL type: {}", type_name))
                                }
                            }
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
