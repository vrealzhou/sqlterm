use crate::{ConnectionConfig, DatabaseConnection, DatabaseType, Result, SqlTermError};

/// Factory for creating database connections based on configuration
pub struct ConnectionFactory;

impl ConnectionFactory {
    /// Create a database connection based on the configuration
    /// Note: This is a placeholder - actual connection creation should be done
    /// in the CLI layer where all database crates are available
    pub async fn create_connection(_config: &ConnectionConfig) -> Result<Box<dyn DatabaseConnection>> {
        Err(SqlTermError::Configuration(
            "Connection creation should be handled in the CLI layer".to_string(),
        ))
    }

    /// Get the default port for a database type
    pub fn get_default_port(db_type: &DatabaseType) -> u16 {
        match db_type {
            DatabaseType::MySQL => 3306,
            DatabaseType::PostgreSQL => 5432,
            DatabaseType::SQLite => 0, // SQLite doesn't use ports
        }
    }

    /// Validate a connection configuration
    pub fn validate_config(config: &ConnectionConfig) -> Result<()> {
        if config.name.is_empty() {
            return Err(SqlTermError::Configuration(
                "Connection name cannot be empty".to_string(),
            ));
        }

        if config.host.is_empty() && config.database_type != DatabaseType::SQLite {
            return Err(SqlTermError::Configuration(
                "Host cannot be empty for network databases".to_string(),
            ));
        }

        if config.database.is_empty() {
            return Err(SqlTermError::Configuration(
                "Database name/path cannot be empty".to_string(),
            ));
        }

        if config.username.is_empty() && config.database_type != DatabaseType::SQLite {
            return Err(SqlTermError::Configuration(
                "Username cannot be empty for network databases".to_string(),
            ));
        }

        // Validate port ranges for network databases
        if config.database_type != DatabaseType::SQLite && config.port == 0 {
            return Err(SqlTermError::Configuration(
                "Port number must be greater than 0 for network databases".to_string(),
            ));
        }

        Ok(())
    }

    /// Create a connection string for display purposes (without password)
    pub fn create_display_string(config: &ConnectionConfig) -> String {
        match config.database_type {
            DatabaseType::SQLite => {
                format!("sqlite://{}", config.database)
            }
            _ => {
                format!(
                    "{}://{}@{}:{}/{}",
                    config.database_type.to_string().to_lowercase(),
                    config.username,
                    config.host,
                    config.port,
                    config.database
                )
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_get_default_port() {
        assert_eq!(ConnectionFactory::get_default_port(&DatabaseType::MySQL), 3306);
        assert_eq!(ConnectionFactory::get_default_port(&DatabaseType::PostgreSQL), 5432);
        assert_eq!(ConnectionFactory::get_default_port(&DatabaseType::SQLite), 0);
    }

    #[test]
    fn test_validate_config() {
        let mut config = ConnectionConfig {
            name: "test".to_string(),
            database_type: DatabaseType::MySQL,
            host: "localhost".to_string(),
            port: 3306,
            database: "testdb".to_string(),
            username: "user".to_string(),
            password: None,
            ssl: false,
            ssh_tunnel: None,
        };

        assert!(ConnectionFactory::validate_config(&config).is_ok());

        // Test empty name
        config.name = "".to_string();
        assert!(ConnectionFactory::validate_config(&config).is_err());

        // Reset and test empty host
        config.name = "test".to_string();
        config.host = "".to_string();
        assert!(ConnectionFactory::validate_config(&config).is_err());

        // Test SQLite (should be OK with empty host)
        config.database_type = DatabaseType::SQLite;
        config.database = "/path/to/db.sqlite".to_string();
        assert!(ConnectionFactory::validate_config(&config).is_ok());
    }

    #[test]
    fn test_create_display_string() {
        let mysql_config = ConnectionConfig {
            name: "test".to_string(),
            database_type: DatabaseType::MySQL,
            host: "localhost".to_string(),
            port: 3306,
            database: "testdb".to_string(),
            username: "user".to_string(),
            password: Some("secret".to_string()),
            ssl: false,
            ssh_tunnel: None,
        };

        let display = ConnectionFactory::create_display_string(&mysql_config);
        assert_eq!(display, "mysql://user@localhost:3306/testdb");
        assert!(!display.contains("secret")); // Password should not be in display string

        let sqlite_config = ConnectionConfig {
            name: "test".to_string(),
            database_type: DatabaseType::SQLite,
            host: "".to_string(),
            port: 0,
            database: "/path/to/db.sqlite".to_string(),
            username: "".to_string(),
            password: None,
            ssl: false,
            ssh_tunnel: None,
        };

        let display = ConnectionFactory::create_display_string(&sqlite_config);
        assert_eq!(display, "sqlite:///path/to/db.sqlite");
    }
}
