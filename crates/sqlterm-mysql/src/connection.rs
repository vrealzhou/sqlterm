use async_trait::async_trait;
use sqlx::{MySql, MySqlPool, Pool, Row};
use sqlterm_core::{
    ConnectionConfig, ConnectionInfo, DatabaseConnection, DatabaseType, Result, SqlTermError,
};

pub struct MySqlConnection {
    pool: Pool<MySql>,
    config: ConnectionConfig,
    connected: bool,
}

impl MySqlConnection {
    fn build_connection_string(config: &ConnectionConfig) -> String {
        let mut url = format!(
            "mysql://{}:{}@{}:{}/{}",
            config.username,
            config.password.as_deref().unwrap_or(""),
            config.host,
            config.port,
            config.database
        );
        
        if config.ssl {
            url.push_str("?ssl-mode=required");
        }
        
        url
    }
}

#[async_trait]
impl DatabaseConnection for MySqlConnection {
    async fn connect(config: &ConnectionConfig) -> Result<Box<dyn DatabaseConnection>> {
        if config.database_type != DatabaseType::MySQL {
            return Err(SqlTermError::Configuration(
                "Invalid database type for MySQL connection".to_string(),
            ));
        }

        let connection_string = Self::build_connection_string(config);
        
        let pool = MySqlPool::connect(&connection_string)
            .await
            .map_err(|e| SqlTermError::Connection(e.to_string()))?;

        Ok(Box::new(MySqlConnection {
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
        let row = sqlx::query("SELECT VERSION() as version, DATABASE() as database, USER() as user")
            .fetch_one(&self.pool)
            .await
            .map_err(|e| SqlTermError::Connection(e.to_string()))?;

        let server_version: String = row.get("version");
        let database_name: String = row.get("database");
        let username: String = row.get("user");

        Ok(ConnectionInfo {
            server_version,
            database_name,
            username,
            connection_id: None,
        })
    }

    async fn close(&mut self) -> Result<()> {
        self.pool.close().await;
        self.connected = false;
        Ok(())
    }

    fn database_type(&self) -> DatabaseType {
        DatabaseType::MySQL
    }

    fn is_connected(&self) -> bool {
        self.connected && !self.pool.is_closed()
    }
}

impl MySqlConnection {
    pub fn pool(&self) -> &Pool<MySql> {
        &self.pool
    }
}
