use async_trait::async_trait;
use sqlx::{Pool, Row, Sqlite, SqlitePool};
use sqlterm_core::{
    ConnectionConfig, ConnectionInfo, DatabaseConnection, DatabaseType, Result, SqlTermError,
};

pub struct SqliteConnection {
    pool: Pool<Sqlite>,
    config: ConnectionConfig,
    connected: bool,
}

impl SqliteConnection {
    fn build_connection_string(config: &ConnectionConfig) -> String {
        // For SQLite, the database field contains the file path
        format!("sqlite:{}", config.database)
    }
}

#[async_trait]
impl DatabaseConnection for SqliteConnection {
    async fn connect(config: &ConnectionConfig) -> Result<Box<dyn DatabaseConnection>> {
        if config.database_type != DatabaseType::SQLite {
            return Err(SqlTermError::Configuration(
                "Invalid database type for SQLite connection".to_string(),
            ));
        }

        let connection_string = Self::build_connection_string(config);
        
        let pool = SqlitePool::connect(&connection_string)
            .await
            .map_err(|e| SqlTermError::Connection(e.to_string()))?;

        Ok(Box::new(SqliteConnection {
            pool,
            config: config.clone(),
            connected: true,
        }))
    }

    async fn ping(&self) -> Result<()> {
        sqlx::query("SELECT 1")
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::Connection(e.to_string()))?;
        Ok(())
    }

    async fn get_connection_info(&self) -> Result<ConnectionInfo> {
        let row = sqlx::query("SELECT sqlite_version() as version")
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::Connection(e.to_string()))?;

        let server_version: String = row.get("version");

        Ok(ConnectionInfo {
            server_version: format!("SQLite {}", server_version),
            database_name: self.config.database.clone(),
            username: "sqlite".to_string(),
            connection_id: None,
        })
    }

    async fn close(&mut self) -> Result<()> {
        self.pool.close().await;
        self.connected = false;
        Ok(())
    }

    fn database_type(&self) -> DatabaseType {
        DatabaseType::SQLite
    }

    fn is_connected(&self) -> bool {
        self.connected && !self.pool.is_closed()
    }
}

impl SqliteConnection {
    pub fn pool(&self) -> &Pool<Sqlite> {
        &self.pool
    }
}
