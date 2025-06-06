use thiserror::Error;

#[derive(Error, Debug)]
pub enum SqlTermError {
    #[error("Connection error: {0}")]
    Connection(String),
    
    #[error("Query execution error: {0}")]
    QueryExecution(String),
    
    #[error("Schema inspection error: {0}")]
    SchemaInspection(String),
    
    #[error("Authentication error: {0}")]
    Authentication(String),
    
    #[error("Network error: {0}")]
    Network(String),
    
    #[error("Configuration error: {0}")]
    Configuration(String),
    
    #[error("Serialization error: {0}")]
    Serialization(String),
    
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
    
    #[error("Unknown error: {0}")]
    Unknown(String),
}

pub type Result<T> = std::result::Result<T, SqlTermError>;
