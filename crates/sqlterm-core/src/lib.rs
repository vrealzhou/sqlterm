pub mod connection;
pub mod error;
pub mod query;
pub mod schema;
pub mod result;

pub use connection::*;
pub use error::*;
pub use query::*;
pub use schema::*;
pub use result::*;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_database_type_display() {
        assert_eq!(DatabaseType::MySQL.to_string(), "MySQL");
        assert_eq!(DatabaseType::PostgreSQL.to_string(), "PostgreSQL");
        assert_eq!(DatabaseType::SQLite.to_string(), "SQLite");
    }

    #[test]
    fn test_value_to_string() {
        assert_eq!(Value::String("test".to_string()).to_string(), "test");
        assert_eq!(Value::Integer(42).to_string(), "42");
        assert_eq!(Value::Float(3.14).to_string(), "3.14");
        assert_eq!(Value::Boolean(true).to_string(), "true");
        assert_eq!(Value::Null.to_string(), "NULL");
    }

    #[test]
    fn test_value_is_null() {
        assert!(Value::Null.is_null());
        assert!(!Value::String("test".to_string()).is_null());
        assert!(!Value::Integer(42).is_null());
    }
}
