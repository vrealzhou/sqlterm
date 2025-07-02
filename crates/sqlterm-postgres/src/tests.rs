#[cfg(test)]
mod tests {
    use super::*;
    use sqlterm_core::{ConnectionConfig, DatabaseConnection, DatabaseType, Value};
    use std::env;

    // Helper function to create test connection config
    fn create_test_config() -> ConnectionConfig {
        ConnectionConfig {
            name: "test_postgres".to_string(),
            database_type: DatabaseType::PostgreSQL,
            host: env::var("POSTGRES_HOST").unwrap_or_else(|_| "localhost".to_string()),
            port: env::var("POSTGRES_PORT")
                .unwrap_or_else(|_| "5432".to_string())
                .parse()
                .unwrap_or(5432),
            database: env::var("POSTGRES_DB").unwrap_or_else(|_| "postgres".to_string()),
            username: env::var("POSTGRES_USER").unwrap_or_else(|_| "postgres".to_string()),
            password: env::var("POSTGRES_PASSWORD").map(|p| p),
            ssl: false,
        }
    }

    // Helper function to create connection for tests
    async fn create_test_connection() -> Result<Box<dyn DatabaseConnection>, sqlterm_core::SqlTermError> {
        let config = create_test_config();
        PostgresConnection::connect(&config).await
    }

    #[tokio::test]
    async fn test_connection() {
        if env::var("SKIP_POSTGRES_TESTS").is_ok() {
            return;
        }

        let connection = create_test_connection().await;
        assert!(connection.is_ok(), "Failed to create PostgreSQL connection");
        
        let mut conn = connection.unwrap();
        assert!(conn.is_connected(), "Connection should be active");
        
        let ping_result = conn.ping().await;
        assert!(ping_result.is_ok(), "Ping should succeed");
    }

    #[tokio::test]
    async fn test_connection_info() {
        if env::var("SKIP_POSTGRES_TESTS").is_ok() {
            return;
        }

        let connection = create_test_connection().await;
        assert!(connection.is_ok());
        
        let conn = connection.unwrap();
        let info = conn.get_connection_info().await;
        assert!(info.is_ok(), "Should get connection info");
        
        let connection_info = info.unwrap();
        assert!(!connection_info.server_version.is_empty(), "Server version should not be empty");
        assert!(!connection_info.database_name.is_empty(), "Database name should not be empty");
        assert!(!connection_info.username.is_empty(), "Username should not be empty");
    }

    #[tokio::test]
    async fn test_data_types() {
        if env::var("SKIP_POSTGRES_TESTS").is_ok() {
            return;
        }

        let connection = create_test_connection().await;
        assert!(connection.is_ok());
        let conn = connection.unwrap();

        // Create test table with various data types
        let create_table_sql = r#"
            CREATE TEMPORARY TABLE test_types (
                id SERIAL PRIMARY KEY,
                test_smallint SMALLINT,
                test_integer INTEGER,
                test_bigint BIGINT,
                test_real REAL,
                test_double DOUBLE PRECISION,
                test_numeric NUMERIC(10,2),
                test_boolean BOOLEAN,
                test_char CHAR(10),
                test_varchar VARCHAR(50),
                test_text TEXT,
                test_date DATE,
                test_time TIME,
                test_timestamp TIMESTAMP,
                test_timestamptz TIMESTAMPTZ
            )
        "#;

        let create_result = conn.execute_query(create_table_sql).await;
        assert!(create_result.is_ok(), "Should create test table");

        // Insert test data
        let insert_sql = r#"
            INSERT INTO test_types (
                test_smallint, test_integer, test_bigint,
                test_real, test_double, test_numeric,
                test_boolean,
                test_char, test_varchar, test_text,
                test_date, test_time, test_timestamp, test_timestamptz
            ) VALUES (
                32767, 2147483647, 9223372036854775807,
                3.14159, 2.718281828459045, 123.45,
                true,
                'char_test', 'varchar_test', 'This is a long text field for testing',
                '2023-12-25', '14:30:00', '2023-12-25 14:30:00', '2023-12-25 14:30:00+00'
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

        // Test INTEGER types
        assert!(matches!(values[0], Value::Integer(_)), "ID should be integer");
        assert!(matches!(values[1], Value::Integer(_)), "SMALLINT should be integer");
        assert!(matches!(values[2], Value::Integer(_)), "INTEGER should be integer");
        assert!(matches!(values[3], Value::Integer(_)), "BIGINT should be integer");

        // Test FLOAT types
        assert!(matches!(values[4], Value::Float(_)), "REAL should be float");
        assert!(matches!(values[5], Value::Float(_)), "DOUBLE PRECISION should be float");
        assert!(matches!(values[6], Value::Float(_)), "NUMERIC should be float");

        // Test BOOLEAN
        assert!(matches!(values[7], Value::Boolean(_)), "BOOLEAN should be boolean");

        // Test STRING types
        assert!(matches!(values[8], Value::String(_)), "CHAR should be string");
        assert!(matches!(values[9], Value::String(_)), "VARCHAR should be string");
        assert!(matches!(values[10], Value::String(_)), "TEXT should be string");

        // Test DATE/TIME types (stored as strings)
        assert!(matches!(values[11], Value::String(_)), "DATE should be string");
        assert!(matches!(values[12], Value::String(_)), "TIME should be string");
        assert!(matches!(values[13], Value::String(_)), "TIMESTAMP should be string");
        assert!(matches!(values[14], Value::String(_)), "TIMESTAMPTZ should be string");
    }

    #[tokio::test]
    async fn test_null_values() {
        if env::var("SKIP_POSTGRES_TESTS").is_ok() {
            return;
        }

        let connection = create_test_connection().await;
        assert!(connection.is_ok());
        let conn = connection.unwrap();

        // Create test table
        let create_table_sql = r#"
            CREATE TEMPORARY TABLE test_nulls (
                id SERIAL PRIMARY KEY,
                nullable_int INTEGER,
                nullable_text TEXT,
                nullable_bool BOOLEAN
            )
        "#;

        let create_result = conn.execute_query(create_table_sql).await;
        assert!(create_result.is_ok());

        // Insert row with nulls
        let insert_sql = "INSERT INTO test_nulls (nullable_int, nullable_text, nullable_bool) VALUES (NULL, NULL, NULL)";
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
    async fn test_list_tables() {
        if env::var("SKIP_POSTGRES_TESTS").is_ok() {
            return;
        }

        let connection = create_test_connection().await;
        assert!(connection.is_ok());
        let conn = connection.unwrap();

        // Create a test table
        let create_result = conn.execute_query("CREATE TEMPORARY TABLE test_list_table (id INTEGER)").await;
        assert!(create_result.is_ok());

        let tables_result = conn.list_tables().await;
        assert!(tables_result.is_ok(), "Should list tables successfully");
        
        // Note: Temporary tables might not show up in public schema listing
        // This test mainly verifies the method doesn't crash
    }

    #[tokio::test]
    async fn test_large_integers() {
        if env::var("SKIP_POSTGRES_TESTS").is_ok() {
            return;
        }

        let connection = create_test_connection().await;
        assert!(connection.is_ok());
        let conn = connection.unwrap();

        // Test various integer sizes
        let query_result = conn.execute_query("SELECT 32767::SMALLINT as small, 2147483647::INTEGER as medium, 9223372036854775807::BIGINT as large").await;
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
            assert_eq!(val, 32767);
        }
        if let Value::Integer(val) = values[1] {
            assert_eq!(val, 2147483647);
        }
        if let Value::Integer(val) = values[2] {
            assert_eq!(val, 9223372036854775807);
        }
    }

    #[tokio::test]
    async fn test_datetime_formatting() {
        if env::var("SKIP_POSTGRES_TESTS").is_ok() {
            return;
        }

        let connection = create_test_connection().await;
        assert!(connection.is_ok());
        let conn = connection.unwrap();

        let query_result = conn.execute_query("SELECT NOW() as current_timestamp, CURRENT_DATE as current_date").await;
        assert!(query_result.is_ok());

        let result = query_result.unwrap();
        assert_eq!(result.rows.len(), 1);

        let row = &result.rows[0];
        let values = &row.values;

        // Both should be formatted as strings
        assert!(matches!(values[0], Value::String(_)));
        assert!(matches!(values[1], Value::String(_)));

        // Verify timestamp format (should contain date and time)
        if let Value::String(timestamp) = &values[0] {
            assert!(timestamp.contains("-"), "Timestamp should contain date separator");
            assert!(timestamp.contains(":"), "Timestamp should contain time separator");
        }
    }
}