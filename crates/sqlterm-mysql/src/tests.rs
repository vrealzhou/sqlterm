#[cfg(test)]
mod tests {
    use super::*;
    use sqlterm_core::{ConnectionConfig, DatabaseConnection, DatabaseType, Value};
    use std::env;

    // Helper function to create test connection config
    fn create_test_config() -> ConnectionConfig {
        ConnectionConfig {
            name: "test_mysql".to_string(),
            database_type: DatabaseType::MySQL,
            host: env::var("MYSQL_HOST").unwrap_or_else(|_| "localhost".to_string()),
            port: env::var("MYSQL_PORT")
                .unwrap_or_else(|_| "3306".to_string())
                .parse()
                .unwrap_or(3306),
            database: env::var("MYSQL_DATABASE").unwrap_or_else(|_| "testdb".to_string()),
            username: env::var("MYSQL_USER").unwrap_or_else(|_| "root".to_string()),
            password: env::var("MYSQL_PASSWORD").map(|p| p),
            ssl: false,
        }
    }

    // Helper function to create connection for tests
    async fn create_test_connection() -> Result<Box<dyn DatabaseConnection>, sqlterm_core::SqlTermError> {
        let config = create_test_config();
        MySqlConnection::connect(&config).await
    }

    #[tokio::test]
    async fn test_connection() {
        if env::var("SKIP_MYSQL_TESTS").is_ok() {
            return;
        }

        let connection = create_test_connection().await;
        assert!(connection.is_ok(), "Failed to create MySQL connection");
        
        let mut conn = connection.unwrap();
        assert!(conn.is_connected(), "Connection should be active");
        
        let ping_result = conn.ping().await;
        assert!(ping_result.is_ok(), "Ping should succeed");
    }

    #[tokio::test]
    async fn test_connection_info() {
        if env::var("SKIP_MYSQL_TESTS").is_ok() {
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
        if env::var("SKIP_MYSQL_TESTS").is_ok() {
            return;
        }

        let connection = create_test_connection().await;
        assert!(connection.is_ok());
        let conn = connection.unwrap();

        // Create test table with various MySQL data types
        let create_table_sql = r#"
            CREATE TEMPORARY TABLE test_types (
                id INT AUTO_INCREMENT PRIMARY KEY,
                test_tinyint TINYINT,
                test_smallint SMALLINT,
                test_mediumint MEDIUMINT,
                test_int INT,
                test_bigint BIGINT,
                test_float FLOAT,
                test_double DOUBLE,
                test_decimal DECIMAL(10,2),
                test_bit BIT(1),
                test_char CHAR(10),
                test_varchar VARCHAR(50),
                test_text TEXT,
                test_date DATE,
                test_time TIME,
                test_datetime DATETIME,
                test_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                test_year YEAR
            )
        "#;

        let create_result = conn.execute_query(create_table_sql).await;
        assert!(create_result.is_ok(), "Should create test table");

        // Insert test data
        let insert_sql = r#"
            INSERT INTO test_types (
                test_tinyint, test_smallint, test_mediumint, test_int, test_bigint,
                test_float, test_double, test_decimal,
                test_bit,
                test_char, test_varchar, test_text,
                test_date, test_time, test_datetime,
                test_year
            ) VALUES (
                127, 32767, 8388607, 2147483647, 9223372036854775807,
                3.14159, 2.718281828459045, 123.45,
                1,
                'char_test', 'varchar_test', 'This is a long text field for testing',
                '2023-12-25', '14:30:00', '2023-12-25 14:30:00',
                2023
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

        // Test INTEGER types - MySQL returns different integer types
        assert!(matches!(values[0], Value::Integer(_)), "ID should be integer");
        assert!(matches!(values[1], Value::Integer(_)), "TINYINT should be integer");
        assert!(matches!(values[2], Value::Integer(_)), "SMALLINT should be integer");
        assert!(matches!(values[3], Value::Integer(_)), "MEDIUMINT should be integer");
        assert!(matches!(values[4], Value::Integer(_)), "INT should be integer");
        assert!(matches!(values[5], Value::Integer(_)), "BIGINT should be integer");

        // Test FLOAT types
        assert!(matches!(values[6], Value::Float(_)), "FLOAT should be float");
        assert!(matches!(values[7], Value::Float(_)), "DOUBLE should be float");
        assert!(matches!(values[8], Value::Float(_)), "DECIMAL should be float");

        // Test BIT (might be integer or boolean depending on MySQL version)
        assert!(matches!(values[9], Value::Integer(_) | Value::Boolean(_)), "BIT should be integer or boolean");

        // Test STRING types
        assert!(matches!(values[10], Value::String(_)), "CHAR should be string");
        assert!(matches!(values[11], Value::String(_)), "VARCHAR should be string");
        assert!(matches!(values[12], Value::String(_)), "TEXT should be string");

        // Test DATE/TIME types (stored as strings)
        assert!(matches!(values[13], Value::String(_)), "DATE should be string");
        assert!(matches!(values[14], Value::String(_)), "TIME should be string");
        assert!(matches!(values[15], Value::String(_)), "DATETIME should be string");
        assert!(matches!(values[16], Value::String(_)), "TIMESTAMP should be string");
        assert!(matches!(values[17], Value::Integer(_)), "YEAR should be integer");
    }

    #[tokio::test]
    async fn test_null_values() {
        if env::var("SKIP_MYSQL_TESTS").is_ok() {
            return;
        }

        let connection = create_test_connection().await;
        assert!(connection.is_ok());
        let conn = connection.unwrap();

        // Create test table
        let create_table_sql = r#"
            CREATE TEMPORARY TABLE test_nulls (
                id INT AUTO_INCREMENT PRIMARY KEY,
                nullable_int INT,
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
    async fn test_boolean_values() {
        if env::var("SKIP_MYSQL_TESTS").is_ok() {
            return;
        }

        let connection = create_test_connection().await;
        assert!(connection.is_ok());
        let conn = connection.unwrap();

        // MySQL BOOLEAN is actually TINYINT(1)
        let query_result = conn.execute_query("SELECT TRUE as true_val, FALSE as false_val, 1 as one_val, 0 as zero_val").await;
        assert!(query_result.is_ok());

        let result = query_result.unwrap();
        assert_eq!(result.rows.len(), 1);

        let row = &result.rows[0];
        let values = &row.values;

        // In MySQL, boolean values might be returned as integers
        for value in values {
            assert!(matches!(value, Value::Boolean(_) | Value::Integer(_)));
        }
    }

    #[tokio::test]
    async fn test_list_tables() {
        if env::var("SKIP_MYSQL_TESTS").is_ok() {
            return;
        }

        let connection = create_test_connection().await;
        assert!(connection.is_ok());
        let conn = connection.unwrap();

        let tables_result = conn.list_tables().await;
        assert!(tables_result.is_ok(), "Should list tables successfully");
        
        let tables = tables_result.unwrap();
        // Should at least return an empty list without error
        assert!(tables.len() >= 0);
    }

    #[tokio::test]
    async fn test_large_integers() {
        if env::var("SKIP_MYSQL_TESTS").is_ok() {
            return;
        }

        let connection = create_test_connection().await;
        assert!(connection.is_ok());
        let conn = connection.unwrap();

        // Test various integer sizes
        let query_result = conn.execute_query("SELECT 127 as tiny_max, 32767 as small_max, 2147483647 as int_max, 9223372036854775807 as bigint_max").await;
        assert!(query_result.is_ok());

        let result = query_result.unwrap();
        assert_eq!(result.rows.len(), 1);

        let row = &result.rows[0];
        let values = &row.values;

        // All should be parsed as integers
        for value in values {
            assert!(matches!(value, Value::Integer(_)));
        }

        // Verify actual values
        if let Value::Integer(val) = values[0] {
            assert_eq!(val, 127);
        }
        if let Value::Integer(val) = values[1] {
            assert_eq!(val, 32767);
        }
        if let Value::Integer(val) = values[2] {
            assert_eq!(val, 2147483647);
        }
        if let Value::Integer(val) = values[3] {
            assert_eq!(val, 9223372036854775807);
        }
    }

    #[tokio::test]
    async fn test_datetime_formatting() {
        if env::var("SKIP_MYSQL_TESTS").is_ok() {
            return;
        }

        let connection = create_test_connection().await;
        assert!(connection.is_ok());
        let conn = connection.unwrap();

        let query_result = conn.execute_query("SELECT NOW() as current_timestamp, CURDATE() as current_date, CURTIME() as current_time").await;
        assert!(query_result.is_ok());

        let result = query_result.unwrap();
        assert_eq!(result.rows.len(), 1);

        let row = &result.rows[0];
        let values = &row.values;

        // All should be formatted as strings
        assert!(matches!(values[0], Value::String(_)));
        assert!(matches!(values[1], Value::String(_)));
        assert!(matches!(values[2], Value::String(_)));

        // Verify timestamp format (should contain date and time)
        if let Value::String(timestamp) = &values[0] {
            assert!(timestamp.contains("-"), "Timestamp should contain date separator");
            assert!(timestamp.contains(":"), "Timestamp should contain time separator");
        }
    }

    #[tokio::test]
    async fn test_unsigned_integers() {
        if env::var("SKIP_MYSQL_TESTS").is_ok() {
            return;
        }

        let connection = create_test_connection().await;
        assert!(connection.is_ok());
        let conn = connection.unwrap();

        // Create table with unsigned integers
        let create_result = conn.execute_query("CREATE TEMPORARY TABLE test_unsigned (id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, big_unsigned BIGINT UNSIGNED)").await;
        assert!(create_result.is_ok());

        let insert_result = conn.execute_query("INSERT INTO test_unsigned (big_unsigned) VALUES (18446744073709551615)").await;
        assert!(insert_result.is_ok());

        let query_result = conn.execute_query("SELECT * FROM test_unsigned").await;
        assert!(query_result.is_ok());

        let result = query_result.unwrap();
        assert_eq!(result.rows.len(), 1);

        let row = &result.rows[0];
        let values = &row.values;

        // Both should be integers (though large unsigned might overflow i64)
        assert!(matches!(values[0], Value::Integer(_)));
        assert!(matches!(values[1], Value::Integer(_)));
    }
}