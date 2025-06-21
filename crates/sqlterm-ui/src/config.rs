use anyhow::Result;
use serde::{Deserialize, Serialize};
use sqlterm_core::ConnectionConfig;
use std::fs;
use std::path::{Path, PathBuf};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StoredConnectionConfig {
    #[serde(flatten)]
    pub connection: ConnectionConfig,
    pub created_at: String,
    pub last_used: Option<String>,
}

pub struct ConfigManager {
    config_dir: PathBuf,
}

impl ConfigManager {
    pub fn new() -> Result<Self> {
        let config_dir = Self::get_config_directory()?;
        
        // Create config directory if it doesn't exist
        if !config_dir.exists() {
            fs::create_dir_all(&config_dir)?;
        }
        
        Ok(ConfigManager { config_dir })
    }

    fn get_config_directory() -> Result<PathBuf> {
        let home_dir = dirs::home_dir()
            .ok_or_else(|| anyhow::anyhow!("Could not find home directory"))?;
        
        Ok(home_dir.join(".config").join("sqlterm"))
    }

    fn get_connection_file_path(&self, connection_name: &str) -> PathBuf {
        // Sanitize connection name for filesystem
        let safe_name = connection_name
            .chars()
            .map(|c| match c {
                'a'..='z' | 'A'..='Z' | '0'..='9' | '-' | '_' => c,
                _ => '_',
            })
            .collect::<String>();
        
        self.config_dir.join(format!("{}.toml", safe_name))
    }

    pub fn save_connection(&self, config: &ConnectionConfig) -> Result<()> {
        let stored_config = StoredConnectionConfig {
            connection: config.clone(),
            created_at: chrono::Utc::now().to_rfc3339(),
            last_used: None,
        };

        let toml_content = toml::to_string_pretty(&stored_config)?;
        let file_path = self.get_connection_file_path(&config.name);
        
        fs::write(&file_path, toml_content)?;
        
        Ok(())
    }

    pub fn load_connection(&self, connection_name: &str) -> Result<ConnectionConfig> {
        let file_path = self.get_connection_file_path(connection_name);
        
        if !file_path.exists() {
            return Err(anyhow::anyhow!("Connection '{}' not found", connection_name));
        }

        let content = fs::read_to_string(&file_path)?;
        let stored_config: StoredConnectionConfig = toml::from_str(&content)?;
        
        Ok(stored_config.connection)
    }

    pub fn list_connections_with_errors(&self) -> Result<(Vec<ConnectionConfig>, Vec<String>)> {
        let mut connections = Vec::new();
        let mut errors = Vec::new();
        
        if !self.config_dir.exists() {
            return Ok((connections, errors));
        }

        for entry in fs::read_dir(&self.config_dir)? {
            let entry = entry?;
            let path = entry.path();
            
            if path.extension().and_then(|s| s.to_str()) == Some("toml") {
                match self.load_connection_from_path(&path) {
                    Ok(config) => connections.push(config),
                    Err(e) => {
                        // Collect errors to be logged by the app
                        errors.push(format!("Failed to load connection from {:?}: {}", path, e));
                    }
                }
            }
        }

        // Sort by name for consistent ordering
        connections.sort_by(|a, b| a.name.cmp(&b.name));
        
        Ok((connections, errors))
    }

    pub fn list_connections(&self) -> Result<Vec<ConnectionConfig>> {
        let (connections, _) = self.list_connections_with_errors()?;
        Ok(connections)
    }

    fn load_connection_from_path(&self, path: &Path) -> Result<ConnectionConfig> {
        let content = fs::read_to_string(path)?;
        let stored_config: StoredConnectionConfig = toml::from_str(&content)?;
        Ok(stored_config.connection)
    }

    pub fn delete_connection(&self, connection_name: &str) -> Result<()> {
        let file_path = self.get_connection_file_path(connection_name);
        
        if !file_path.exists() {
            return Err(anyhow::anyhow!("Connection '{}' not found", connection_name));
        }

        fs::remove_file(&file_path)?;
        Ok(())
    }

    pub fn connection_exists(&self, connection_name: &str) -> bool {
        let file_path = self.get_connection_file_path(connection_name);
        file_path.exists()
    }

    pub fn update_last_used(&self, connection_name: &str) -> Result<()> {
        let file_path = self.get_connection_file_path(connection_name);
        
        if !file_path.exists() {
            return Err(anyhow::anyhow!("Connection '{}' not found", connection_name));
        }

        let content = fs::read_to_string(&file_path)?;
        let mut stored_config: StoredConnectionConfig = toml::from_str(&content)?;
        
        stored_config.last_used = Some(chrono::Utc::now().to_rfc3339());
        
        let toml_content = toml::to_string_pretty(&stored_config)?;
        fs::write(&file_path, toml_content)?;
        
        Ok(())
    }

    pub fn get_config_directory_path(&self) -> &PathBuf {
        &self.config_dir
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use sqlterm_core::DatabaseType;
    use tempfile::TempDir;

    fn create_test_config_manager() -> (ConfigManager, TempDir) {
        let temp_dir = TempDir::new().unwrap();
        let config_manager = ConfigManager {
            config_dir: temp_dir.path().to_path_buf(),
        };
        (config_manager, temp_dir)
    }

    fn create_test_connection() -> ConnectionConfig {
        ConnectionConfig {
            name: "test-connection".to_string(),
            database_type: DatabaseType::SQLite,
            host: "localhost".to_string(),
            port: 0,
            database: ":memory:".to_string(),
            username: "test".to_string(),
            password: Some("password".to_string()),
            ssl: false,
            ssh_tunnel: None,
        }
    }

    #[test]
    fn test_save_and_load_connection() {
        let (config_manager, _temp_dir) = create_test_config_manager();
        let connection = create_test_connection();

        // Save connection
        config_manager.save_connection(&connection).unwrap();

        // Load connection
        let loaded = config_manager.load_connection(&connection.name).unwrap();
        
        assert_eq!(connection.name, loaded.name);
        assert_eq!(connection.database_type, loaded.database_type);
        assert_eq!(connection.host, loaded.host);
        assert_eq!(connection.port, loaded.port);
        assert_eq!(connection.database, loaded.database);
        assert_eq!(connection.username, loaded.username);
        assert_eq!(connection.password, loaded.password);
    }

    #[test]
    fn test_list_connections() {
        let (config_manager, _temp_dir) = create_test_config_manager();
        
        // Initially empty
        let connections = config_manager.list_connections().unwrap();
        assert_eq!(connections.len(), 0);

        // Add a connection
        let connection = create_test_connection();
        config_manager.save_connection(&connection).unwrap();

        // Should have one connection
        let connections = config_manager.list_connections().unwrap();
        assert_eq!(connections.len(), 1);
        assert_eq!(connections[0].name, connection.name);
    }

    #[test]
    fn test_delete_connection() {
        let (config_manager, _temp_dir) = create_test_config_manager();
        let connection = create_test_connection();

        // Save and verify exists
        config_manager.save_connection(&connection).unwrap();
        assert!(config_manager.connection_exists(&connection.name));

        // Delete and verify doesn't exist
        config_manager.delete_connection(&connection.name).unwrap();
        assert!(!config_manager.connection_exists(&connection.name));
    }

    #[test]
    fn test_connection_name_sanitization() {
        let (config_manager, _temp_dir) = create_test_config_manager();
        
        let mut connection = create_test_connection();
        connection.name = "test/connection:with*special|chars".to_string();
        
        config_manager.save_connection(&connection).unwrap();
        
        // Should be able to load it back
        let loaded = config_manager.load_connection(&connection.name).unwrap();
        assert_eq!(connection.name, loaded.name);
    }
}