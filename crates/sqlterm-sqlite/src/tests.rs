#[cfg(test)]
mod tests {
    use super::*;
    use sqlterm_core::{ConnectionConfig, DatabaseConnection, DatabaseType, Value};
    use std::env;
    use std::fs;
    use tempfile::NamedTempFile;

    // Helper function to create test connection config with temporary database
    fn create_test_config() -> (ConnectionConfig, NamedTempFile) {
        let temp_file = NamedTempFile::new().expect("Failed to create temp file");
        let config = ConnectionConfig {
            name: "test_sqlite".to_string(),
            database_type: DatabaseType::SQLite,
            host: "localhost".to_string(), // Not used for SQLite
            port: 0, // Not used for SQLite
            database: temp_file.path().to_string_lossy().to_string(),
            username: "sqlite".to_string(), // Not used for SQLite
            password: None, // Not used for SQLite
            ssl: false, // Not used for SQLite
        };
        (config, temp_file)
    }

    // Helper function to create connection for tests
    async fn create_test_connection() -> Result<(Box<dyn DatabaseConnection>, NamedTempFile), sqlterm_core::SqlTermError> {
        let (config, temp_file) = create_test_config();
        let connection = SqliteConnection::connect(&config).await?;
        Ok((connection, temp_file))
    }

    #[tokio::test]
    async fn test_connection() {
        if env::var("SKIP_SQLITE_TESTS").is_ok() {
            return;
        }

        let connection_result = create_test_connection().await;
        assert!(connection_result.is_ok(), "Failed to create SQLite connection");
        
        let (mut conn, _temp_file) = connection_result.unwrap();
        assert!(conn.is_connected(), "Connection should be active");
        
        let ping_result = conn.ping().await;
        assert!(ping_result.is_ok(), "Ping should succeed");
    }

    #[tokio::test]
    async fn test_connection_info() {
        if env::var("SKIP_SQLITE_TESTS").is_ok() {
            return;
        }

        let connection_result = create_test_connection().await;
        assert!(connection_result.is_ok());
        
        let (conn, _temp_file) = connection_result.unwrap();
        let info = conn.get_connection_info().await;
        assert!(info.is_ok(), "Should get connection info");
        
        let connection_info = info.unwrap();
        assert!(connection_info.server_version.contains("SQLite"), "Server version should contain SQLite");
        assert!(!connection_info.database_name.is_empty(), "Database name should not be empty");
        assert_eq!(connection_info.username, "sqlite", "Username should be sqlite");
    }

    #[tokio::test]
    async fn test_data_types() {
        if env::var("SKIP_SQLITE_TESTS").is_ok() {
            return;
        }

        let connection_result = create_test_connection().await;
        assert!(connection_result.is_ok());
        let (conn, _temp_file) = connection_result.unwrap();

        // Create test table with various SQLite data types
        let create_table_sql = r#"
            CREATE TABLE test_types (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                test_integer INTEGER,
                test_real REAL,
                test_text TEXT,
                test_blob BLOB,
                test_numeric NUMERIC,
                test_boolean BOOLEAN,
                test_date DATE,
                test_datetime DATETIME,
                test_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
            )
        "#;

        let create_result = conn.execute_query(create_table_sql).await;
        assert!(create_result.is_ok(), "Should create test table");

        // Insert test data
        let insert_sql = r#"
            INSERT INTO test_types (
                test_integer, test_real, test_text, test_blob,
                test_numeric, test_boolean,
                test_date, test_datetime
            ) VALUES (
                42, 3.14159, 'Hello SQLite', X'48656C6C6F',
                123.45, 1,
                '2023-12-25', '2023-12-25 14:30:00'
            )
        "#;

        let insert_result = conn.execute_query(insert_sql).await;
        assert!(insert_result.is_ok(), "Should insert test data");

        // Query and verify data types
        let query_result = conn.execute_query("SELECT * FROM test_types").await;
        assert!(query_result.is_ok(), "Should query test data");

        let result = query_result.unwrap();
        assert_eq!(result.rows.len(), 1, "Should have one row");

        let row = &result.rows[0];
        let values = &row.values;

        // Test INTEGER (SQLite stores as i64)
        assert!(matches!(values[0], Value::Integer(_)), "ID should be integer");
        assert!(matches!(values[1], Value::Integer(_)), "INTEGER should be integer");

        // Test REAL (SQLite stores as f64)
        assert!(matches!(values[2], Value::Float(_)), "REAL should be float");

        // Test TEXT (SQLite stores as String)
        assert!(matches!(values[3], Value::String(_)), "TEXT should be string");

        // Test BLOB (SQLite might store as String or bytes)
        assert!(matches!(values[4], Value::String(_)), "BLOB should be string");

        // Test NUMERIC (SQLite might store as float or string)
        assert!(matches!(values[5], Value::Float(_) | Value::String(_)), "NUMERIC should be float or string");

        // Test BOOLEAN (SQLite stores as integer)
        assert!(matches!(values[6], Value::Integer(_) | Value::Boolean(_)), "BOOLEAN should be integer or boolean");

        // Test DATE/DATETIME (SQLite stores as text)
        assert!(matches!(values[7], Value::String(_)), "DATE should be string");
        assert!(matches!(values[8], Value::String(_)), "DATETIME should be string");
        assert!(matches!(values[9], Value::String(_)), "TIMESTAMP should be string");
    }

    #[tokio::test]
    async fn test_null_values() {
        if env::var("SKIP_SQLITE_TESTS").is_ok() {
            return;
        }

        let connection_result = create_test_connection().await;
        assert!(connection_result.is_ok());
        let (conn, _temp_file) = connection_result.unwrap();

        // Create test table
        let create_table_sql = r#"
            CREATE TABLE test_nulls (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                nullable_int INTEGER,
                nullable_text TEXT,
                nullable_real REAL
            )
        "#;

        let create_result = conn.execute_query(create_table_sql).await;
        assert!(create_result.is_ok());

        // Insert row with nulls
        let insert_sql = "INSERT INTO test_nulls (nullable_int, nullable_text, nullable_real) VALUES (NULL, NULL, NULL)";
        let insert_result = conn.execute_query(insert_sql).await;
        assert!(insert_result.is_ok());

        // Query and verify nulls
        let query_result = conn.execute_query("SELECT * FROM test_nulls").await;
        assert!(query_result.is_ok());

        let result = query_result.unwrap();
        assert_eq!(result.rows.len(), 1);

        let row = &result.rows[0];
        let values = &row.values;

        // ID should not be null (auto-increment)
        assert!(matches!(values[0], Value::Integer(_)));
        
        // Other values should be null
        assert!(matches!(values[1], Value::Null));
        assert!(matches!(values[2], Value::Null));
        assert!(matches!(values[3], Value::Null));
    }

    #[tokio::test]
    async fn test_dynamic_typing() {
        if env::var("SKIP_SQLITE_TESTS").is_ok() {
            return;
        }

        let connection_result = create_test_connection().await;
        assert!(connection_result.is_ok());
        let (conn, _temp_file) = connection_result.unwrap();

        // SQLite allows different types in the same column
        let create_table_sql = "CREATE TABLE test_dynamic (id INTEGER PRIMARY KEY, value)";
        let create_result = conn.execute_query(create_table_sql).await;
        assert!(create_result.is_ok());

        // Insert different types
        let inserts = vec![
            "INSERT INTO test_dynamic (value) VALUES (42)",
            "INSERT INTO test_dynamic (value) VALUES (3.14)",
            "INSERT INTO test_dynamic (value) VALUES ('hello')",
            "INSERT INTO test_dynamic (value) VALUES (1)", // boolean-like
        ];

        for insert in inserts {
            let insert_result = conn.execute_query(insert).await;
            assert!(insert_result.is_ok());
        }

        // Query and verify dynamic types
        let query_result = conn.execute_query("SELECT * FROM test_dynamic ORDER BY id").await;
        assert!(query_result.is_ok());

        let result = query_result.unwrap();
        assert_eq!(result.rows.len(), 4);

        // First row: integer
        assert!(matches!(result.rows[0].values[1], Value::Integer(_)));
        
        // Second row: float
        assert!(matches!(result.rows[1].values[1], Value::Float(_)));
        
        // Third row: string
        assert!(matches!(result.rows[2].values[1], Value::String(_)));
        
        // Fourth row: integer (boolean-like)
        assert!(matches!(result.rows[3].values[1], Value::Integer(_)));
    }

    #[tokio::test]
    async fn test_list_tables() {
        if env::var("SKIP_SQLITE_TESTS").is_ok() {
            return;
        }

        let connection_result = create_test_connection().await;
        assert!(connection_result.is_ok());
        let (conn, _temp_file) = connection_result.unwrap();

        // Initially no tables
        let tables_result = conn.list_tables().await;
        assert!(tables_result.is_ok());
        let tables = tables_result.unwrap();
        assert_eq!(tables.len(), 0);

        // Create a test table
        let create_result = conn.execute_query("CREATE TABLE test_table (id INTEGER)").await;
        assert!(create_result.is_ok());

        // Now should have one table
        let tables_result = conn.list_tables().await;
        assert!(tables_result.is_ok());
        let tables = tables_result.unwrap();
        assert_eq!(tables.len(), 1);
        assert_eq!(tables[0], "test_table");
    }

    #[tokio::test]
    async fn test_table_details() {
        if env::var("SKIP_SQLITE_TESTS").is_ok() {
            return;
        }

        let connection_result = create_test_connection().await;
        assert!(connection_result.is_ok());
        let (conn, _temp_file) = connection_result.unwrap();

        // Create test table
        let create_table_sql = r#"
            CREATE TABLE test_details (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                name TEXT NOT NULL,
                age INTEGER,
                salary REAL DEFAULT 0.0
            )
        "#;

        let create_result = conn.execute_query(create_table_sql).await;
        assert!(create_result.is_ok());

        // Insert some data
        let insert_result = conn.execute_query("INSERT INTO test_details (name, age, salary) VALUES ('Alice', 30, 50000.0), ('Bob', 25, 45000.0)").await;
        assert!(insert_result.is_ok());

        // Get table details
        let details_result = conn.get_table_details("test_details").await;
        assert!(details_result.is_ok());

        let details = details_result.unwrap();
        assert_eq!(details.table.name, "test_details");
        assert_eq!(details.columns.len(), 4);
        assert_eq!(details.statistics.row_count, 2);

        // Check column details
        let id_col = &details.columns[0];
        assert_eq!(id_col.name, "id");
        assert_eq!(id_col.data_type, "INTEGER");
        assert!(id_col.is_primary_key);

        let name_col = &details.columns[1];
        assert_eq!(name_col.name, "name");
        assert_eq!(name_col.data_type, "TEXT");
        assert!(!name_col.nullable);
    }

    #[tokio::test]
    async fn test_large_integers() {
        if env::var("SKIP_SQLITE_TESTS").is_ok() {
            return;
        }

        let connection_result = create_test_connection().await;
        assert!(connection_result.is_ok());
        let (conn, _temp_file) = connection_result.unwrap();

        // Test various integer values
        let query_result = conn.execute_query("SELECT 9223372036854775807 as max_int, -9223372036854775808 as min_int, 0 as zero").await;
        assert!(query_result.is_ok());

        let result = query_result.unwrap();
        assert_eq!(result.rows.len(), 1);

        let row = &result.rows[0];
        let values = &row.values;

        // All should be parsed as integers
        assert!(matches!(values[0], Value::Integer(_)));
        assert!(matches!(values[1], Value::Integer(_)));
        assert!(matches!(values[2], Value::Integer(_)));

        // Verify actual values
        if let Value::Integer(val) = values[0] {
            assert_eq!(val, 9223372036854775807);
        }
        if let Value::Integer(val) = values[1] {
            assert_eq!(val, -9223372036854775808);
        }
        if let Value::Integer(val) = values[2] {
            assert_eq!(val, 0);
        }
    }

    #[tokio::test]
    async fn test_datetime_functions() {
        if env::var("SKIP_SQLITE_TESTS").is_ok() {
            return;
        }

        let connection_result = create_test_connection().await;
        assert!(connection_result.is_ok());
        let (conn, _temp_file) = connection_result.unwrap();

        let query_result = conn.execute_query("SELECT datetime('now') as current_datetime, date('now') as current_date, time('now') as current_time").await;
        assert!(query_result.is_ok());

        let result = query_result.unwrap();
        assert_eq!(result.rows.len(), 1);

        let row = &result.rows[0];
        let values = &row.values;

        // All should be formatted as strings
        assert!(matches!(values[0], Value::String(_)));
        assert!(matches!(values[1], Value::String(_)));
        assert!(matches!(values[2], Value::String(_)));

        // Verify datetime format
        if let Value::String(datetime) = &values[0] {
            assert!(datetime.contains("-"), "Datetime should contain date separator");
            assert!(datetime.contains(":"), "Datetime should contain time separator");
        }
    }

    #[tokio::test]
    async fn test_blob_data() {
        if env::var("SKIP_SQLITE_TESTS").is_ok() {
            return;
        }

        let connection_result = create_test_connection().await;
        assert!(connection_result.is_ok());
        let (conn, _temp_file) = connection_result.unwrap();

        // Create table with blob
        let create_result = conn.execute_query("CREATE TABLE test_blob (id INTEGER PRIMARY KEY, data BLOB)").await;
        assert!(create_result.is_ok());

        // Insert blob data
        let insert_result = conn.execute_query("INSERT INTO test_blob (data) VALUES (X'48656C6C6F20576F726C64')").await;
        assert!(insert_result.is_ok());

        // Query blob data
        let query_result = conn.execute_query("SELECT * FROM test_blob").await;
        assert!(query_result.is_ok());

        let result = query_result.unwrap();
        assert_eq!(result.rows.len(), 1);

        let row = &result.rows[0];
        let values = &row.values;

        // ID should be integer
        assert!(matches!(values[0], Value::Integer(_)));
        
        // BLOB data should be string (hex representation or decoded)
        assert!(matches!(values[1], Value::String(_)));
    }

    #[tokio::test]
    async fn test_mixed_case_column_names() {
        if env::var("SKIP_SQLITE_TESTS").is_ok() {
            return;
        }

        let connection_result = create_test_connection().await;
        assert!(connection_result.is_ok());
        let (conn, _temp_file) = connection_result.unwrap();

        // SQLite is case-insensitive for column names
        let create_result = conn.execute_query("CREATE TABLE test_case (ID INTEGER, Name TEXT, AGE integer)").await;
        assert!(create_result.is_ok());

        let insert_result = conn.execute_query("INSERT INTO test_case (id, name, age) VALUES (1, 'Alice', 30)").await;
        assert!(insert_result.is_ok());

        let query_result = conn.execute_query("SELECT * FROM test_case").await;
        assert!(query_result.is_ok());

        let result = query_result.unwrap();
        assert_eq!(result.rows.len(), 1);
        assert_eq!(result.columns.len(), 3);

        // Verify column names (SQLite preserves case from CREATE TABLE)
        assert_eq!(result.columns[0].name, "ID");
        assert_eq!(result.columns[1].name, "Name");
        assert_eq!(result.columns[2].name, "AGE");
    }
}