use anyhow::Result;
use clap::{Parser, Subcommand};
use crossterm::event::KeyEvent;
use sqlterm_core::{ConnectionConfig, DatabaseType, ConfigManager, ConnectionFactory};
use std::path::PathBuf;
use tracing::{info, Level};

mod app;
mod components;
mod events;
mod ui;

use app::{App, AppState, InputMode};
use events::{Event, EventHandler};
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

    // Add some sample connections for testing
    app.add_connection(ConnectionConfig {
        name: "Local MySQL (Demo)".to_string(),
        database_type: DatabaseType::MySQL,
        host: "localhost".to_string(),
        port: 3306,
        database: "mysql".to_string(), // Use system database
        username: "root".to_string(),
        password: None, // No password for demo
        ssl: false,
        ssh_tunnel: None,
    });

    app.add_connection(ConnectionConfig {
        name: "Local PostgreSQL (Demo)".to_string(),
        database_type: DatabaseType::PostgreSQL,
        host: "localhost".to_string(),
        port: 5432,
        database: "postgres".to_string(), // Use system database
        username: "postgres".to_string(),
        password: None, // No password for demo
        ssl: false,
        ssh_tunnel: None,
    });

    app.add_connection(ConnectionConfig {
        name: "In-Memory SQLite (Demo)".to_string(),
        database_type: DatabaseType::SQLite,
        host: "".to_string(),
        port: 0,
        database: ":memory:".to_string(), // In-memory database
        username: "".to_string(),
        password: None,
        ssl: false,
        ssh_tunnel: None,
    });

    // Add some sample tables for testing
    app.tables = vec![
        "users".to_string(),
        "posts".to_string(),
        "categories".to_string(),
        "post_categories".to_string(),
        "published_posts".to_string(),
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

async fn connect_and_run_tui(config: ConnectionConfig) -> Result<()> {
    println!("Connecting to {}...", config.name);
    // TODO: Implement direct connection and TUI launch
    run_tui().await
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
            app.set_error("Add connection not implemented yet".to_string());
        }
        KeyCode::Char('d') => {
            app.set_error("Delete connection not implemented yet".to_string());
        }
        KeyCode::Char('e') => {
            app.switch_to_query_editor();
        }
        KeyCode::Char('q') => {
            app.quit();
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
                load_table_details(app, &table).await?;
            }
        }
        KeyCode::Char('d') => {
            if let Some(table) = app.get_selected_table() {
                // Show quick table description
                app.set_error(format!("Describing table: {} (columns, indexes, etc.)", table));
            }
        }
        KeyCode::Char('e') => {
            app.switch_to_query_editor();
        }
        KeyCode::Char('c') => {
            app.switch_to_connection_manager();
        }
        KeyCode::Char('r') => {
            app.switch_to_results();
        }
        KeyCode::Char('q') => {
            app.quit();
        }
        _ => {}
    }
    Ok(())
}

async fn handle_query_editor_keys(app: &mut App, key_event: KeyEvent) -> Result<()> {
    use crossterm::event::{KeyCode, KeyModifiers};



    match (key_event.code, key_event.modifiers) {
        (KeyCode::Esc, _) => {
            if app.input_mode == InputMode::Editing {
                app.exit_edit_mode();
            } else {
                app.switch_to_database_browser();
            }
        }
        (KeyCode::Enter, KeyModifiers::CONTROL) => {
            // Execute query with Ctrl+Enter
            if !app.query_input.trim().is_empty() {
                execute_query(app).await?;
                app.switch_to_results();
            } else {
                app.set_error("No query to execute. Type a query first.".to_string());
            }
        }
        (KeyCode::Char('r'), KeyModifiers::CONTROL) => {
            // Execute query with Ctrl+R (alternative)
            if !app.query_input.trim().is_empty() {
                execute_query(app).await?;
                app.switch_to_results();
            } else {
                app.set_error("No query to execute. Type a query first.".to_string());
            }
        }
        (KeyCode::Enter, _) if app.input_mode == InputMode::Editing => {
            // Regular Enter in editing mode - add newline
            app.query_input.push('\n');
            app.cursor_position += 1;
        }
        (KeyCode::Char('i'), KeyModifiers::NONE) if app.input_mode == InputMode::Normal => {
            app.enter_edit_mode();
        }
        (KeyCode::Char(c), _) if app.input_mode == InputMode::Editing => {
            app.query_input.push(c);
            app.cursor_position += 1;
        }
        (KeyCode::Backspace, _) if app.input_mode == InputMode::Editing => {
            if app.cursor_position > 0 {
                app.query_input.remove(app.cursor_position - 1);
                app.cursor_position -= 1;
            }
        }
        (KeyCode::Char('b'), _) if app.input_mode == InputMode::Normal => {
            app.switch_to_database_browser();
        }
        (KeyCode::Char('c'), _) if app.input_mode == InputMode::Normal => {
            app.switch_to_connection_manager();
        }
        (KeyCode::Char('q'), _) if app.input_mode == InputMode::Normal => {
            app.quit();
        }
        _ => {}
    }
    Ok(())
}

async fn handle_results_keys(app: &mut App, key_event: KeyEvent) -> Result<()> {
    use crossterm::event::KeyCode;

    match key_event.code {
        KeyCode::Char('s') => {
            // Export results to file
            export_results_to_file(app).await?;
        }
        KeyCode::Char('f') => {
            // Show full results (remove truncation)
            show_full_results(app).await?;
        }
        KeyCode::Char('c') => {
            // Copy results to clipboard (placeholder)
            app.set_error("Copy to clipboard not implemented yet".to_string());
        }
        KeyCode::Char('e') => {
            app.switch_to_query_editor();
        }
        KeyCode::Char('b') => {
            app.switch_to_database_browser();
        }
        KeyCode::Esc => {
            app.switch_to_query_editor();
        }
        KeyCode::Char('q') => {
            app.quit();
        }
        _ => {}
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
    ConnectionFactory::get_default_port(db_type)
}

async fn list_connections() -> Result<()> {
    let config_manager = ConfigManager::new()?;
    let config = config_manager.config();

    if config.connections.is_empty() {
        println!("No saved connections found.");
        println!("Use 'sqlterm add <name> --db-type <type> ...' to add a connection.");
        return Ok(());
    }

    println!("Saved connections:");
    println!("{:<20} {:<15} {:<30}", "Name", "Type", "Connection String");
    println!("{}", "-".repeat(65));

    for conn in &config.connections {
        let display_string = ConnectionFactory::create_display_string(conn);
        println!("{:<20} {:<15} {:<30}",
                 conn.name,
                 conn.database_type.to_string(),
                 display_string);
    }

    if let Some(default) = &config.settings.default_connection {
        println!("\nDefault connection: {}", default);
    }

    Ok(())
}

async fn add_connection(config: ConnectionConfig) -> Result<()> {
    // Validate the configuration
    ConnectionFactory::validate_config(&config)?;

    let mut config_manager = ConfigManager::new()?;

    // Add the connection
    config_manager.config_mut().add_connection(config.clone())?;

    // Save the configuration
    config_manager.save()?;

    println!("✓ Added connection: {}", config.name);
    println!("  Type: {}", config.database_type);
    println!("  Connection: {}", ConnectionFactory::create_display_string(&config));

    Ok(())
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
                .map_err(|e| anyhow::anyhow!(\"MySQL connection failed: {}\", e))?
        }
        DatabaseType::PostgreSQL => {
            use sqlterm_core::DatabaseConnection;
            sqlterm_postgres::PostgresConnection::connect(&config).await
                .map_err(|e| anyhow::anyhow!(\"PostgreSQL connection failed: {}\", e))?
        }
        DatabaseType::SQLite => {
            use sqlterm_core::DatabaseConnection;
            sqlterm_sqlite::SqliteConnection::connect(&config).await
                .map_err(|e| anyhow::anyhow!(\"SQLite connection failed: {}\", e))?
        }
    };
    
    // Test the connection
    connection.ping().await
        .map_err(|e| anyhow::anyhow!(\"Connection test failed: {}\", e))?;
    
    // Get connection info for display
    let conn_info = connection.get_connection_info().await?;
    
    // Store the connection in the app state
    app.active_connection = Some(connection);
    app.current_database = Some(conn_info.database_name.clone());
    
    // For now, use mock table data - real table loading will be implemented later
    app.tables = vec![
        "users".to_string(),
        "posts".to_string(),
        "categories".to_string(),
        "orders".to_string(),
    ];
    app.selected_table = 0;
    
    Ok(())
}

async fn load_table_details(app: &mut App, table_name: &str) -> Result<()> {

    // Create mock table details for demonstration
    // In a real implementation, this would use the active database connection
    let table_details = create_mock_table_details(table_name);

    app.set_table_details(table_details);
    Ok(())
}

fn create_mock_table_details(table_name: &str) -> sqlterm_core::TableDetails {
    use sqlterm_core::{TableDetails, Table, Column, Index, ForeignKey, TableStatistics, TableType};

    let table = Table {
        name: table_name.to_string(),
        schema: Some("public".to_string()),
        table_type: TableType::Table,
        row_count: Some(match table_name {
            "users" => 3,
            "posts" => 5,
            "categories" => 3,
            "post_categories" => 5,
            "published_posts" => 4,
            _ => 0,
        }),
        size: Some(8192),
        comment: Some(format!("Table: {}", table_name)),
    };

    let columns = match table_name {
        "users" => vec![
            Column {
                name: "id".to_string(),
                data_type: "SERIAL".to_string(),
                nullable: false,
                default_value: Some("nextval(\"users_id_seq\")".to_string()),
                is_primary_key: true,
                is_foreign_key: false,
                is_unique: true,
                is_auto_increment: true,
                max_length: None,
                precision: None,
                scale: None,
                comment: Some("Primary key".to_string()),
            },
            Column {
                name: "username".to_string(),
                data_type: "VARCHAR(50)".to_string(),
                nullable: false,
                default_value: None,
                is_primary_key: false,
                is_foreign_key: false,
                is_unique: true,
                is_auto_increment: false,
                max_length: Some(50),
                precision: None,
                scale: None,
                comment: Some("Unique username".to_string()),
            },
            Column {
                name: "email".to_string(),
                data_type: "VARCHAR(100)".to_string(),
                nullable: false,
                default_value: None,
                is_primary_key: false,
                is_foreign_key: false,
                is_unique: false,
                is_auto_increment: false,
                max_length: Some(100),
                precision: None,
                scale: None,
                comment: Some("User email address".to_string()),
            },
            Column {
                name: "created_at".to_string(),
                data_type: "TIMESTAMP".to_string(),
                nullable: false,
                default_value: Some("CURRENT_TIMESTAMP".to_string()),
                is_primary_key: false,
                is_foreign_key: false,
                is_unique: false,
                is_auto_increment: false,
                max_length: None,
                precision: None,
                scale: None,
                comment: Some("Account creation timestamp".to_string()),
            },
        ],
        "posts" => vec![
            Column {
                name: "id".to_string(),
                data_type: "SERIAL".to_string(),
                nullable: false,
                default_value: Some("nextval(\"posts_id_seq\")".to_string()),
                is_primary_key: true,
                is_foreign_key: false,
                is_unique: true,
                is_auto_increment: true,
                max_length: None,
                precision: None,
                scale: None,
                comment: Some("Primary key".to_string()),
            },
            Column {
                name: "user_id".to_string(),
                data_type: "INTEGER".to_string(),
                nullable: false,
                default_value: None,
                is_primary_key: false,
                is_foreign_key: true,
                is_unique: false,
                is_auto_increment: false,
                max_length: None,
                precision: None,
                scale: None,
                comment: Some("Foreign key to users".to_string()),
            },
            Column {
                name: "title".to_string(),
                data_type: "VARCHAR(200)".to_string(),
                nullable: false,
                default_value: None,
                is_primary_key: false,
                is_foreign_key: false,
                is_unique: false,
                is_auto_increment: false,
                max_length: Some(200),
                precision: None,
                scale: None,
                comment: Some("Post title".to_string()),
            },
            Column {
                name: "content".to_string(),
                data_type: "TEXT".to_string(),
                nullable: true,
                default_value: None,
                is_primary_key: false,
                is_foreign_key: false,
                is_unique: false,
                is_auto_increment: false,
                max_length: None,
                precision: None,
                scale: None,
                comment: Some("Post content".to_string()),
            },
            Column {
                name: "published".to_string(),
                data_type: "BOOLEAN".to_string(),
                nullable: false,
                default_value: Some("false".to_string()),
                is_primary_key: false,
                is_foreign_key: false,
                is_unique: false,
                is_auto_increment: false,
                max_length: None,
                precision: None,
                scale: None,
                comment: Some("Publication status".to_string()),
            },
        ],
        _ => vec![
            Column {
                name: "id".to_string(),
                data_type: "INTEGER".to_string(),
                nullable: false,
                default_value: None,
                is_primary_key: true,
                is_foreign_key: false,
                is_unique: true,
                is_auto_increment: true,
                max_length: None,
                precision: None,
                scale: None,
                comment: Some("Primary key".to_string()),
            },
            Column {
                name: "name".to_string(),
                data_type: "VARCHAR(100)".to_string(),
                nullable: false,
                default_value: None,
                is_primary_key: false,
                is_foreign_key: false,
                is_unique: false,
                is_auto_increment: false,
                max_length: Some(100),
                precision: None,
                scale: None,
                comment: Some("Name field".to_string()),
            },
        ],
    };

    let indexes = vec![
        Index {
            name: format!("{}_pkey", table_name),
            table_name: table_name.to_string(),
            columns: vec!["id".to_string()],
            is_unique: true,
            is_primary: true,
            index_type: "btree".to_string(),
        },
    ];

    let foreign_keys = if table_name == "posts" {
        vec![
            ForeignKey {
                name: "posts_user_id_fkey".to_string(),
                table_name: "posts".to_string(),
                column_name: "user_id".to_string(),
                referenced_table: "users".to_string(),
                referenced_column: "id".to_string(),
                on_delete: Some("CASCADE".to_string()),
                on_update: Some("NO ACTION".to_string()),
            },
        ]
    } else {
        vec![]
    };

    let statistics = TableStatistics {
        row_count: table.row_count.unwrap_or(0),
        size_bytes: table.size,
        last_updated: Some("2024-01-15 10:30:00".to_string()),
        auto_increment_value: Some(match table_name {
            "users" => 4,
            "posts" => 6,
            _ => 1,
        }),
    };

    TableDetails {
        table,
        columns,
        indexes,
        foreign_keys,
        statistics,
    }
}

async fn execute_query(app: &mut App) -> Result<()> {
    use std::time::Instant;

    let start = Instant::now();
    let query = app.query_input.trim();

    // Create mock query results based on the query
    let results = create_mock_query_results(query);

    // Apply default truncation (200 rows)
    let truncated_results = if results.rows.len() > 200 {
        sqlterm_core::QueryResult::truncated(
            results.columns,
            results.rows,
            start.elapsed(),
            results.total_rows,
            200,
        )
    } else {
        results
    };

    app.set_query_results(truncated_results);
    Ok(())
}

fn create_mock_query_results(query: &str) -> sqlterm_core::QueryResult {
    use sqlterm_core::{QueryResult, ColumnInfo, Row, Value};
    use std::time::Duration;

    let query_lower = query.to_lowercase();

    if query_lower.contains("select") && query_lower.contains("users") {
        // Mock users query
        let columns = vec![
            ColumnInfo {
                name: "id".to_string(),
                data_type: "INTEGER".to_string(),
                nullable: false,
                max_length: None,
                precision: None,
                scale: None,
            },
            ColumnInfo {
                name: "username".to_string(),
                data_type: "VARCHAR".to_string(),
                nullable: false,
                max_length: Some(50),
                precision: None,
                scale: None,
            },
            ColumnInfo {
                name: "email".to_string(),
                data_type: "VARCHAR".to_string(),
                nullable: false,
                max_length: Some(100),
                precision: None,
                scale: None,
            },
            ColumnInfo {
                name: "created_at".to_string(),
                data_type: "TIMESTAMP".to_string(),
                nullable: false,
                max_length: None,
                precision: None,
                scale: None,
            },
        ];

        let rows = vec![
            Row {
                values: vec![
                    Value::Integer(1),
                    Value::String("alice".to_string()),
                    Value::String("alice@example.com".to_string()),
                    Value::DateTime("2024-01-15 10:30:00".to_string()),
                ],
            },
            Row {
                values: vec![
                    Value::Integer(2),
                    Value::String("bob".to_string()),
                    Value::String("bob@example.com".to_string()),
                    Value::DateTime("2024-01-15 11:15:00".to_string()),
                ],
            },
            Row {
                values: vec![
                    Value::Integer(3),
                    Value::String("charlie".to_string()),
                    Value::String("charlie@example.com".to_string()),
                    Value::DateTime("2024-01-15 12:00:00".to_string()),
                ],
            },
        ];

        QueryResult::new(columns, rows, Duration::from_millis(45))
    } else if query_lower.contains("select") && query_lower.contains("posts") {
        // Mock posts query
        let columns = vec![
            ColumnInfo {
                name: "id".to_string(),
                data_type: "INTEGER".to_string(),
                nullable: false,
                max_length: None,
                precision: None,
                scale: None,
            },
            ColumnInfo {
                name: "title".to_string(),
                data_type: "VARCHAR".to_string(),
                nullable: false,
                max_length: Some(200),
                precision: None,
                scale: None,
            },
            ColumnInfo {
                name: "author".to_string(),
                data_type: "VARCHAR".to_string(),
                nullable: false,
                max_length: Some(50),
                precision: None,
                scale: None,
            },
            ColumnInfo {
                name: "published".to_string(),
                data_type: "BOOLEAN".to_string(),
                nullable: false,
                max_length: None,
                precision: None,
                scale: None,
            },
        ];

        let rows = vec![
            Row {
                values: vec![
                    Value::Integer(1),
                    Value::String("Getting Started with Rust".to_string()),
                    Value::String("alice".to_string()),
                    Value::Boolean(true),
                ],
            },
            Row {
                values: vec![
                    Value::Integer(2),
                    Value::String("Database Design Patterns".to_string()),
                    Value::String("alice".to_string()),
                    Value::Boolean(true),
                ],
            },
            Row {
                values: vec![
                    Value::Integer(3),
                    Value::String("My Trip to Japan".to_string()),
                    Value::String("bob".to_string()),
                    Value::Boolean(true),
                ],
            },
            Row {
                values: vec![
                    Value::Integer(4),
                    Value::String("Cooking at Home".to_string()),
                    Value::String("bob".to_string()),
                    Value::Boolean(false),
                ],
            },
            Row {
                values: vec![
                    Value::Integer(5),
                    Value::String("Remote Work Tips".to_string()),
                    Value::String("charlie".to_string()),
                    Value::Boolean(true),
                ],
            },
        ];

        QueryResult::new(columns, rows, Duration::from_millis(32))
    } else {
        // Generic query result
        let columns = vec![
            ColumnInfo {
                name: "result".to_string(),
                data_type: "VARCHAR".to_string(),
                nullable: false,
                max_length: Some(100),
                precision: None,
                scale: None,
            },
        ];

        let rows = vec![
            Row {
                values: vec![Value::String("Query executed successfully".to_string())],
            },
        ];

        QueryResult::new(columns, rows, Duration::from_millis(15))
    }
}

async fn export_results_to_file(app: &mut App) -> Result<()> {
    if let Some(results) = app.get_query_results() {
        let timestamp = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_secs();
        let filename = format!("sqlterm_results_{}.csv", timestamp);

        let csv_content = results.to_csv();
        std::fs::write(&filename, csv_content)?;

        app.set_error(format!("Results exported to: {}", filename));
    } else {
        app.set_error("No results to export".to_string());
    }
    Ok(())
}

async fn show_full_results(app: &mut App) -> Result<()> {
    if let Some(current_results) = app.get_query_results() {
        if current_results.is_truncated {
            // Create a new result set without truncation
            let query = app.query_input.trim();
            let full_results = create_mock_query_results(query);
            app.set_query_results(full_results);
            app.set_error("Showing full results (no truncation)".to_string());
        } else {
            app.set_error("Results are already showing in full".to_string());
        }
    } else {
        app.set_error("No results to show".to_string());
    }
    Ok(())
}
