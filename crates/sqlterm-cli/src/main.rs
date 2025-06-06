use anyhow::Result;
use clap::{Parser, Subcommand};
use sqlterm_core::{ConnectionConfig, DatabaseType};
use sqlterm_ui::{App, EventHandler};
use std::path::PathBuf;
use tracing::{info, Level};

#[derive(Parser)]
#[command(name = "sqlterm")]
#[command(about = "A terminal-based SQL database tool")]
#[command(version)]
struct Cli {
    #[command(subcommand)]
    command: Option<Commands>,
    
    /// Configuration file path
    #[arg(short, long)]
    config: Option<PathBuf>,
    
    /// Verbose logging
    #[arg(short, long)]
    verbose: bool,
}

#[derive(Subcommand)]
enum Commands {
    /// Start the interactive terminal UI
    Tui,
    /// Connect to a database directly
    Connect {
        /// Database type (mysql, postgres, sqlite)
        #[arg(short, long)]
        db_type: String,
        /// Host
        #[arg(short = 'H', long, default_value = "localhost")]
        host: String,
        /// Port
        #[arg(short, long)]
        port: Option<u16>,
        /// Database name
        #[arg(short, long)]
        database: String,
        /// Username
        #[arg(short, long)]
        username: String,
        /// Password (will prompt if not provided)
        #[arg(short = 'P', long)]
        password: Option<String>,
    },
    /// List saved connections
    List,
    /// Add a new connection
    Add {
        /// Connection name
        name: String,
        /// Database type (mysql, postgres, sqlite)
        #[arg(short, long)]
        db_type: String,
        /// Host
        #[arg(short = 'H', long, default_value = "localhost")]
        host: String,
        /// Port
        #[arg(short, long)]
        port: Option<u16>,
        /// Database name
        #[arg(short, long)]
        database: String,
        /// Username
        #[arg(short, long)]
        username: String,
    },
}

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();
    
    // Initialize logging
    let level = if cli.verbose { Level::DEBUG } else { Level::INFO };
    tracing_subscriber::fmt()
        .with_max_level(level)
        .init();

    info!("Starting sqlterm");

    match cli.command {
        Some(Commands::Tui) | None => {
            run_tui().await?;
        }
        Some(Commands::Connect { 
            db_type, 
            host, 
            port, 
            database, 
            username, 
            password 
        }) => {
            let database_type = parse_database_type(&db_type)?;
            let default_port = get_default_port(&database_type);
            
            let config = ConnectionConfig {
                name: format!("{}@{}", username, host),
                database_type,
                host,
                port: port.unwrap_or(default_port),
                database,
                username,
                password,
                ssl: false,
                ssh_tunnel: None,
            };
            
            connect_and_run_tui(config).await?;
        }
        Some(Commands::List) => {
            println!("Listing saved connections...");
            // TODO: Implement connection listing
        }
        Some(Commands::Add { 
            name, 
            db_type, 
            host, 
            port, 
            database, 
            username 
        }) => {
            let database_type = parse_database_type(&db_type)?;
            let default_port = get_default_port(&database_type);
            
            let config = ConnectionConfig {
                name,
                database_type,
                host,
                port: port.unwrap_or(default_port),
                database,
                username,
                password: None,
                ssl: false,
                ssh_tunnel: None,
            };
            
            println!("Adding connection: {}", config.name);
            // TODO: Implement connection saving
        }
    }

    Ok(())
}

async fn run_tui() -> Result<()> {
    let mut terminal = sqlterm_ui::init()?;
    let mut app = App::new();
    let mut event_handler = EventHandler::new(250);

    // Add some sample connections for testing
    app.add_connection(ConnectionConfig {
        name: "Local MySQL".to_string(),
        database_type: DatabaseType::MySQL,
        host: "localhost".to_string(),
        port: 3306,
        database: "test".to_string(),
        username: "root".to_string(),
        password: Some("password".to_string()),
        ssl: false,
        ssh_tunnel: None,
    });

    loop {
        terminal.draw(|f| sqlterm_ui::ui::render(f, &app))?;

        if let Ok(event) = event_handler.next().await {
            if let Err(e) = handle_event(&mut app, event).await {
                app.set_error(e.to_string());
            }
        }

        if app.should_quit {
            break;
        }
    }

    sqlterm_ui::restore()?;
    Ok(())
}

async fn connect_and_run_tui(config: ConnectionConfig) -> Result<()> {
    println!("Connecting to {}...", config.name);
    // TODO: Implement direct connection and TUI launch
    run_tui().await
}

async fn handle_event(app: &mut App, event: sqlterm_ui::Event) -> Result<()> {
    use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};
    use sqlterm_ui::Event;

    match event {
        Event::Key(key_event) => {
            match key_event {
                KeyEvent {
                    code: KeyCode::Char('q'),
                    modifiers: KeyModifiers::NONE,
                    ..
                } => {
                    app.quit();
                }
                KeyEvent {
                    code: KeyCode::Char('c'),
                    modifiers: KeyModifiers::CONTROL,
                    ..
                } => {
                    app.quit();
                }
                _ => {
                    // Handle other key events based on current state
                    // TODO: Implement state-specific key handling
                }
            }
        }
        Event::Tick => {
            // Handle periodic updates
        }
        Event::Mouse(_) => {
            // Handle mouse events (not implemented yet)
        }
        Event::Resize(_, _) => {
            // Handle terminal resize (not implemented yet)
        }
    }

    Ok(())
}

fn parse_database_type(db_type: &str) -> Result<DatabaseType> {
    match db_type.to_lowercase().as_str() {
        "mysql" => Ok(DatabaseType::MySQL),
        "postgres" | "postgresql" => Ok(DatabaseType::PostgreSQL),
        "sqlite" => Ok(DatabaseType::SQLite),
        _ => Err(anyhow::anyhow!("Unsupported database type: {}", db_type)),
    }
}

fn get_default_port(db_type: &DatabaseType) -> u16 {
    match db_type {
        DatabaseType::MySQL => 3306,
        DatabaseType::PostgreSQL => 5432,
        DatabaseType::SQLite => 0, // SQLite doesn't use ports
    }
}
