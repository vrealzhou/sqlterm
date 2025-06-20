use sqlterm_core::{ConnectionConfig, DatabaseConnection, TableDetails};

#[derive(Debug, Clone, PartialEq)]
pub enum AppState {
    ConnectionManager,
    DatabaseBrowser,
    QueryEditor,
    Results,
}

#[derive(Debug, Clone, PartialEq)]
pub enum InputMode {
    Normal,
    Editing,
}

pub struct App {
    pub state: AppState,
    pub input_mode: InputMode,
    pub should_quit: bool,
    pub connections: Vec<ConnectionConfig>,
    pub active_connection: Option<Box<dyn DatabaseConnection>>,
    pub selected_connection: usize,
    pub current_database: Option<String>,
    pub tables: Vec<String>,
    pub selected_table: usize,
    pub query_input: String,
    pub query_results: Option<sqlterm_core::QueryResult>,
    pub table_details: Option<TableDetails>,
    pub error_message: Option<String>,
    pub cursor_position: usize,
}

impl Default for App {
    fn default() -> Self {
        Self::new()
    }
}

impl App {
    pub fn new() -> Self {
        Self {
            state: AppState::ConnectionManager,
            input_mode: InputMode::Normal,
            should_quit: false,
            connections: Vec::new(),
            active_connection: None,
            selected_connection: 0,
            current_database: None,
            tables: Vec::new(),
            selected_table: 0,
            query_input: String::new(),
            query_results: None,
            table_details: None,
            error_message: None,
            cursor_position: 0,
        }
    }

    pub fn quit(&mut self) {
        self.should_quit = true;
    }

    pub fn switch_to_connection_manager(&mut self) {
        self.state = AppState::ConnectionManager;
        self.input_mode = InputMode::Normal;
    }

    pub fn switch_to_database_browser(&mut self) {
        self.state = AppState::DatabaseBrowser;
        self.input_mode = InputMode::Normal;
    }

    pub fn switch_to_query_editor(&mut self) {
        self.state = AppState::QueryEditor;
        self.input_mode = InputMode::Normal;
    }

    pub fn switch_to_results(&mut self) {
        self.state = AppState::Results;
        self.input_mode = InputMode::Normal;
    }

    pub fn enter_edit_mode(&mut self) {
        self.input_mode = InputMode::Editing;
    }

    pub fn exit_edit_mode(&mut self) {
        self.input_mode = InputMode::Normal;
    }

    pub fn add_connection(&mut self, config: ConnectionConfig) {
        self.connections.push(config);
    }

    pub fn select_next_connection(&mut self) {
        if !self.connections.is_empty() {
            self.selected_connection = (self.selected_connection + 1) % self.connections.len();
        }
    }

    pub fn select_previous_connection(&mut self) {
        if !self.connections.is_empty() {
            self.selected_connection = if self.selected_connection == 0 {
                self.connections.len() - 1
            } else {
                self.selected_connection - 1
            };
        }
    }

    pub fn select_next_table(&mut self) {
        if !self.tables.is_empty() {
            self.selected_table = (self.selected_table + 1) % self.tables.len();
        }
    }

    pub fn select_previous_table(&mut self) {
        if !self.tables.is_empty() {
            self.selected_table = if self.selected_table == 0 {
                self.tables.len() - 1
            } else {
                self.selected_table - 1
            };
        }
    }

    pub fn set_error(&mut self, message: String) {
        self.error_message = Some(message);
    }

    pub fn clear_error(&mut self) {
        self.error_message = None;
    }

    pub fn get_selected_connection(&self) -> Option<&ConnectionConfig> {
        self.connections.get(self.selected_connection)
    }

    pub fn get_selected_table(&self) -> Option<&String> {
        self.tables.get(self.selected_table)
    }

    pub fn set_table_details(&mut self, details: TableDetails) {
        self.table_details = Some(details);
    }

    pub fn clear_table_details(&mut self) {
        self.table_details = None;
    }

    pub fn get_table_details(&self) -> Option<&TableDetails> {
        self.table_details.as_ref()
    }

    pub fn set_query_results(&mut self, results: sqlterm_core::QueryResult) {
        self.query_results = Some(results);
    }

    pub fn clear_query_results(&mut self) {
        self.query_results = None;
    }

    pub fn get_query_results(&self) -> Option<&sqlterm_core::QueryResult> {
        self.query_results.as_ref()
    }
}
