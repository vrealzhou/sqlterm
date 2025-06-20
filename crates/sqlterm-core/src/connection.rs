use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use crate::Result;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConnectionConfig {
    pub name: String,
    pub database_type: DatabaseType,
    pub host: String,
    pub port: u16,
    pub database: String,
    pub username: String,
    pub password: Option<String>,
    pub ssl: bool,
    pub ssh_tunnel: Option<SshConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SshConfig {
    pub host: String,
    pub port: u16,
    pub username: String,
    pub private_key_path: Option<String>,
    pub password: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum DatabaseType {
    MySQL,
    PostgreSQL,
    SQLite,
}

impl std::fmt::Display for DatabaseType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            DatabaseType::MySQL => write!(f, "MySQL"),
            DatabaseType::PostgreSQL => write!(f, "PostgreSQL"),
            DatabaseType::SQLite => write!(f, "SQLite"),
        }
    }
}

#[derive(Debug, Clone)]
pub struct ConnectionInfo {
    pub server_version: String,
    pub database_name: String,
    pub username: String,
    pub connection_id: Option<String>,
}

#[async_trait]
pub trait DatabaseConnection: Send + Sync {
    /// Connect to the database using the provided configuration
    async fn connect(config: &ConnectionConfig) -> Result<Box<dyn DatabaseConnection>>
    where
        Self: Sized;
    
    /// Test if the connection is still alive
    async fn ping(&self) -> Result<()>;
    
    /// Get connection information
    async fn get_connection_info(&self) -> Result<ConnectionInfo>;
    
    /// Close the connection
    async fn close(&mut self) -> Result<()>;
    
    /// Get the database type
    fn database_type(&self) -> DatabaseType;
    
    /// Check if connection is active
    fn is_connected(&self) -> bool;
}
