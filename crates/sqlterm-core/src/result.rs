use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QueryResult {
    pub columns: Vec<ColumnInfo>,
    pub rows: Vec<Row>,
    pub execution_time: std::time::Duration,
    pub total_rows: usize,
    pub is_truncated: bool,
    pub truncated_at: Option<usize>,
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
    Unknown(String), // For types we can't convert, store the raw string representation
}

impl std::fmt::Display for Value {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Value::String(s) => write!(f, "{}", s),
            Value::Integer(i) => write!(f, "{}", i),
            Value::Float(fl) => write!(f, "{}", fl),
            Value::Boolean(b) => write!(f, "{}", b),
            Value::Bytes(b) => write!(f, "<{} bytes>", b.len()),
            Value::Date(d) => write!(f, "{}", d),
            Value::DateTime(dt) => write!(f, "{}", dt),
            Value::Time(t) => write!(f, "{}", t),
            Value::Null => write!(f, "NULL"),
            Value::Unknown(s) => write!(f, "{}", s),
        }
    }
}

impl Value {
    
    pub fn is_null(&self) -> bool {
        matches!(self, Value::Null)
    }
}

impl QueryResult {
    /// Create a new QueryResult
    pub fn new(
        columns: Vec<ColumnInfo>,
        rows: Vec<Row>,
        execution_time: std::time::Duration,
    ) -> Self {
        let total_rows = rows.len();
        Self {
            columns,
            rows,
            execution_time,
            total_rows,
            is_truncated: false,
            truncated_at: None,
        }
    }

    /// Create a truncated QueryResult
    pub fn truncated(
        columns: Vec<ColumnInfo>,
        mut rows: Vec<Row>,
        execution_time: std::time::Duration,
        total_rows: usize,
        limit: usize,
    ) -> Self {
        let is_truncated = rows.len() > limit;
        let truncated_at = if is_truncated {
            rows.truncate(limit);
            Some(limit)
        } else {
            None
        };

        Self {
            columns,
            rows,
            execution_time,
            total_rows,
            is_truncated,
            truncated_at,
        }
    }

    /// Export results to CSV format
    pub fn to_csv(&self) -> String {
        let mut csv = String::new();

        // Header
        let headers: Vec<String> = self.columns.iter().map(|c| c.name.clone()).collect();
        csv.push_str(&headers.join(","));
        csv.push('\n');

        // Rows
        for row in &self.rows {
            let values: Vec<String> = row.values.iter().map(|v| {
                let s = v.to_string();
                if s.contains(',') || s.contains('"') || s.contains('\n') {
                    format!("\"{}\"", s.replace('"', "\"\""))
                } else {
                    s
                }
            }).collect();
            csv.push_str(&values.join(","));
            csv.push('\n');
        }

        csv
    }

    /// Export results to JSON format
    pub fn to_json(&self) -> serde_json::Result<String> {
        serde_json::to_string_pretty(self)
    }

    /// Export results to TSV format
    pub fn to_tsv(&self) -> String {
        let mut tsv = String::new();

        // Header
        let headers: Vec<String> = self.columns.iter().map(|c| c.name.clone()).collect();
        tsv.push_str(&headers.join("\t"));
        tsv.push('\n');

        // Rows
        for row in &self.rows {
            let values: Vec<String> = row.values.iter().map(|v| {
                v.to_string().replace(['\t', '\n'], " ")
            }).collect();
            tsv.push_str(&values.join("\t"));
            tsv.push('\n');
        }

        tsv
    }

    /// Get summary information
    pub fn get_summary(&self) -> String {
        let mut summary = format!(
            "Rows: {} | Execution time: {:.2}ms",
            self.total_rows,
            self.execution_time.as_millis()
        );

        if self.is_truncated {
            summary.push_str(&format!(
                " | Showing {} of {} rows (truncated)",
                self.truncated_at.unwrap_or(0),
                self.total_rows
            ));
        }

        summary
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
