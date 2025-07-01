// Minimal test version
use anyhow::Result;

pub struct TestApp {
    active: bool,
}

impl TestApp {
    pub fn new() -> Result<Self> {
        Ok(Self { active: false })
    }
    
    pub async fn run(&mut self) -> Result<()> {
        println!("Test app running");
        Ok(())
    }
}

#[tokio::main]
async fn main() -> Result<()> {
    let mut app = TestApp::new()?;
    app.run().await?;
    Ok(())
}