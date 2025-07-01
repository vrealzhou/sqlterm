// Simple test to check basic imports work
use anyhow::Result;
use chrono;
use regex;

fn main() -> Result<()> {
    println!("Testing basic imports...");
    
    let now = chrono::Utc::now();
    println!("Current time: {}", now);
    
    let re = regex::Regex::new(r"test")?;
    let result = re.is_match("test string");
    println!("Regex test: {}", result);
    
    println!("All imports working correctly!");
    Ok(())
}