use crate::{ConnectionConfig, Result, SqlTermError};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct SqlTermConfig {
    #[serde(default)]
    pub connections: Vec<ConnectionConfig>,
    #[serde(default)]
    pub settings: Settings,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Settings {
    pub default_connection: Option<String>,
    pub query_history_size: usize,
    pub auto_save_queries: bool,
    pub theme: String,
}

impl Default for Settings {
    fn default() -> Self {
        Self {
            default_connection: None,
            query_history_size: 100,
            auto_save_queries: true,
            theme: "default".to_string(),
        }
    }
}

impl SqlTermConfig {
    /// Get the default configuration file path
    pub fn default_config_path() -> Result<PathBuf> {
        let config_dir = dirs::config_dir()
            .ok_or_else(|| SqlTermError::Configuration("Could not find config directory".to_string()))?;
        
        let sqlterm_dir = config_dir.join("sqlterm");
        Ok(sqlterm_dir.join("config.toml"))
    }

    /// Load configuration from file
    pub fn load_from_file(path: &PathBuf) -> Result<Self> {
        if !path.exists() {
            return Ok(Self::default());
        }

        let content = std::fs::read_to_string(path)
            .map_err(|e| SqlTermError::Configuration(format!("Failed to read config file: {}", e)))?;

        let config: SqlTermConfig = toml::from_str(&content)
            .map_err(|e| SqlTermError::Configuration(format!("Failed to parse config file: {}", e)))?;

        Ok(config)
    }

    /// Save configuration to file
    pub fn save_to_file(&self, path: &PathBuf) -> Result<()> {
        // Create parent directories if they don't exist
        if let Some(parent) = path.parent() {
            std::fs::create_dir_all(parent)
                .map_err(|e| SqlTermError::Configuration(format!("Failed to create config directory: {}", e)))?;
        }

        let content = toml::to_string_pretty(self)
            .map_err(|e| SqlTermError::Configuration(format!("Failed to serialize config: {}", e)))?;

        std::fs::write(path, content)
            .map_err(|e| SqlTermError::Configuration(format!("Failed to write config file: {}", e)))?;

        Ok(())
    }

    /// Add a new connection
    pub fn add_connection(&mut self, connection: ConnectionConfig) -> Result<()> {
        // Check if connection with same name already exists
        if self.connections.iter().any(|c| c.name == connection.name) {
            return Err(SqlTermError::Configuration(
                format!("Connection with name '{}' already exists", connection.name),
            ));
        }

        self.connections.push(connection);
        Ok(())
    }

    /// Remove a connection by name
    pub fn remove_connection(&mut self, name: &str) -> Result<()> {
        let initial_len = self.connections.len();
        self.connections.retain(|c| c.name != name);
        
        if self.connections.len() == initial_len {
            return Err(SqlTermError::Configuration(
                format!("Connection '{}' not found", name),
            ));
        }

        // Clear default connection if it was removed
        if self.settings.default_connection.as_ref() == Some(&name.to_string()) {
            self.settings.default_connection = None;
        }

        Ok(())
    }

    /// Get a connection by name
    pub fn get_connection(&self, name: &str) -> Option<&ConnectionConfig> {
        self.connections.iter().find(|c| c.name == name)
    }

    /// Get the default connection
    pub fn get_default_connection(&self) -> Option<&ConnectionConfig> {
        if let Some(default_name) = &self.settings.default_connection {
            self.get_connection(default_name)
        } else {
            self.connections.first()
        }
    }

    /// Set the default connection
    pub fn set_default_connection(&mut self, name: &str) -> Result<()> {
        if !self.connections.iter().any(|c| c.name == name) {
            return Err(SqlTermError::Configuration(
                format!("Connection '{}' not found", name),
            ));
        }

        self.settings.default_connection = Some(name.to_string());
        Ok(())
    }

    /// List all connection names
    pub fn list_connection_names(&self) -> Vec<&str> {
        self.connections.iter().map(|c| c.name.as_str()).collect()
    }
}

/// Configuration manager for handling config file operations
pub struct ConfigManager {
    config_path: PathBuf,
    config: SqlTermConfig,
}

impl ConfigManager {
    /// Create a new config manager with default path
    pub fn new() -> Result<Self> {
        let config_path = SqlTermConfig::default_config_path()?;
        let config = SqlTermConfig::load_from_file(&config_path)?;
        
        Ok(Self {
            config_path,
            config,
        })
    }

    /// Create a new config manager with custom path
    pub fn with_path(config_path: PathBuf) -> Result<Self> {
        let config = SqlTermConfig::load_from_file(&config_path)?;
        
        Ok(Self {
            config_path,
            config,
        })
    }

    /// Get the current configuration
    pub fn config(&self) -> &SqlTermConfig {
        &self.config
    }

    /// Get mutable access to the configuration
    pub fn config_mut(&mut self) -> &mut SqlTermConfig {
        &mut self.config
    }

    /// Save the current configuration to file
    pub fn save(&self) -> Result<()> {
        self.config.save_to_file(&self.config_path)
    }

    /// Reload configuration from file
    pub fn reload(&mut self) -> Result<()> {
        self.config = SqlTermConfig::load_from_file(&self.config_path)?;
        Ok(())
    }

    /// Get the config file path
    pub fn config_path(&self) -> &PathBuf {
        &self.config_path
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::DatabaseType;
    use tempfile::tempdir;

    #[test]
    fn test_config_serialization() {
        let mut config = SqlTermConfig::default();
        
        config.add_connection(ConnectionConfig {
            name: "test".to_string(),
            database_type: DatabaseType::MySQL,
            host: "localhost".to_string(),
            port: 3306,
            database: "testdb".to_string(),
            username: "user".to_string(),
            password: Some("pass".to_string()),
            ssl: false,
            ssh_tunnel: None,
        }).unwrap();

        let serialized = toml::to_string_pretty(&config).unwrap();
        let deserialized: SqlTermConfig = toml::from_str(&serialized).unwrap();
        
        assert_eq!(config.connections.len(), deserialized.connections.len());
        assert_eq!(config.connections[0].name, deserialized.connections[0].name);
    }

    #[test]
    fn test_config_file_operations() {
        let temp_dir = tempdir().unwrap();
        let config_path = temp_dir.path().join("test_config.toml");
        
        let mut config = SqlTermConfig::default();
        config.add_connection(ConnectionConfig {
            name: "test".to_string(),
            database_type: DatabaseType::SQLite,
            host: "".to_string(),
            port: 0,
            database: "/tmp/test.db".to_string(),
            username: "".to_string(),
            password: None,
            ssl: false,
            ssh_tunnel: None,
        }).unwrap();

        // Save and reload
        config.save_to_file(&config_path).unwrap();
        let loaded_config = SqlTermConfig::load_from_file(&config_path).unwrap();
        
        assert_eq!(config.connections.len(), loaded_config.connections.len());
        assert_eq!(config.connections[0].name, loaded_config.connections[0].name);
    }
}
