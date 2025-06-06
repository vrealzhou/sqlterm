use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QueryResult {
    pub columns: Vec<ColumnInfo>,
    pub rows: Vec<Row>,
    pub execution_time: std::time::Duration,
    pub total_rows: usize,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ColumnInfo {
    pub name: String,
    pub data_type: String,
    pub nullable: bool,
    pub max_length: Option<usize>,
    pub precision: Option<u8>,
    pub scale: Option<u8>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Row {
    pub values: Vec<Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum Value {
    String(String),
    Integer(i64),
    Float(f64),
    Boolean(bool),
    Bytes(Vec<u8>),
    Date(String),
    DateTime(String),
    Time(String),
    Null,
}

impl Value {
    pub fn to_string(&self) -> String {
        match self {
            Value::String(s) => s.clone(),
            Value::Integer(i) => i.to_string(),
            Value::Float(f) => f.to_string(),
            Value::Boolean(b) => b.to_string(),
            Value::Bytes(b) => format!("<{} bytes>", b.len()),
            Value::Date(d) => d.clone(),
            Value::DateTime(dt) => dt.clone(),
            Value::Time(t) => t.clone(),
            Value::Null => "NULL".to_string(),
        }
    }
    
    pub fn is_null(&self) -> bool {
        matches!(self, Value::Null)
    }
}

impl Row {
    pub fn get_value(&self, index: usize) -> Option<&Value> {
        self.values.get(index)
    }
    
    pub fn to_map(&self, columns: &[ColumnInfo]) -> HashMap<String, &Value> {
        let mut map = HashMap::new();
        for (i, column) in columns.iter().enumerate() {
            if let Some(value) = self.values.get(i) {
                map.insert(column.name.clone(), value);
            }
        }
        map
    }
}
