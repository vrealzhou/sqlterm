use anyhow::Result;
use clap::{Parser, Subcommand};
use crossterm::event::KeyEvent;
use sqlterm_core::{ConnectionConfig, DatabaseType};
use std::path::PathBuf;
use tracing::{info, Level};

mod app;
mod components;
mod config;
mod events;
mod ui;

use app::{App, AppState, InputMode, ConnectionForm};
use events::{Event, EventHandler};

use anyhow::Result as AnyhowResult;
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

type Tui = Terminal<CrosstermBackend<io::Stdout>>;

/// Initialize the terminal
fn init() -> Result<Tui> {
    // Enable raw mode first to check if we have a valid terminal
    enable_raw_mode().map_err(|e| anyhow::anyhow!("Failed to enable raw mode: {}. This application requires a TTY.", e))?;
    
    execute!(io::stdout(), EnterAlternateScreen, EnableMouseCapture)
        .map_err(|e| anyhow::anyhow!("Failed to initialize terminal: {}", e))?;
    
    let backend = CrosstermBackend::new(io::stdout());
    let terminal = Terminal::new(backend)?;
    
    Ok(terminal)
}

/// Restore the terminal to its original state
fn restore() -> Result<()> {
    disable_raw_mode()?;
    execute!(io::stdout(), LeaveAlternateScreen, DisableMouseCapture)?;
    Ok(())
}

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
        #[arg(short = 't', long)]
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
        #[arg(short = 't', long)]
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
                name: format!("{} Connection", db_type),
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
            list_connections().await?;
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
            
            add_connection(config).await?;
        }
    }

    Ok(())
}

async fn run_tui() -> Result<()> {
    let mut terminal = init().map_err(|e| {
        eprintln!("Failed to initialize terminal: {}", e);
        e
    })?;
    
    let mut app = App::new();
    let mut event_handler = EventHandler::new(250);

    // Load saved connections
    match load_saved_connections(&mut app).await {
        Ok(connections) => {
            for connection in connections {
                app.add_connection(connection);
            }
            if !app.connections.is_empty() {
                app.add_log("INFO", &format!("Loaded {} saved connections", app.connections.len()));
            }
        }
        Err(e) => {
            app.add_log("WARN", &format!("Failed to load saved connections: {}", e));
        }
    }

    // Add demo connection if no saved connections exist
    if app.connections.is_empty() {
        app.add_connection(ConnectionConfig {
            name: "In-Memory SQLite (Demo)".to_string(),
            database_type: DatabaseType::SQLite,
            host: "".to_string(),
            port: 0,
            database: ":memory:".to_string(),
            username: "".to_string(),
            password: None,
            ssl: false,
            ssh_tunnel: None,
        });
        app.add_log("INFO", "Added demo SQLite connection");
    }

    // Add sample tables for demo
    app.tables = vec![
        "users".to_string(),
        "posts".to_string(),
        "categories".to_string(),
        "orders".to_string(),
    ];

    let result = run_app_loop(&mut terminal, &mut app, &mut event_handler).await;
    
    // Always attempt to restore terminal, even if the app loop failed
    if let Err(restore_error) = restore() {
        eprintln!("Failed to restore terminal: {}", restore_error);
    }
    
    result
}

async fn run_app_loop(
    terminal: &mut Tui,
    app: &mut App,
    event_handler: &mut EventHandler,
) -> Result<()> {
    loop {
        // Draw the UI
        if let Err(e) = terminal.draw(|f| ui::render(f, app)) {
            app.set_error(format!("Failed to render UI: {}", e));
        }

        // Handle events
        match event_handler.next().await {
            Ok(event) => {
                if let Err(e) = handle_event(app, event).await {
                    app.set_error(e.to_string());
                }
            }
            Err(e) => {
                app.set_error(format!("Event handling error: {}", e));
            }
        }

        // Check for quit condition
        if app.should_quit {
            break;
        }
    }
    
    Ok(())
}

async fn handle_event(app: &mut App, event: Event) -> Result<()> {
    use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};

    match event {
        Event::Key(key_event) => {
            // Clear any existing error when user presses a key
            if app.error_message.is_some() {
                app.clear_error();
                return Ok(()); // Just clear the error, don't process the key
            }
            
            // Global key handlers - these work in any state
            match key_event {
                KeyEvent {
                    code: KeyCode::Char('q'),
                    modifiers: KeyModifiers::NONE,
                    ..
                } => {
                    // Only quit if not in editing mode, or if we're in connection manager
                    if app.input_mode == InputMode::Normal || app.state == AppState::ConnectionManager {
                        app.quit();
                        return Ok(());
                    }
                }
                KeyEvent {
                    code: KeyCode::Char('c'),
                    modifiers: KeyModifiers::CONTROL,
                    ..
                } => {
                    // Ctrl+C always quits
                    app.quit();
                    return Ok(());
                }
                KeyEvent {
                    code: KeyCode::Esc,
                    modifiers: KeyModifiers::NONE,
                    ..
                } => {
                    // Esc in normal mode goes back or quits
                    if app.input_mode == InputMode::Normal {
                        match app.state {
                            AppState::ConnectionManager => {
                                app.quit();
                                return Ok(());
                            }
                            _ => {
                                app.switch_to_connection_manager();
                                return Ok(());
                            }
                        }
                    }
                }
                _ => {}
            }

            // State-specific key handling
            match app.state {
                AppState::ConnectionManager => handle_connection_manager_keys(app, key_event).await?,
                AppState::DatabaseBrowser => handle_database_browser_keys(app, key_event).await?,
                AppState::QueryEditor => handle_query_editor_keys(app, key_event).await?,
                AppState::Results => handle_results_keys(app, key_event).await?,
                AppState::AddConnection => handle_add_connection_keys(app, key_event).await?,
            }
        }
        Event::Tick => {
            // Handle periodic updates - don't clear error immediately
            // Errors will be cleared when user presses a key or after some time
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

async fn handle_connection_manager_keys(app: &mut App, key_event: KeyEvent) -> Result<()> {
    use crossterm::event::KeyCode;

    match key_event.code {
        KeyCode::Up => {
            app.select_previous_connection();
        }
        KeyCode::Down => {
            app.select_next_connection();
        }
        KeyCode::Enter => {
            if let Some(config) = app.get_selected_connection().cloned() {
                match connect_to_database(app, config.clone()).await {
                    Ok(()) => {
                        app.switch_to_database_browser();
                    }
                    Err(e) => {
                        app.set_error(format!("Failed to connect to {}: {}", config.name, e));
                    }
                }
            }
        }
        KeyCode::Char('a') => {
            app.switch_to_add_connection();
        }
        KeyCode::Char('d') => {
            app.set_error("Delete connection not implemented yet".to_string());
        }
        KeyCode::Char('e') => {
            app.switch_to_query_editor();
        }
        _ => {}
    }

    Ok(())
}

async fn handle_database_browser_keys(app: &mut App, key_event: KeyEvent) -> Result<()> {
    use crossterm::event::KeyCode;

    match key_event.code {
        KeyCode::Up => {
            app.select_previous_table();
        }
        KeyCode::Down => {
            app.select_next_table();
        }
        KeyCode::Enter => {
            if let Some(table) = app.get_selected_table().map(|s| s.clone()) {
                // Load table details
                match load_table_details(app, &table).await {
                    Ok(()) => {
                        app.add_log("INFO", &format!("Loaded details for table '{}'", table));
                    }
                    Err(e) => {
                        app.set_error(format!("Failed to load table details: {}", e));
                        app.add_log("ERROR", &format!("Failed to load table details for '{}': {}", table, e));
                    }
                }
            }
        }
        KeyCode::Char('c') => {
            app.switch_to_connection_manager();
        }
        KeyCode::Char('e') => {
            app.switch_to_query_editor();
        }
        _ => {}
    }

    Ok(())
}

async fn handle_query_editor_keys(app: &mut App, key_event: KeyEvent) -> Result<()> {
    use crossterm::event::{KeyCode, KeyModifiers};

    match app.input_mode {
        InputMode::Normal => {
            match (key_event.code, key_event.modifiers) {
                // Vim-like movement
                (KeyCode::Char('h'), KeyModifiers::NONE) | (KeyCode::Left, _) => {
                    app.move_cursor_left();
                }
                (KeyCode::Char('l'), KeyModifiers::NONE) | (KeyCode::Right, _) => {
                    app.move_cursor_right();
                }
                (KeyCode::Char('k'), KeyModifiers::NONE) | (KeyCode::Up, _) => {
                    app.move_cursor_up();
                }
                (KeyCode::Char('j'), KeyModifiers::NONE) | (KeyCode::Down, _) => {
                    app.move_cursor_down();
                }
                (KeyCode::Char('0'), KeyModifiers::NONE) | (KeyCode::Home, _) => {
                    app.move_to_line_start();
                }
                (KeyCode::Char('$'), KeyModifiers::NONE) | (KeyCode::End, _) => {
                    app.move_to_line_end();
                }
                
                // Mode changes
                (KeyCode::Char('i'), KeyModifiers::NONE) => {
                    app.enter_edit_mode();
                }
                (KeyCode::Char('v'), KeyModifiers::NONE) => {
                    app.enter_visual_mode();
                }
                
                // Navigation
                (KeyCode::Char('b'), KeyModifiers::NONE) => {
                    app.switch_to_database_browser();
                }
                (KeyCode::Char('c'), KeyModifiers::NONE) => {
                    app.switch_to_connection_manager();
                }
                
                // Actions
                (KeyCode::Enter, KeyModifiers::CONTROL) => {
                    app.add_log("DEBUG", "Ctrl+Enter detected in Normal mode");
                    execute_current_query(app).await?;
                }
                (KeyCode::Char('r'), KeyModifiers::CONTROL) => {
                    execute_current_query(app).await?;
                }
                (KeyCode::Char('L'), KeyModifiers::NONE) => {
                    app.toggle_logs();
                    app.add_log("INFO", &format!("Toggled logs panel (now: {})", app.show_logs));
                }
                (KeyCode::Char('y'), KeyModifiers::NONE) => {
                    if let Err(e) = app.copy_to_clipboard() {
                        app.set_error(format!("Failed to copy to clipboard: {}", e));
                    } else {
                        app.add_log("INFO", "Copied query to clipboard");
                    }
                }
                (KeyCode::Char('p'), KeyModifiers::NONE) => {
                    if let Err(e) = app.paste_from_clipboard() {
                        app.set_error(format!("Failed to paste from clipboard: {}", e));
                    } else {
                        app.add_log("INFO", "Pasted from clipboard");
                    }
                }
                _ => {}
            }
        }
        InputMode::Editing => {
            match (key_event.code, key_event.modifiers) {
                (KeyCode::Esc, _) => {
                    app.exit_edit_mode();
                }
                (KeyCode::Enter, KeyModifiers::CONTROL) => {
                    app.add_log("DEBUG", "Ctrl+Enter detected in Editing mode");
                    execute_current_query(app).await?;
                }
                (KeyCode::Char('L'), KeyModifiers::CONTROL) => {
                    app.toggle_logs();
                    app.add_log("INFO", &format!("Toggled logs panel (now: {})", app.show_logs));
                }
                (KeyCode::Enter, _) => {
                    app.insert_newline();
                }
                (KeyCode::Backspace, _) => {
                    app.delete_char();
                }
                (KeyCode::Char(c), _) => {
                    app.insert_char(c);
                }
                (KeyCode::Left, _) => {
                    app.move_cursor_left();
                }
                (KeyCode::Right, _) => {
                    app.move_cursor_right();
                }
                (KeyCode::Up, _) => {
                    app.move_cursor_up();
                }
                (KeyCode::Down, _) => {
                    app.move_cursor_down();
                }
                (KeyCode::Home, _) => {
                    app.move_to_line_start();
                }
                (KeyCode::End, _) => {
                    app.move_to_line_end();
                }
                _ => {}
            }
        }
        InputMode::Visual => {
            match (key_event.code, key_event.modifiers) {
                (KeyCode::Esc, _) => {
                    app.exit_visual_mode();
                }
                (KeyCode::Enter, KeyModifiers::CONTROL) => {
                    app.add_log("DEBUG", "Ctrl+Enter detected in Visual mode");
                    execute_selected_query(app).await?;
                }
                (KeyCode::Char('h'), KeyModifiers::NONE) | (KeyCode::Left, _) => {
                    app.move_cursor_left();
                }
                (KeyCode::Char('l'), KeyModifiers::NONE) | (KeyCode::Right, _) => {
                    app.move_cursor_right();
                }
                (KeyCode::Char('k'), KeyModifiers::NONE) | (KeyCode::Up, _) => {
                    app.move_cursor_up();
                }
                (KeyCode::Char('j'), KeyModifiers::NONE) | (KeyCode::Down, _) => {
                    app.move_cursor_down();
                }
                (KeyCode::Char('0'), KeyModifiers::NONE) | (KeyCode::Home, _) => {
                    app.move_to_line_start();
                }
                (KeyCode::Char('$'), KeyModifiers::NONE) | (KeyCode::End, _) => {
                    app.move_to_line_end();
                }
                (KeyCode::Char('L'), KeyModifiers::CONTROL) => {
                    app.toggle_logs();
                    app.add_log("INFO", &format!("Toggled logs panel (now: {})", app.show_logs));
                }
                (KeyCode::Char('y'), KeyModifiers::NONE) => {
                    if let Err(e) = app.copy_to_clipboard() {
                        app.set_error(format!("Failed to copy selection to clipboard: {}", e));
                    } else {
                        app.add_log("INFO", "Copied selection to clipboard");
                        app.exit_visual_mode();
                    }
                }
                _ => {}
            }
        }
    }

    Ok(())
}

async fn handle_results_keys(app: &mut App, key_event: KeyEvent) -> Result<()> {
    use crossterm::event::KeyCode;

    match key_event.code {
        KeyCode::Char('e') => {
            app.switch_to_query_editor();
        }
        KeyCode::Char('b') => {
            app.switch_to_database_browser();
        }
        KeyCode::Char('c') => {
            app.switch_to_connection_manager();
        }
        KeyCode::Char('s') => {
            app.set_error("Export to file not implemented yet".to_string());
        }
        KeyCode::Char('f') => {
            app.set_error("Show full results not implemented yet".to_string());
        }
        _ => {}
    }

    Ok(())
}

async fn handle_add_connection_keys(app: &mut App, key_event: KeyEvent) -> Result<()> {
    use crossterm::event::{KeyCode, KeyModifiers};

    match (key_event.code, key_event.modifiers) {
        (KeyCode::Esc, _) => {
            app.connection_form.is_active = false;
            app.switch_to_connection_manager();
        }
        (KeyCode::Tab, _) | (KeyCode::Down, _) => {
            app.connection_form.selected_field = (app.connection_form.selected_field + 1) % 7;
        }
        (KeyCode::BackTab, _) | (KeyCode::Up, _) => {
            app.connection_form.selected_field = if app.connection_form.selected_field == 0 {
                6
            } else {
                app.connection_form.selected_field - 1
            };
        }
        (KeyCode::Enter, _) => {
            // Try to create the connection
            if let Ok(config) = create_connection_from_form(&app.connection_form) {
                match connect_to_database(app, config.clone()).await {
                    Ok(()) => {
                        // Connection is auto-saved and added to app by connect_to_database
                        app.connection_form = ConnectionForm::default();
                        app.switch_to_database_browser();
                    }
                    Err(e) => {
                        app.set_error(format!("Connection failed: {}", e));
                    }
                }
            } else {
                app.set_error("Please fill in all required fields".to_string());
            }
        }
        (KeyCode::Char(c), _) => {
            let field = match app.connection_form.selected_field {
                0 => &mut app.connection_form.name,
                1 => {
                    // Database type cycling
                    match c {
                        's' => app.connection_form.database_type = DatabaseType::SQLite,
                        'm' => app.connection_form.database_type = DatabaseType::MySQL,
                        'p' => app.connection_form.database_type = DatabaseType::PostgreSQL,
                        _ => {}
                    }
                    return Ok(());
                }
                2 => &mut app.connection_form.host,
                3 => &mut app.connection_form.port,
                4 => &mut app.connection_form.database,
                5 => &mut app.connection_form.username,
                6 => &mut app.connection_form.password,
                _ => return Ok(()),
            };
            field.push(c);
        }
        (KeyCode::Backspace, _) => {
            let field = match app.connection_form.selected_field {
                0 => &mut app.connection_form.name,
                2 => &mut app.connection_form.host,
                3 => &mut app.connection_form.port,
                4 => &mut app.connection_form.database,
                5 => &mut app.connection_form.username,
                6 => &mut app.connection_form.password,
                _ => return Ok(()),
            };
            field.pop();
        }
        _ => {}
    }

    Ok(())
}

fn create_connection_from_form(form: &ConnectionForm) -> Result<ConnectionConfig> {
    if form.name.is_empty() || form.database.is_empty() {
        return Err(anyhow::anyhow!("Name and database are required"));
    }

    let port = if form.port.is_empty() {
        get_default_port(&form.database_type)
    } else {
        form.port.parse().unwrap_or_else(|_| get_default_port(&form.database_type))
    };

    Ok(ConnectionConfig {
        name: form.name.clone(),
        database_type: form.database_type.clone(),
        host: if form.host.is_empty() { "localhost".to_string() } else { form.host.clone() },
        port,
        database: form.database.clone(),
        username: form.username.clone(),
        password: if form.password.is_empty() { None } else { Some(form.password.clone()) },
        ssl: false,
        ssh_tunnel: None,
    })
}

async fn connect_to_database(app: &mut App, config: ConnectionConfig) -> Result<()> {
    use sqlterm_core::{DatabaseConnection, DatabaseType};
    
    // Validate the configuration first
    if let Err(e) = sqlterm_core::ConnectionFactory::validate_config(&config) {
        return Err(anyhow::anyhow!("Configuration error: {}", e));
    }
    
    // Log connection attempt for debugging
    tracing::info!("Attempting to connect to {} ({:?}) at {}:{}", 
                  config.name, config.database_type, config.host, config.port);
    
    // Create the connection based on database type
    let connection: Box<dyn DatabaseConnection> = match config.database_type {
        DatabaseType::MySQL => {
            use sqlterm_core::DatabaseConnection;
            sqlterm_mysql::MySqlConnection::connect(&config).await
                .map_err(|e| anyhow::anyhow!("MySQL connection failed: {}", e))?
        }
        DatabaseType::PostgreSQL => {
            use sqlterm_core::DatabaseConnection;
            sqlterm_postgres::PostgresConnection::connect(&config).await
                .map_err(|e| anyhow::anyhow!("PostgreSQL connection failed: {}", e))?
        }
        DatabaseType::SQLite => {
            use sqlterm_core::DatabaseConnection;
            sqlterm_sqlite::SqliteConnection::connect(&config).await
                .map_err(|e| anyhow::anyhow!("SQLite connection failed: {}", e))?
        }
    };
    
    // Test the connection
    connection.ping().await
        .map_err(|e| anyhow::anyhow!("Connection test failed: {}", e))?;
    
    // Get connection info for display
    let conn_info = connection.get_connection_info().await?;
    
    // Load real table list before storing connection
    let tables_result = connection.list_tables().await;
    
    // Store the connection in the app state
    app.active_connection = Some(connection);
    app.current_database = Some(conn_info.database_name.clone());
    
    // Process table loading results
    match tables_result {
        Ok(tables) => {
            app.tables = tables;
            app.selected_table = 0;
            app.add_log("INFO", &format!("Loaded {} tables from database", app.tables.len()));
        }
        Err(e) => {
            app.add_log("ERROR", &format!("Failed to load tables: {}", e));
            // Fallback to empty list
            app.tables = vec![];
            app.selected_table = 0;
        }
    }
    
    // Auto-save connection to config if it's not already saved
    if let Err(e) = auto_save_connection_config(app, &config).await {
        app.add_log("WARN", &format!("Failed to auto-save connection config: {}", e));
    }
    
    Ok(())
}

async fn auto_save_connection_config(app: &mut App, config: &ConnectionConfig) -> Result<()> {
    let config_manager = crate::config::ConfigManager::new()?;
    
    // Check if this connection already exists in the config
    if config_manager.connection_exists(&config.name) {
        app.add_log("DEBUG", &format!("Connection '{}' already exists in config, skipping auto-save", config.name));
        return Ok(());
    }
    
    // Save the connection config
    match config_manager.save_connection(config) {
        Ok(()) => {
            app.add_log("INFO", &format!("Auto-saved connection '{}' to config", config.name));
            
            // Add the connection to the app's connection list if it's not already there
            let connection_exists_in_app = app.connections.iter().any(|c| c.name == config.name);
            if !connection_exists_in_app {
                app.add_connection(config.clone());
                app.add_log("DEBUG", &format!("Added connection '{}' to app connection list", config.name));
            }
        }
        Err(e) => {
            return Err(anyhow::anyhow!("Failed to save connection config: {}", e));
        }
    }
    
    Ok(())
}

// CLI helper functions
fn parse_database_type(db_type: &str) -> Result<DatabaseType> {
    match db_type.to_lowercase().as_str() {
        "mysql" => Ok(DatabaseType::MySQL),
        "postgres" | "postgresql" => Ok(DatabaseType::PostgreSQL),
        "sqlite" => Ok(DatabaseType::SQLite),
        _ => Err(anyhow::anyhow!("Unsupported database type: {}. Supported types: mysql, postgres, sqlite", db_type)),
    }
}

fn get_default_port(database_type: &DatabaseType) -> u16 {
    match database_type {
        DatabaseType::MySQL => 3306,
        DatabaseType::PostgreSQL => 5432,
        DatabaseType::SQLite => 0,
    }
}

async fn connect_and_run_tui(config: ConnectionConfig) -> Result<()> {
    println!("Connecting to {}...", config.name);
    
    // Test the connection first
    let mut app = App::new();
    match connect_to_database(&mut app, config.clone()).await {
        Ok(()) => {
            println!("✓ Connected successfully to {}", config.name);
            println!("Starting TUI...");
            
            // Remove demo connections but keep the newly added connection
            app.connections.retain(|c| c.name == config.name);
            app.switch_to_database_browser();
            
            // Start the TUI with this connection
            let mut terminal = init().map_err(|e| {
                eprintln!("Failed to initialize terminal: {}", e);
                e
            })?;
            
            let mut event_handler = EventHandler::new(250);
            let result = run_app_loop(&mut terminal, &mut app, &mut event_handler).await;
            
            if let Err(restore_error) = restore() {
                eprintln!("Failed to restore terminal: {}", restore_error);
            }
            
            result
        }
        Err(e) => {
            eprintln!("✗ Failed to connect: {}", e);
            Err(e)
        }
    }
}

async fn list_connections() -> Result<()> {
    let config_manager = crate::config::ConfigManager::new()?;
    match config_manager.list_connections_with_errors() {
        Ok((connections, errors)) => {
            // Print any errors that occurred during loading
            for error in &errors {
                eprintln!("Warning: {}", error);
            }
            
            if connections.is_empty() {
                println!("No saved connections found.");
                println!("Add a connection with: sqlterm add <name> --db-type <type> --host <host> --database <db> --username <user>");
            } else {
                println!("Saved connections:");
                for (i, conn) in connections.iter().enumerate() {
                    println!("{}. {} ({}) - {}://{}:{}/{}", 
                            i + 1,
                            conn.name,
                            conn.database_type,
                            conn.database_type.to_string().to_lowercase(),
                            conn.host,
                            conn.port,
                            conn.database);
                }
            }
        }
        Err(e) => {
            eprintln!("Failed to load connections: {}", e);
        }
    }
    Ok(())
}

async fn add_connection(config: ConnectionConfig) -> Result<()> {
    // Validate the configuration
    if let Err(e) = sqlterm_core::ConnectionFactory::validate_config(&config) {
        return Err(anyhow::anyhow!("Invalid configuration: {}", e));
    }
    
    // Test the connection
    println!("Testing connection to {}...", config.name);
    let mut app = App::new();
    match connect_to_database(&mut app, config.clone()).await {
        Ok(()) => {
            println!("✓ Connection test successful");
            println!("✓ Connection '{}' saved automatically", config.name);
            println!("Use 'sqlterm list' to see all connections");
            println!("Use 'sqlterm tui' to start the interactive interface");
        }
        Err(e) => {
            eprintln!("✗ Connection test failed: {}", e);
            println!("Please check your connection parameters and try again");
            return Err(e);
        }
    }
    
    Ok(())
}

async fn load_saved_connections(app: &mut App) -> Result<Vec<ConnectionConfig>> {
    let config_manager = crate::config::ConfigManager::new()?;
    let (connections, errors) = config_manager.list_connections_with_errors()?;
    
    // Log any errors that occurred during loading
    for error in errors {
        app.add_log("ERROR", &error);
    }
    
    Ok(connections)
}


async fn execute_current_query(app: &mut App) -> Result<()> {
    let query = app.get_current_query().trim().to_string();
    if query.is_empty() {
        app.set_error("No query to execute. Type a query first.".to_string());
        return Ok(());
    }
    
    execute_query(app, &query).await
}

async fn execute_selected_query(app: &mut App) -> Result<()> {
    let query = if let Some(selected) = app.get_selected_query() {
        selected.trim().to_string()
    } else {
        app.get_current_query().trim().to_string()
    };
    
    if query.is_empty() {
        app.set_error("No query to execute.".to_string());
        return Ok(());
    }
    
    execute_query(app, &query).await
}

async fn execute_query(app: &mut App, query: &str) -> Result<()> {
    app.add_log("INFO", &format!("Executing query: {}", query.lines().next().unwrap_or("").chars().take(50).collect::<String>()));
    
    if let Some(connection) = &app.active_connection {
        match connection.execute_query(query).await {
            Ok(result) => {
                app.add_log("INFO", &format!("Query executed successfully, {} rows returned", result.total_rows));
                app.set_query_results(result);
                app.switch_to_results();
            }
            Err(e) => {
                app.add_log("ERROR", &format!("Query execution failed: {}", e));
                app.set_error(format!("Query execution failed: {}", e));
            }
        }
    } else {
        app.set_error("No active database connection. Connect to a database first.".to_string());
        app.add_log("ERROR", "Attempted to execute query without active connection");
    }
    
    Ok(())
}

async fn load_table_details(app: &mut App, table_name: &str) -> Result<()> {
    if let Some(connection) = &app.active_connection {
        match connection.get_table_details(table_name).await {
            Ok(details) => {
                app.set_table_details(details);
                app.add_log("INFO", &format!("Successfully loaded details for table '{}'", table_name));
            }
            Err(e) => {
                app.add_log("ERROR", &format!("Failed to load table details for '{}': {}", table_name, e));
                return Err(anyhow::anyhow!("Failed to load table details: {}", e));
            }
        }
    } else {
        app.add_log("ERROR", "No active database connection");
        return Err(anyhow::anyhow!("No active database connection"));
    }
    
    Ok(())
}