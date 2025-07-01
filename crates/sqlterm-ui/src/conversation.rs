use anyhow::Result;
use chrono;
use regex;
use rustyline::error::ReadlineError;
use rustyline::{Helper, Editor, Config};
use rustyline::completion::{Completer, Pair};
use rustyline::hint::{Hinter, HistoryHinter};
use rustyline::validate::{Validator, MatchingBracketValidator};
use rustyline::highlight::Highlighter;
use rustyline::history::DefaultHistory;
use sqlterm_core::{ConnectionConfig, DatabaseConnection, DatabaseType};
use std::borrow::Cow;
use std::fs;
use std::path::{Path, PathBuf};
use std::io::Write;

struct SQLTermHelper {
    completer: SQLTermCompleter,
    hinter: HistoryHinter,
    validator: MatchingBracketValidator,
}

impl Helper for SQLTermHelper {}

impl Completer for SQLTermHelper {
    type Candidate = Pair;

    fn complete(
        &self,
        line: &str,
        pos: usize,
        _ctx: &rustyline::Context<'_>,
    ) -> rustyline::Result<(usize, Vec<Self::Candidate>)> {
        self.completer.complete(line, pos, _ctx)
    }
}

impl Hinter for SQLTermHelper {
    type Hint = String;

    fn hint(&self, line: &str, pos: usize, ctx: &rustyline::Context<'_>) -> Option<Self::Hint> {
        self.hinter.hint(line, pos, ctx)
    }
}

impl Validator for SQLTermHelper {
    fn validate(&self, ctx: &mut rustyline::validate::ValidationContext) -> rustyline::Result<rustyline::validate::ValidationResult> {
        self.validator.validate(ctx)
    }
}

impl Highlighter for SQLTermHelper {
    fn highlight_prompt<'b, 's: 'b, 'p: 'b>(
        &'s self,
        prompt: &'p str,
        _default: bool,
    ) -> Cow<'b, str> {
        Cow::Borrowed(prompt)
    }

    fn highlight_hint<'h>(&self, hint: &'h str) -> Cow<'h, str> {
        Cow::Borrowed(hint)
    }

    fn highlight<'l>(&self, line: &'l str, _pos: usize) -> Cow<'l, str> {
        Cow::Borrowed(line)
    }

    fn highlight_char(&self, _line: &str, _pos: usize, _forced: bool) -> bool {
        false
    }
}

struct SQLTermCompleter {
    config_manager: crate::config::ConfigManager,
    tables: Vec<String>,
}

impl SQLTermCompleter {
    fn new(config_manager: crate::config::ConfigManager) -> Self {
        Self {
            config_manager,
            tables: Vec::new(),
        }
    }

    fn update_tables(&mut self, tables: Vec<String>) {
        self.tables = tables;
    }

    fn get_commands() -> Vec<&'static str> {
        vec![
            "/help", "/connect", "/list-connections", "/connections", "/tables",
            "/describe", "/desc", "/use", "/status", "/clear",
            "/list-sessions", "/sessions", "/save-session", "/load-session", 
            "/delete-session", "/copy-result", "/copy-query", "/paste", 
            "/quit", "/exit"
        ]
    }

    fn get_sql_keywords() -> Vec<&'static str> {
        vec![
            "SELECT", "FROM", "WHERE", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP",
            "ALTER", "INDEX", "TABLE", "DATABASE", "SHOW", "DESCRIBE", "EXPLAIN",
            "ORDER", "BY", "GROUP", "HAVING", "LIMIT", "OFFSET", "JOIN", "INNER",
            "LEFT", "RIGHT", "OUTER", "ON", "AS", "AND", "OR", "NOT", "NULL",
            "PRIMARY", "KEY", "FOREIGN", "REFERENCES", "UNIQUE", "AUTO_INCREMENT",
            "DEFAULT", "CHECK", "CONSTRAINT"
        ]
    }

    fn complete_file_path(&self, input: &str) -> Vec<Pair> {
        let path_part = if input.starts_with('@') {
            &input[1..]
        } else {
            input
        };

        let mut completions = Vec::new();
        let current_dir = std::env::current_dir().unwrap_or_else(|_| PathBuf::from("."));
        
        // Check if we're completing a query range (e.g., "@file.sql 2" or "@file.sql 2-")
        if let Some(space_pos) = path_part.find(' ') {
            let file_part = &path_part[..space_pos];
            let range_part = &path_part[space_pos + 1..];
            
            // Check if the file exists and suggest query ranges
            let file_path = current_dir.join(file_part);
            if file_path.exists() && file_path.is_file() {
                // Count queries in the file to suggest ranges
                if let Ok(content) = fs::read_to_string(&file_path) {
                    let query_count = self.count_queries_in_file(&content);
                    
                    // Suggest individual query numbers
                    for i in 1..=query_count {
                        let i_str = i.to_string();
                        if i_str.starts_with(range_part) {
                            completions.push(Pair {
                                display: format!("@{} {}", file_part, i),
                                replacement: format!("@{} {}", file_part, i),
                            });
                        }
                    }
                    
                    // Suggest ranges if user is typing a range
                    if range_part.contains('-') || range_part.ends_with('-') {
                        let range_prefix = if range_part.ends_with('-') {
                            range_part
                        } else {
                            &range_part[..range_part.find('-').unwrap_or(0) + 1]
                        };
                        
                        for end in 2..=query_count {
                            let range_str = format!("{}{}",range_prefix, end);
                            if range_str.starts_with(range_part) {
                                completions.push(Pair {
                                    display: format!("@{} {}", file_part, range_str),
                                    replacement: format!("@{} {}", file_part, range_str),
                                });
                            }
                        }
                    }
                }
            }
            
            return completions;
        }
        
        if path_part.is_empty() {
            // Complete with files in current directory
            if let Ok(entries) = fs::read_dir(&current_dir) {
                for entry in entries.flatten() {
                    let path = entry.path();
                    if let Some(name) = path.file_name().and_then(|n| n.to_str()) {
                        if path.is_file() && (name.ends_with(".sql") || name.ends_with(".txt")) {
                            completions.push(Pair {
                                display: format!("@{}", name),
                                replacement: format!("@{}", name),
                            });
                        } else if path.is_dir() {
                            completions.push(Pair {
                                display: format!("@{}/", name),
                                replacement: format!("@{}/", name),
                            });
                        }
                    }
                }
            }
        } else {
            // Complete path components
            let path = Path::new(path_part);
            let (parent, prefix) = if path_part.ends_with('/') {
                (path, "")
            } else {
                (path.parent().unwrap_or(Path::new(".")), 
                 path.file_name().and_then(|n| n.to_str()).unwrap_or(""))
            };

            if let Ok(entries) = fs::read_dir(parent) {
                for entry in entries.flatten() {
                    let entry_path = entry.path();
                    if let Some(name) = entry_path.file_name().and_then(|n| n.to_str()) {
                        if name.starts_with(prefix) {
                            let full_path = if parent == Path::new(".") {
                                name.to_string()
                            } else {
                                format!("{}/{}", parent.display(), name)
                            };

                            if entry_path.is_file() && (name.ends_with(".sql") || name.ends_with(".txt")) {
                                completions.push(Pair {
                                    display: format!("@{}", full_path),
                                    replacement: format!("@{}", full_path),
                                });
                            } else if entry_path.is_dir() {
                                completions.push(Pair {
                                    display: format!("@{}/", full_path),
                                    replacement: format!("@{}/", full_path),
                                });
                            }
                        }
                    }
                }
            }
        }

        completions
    }

    fn count_queries_in_file(&self, content: &str) -> usize {
        content
            .split(';')
            .map(|s| s.trim())
            .filter(|s| !s.is_empty() && !s.chars().all(|c| c.is_whitespace() || c == '\n'))
            .count()
    }

    fn complete_command(&self, line: &str) -> rustyline::Result<(usize, Vec<Pair>)> {
        let parts: Vec<&str> = line.split_whitespace().collect();
        
        if parts.is_empty() {
            return Ok((0, vec![]));
        }

        let command = parts[0];
        
        // Context-aware completion based on command
        match command {
            "/describe" | "/desc" => {
                if parts.len() <= 2 {
                    // Complete with table names
                    let prefix = if parts.len() == 2 { parts[1] } else { "" };
                    let mut completions = Vec::new();
                    
                    for table in &self.tables {
                        if table.to_lowercase().starts_with(&prefix.to_lowercase()) {
                            completions.push(Pair {
                                display: format!("/describe {}", table),
                                replacement: format!("/describe {}", table),
                            });
                        }
                    }
                    
                    let start_pos = if parts.len() == 2 {
                        line.rfind(parts[1]).unwrap_or(0)
                    } else {
                        line.len()
                    };
                    
                    return Ok((0, completions));
                }
            }
            "/connect" => {
                if parts.len() <= 2 {
                    // Complete with saved connection names
                    let prefix = if parts.len() == 2 { parts[1] } else { "" };
                    let mut completions = Vec::new();
                    
                    if let Ok(connections) = self.config_manager.list_connections() {
                        for conn in connections {
                            if conn.name.to_lowercase().starts_with(&prefix.to_lowercase()) {
                                completions.push(Pair {
                                    display: format!("/connect {}", conn.name),
                                    replacement: format!("/connect {}", conn.name),
                                });
                            }
                        }
                    }
                    
                    return Ok((0, completions));
                }
            }
            "/load-session" => {
                if parts.len() <= 2 {
                    // Complete with saved session names
                    let prefix = if parts.len() == 2 { parts[1] } else { "" };
                    let mut completions = Vec::new();
                    
                    if let Ok(sessions) = self.config_manager.list_sessions() {
                        for session in sessions {
                            if session.to_lowercase().starts_with(&prefix.to_lowercase()) {
                                completions.push(Pair {
                                    display: format!("/load-session {}", session),
                                    replacement: format!("/load-session {}", session),
                                });
                            }
                        }
                    }
                    
                    return Ok((0, completions));
                }
            }
            "/delete-session" => {
                if parts.len() <= 2 {
                    // Complete with saved session names
                    let prefix = if parts.len() == 2 { parts[1] } else { "" };
                    let mut completions = Vec::new();
                    
                    if let Ok(sessions) = self.config_manager.list_sessions() {
                        for session in sessions {
                            if session.to_lowercase().starts_with(&prefix.to_lowercase()) {
                                completions.push(Pair {
                                    display: format!("/delete-session {}", session),
                                    replacement: format!("/delete-session {}", session),
                                });
                            }
                        }
                    }
                    
                    return Ok((0, completions));
                }
            }
            "/copy-result" => {
                if parts.len() <= 2 {
                    // Complete with result IDs (placeholder - in real implementation would track result IDs)
                    let prefix = if parts.len() == 2 { parts[1] } else { "" };
                    let mut completions = Vec::new();
                    
                    // Suggest some common result IDs
                    for id in 1..=5 {
                        let id_str = id.to_string();
                        if id_str.starts_with(prefix) {
                            completions.push(Pair {
                                display: format!("/copy-result {}", id),
                                replacement: format!("/copy-result {}", id),
                            });
                        }
                    }
                    
                    return Ok((0, completions));
                }
            }
            _ => {
                // Default command completion
                let completions = Self::get_commands()
                    .into_iter()
                    .filter(|cmd| cmd.starts_with(line))
                    .map(|cmd| Pair {
                        display: cmd.to_string(),
                        replacement: cmd.to_string(),
                    })
                    .collect();
                return Ok((0, completions));
            }
        }
        
        Ok((0, vec![]))
    }
}

impl Completer for SQLTermCompleter {
    type Candidate = Pair;

    fn complete(
        &self,
        line: &str,
        pos: usize,
        _ctx: &rustyline::Context<'_>,
    ) -> rustyline::Result<(usize, Vec<Pair>)> {
        let line = &line[..pos];
        
        // File completion for @ references (with optional query range)
        if line.starts_with('@') || line.contains(" @") {
            let at_pos = line.rfind('@').unwrap_or(0);
            let file_part = &line[at_pos..];
            let completions = self.complete_file_path(file_part);
            return Ok((at_pos, completions));
        }

        // Context-aware command completion for / commands
        if line.starts_with('/') {
            return self.complete_command(line);
        }

        // SQL keyword completion
        let words: Vec<&str> = line.split_whitespace().collect();
        if let Some(last_word) = words.last() {
            if last_word.len() > 1 {
                let mut completions = Vec::new();

                // SQL keywords
                for keyword in Self::get_sql_keywords() {
                    if keyword.to_lowercase().starts_with(&last_word.to_lowercase()) {
                        completions.push(Pair {
                            display: keyword.to_string(),
                            replacement: keyword.to_string(),
                        });
                    }
                }

                // Table names
                for table in &self.tables {
                    if table.to_lowercase().starts_with(&last_word.to_lowercase()) {
                        completions.push(Pair {
                            display: table.clone(),
                            replacement: table.clone(),
                        });
                    }
                }

                // Connection names for /connect command
                if line.starts_with("/connect ") {
                    if let Ok((connections, _)) = self.config_manager.list_connections_with_errors() {
                        for conn in connections {
                            if conn.name.to_lowercase().starts_with(&last_word.to_lowercase()) {
                                completions.push(Pair {
                                    display: conn.name.clone(),
                                    replacement: conn.name.clone(),
                                });
                            }
                        }
                    }
                }

                let start_pos = line.len() - last_word.len();
                return Ok((start_pos, completions));
            }
        }

        Ok((0, vec![]))
    }
}

pub struct ConversationApp {
    active_connection: Option<Box<dyn DatabaseConnection>>,
    current_database: Option<String>,
    config_manager: crate::config::ConfigManager,
    editor: Editor<SQLTermHelper, DefaultHistory>,
    tables: Vec<String>,
}

impl ConversationApp {
    pub fn new() -> Result<Self> {
        let config_manager = crate::config::ConfigManager::new()?;
        
        // Set up rustyline editor with custom helper
        let config = Config::builder()
            .history_ignore_space(true)
            .completion_type(rustyline::CompletionType::List)
            .build();
        
        let mut editor = Editor::with_config(config)?;
        
        // Set up history file
        let history_path = config_manager.get_config_directory_path().join("history.txt");
        let _ = editor.load_history(&history_path); // Ignore errors if history doesn't exist
        
        // Create completer with config manager
        let completer = SQLTermCompleter::new(config_manager.clone());
        let helper = SQLTermHelper {
            completer,
            hinter: HistoryHinter::new(),
            validator: MatchingBracketValidator::new(),
        };
        editor.set_helper(Some(helper));
        
        Ok(Self {
            active_connection: None,
            current_database: None,
            config_manager,
            editor,
            tables: Vec::new(),
        })
    }

    pub async fn run(&mut self) -> Result<()> {
        println!("🗄️  SQLTerm - Conversation Mode");
        println!("Type /help for available commands, or enter SQL queries directly.");
        println!("Use @ to reference SQL files (e.g., @queries.sql)");
        println!("📝 Features: Tab completion, ↑/↓ for history, Ctrl+C to copy, Ctrl+V to paste");
        println!();

        loop {
            // Create prompt with database info
            let prompt = self.get_prompt();
            
            // Read input with rustyline (includes history, completion, etc.)
            match self.editor.readline(&prompt) {
                Ok(line) => {
                    let input = line.trim();
                    
                    if input.is_empty() {
                        continue;
                    }

                    // Add to history
                    self.editor.add_history_entry(input)?;

                    // Handle exit
                    if input == "/quit" || input == "/exit" || input == "\\q" {
                        // Save history before exiting
                        let history_path = self.config_manager.get_config_directory_path().join("history.txt");
                        let _ = self.editor.save_history(&history_path);
                        println!("Goodbye! 👋");
                        break;
                    }

                    // Process command or query
                    if let Err(e) = self.process_input(input).await {
                        println!("❌ Error: {}", e);
                    }
                    
                    println!(); // Add spacing between commands
                }
                Err(ReadlineError::Interrupted) => {
                    // Ctrl+C pressed - check if user wants to copy to clipboard
                    if let Err(e) = self.handle_copy_request().await {
                        println!("❌ Copy failed: {}", e);
                    }
                }
                Err(ReadlineError::Eof) => {
                    // Ctrl+D pressed - exit
                    println!("Goodbye! 👋");
                    break;
                }
                Err(err) => {
                    println!("❌ Input error: {}", err);
                    break;
                }
            }
        }

        // Save history on exit
        let history_path = self.config_manager.get_config_directory_path().join("history.txt");
        let _ = self.editor.save_history(&history_path);

        Ok(())
    }

    fn get_prompt(&self) -> String {
        let db_info = if let Some(db) = &self.current_database {
            format!("({}) ", db)
        } else {
            String::new()
        };
        
        format!("sqlterm{} > ", db_info)
    }

    async fn handle_copy_request(&self) -> Result<()> {
        // For now, just show a message. In a more advanced implementation,
        // you could copy the last query result or current buffer to clipboard
        println!("💡 Use Ctrl+C in your terminal to copy text, or use the /copy command (future feature)");
        Ok(())
    }

    async fn process_input(&mut self, input: &str) -> Result<()> {
        if input.starts_with('/') {
            self.handle_command(input).await
        } else if input.starts_with('@') {
            self.handle_file_reference(input).await
        } else {
            self.execute_query(input).await
        }
    }

    async fn handle_command(&mut self, command: &str) -> Result<()> {
        let parts: Vec<&str> = command.split_whitespace().collect();
        if parts.is_empty() {
            return Ok(());
        }

        match parts[0] {
            "/help" => self.show_help(),
            "/connect" => self.handle_connect_command(&parts[1..]).await?,
            "/list-connections" | "/connections" => self.list_connections().await?,
            "/tables" => self.list_tables().await?,
            "/describe" | "/desc" => self.describe_table(&parts[1..]).await?,
            "/use" => self.use_database(&parts[1..]).await?,
            "/status" => self.show_status(),
            "/clear" => self.clear_screen(),
            "/list-sessions" | "/sessions" => self.list_sessions().await?,
            "/save-session" => self.save_session_command(&parts[1..]).await?,
            "/load-session" => self.load_session_command(&parts[1..]).await?,
            "/delete-session" => self.delete_session_command(&parts[1..]).await?,
            "/copy-result" => self.copy_result_command(&parts[1..]).await?,
            "/copy-query" => self.copy_query_command().await?,
            "/paste" => self.paste_command().await?,
            _ => println!("❓ Unknown command: {}. Type /help for available commands.", parts[0]),
        }
        Ok(())
    }

    fn show_help(&self) {
        println!("📋 Available Commands:");
        println!("  /help                    - Show this help message");
        println!("  /connect                 - Interactive connection setup wizard");
        println!("  /connect <name>          - Connect to saved connection");
        println!("  /connect <conn_string>   - Connect with connection string");
        println!("  /list-connections        - List all saved connections");
        println!("  /tables                  - List tables in current database");
        println!("  /describe <table>        - Show table structure");
        println!("  /use <database>          - Switch to different database");
        println!("  /status                  - Show connection status");
        println!("  /clear                   - Clear screen");
        println!("  /list-sessions           - List saved conversation sessions");
        println!("  /save-session <name>     - Save current session");
        println!("  /load-session <name>     - Load a saved session");
        println!("  /delete-session <name>   - Delete a saved session");
        println!("  /copy-result <id>        - Copy query result to clipboard");
        println!("  /copy-query              - Copy last query to clipboard");
        println!("  /paste                   - Show clipboard content");
        println!("  /quit, /exit, \\q         - Exit SQLTerm");
        println!();
        println!("📁 File References:");
        println!("  @file.sql               - Execute all queries in SQL file");
        println!("  @file.sql 2             - Execute only query #2 from file");
        println!("  @file.sql 2-5           - Execute queries #2 through #5 from file");
        println!("  @file.sql 3-            - Execute queries #3 to end of file");
        println!("  @path/to/queries.sql    - Execute SQL file with path");
        println!();
        println!("💡 Examples:");
        println!("  /connect                 - Start interactive setup");
        println!("  /connect mydb            - Connect to saved 'mydb'");
        println!("  /connect mysql://user:pass@localhost:3306/mydb");
        println!("  SELECT * FROM users;");
        println!("  @demo.sql                - Execute all queries in demo.sql");
        println!("  @queries/test.sql 1      - Execute only first query");
        println!("  @queries/test.sql 2-4    - Execute queries 2, 3, and 4");
        println!("  /describe users");
        println!();
        println!("🔧 Interactive Setup:");
        println!("  Just type '/connect' and follow the prompts to:");
        println!("  • Choose database type (MySQL/PostgreSQL/SQLite)");
        println!("  • Enter connection details step by step");
        println!("  • Test and save the connection automatically");
        println!();
        println!("📋 Clipboard Support:");
        println!("  /copy-result <id>         - Copy query result to clipboard");
        println!("  /copy-query               - Copy last query to clipboard");
        println!("  /paste                    - Paste from clipboard");
        println!();
        println!("🎯 Auto-completion:");
        println!("  • Tab to complete commands, SQL keywords, table names");
        println!("  • File paths with @ references");
        println!("  • Connection names for /connect");
        println!();
        println!("📂 File Storage:");
        println!("  • Connections: ~/.config/sqlterm/connections/<name>.toml");
        println!("  • Sessions: ~/.config/sqlterm/sessions/<name>.txt");
        println!("  • History: ~/.config/sqlterm/history.txt");
    }

    async fn handle_connect_command(&mut self, args: &[&str]) -> Result<()> {
        if args.is_empty() {
            // Interactive connection wizard
            return self.interactive_connect_wizard().await;
        }

        // Try to load as saved connection first
        if args.len() == 1 {
            match self.config_manager.load_connection(args[0]) {
                Ok(config) => {
                    return self.connect_to_database(config).await;
                }
                Err(_) => {
                    // Fall through to parse as connection string
                }
            }
        }

        // Parse as connection string or parameters
        if args.len() == 1 && args[0].contains("://") {
            // Connection string format
            let config = self.parse_connection_string(args[0])?;
            self.connect_to_database(config).await
        } else {
            println!("❓ Connection string format: <type>://user:pass@host:port/database");
            println!("❓ Or use a saved connection name");
            println!("💡 Use '/connect' without arguments for interactive setup");
            Ok(())
        }
    }

    async fn interactive_connect_wizard(&mut self) -> Result<()> {
        println!("🔧 Interactive Connection Setup");
        println!("Press Ctrl+C to cancel at any time.\n");

        // Step 1: Connection name
        let name = self.prompt_for_input("Enter connection name", None)?;
        if name.trim().is_empty() {
            println!("❌ Connection name cannot be empty");
            return Ok(());
        }

        // Check if connection already exists
        if self.config_manager.connection_exists(&name) {
            print!("⚠️  Connection '{}' already exists. Overwrite? (y/N): ", name);
            std::io::stdout().flush().unwrap();
            let mut confirm = String::new();
            std::io::stdin().read_line(&mut confirm)?;
            if !confirm.trim().to_lowercase().starts_with('y') {
                println!("❌ Connection setup cancelled");
                return Ok(());
            }
        }

        // Step 2: Database type
        let db_type = self.prompt_for_database_type()?;

        // Step 3: Host
        let default_host = "localhost".to_string();
        let host = self.prompt_for_input("Enter host", Some(&default_host))?;
        let host = if host.trim().is_empty() { default_host } else { host };

        // Step 4: Port
        let default_port = self.get_default_port_for_type(&db_type);
        let port_input = self.prompt_for_input(&format!("Enter port"), Some(&default_port.to_string()))?;
        let port = if port_input.trim().is_empty() {
            default_port
        } else {
            port_input.parse::<u16>().unwrap_or_else(|_| {
                println!("⚠️  Invalid port, using default: {}", default_port);
                default_port
            })
        };

        // Step 5: Database name
        let database = if db_type == DatabaseType::SQLite {
            let default_db = ":memory:".to_string();
            let db_input = self.prompt_for_input("Enter database path (or :memory: for in-memory)", Some(&default_db))?;
            if db_input.trim().is_empty() { default_db } else { db_input }
        } else {
            let db_input = self.prompt_for_input("Enter database name", None)?;
            if db_input.trim().is_empty() {
                println!("❌ Database name cannot be empty");
                return Ok(());
            }
            db_input
        };

        // Step 6: Username (skip for SQLite)
        let username = if db_type == DatabaseType::SQLite {
            String::new()
        } else {
            let user_input = self.prompt_for_input("Enter username", None)?;
            if user_input.trim().is_empty() {
                println!("❌ Username cannot be empty");
                return Ok(());
            }
            user_input
        };

        // Step 7: Password (skip for SQLite)
        let password = if db_type == DatabaseType::SQLite {
            None
        } else {
            let pass_input = self.prompt_for_password("Enter password (leave empty for no password)")?;
            if pass_input.is_empty() { None } else { Some(pass_input) }
        };

        // Create connection config
        let config = ConnectionConfig {
            name: name.clone(),
            database_type: db_type,
            host,
            port,
            database,
            username,
            password,
            ssl: false,
            ssh_tunnel: None,
        };

        // Display summary
        println!("\n📋 Connection Summary:");
        println!("  Name: {}", config.name);
        println!("  Type: {:?}", config.database_type);
        println!("  Host: {}", config.host);
        println!("  Port: {}", config.port);
        println!("  Database: {}", config.database);
        if !config.username.is_empty() {
            println!("  Username: {}", config.username);
        }
        if config.password.is_some() {
            println!("  Password: [hidden]");
        }

        // Confirm before connecting
        print!("\n🔌 Test connection and save? (Y/n): ");
        std::io::stdout().flush().unwrap();
        let mut confirm = String::new();
        std::io::stdin().read_line(&mut confirm)?;
        
        if confirm.trim().to_lowercase() == "n" {
            println!("❌ Connection setup cancelled");
            return Ok(());
        }

        // Test connection and save
        println!("\n🔄 Testing connection...");
        match self.connect_to_database(config.clone()).await {
            Ok(()) => {
                // Save the connection
                match self.config_manager.save_connection(&config) {
                    Ok(()) => {
                        println!("💾 Connection '{}' saved successfully!", name);
                        println!("💡 Use '/connect {}' to connect in the future", name);
                    }
                    Err(e) => {
                        println!("⚠️  Connected successfully but failed to save: {}", e);
                    }
                }
            }
            Err(e) => {
                println!("❌ Connection test failed: {}", e);
                print!("💾 Save connection anyway for later use? (y/N): ");
                std::io::stdout().flush().unwrap();
                let mut save_confirm = String::new();
                std::io::stdin().read_line(&mut save_confirm)?;
                
                if save_confirm.trim().to_lowercase().starts_with('y') {
                    match self.config_manager.save_connection(&config) {
                        Ok(()) => println!("💾 Connection '{}' saved (but not tested)", name),
                        Err(e) => println!("❌ Failed to save connection: {}", e),
                    }
                }
            }
        }

        Ok(())
    }

    fn prompt_for_input(&self, prompt: &str, default: Option<&str>) -> Result<String> {
        let default_text = if let Some(def) = default {
            format!(" [{}]", def)
        } else {
            String::new()
        };
        
        print!("📝 {}{}: ", prompt, default_text);
        std::io::stdout().flush().unwrap();
        
        let mut input = String::new();
        std::io::stdin().read_line(&mut input)?;
        Ok(input.trim().to_string())
    }

    fn prompt_for_password(&self, prompt: &str) -> Result<String> {
        print!("🔐 {}: ", prompt);
        std::io::stdout().flush().unwrap();
        
        // Use rpassword for secure password input (hidden)
        match rpassword::read_password() {
            Ok(password) => Ok(password),
            Err(e) => {
                println!("❌ Failed to read password: {}", e);
                Ok(String::new())
            }
        }
    }

    fn prompt_for_database_type(&self) -> Result<DatabaseType> {
        loop {
            println!("📊 Select database type:");
            println!("  1. MySQL");
            println!("  2. PostgreSQL");
            println!("  3. SQLite");
            print!("Enter choice (1-3): ");
            std::io::stdout().flush().unwrap();
            
            let mut input = String::new();
            std::io::stdin().read_line(&mut input)?;
            
            match input.trim() {
                "1" => return Ok(DatabaseType::MySQL),
                "2" => return Ok(DatabaseType::PostgreSQL),
                "3" => return Ok(DatabaseType::SQLite),
                _ => println!("❌ Invalid choice. Please enter 1, 2, or 3."),
            }
        }
    }

    fn get_default_port_for_type(&self, db_type: &DatabaseType) -> u16 {
        match db_type {
            DatabaseType::MySQL => 3306,
            DatabaseType::PostgreSQL => 5432,
            DatabaseType::SQLite => 0,
        }
    }

    fn parse_connection_string(&self, conn_str: &str) -> Result<ConnectionConfig> {
        // Simple parsing for mysql://user:pass@host:port/database format
        let re = regex::Regex::new(r"^(\w+)://([^:]+):([^@]+)@([^:]+):(\d+)/(.+)$")
            .map_err(|e| anyhow::anyhow!("Invalid regex pattern: {}", e))?;
            
        if let Some(captures) = re.captures(conn_str) {
            
            let db_type = match &captures[1] {
                "mysql" => DatabaseType::MySQL,
                "postgres" | "postgresql" => DatabaseType::PostgreSQL,
                "sqlite" => DatabaseType::SQLite,
                _ => return Err(anyhow::anyhow!("Unsupported database type")),
            };

            Ok(ConnectionConfig {
                name: format!("temp-{}", chrono::Utc::now().timestamp()),
                database_type: db_type,
                host: captures[4].to_string(),
                port: captures[5].parse()?,
                database: captures[6].to_string(),
                username: captures[2].to_string(),
                password: Some(captures[3].to_string()),
                ssl: false,
                ssh_tunnel: None,
            })
        } else {
            Err(anyhow::anyhow!("Invalid connection string format"))
        }
    }

    async fn connect_to_database(&mut self, config: ConnectionConfig) -> Result<()> {
        println!("🔌 Connecting to {}...", config.name);

        // Create connection based on database type
        let connection: Box<dyn DatabaseConnection> = match config.database_type {
            DatabaseType::MySQL => {
                sqlterm_mysql::MySqlConnection::connect(&config).await
                    .map_err(|e| anyhow::anyhow!("MySQL connection failed: {}", e))?
            }
            DatabaseType::PostgreSQL => {
                sqlterm_postgres::PostgresConnection::connect(&config).await
                    .map_err(|e| anyhow::anyhow!("PostgreSQL connection failed: {}", e))?
            }
            DatabaseType::SQLite => {
                sqlterm_sqlite::SqliteConnection::connect(&config).await
                    .map_err(|e| anyhow::anyhow!("SQLite connection failed: {}", e))?
            }
        };

        // Test the connection
        connection.ping().await?;

        // Get connection info
        let conn_info = connection.get_connection_info().await?;

        // Store the connection
        self.active_connection = Some(connection);
        self.current_database = Some(conn_info.database_name.clone());

        println!("✅ Connected to {} ({})", config.name, conn_info.database_name);

        // Auto-save connection if it's not temporary and not already saved
        if !config.name.starts_with("temp-") && !self.config_manager.connection_exists(&config.name) {
            if let Err(e) = self.auto_save_connection(&config).await {
                println!("⚠️  Warning: Failed to save connection: {}", e);
            }
        }

        Ok(())
    }

    async fn auto_save_connection(&self, config: &ConnectionConfig) -> Result<()> {
        if !self.config_manager.connection_exists(&config.name) {
            self.config_manager.save_connection(config)?;
            println!("💾 Connection '{}' saved", config.name);
        }
        Ok(())
    }

    async fn list_connections(&self) -> Result<()> {
        let (connections, errors) = self.config_manager.list_connections_with_errors()?;

        if !errors.is_empty() {
            for error in &errors {
                println!("⚠️  {}", error);
            }
        }

        if connections.is_empty() {
            println!("📭 No saved connections found.");
            println!("💡 Use /connect to create and save connections.");
        } else {
            println!("📋 Saved Connections:");
            for (i, conn) in connections.iter().enumerate() {
                let active = if let Some(current_db) = &self.current_database {
                    if conn.database == *current_db { " (active)" } else { "" }
                } else { "" };
                
                println!("  {}. {} - {}://{}:{}/{}{}",
                    i + 1,
                    conn.name,
                    conn.database_type.to_string().to_lowercase(),
                    conn.host,
                    conn.port,
                    conn.database,
                    active
                );
            }
        }
        Ok(())
    }

    async fn list_tables(&mut self) -> Result<()> {
        if let Some(connection) = &self.active_connection {
            println!("📊 Listing tables...");
            let tables = connection.list_tables().await?;
            
            if tables.is_empty() {
                println!("📭 No tables found in current database.");
                self.tables = vec![];
            } else {
                println!("📋 Tables in {}:", self.current_database.as_ref().unwrap_or(&"database".to_string()));
                for (i, table) in tables.iter().enumerate() {
                    println!("  {}. {}", i + 1, table);
                }
                
                // Update completer with table names for auto-completion
                self.tables = tables.clone();
                if let Some(helper) = self.editor.helper_mut() {
                    helper.completer.update_tables(tables);
                }
            }
        } else {
            println!("❌ No active connection. Use /connect first.");
        }
        Ok(())
    }

    async fn describe_table(&self, args: &[&str]) -> Result<()> {
        if args.is_empty() {
            println!("❓ Usage: /describe <table_name>");
            return Ok(());
        }

        if let Some(connection) = &self.active_connection {
            let table_name = args[0];
            println!("📊 Describing table '{}'...", table_name);
            
            let details = connection.get_table_details(table_name).await?;
            
            // Display table info
            println!("📋 Table: {}", details.table.name);
            println!("   Type: {:?}", details.table.table_type);
            println!("   Schema: {}", details.table.schema.as_deref().unwrap_or("default"));
            println!("   Rows: {}", details.statistics.row_count);
            
            // Display columns
            println!("\n📊 Columns:");
            println!("   {:<20} {:<15} {:<8} {:<8} {}", "Name", "Type", "Null", "Key", "Default");
            println!("   {}", "-".repeat(70));
            
            for col in &details.columns {
                let key_info = if col.is_primary_key {
                    "PRI"
                } else if col.is_foreign_key {
                    "FOR"
                } else if col.is_unique {
                    "UNI"
                } else {
                    ""
                };
                
                println!("   {:<20} {:<15} {:<8} {:<8} {}",
                    col.name,
                    col.data_type,
                    if col.nullable { "YES" } else { "NO" },
                    key_info,
                    col.default_value.as_deref().unwrap_or("")
                );
            }
            
            // Display indexes if any
            if !details.indexes.is_empty() {
                println!("\n🔍 Indexes:");
                for index in &details.indexes {
                    println!("   {} ({})", index.name, index.columns.join(", "));
                }
            }
            
        } else {
            println!("❌ No active connection. Use /connect first.");
        }
        Ok(())
    }

    async fn use_database(&mut self, args: &[&str]) -> Result<()> {
        if args.is_empty() {
            println!("❓ Usage: /use <database_name>");
            return Ok(());
        }

        println!("⚠️  Database switching not implemented yet.");
        println!("💡 Use /connect with a different database instead.");
        Ok(())
    }

    fn show_status(&self) {
        println!("📊 Connection Status:");
        if let Some(db) = &self.current_database {
            println!("  Connected to: {}", db);
            println!("  Status: ✅ Active");
        } else {
            println!("  Status: ❌ Not connected");
            println!("  💡 Use /connect to establish a connection");
        }
    }

    fn clear_screen(&self) {
        print!("\x1B[2J\x1B[1;1H");
        std::io::stdout().flush().unwrap();
    }

    async fn list_sessions(&self) -> Result<()> {
        match self.config_manager.list_sessions() {
            Ok(sessions) => {
                if sessions.is_empty() {
                    println!("📭 No saved sessions found.");
                    println!("💡 Use /save-session <name> to save your current conversation.");
                } else {
                    println!("📋 Saved Sessions:");
                    for (i, session) in sessions.iter().enumerate() {
                        println!("  {}. {}", i + 1, session);
                    }
                    println!();
                    println!("💡 Use /load-session <name> to restore a session");
                    println!("💡 Use /delete-session <name> to remove a session");
                }
            }
            Err(e) => {
                println!("❌ Failed to list sessions: {}", e);
            }
        }
        Ok(())
    }

    async fn save_session_command(&self, args: &[&str]) -> Result<()> {
        if args.is_empty() {
            println!("❓ Usage: /save-session <session_name>");
            println!("💡 Example: /save-session my-analysis-work");
            return Ok(());
        }

        let session_name = args[0];
        
        // For now, save a simple placeholder - in a full implementation,
        // you'd save the conversation history, current connection, etc.
        let session_content = format!(
            "# SQLTerm Session: {}\n# Saved at: {}\n# Connection: {}\n# Database: {}\n\n# This is a placeholder for session restoration\n",
            session_name,
            chrono::Utc::now().format("%Y-%m-%d %H:%M:%S UTC"),
            if self.active_connection.is_some() { "Active" } else { "None" },
            self.current_database.as_deref().unwrap_or("None")
        );

        match self.config_manager.save_session(session_name, &session_content) {
            Ok(()) => {
                println!("💾 Session '{}' saved successfully!", session_name);
                println!("📁 Location: ~/.config/sqlterm/sessions/{}.txt", session_name);
            }
            Err(e) => {
                println!("❌ Failed to save session: {}", e);
            }
        }
        Ok(())
    }

    async fn load_session_command(&self, args: &[&str]) -> Result<()> {
        if args.is_empty() {
            println!("❓ Usage: /load-session <session_name>");
            println!("💡 Use /list-sessions to see available sessions");
            return Ok(());
        }

        let session_name = args[0];
        
        match self.config_manager.load_session(session_name) {
            Ok(content) => {
                println!("📂 Loading session '{}'...", session_name);
                println!("📄 Session Content:");
                println!("{}", content);
                println!("✅ Session '{}' loaded", session_name);
                println!("💡 In a full implementation, this would restore:");
                println!("   • Conversation history");
                println!("   • Active connection state");
                println!("   • Query history");
            }
            Err(e) => {
                println!("❌ Failed to load session: {}", e);
                println!("💡 Use /list-sessions to see available sessions");
            }
        }
        Ok(())
    }

    async fn delete_session_command(&self, args: &[&str]) -> Result<()> {
        if args.is_empty() {
            println!("❓ Usage: /delete-session <session_name>");
            println!("💡 Use /list-sessions to see available sessions");
            return Ok(());
        }

        let session_name = args[0];
        
        print!("⚠️  Are you sure you want to delete session '{}'? (y/N): ", session_name);
        std::io::stdout().flush().unwrap();
        
        let mut confirm = String::new();
        std::io::stdin().read_line(&mut confirm)?;
        
        if !confirm.trim().to_lowercase().starts_with('y') {
            println!("❌ Session deletion cancelled");
            return Ok(());
        }

        match self.config_manager.delete_session(session_name) {
            Ok(()) => {
                println!("🗑️  Session '{}' deleted successfully", session_name);
            }
            Err(e) => {
                println!("❌ Failed to delete session: {}", e);
            }
        }
        Ok(())
    }


    async fn handle_file_reference(&mut self, input: &str) -> Result<()> {
        let input_without_at = &input[1..]; // Remove the '@' prefix
        
        // Parse file path and optional query range
        let (file_path, query_range) = if let Some(space_pos) = input_without_at.find(' ') {
            let file_part = &input_without_at[..space_pos];
            let range_part = &input_without_at[space_pos + 1..];
            (file_part, Some(range_part))
        } else {
            (input_without_at, None)
        };
        
        if !Path::new(file_path).exists() {
            return Err(anyhow::anyhow!("File not found: {}", file_path));
        }

        let content = fs::read_to_string(file_path)?;
        
        // Split into individual queries
        let all_queries = self.split_queries(&content);
        
        if all_queries.is_empty() {
            println!("📭 No queries found in file.");
            return Ok(());
        }

        // Determine which queries to execute based on range
        let (queries_to_execute, start_index) = if let Some(range_str) = query_range {
            self.parse_query_range(range_str, &all_queries)?
        } else {
            // Execute all queries
            (all_queries.clone(), 0)
        };

        if queries_to_execute.is_empty() {
            println!("📭 No queries found in specified range.");
            return Ok(());
        }

        if query_range.is_some() {
            println!("📁 Executing {} SQL file: {} (queries {})", 
                queries_to_execute.len(), file_path, query_range.unwrap());
        } else {
            println!("📁 Executing SQL file: {}", file_path);
        }
        println!("🔄 Found {} queries total, executing {} queries", all_queries.len(), queries_to_execute.len());
        
        // Execute selected queries
        for (i, query) in queries_to_execute.iter().enumerate() {
            let result_id = start_index + i + 1;
            let actual_query_num = start_index + i + 1;
            println!("\n📝 Query {} of {} (file query #{}):", 
                i + 1, queries_to_execute.len(), actual_query_num);
            println!("```sql");
            println!("{}", query.trim());
            println!("```");
            
            if let Err(e) = self.execute_single_query(query).await {
                println!("❌ Error in query {}: {}", result_id, e);
            }
        }

        Ok(())
    }

    fn parse_query_range(&self, range_str: &str, all_queries: &[String]) -> Result<(Vec<String>, usize)> {
        let total_queries = all_queries.len();
        
        if range_str.contains('-') {
            // Range format: "2-5" or "2-"
            let parts: Vec<&str> = range_str.split('-').collect();
            if parts.len() != 2 {
                return Err(anyhow::anyhow!("Invalid range format. Use: 2-5 or 2-"));
            }
            
            let start: usize = parts[0].parse()
                .map_err(|_| anyhow::anyhow!("Invalid start number in range"))?;
            
            if start == 0 || start > total_queries {
                return Err(anyhow::anyhow!("Start query number {} is out of range (1-{})", start, total_queries));
            }
            
            let end = if parts[1].is_empty() {
                // Format: "2-" means from 2 to end
                total_queries
            } else {
                let end_num: usize = parts[1].parse()
                    .map_err(|_| anyhow::anyhow!("Invalid end number in range"))?;
                if end_num == 0 || end_num > total_queries {
                    return Err(anyhow::anyhow!("End query number {} is out of range (1-{})", end_num, total_queries));
                }
                if end_num < start {
                    return Err(anyhow::anyhow!("End query number {} must be >= start query number {}", end_num, start));
                }
                end_num
            };
            
            let start_index = start - 1;
            let end_index = end;
            let selected_queries = all_queries[start_index..end_index].to_vec();
            Ok((selected_queries, start_index))
        } else {
            // Single query number: "2"
            let query_num: usize = range_str.parse()
                .map_err(|_| anyhow::anyhow!("Invalid query number"))?;
            
            if query_num == 0 || query_num > total_queries {
                return Err(anyhow::anyhow!("Query number {} is out of range (1-{})", query_num, total_queries));
            }
            
            let start_index = query_num - 1;
            let selected_query = vec![all_queries[start_index].clone()];
            Ok((selected_query, start_index))
        }
    }

    fn split_queries(&self, content: &str) -> Vec<String> {
        content
            .split(';')
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty() && !s.chars().all(|c| c.is_whitespace() || c == '\n'))
            .collect()
    }

    async fn execute_query(&mut self, query: &str) -> Result<()> {
        if query.trim().is_empty() {
            return Ok(());
        }

        // Check if query contains multiple statements
        let queries = self.split_queries(query);
        
        if queries.len() > 1 {
            println!("🔄 Executing {} queries...", queries.len());
            for (i, q) in queries.iter().enumerate() {
                println!("\n📝 Query {} of {}:", i + 1, queries.len());
                println!("```sql");
                println!("{}", q.trim());
                println!("```");
                
                if let Err(e) = self.execute_single_query(q).await {
                    println!("❌ Error in query {}: {}", i + 1, e);
                }
            }
        } else {
            self.execute_single_query(query).await?;
        }

        Ok(())
    }

    async fn execute_single_query(&self, query: &str) -> Result<()> {
        if let Some(connection) = &self.active_connection {
            let result = connection.execute_query(query).await?;
            
            if result.columns.is_empty() {
                println!("✅ Query executed successfully ({} rows affected)", result.total_rows);
            } else {
                println!("📊 Query Results ({} rows):", result.total_rows);
                
                // Display results as table
                self.display_table_results(&result);
            }
        } else {
            println!("❌ No active connection. Use /connect first.");
        }
        Ok(())
    }

    fn display_table_results(&self, result: &sqlterm_core::QueryResult) {
        if result.rows.is_empty() {
            println!("📭 No rows returned.");
            return;
        }

        // Calculate column widths
        let mut col_widths: Vec<usize> = result.columns.iter()
            .map(|col| col.name.len())
            .collect();

        for row in &result.rows {
            for (i, value) in row.values.iter().enumerate() {
                let value_str = value.to_string();
                if i < col_widths.len() {
                    col_widths[i] = col_widths[i].max(value_str.len());
                }
            }
        }

        // Limit column widths for readability
        for width in &mut col_widths {
            *width = (*width).min(30);
        }

        // Print header
        print!("│");
        for (i, col) in result.columns.iter().enumerate() {
            print!(" {:<width$} │", col.name, width = col_widths[i]);
        }
        println!();

        // Print separator
        print!("├");
        for width in &col_widths {
            print!("{}", "─".repeat(width + 2));
            print!("┼");
        }
        println!();

        // Print rows (limit to first 50 for readability)
        for (row_idx, row) in result.rows.iter().enumerate() {
            if row_idx >= 50 {
                println!("│ ... and {} more rows (use LIMIT to see specific ranges)", result.total_rows - 50);
                break;
            }

            print!("│");
            for (i, value) in row.values.iter().enumerate() {
                let value_str = value.to_string();
                let truncated = if value_str.len() > 30 {
                    format!("{}...", &value_str[..27])
                } else {
                    value_str
                };
                print!(" {:<width$} │", truncated, width = col_widths[i]);
            }
            println!();
        }

        // Print footer
        print!("└");
        for width in &col_widths {
            print!("{}", "─".repeat(width + 2));
            print!("┴");
        }
        println!();
    }

    // Clipboard support methods
    async fn copy_result_command(&self, args: &[&str]) -> Result<()> {
        if args.is_empty() {
            println!("❓ Usage: /copy-result <result_id>");
            println!("💡 Use /toggle <id> to see result IDs");
            return Ok(());
        }

        let result_id: usize = args[0].parse()
            .map_err(|_| anyhow::anyhow!("Invalid result ID"))?;
        
        // For now, just show a placeholder message
        // In a full implementation, you'd store query results and copy them
        println!("📋 Result {} would be copied to clipboard (feature placeholder)", result_id);
        println!("💡 This will copy the full query result as CSV/TSV format");
        
        Ok(())
    }

    async fn copy_query_command(&self) -> Result<()> {
        // Get the last command from history
        if let Some(last_entry) = self.editor.history().iter().last() {
            match self.copy_to_clipboard(last_entry) {
                Ok(()) => {
                    println!("📋 Last query copied to clipboard: {}", 
                        if last_entry.len() > 50 { 
                            format!("{}...", &last_entry[..47]) 
                        } else { 
                            last_entry.to_string() 
                        }
                    );
                }
                Err(e) => {
                    println!("❌ Failed to copy to clipboard: {}", e);
                }
            }
        } else {
            println!("📭 No query in history to copy");
        }
        Ok(())
    }

    async fn paste_command(&self) -> Result<()> {
        match self.paste_from_clipboard() {
            Ok(content) => {
                println!("📋 Clipboard content:");
                println!("```");
                println!("{}", content);
                println!("```");
                println!("💡 You can now edit and execute this content");
            }
            Err(e) => {
                println!("❌ Failed to paste from clipboard: {}", e);
            }
        }
        Ok(())
    }

    fn copy_to_clipboard(&self, text: &str) -> Result<()> {
        use arboard::Clipboard;
        let mut clipboard = Clipboard::new()
            .map_err(|e| anyhow::anyhow!("Failed to access clipboard: {}", e))?;
        clipboard.set_text(text)
            .map_err(|e| anyhow::anyhow!("Failed to set clipboard text: {}", e))?;
        Ok(())
    }

    fn paste_from_clipboard(&self) -> Result<String> {
        use arboard::Clipboard;
        let mut clipboard = Clipboard::new()
            .map_err(|e| anyhow::anyhow!("Failed to access clipboard: {}", e))?;
        let text = clipboard.get_text()
            .map_err(|e| anyhow::anyhow!("Failed to get clipboard text: {}", e))?;
        Ok(text)
    }
}