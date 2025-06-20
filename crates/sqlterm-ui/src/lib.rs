pub mod app;
pub mod components;
pub mod events;
pub mod ui;

pub use app::App;
pub use events::{Event, EventHandler};

use anyhow::Result;
use crossterm::{
    event::{DisableMouseCapture, EnableMouseCapture},
    execute,
    terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
};
use ratatui::{
    backend::CrosstermBackend,
    Terminal,
};
use std::io;

pub type Tui = Terminal<CrosstermBackend<io::Stdout>>;

/// Initialize the terminal
pub fn init() -> Result<Tui> {
    // Enable raw mode first to check if we have a valid terminal
    enable_raw_mode().map_err(|e| anyhow::anyhow!("Failed to enable raw mode: {}. This application requires a TTY.", e))?;
    
    execute!(io::stdout(), EnterAlternateScreen, EnableMouseCapture)
        .map_err(|e| anyhow::anyhow!("Failed to initialize terminal: {}", e))?;
    
    let backend = CrosstermBackend::new(io::stdout());
    let terminal = Terminal::new(backend)?;
    
    Ok(terminal)
}

/// Restore the terminal to its original state
pub fn restore() -> Result<()> {
    disable_raw_mode()?;
    execute!(io::stdout(), LeaveAlternateScreen, DisableMouseCapture)?;
    Ok(())
}
