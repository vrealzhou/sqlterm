use sqlterm_core::{ConnectionConfig, DatabaseConnection, TableDetails, DatabaseType};
use std::collections::VecDeque;
use rustyline::completion::{Pair, Completer};
use rustyline::history::MemHistory;
use rustyline::Context;

#[derive(Debug, Clone)]
pub struct ConnectionForm {
    pub name: String,
    pub database_type: DatabaseType,
    pub host: String,
    pub port: String,
    pub database: String,
    pub username: String,
    pub password: String,
    pub selected_field: usize,
    pub is_active: bool,
}

impl Default for ConnectionForm {
    fn default() -> Self {
        Self {
            name: String::new(),
            database_type: DatabaseType::SQLite,
            host: "localhost".to_string(),
            port: "0".to_string(),
            database: ":memory:".to_string(),
            username: String::new(),
            password: String::new(),
            selected_field: 0,
            is_active: false,
        }
    }
}

#[derive(Debug, Clone, PartialEq)]
pub enum AppState {
    ConnectionManager,
    DatabaseBrowser,
    QueryEditor,
    Results,
    AddConnection,
    Conversation, // New Claude Code-style conversation mode
}

#[derive(Debug, Clone, PartialEq)]
pub enum InputMode {
    Normal,
    Editing,
    Visual,
}

#[derive(Debug, Clone)]
pub struct LogEntry {
    pub timestamp: String,
    pub level: String,
    pub message: String,
}

#[derive(Debug, Clone)]
pub struct ConversationEntry {
    pub timestamp: String,
    pub input: String,
    pub output: Option<String>,
    pub is_query: bool,
    pub results: Option<sqlterm_core::QueryResult>,
}

#[derive(Clone)]
pub struct CompletionState {
    pub completions: Vec<Pair>,
    pub selected_index: usize,
    pub is_visible: bool,
    pub trigger_pos: usize,
}

#[derive(Debug, Clone)]
pub struct QueryEditor {
    pub content: Vec<String>,
    pub cursor_line: usize,
    pub cursor_col: usize,
    pub scroll_offset: usize,
    pub visual_start: Option<(usize, usize)>,
    pub visual_end: Option<(usize, usize)>,
    pub show_logs: bool,
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
    // Connection form fields
    pub connection_form: ConnectionForm,
    // Enhanced query editor
    pub query_editor: QueryEditor,
    // Logs
    pub logs: VecDeque<LogEntry>,
    pub max_logs: usize,
    pub show_logs: bool,
    // Conversation mode fields
    pub conversation_input: String,
    pub conversation_cursor: usize,
    pub conversation_history: VecDeque<ConversationEntry>,
    pub completion_state: CompletionState,
    pub scroll_offset: usize,
}

impl Default for App {
    fn default() -> Self {
        Self::new()
    }
}

impl Default for QueryEditor {
    fn default() -> Self {
        Self {
            content: vec![String::new()],
            cursor_line: 0,
            cursor_col: 0,
            scroll_offset: 0,
            visual_start: None,
            visual_end: None,
            show_logs: false,
        }
    }
}

impl App {
    pub fn new() -> Self {
        Self {
            state: AppState::Conversation, // Start in conversation mode
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
            connection_form: ConnectionForm::default(),
            query_editor: QueryEditor::default(),
            logs: VecDeque::new(),
            max_logs: 1000,
            show_logs: false,
            conversation_input: String::new(),
            conversation_cursor: 0,
            conversation_history: VecDeque::new(),
            completion_state: CompletionState {
                completions: Vec::new(),
                selected_index: 0,
                is_visible: false,
                trigger_pos: 0,
            },
            scroll_offset: 0,
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

    pub fn switch_to_add_connection(&mut self) {
        self.state = AppState::AddConnection;
        self.input_mode = InputMode::Editing;
        self.connection_form.is_active = true;
        self.connection_form.selected_field = 0;
    }

    pub fn switch_to_conversation(&mut self) {
        self.state = AppState::Conversation;
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

    // Logging methods
    pub fn add_log(&mut self, level: &str, message: &str) {
        use chrono::Utc;
        let timestamp = Utc::now().format("%H:%M:%S%.3f").to_string();
        let entry = LogEntry {
            timestamp,
            level: level.to_string(),
            message: message.to_string(),
        };
        
        self.logs.push_back(entry);
        if self.logs.len() > self.max_logs {
            self.logs.pop_front();
        }
    }

    pub fn toggle_logs(&mut self) {
        self.show_logs = !self.show_logs;
    }

    // Query editor methods
    pub fn get_current_query(&self) -> String {
        self.query_editor.content.join("\n")
    }

    pub fn get_selected_query(&self) -> Option<String> {
        if let (Some(start), Some(end)) = (self.query_editor.visual_start, self.query_editor.visual_end) {
            let start_line = start.0.min(end.0);
            let end_line = start.0.max(end.0);
            let start_col = if start.0 == start_line { start.1 } else { end.1 };
            let end_col = if start.0 == start_line { end.1 } else { start.1 };

            if start_line == end_line {
                // Single line selection
                if let Some(line) = self.query_editor.content.get(start_line) {
                    let start_col = start_col.min(line.len());
                    let end_col = end_col.min(line.len());
                    if start_col < end_col {
                        return Some(line[start_col..end_col].to_string());
                    }
                }
            } else {
                // Multi-line selection
                let mut selected = String::new();
                for line_idx in start_line..=end_line {
                    if let Some(line) = self.query_editor.content.get(line_idx) {
                        if line_idx == start_line {
                            let start_col = start_col.min(line.len());
                            selected.push_str(&line[start_col..]);
                        } else if line_idx == end_line {
                            let end_col = end_col.min(line.len());
                            selected.push_str(&line[..end_col]);
                        } else {
                            selected.push_str(line);
                        }
                        if line_idx < end_line {
                            selected.push('\n');
                        }
                    }
                }
                return Some(selected);
            }
        }
        None
    }

    pub fn enter_visual_mode(&mut self) {
        self.input_mode = InputMode::Visual;
        self.query_editor.visual_start = Some((self.query_editor.cursor_line, self.query_editor.cursor_col));
        self.query_editor.visual_end = Some((self.query_editor.cursor_line, self.query_editor.cursor_col));
        self.add_log("DEBUG", &format!("Entered visual mode at line:{}, col:{}", self.query_editor.cursor_line, self.query_editor.cursor_col));
    }

    pub fn exit_visual_mode(&mut self) {
        if self.input_mode == InputMode::Visual {
            self.input_mode = InputMode::Normal;
            self.query_editor.visual_start = None;
            self.query_editor.visual_end = None;
        }
    }

    pub fn update_visual_selection(&mut self) {
        if self.input_mode == InputMode::Visual {
            self.query_editor.visual_end = Some((self.query_editor.cursor_line, self.query_editor.cursor_col));
            if let Some(start) = self.query_editor.visual_start {
                self.add_log("DEBUG", &format!("Visual selection: ({},{}) to ({},{})", 
                    start.0, start.1, self.query_editor.cursor_line, self.query_editor.cursor_col));
            }
        }
    }

    // Vim-like cursor movement
    pub fn move_cursor_left(&mut self) {
        if self.query_editor.cursor_col > 0 {
            self.query_editor.cursor_col -= 1;
        } else if self.query_editor.cursor_line > 0 {
            self.query_editor.cursor_line -= 1;
            if let Some(line) = self.query_editor.content.get(self.query_editor.cursor_line) {
                self.query_editor.cursor_col = line.len();
            }
        }
        self.update_visual_selection();
    }

    pub fn move_cursor_right(&mut self) {
        if let Some(current_line) = self.query_editor.content.get(self.query_editor.cursor_line) {
            if self.query_editor.cursor_col < current_line.len() {
                self.query_editor.cursor_col += 1;
            } else if self.query_editor.cursor_line < self.query_editor.content.len() - 1 {
                self.query_editor.cursor_line += 1;
                self.query_editor.cursor_col = 0;
            }
        }
        self.update_visual_selection();
    }

    pub fn move_cursor_up(&mut self) {
        if self.query_editor.cursor_line > 0 {
            self.query_editor.cursor_line -= 1;
            if let Some(line) = self.query_editor.content.get(self.query_editor.cursor_line) {
                self.query_editor.cursor_col = self.query_editor.cursor_col.min(line.len());
            }
        }
        self.update_visual_selection();
    }

    pub fn move_cursor_down(&mut self) {
        if self.query_editor.cursor_line < self.query_editor.content.len() - 1 {
            self.query_editor.cursor_line += 1;
            if let Some(line) = self.query_editor.content.get(self.query_editor.cursor_line) {
                self.query_editor.cursor_col = self.query_editor.cursor_col.min(line.len());
            }
        }
        self.update_visual_selection();
    }

    pub fn move_to_line_start(&mut self) {
        self.query_editor.cursor_col = 0;
        self.update_visual_selection();
    }

    pub fn move_to_line_end(&mut self) {
        if let Some(line) = self.query_editor.content.get(self.query_editor.cursor_line) {
            self.query_editor.cursor_col = line.len();
        }
        self.update_visual_selection();
    }

    // Text editing methods
    pub fn insert_char(&mut self, c: char) {
        if let Some(line) = self.query_editor.content.get_mut(self.query_editor.cursor_line) {
            line.insert(self.query_editor.cursor_col, c);
            self.query_editor.cursor_col += 1;
        }
    }

    pub fn insert_newline(&mut self) {
        if let Some(current_line) = self.query_editor.content.get_mut(self.query_editor.cursor_line) {
            let new_line = current_line.split_off(self.query_editor.cursor_col);
            self.query_editor.content.insert(self.query_editor.cursor_line + 1, new_line);
            self.query_editor.cursor_line += 1;
            self.query_editor.cursor_col = 0;
        }
    }

    pub fn delete_char(&mut self) {
        if let Some(line) = self.query_editor.content.get_mut(self.query_editor.cursor_line) {
            if self.query_editor.cursor_col > 0 {
                line.remove(self.query_editor.cursor_col - 1);
                self.query_editor.cursor_col -= 1;
            } else if self.query_editor.cursor_line > 0 {
                // Join with previous line
                let current_line = self.query_editor.content.remove(self.query_editor.cursor_line);
                self.query_editor.cursor_line -= 1;
                if let Some(prev_line) = self.query_editor.content.get_mut(self.query_editor.cursor_line) {
                    self.query_editor.cursor_col = prev_line.len();
                    prev_line.push_str(&current_line);
                }
            }
        }
    }

    pub fn copy_to_clipboard(&self) -> Result<(), Box<dyn std::error::Error>> {
        let text = if let Some(selected) = self.get_selected_query() {
            selected
        } else {
            self.get_current_query()
        };
        
        // Use arboard for cross-platform clipboard support
        use arboard::Clipboard;
        let mut clipboard = Clipboard::new()?;
        clipboard.set_text(text)?;
        Ok(())
    }

    pub fn paste_from_clipboard(&mut self) -> Result<(), Box<dyn std::error::Error>> {
        use arboard::Clipboard;
        let mut clipboard = Clipboard::new()?;
        let text = clipboard.get_text()?;
        
        for c in text.chars() {
            if c == '\n' {
                self.insert_newline();
            } else {
                self.insert_char(c);
            }
        }
        Ok(())
    }

    // Conversation mode methods
    pub fn conversation_insert_char(&mut self, c: char) {
        self.conversation_input.insert(self.conversation_cursor, c);
        self.conversation_cursor += 1;
        self.update_completions();
    }

    pub fn conversation_delete_char(&mut self) {
        if self.conversation_cursor > 0 {
            self.conversation_input.remove(self.conversation_cursor - 1);
            self.conversation_cursor -= 1;
            self.update_completions();
        }
    }

    pub fn conversation_move_cursor_left(&mut self) {
        if self.conversation_cursor > 0 {
            self.conversation_cursor -= 1;
            self.update_completions();
        }
    }

    pub fn conversation_move_cursor_right(&mut self) {
        if self.conversation_cursor < self.conversation_input.len() {
            self.conversation_cursor += 1;
            self.update_completions();
        }
    }

    pub fn conversation_move_cursor_to_start(&mut self) {
        self.conversation_cursor = 0;
        self.update_completions();
    }

    pub fn conversation_move_cursor_to_end(&mut self) {
        self.conversation_cursor = self.conversation_input.len();
        self.update_completions();
    }

    pub fn conversation_clear_input(&mut self) {
        self.conversation_input.clear();
        self.conversation_cursor = 0;
        self.hide_completions();
    }

    pub fn add_conversation_entry(&mut self, input: String, output: Option<String>, is_query: bool, results: Option<sqlterm_core::QueryResult>) {
        use chrono::Utc;
        let timestamp = Utc::now().format("%H:%M:%S").to_string();
        
        let entry = ConversationEntry {
            timestamp,
            input,
            output,
            is_query,
            results,
        };
        
        self.conversation_history.push_back(entry);
        if self.conversation_history.len() > 100 {
            self.conversation_history.pop_front();
        }
    }

    // Completion methods
    pub fn update_completions(&mut self) {
        // Import the completion logic from conversation.rs
        if let Ok(completer) = crate::conversation::create_completer(&self.tables) {
            let history = MemHistory::new();
            let ctx = Context::new(&history);
            if let Ok((trigger_pos, completions)) = completer.complete(&self.conversation_input, self.conversation_cursor, &ctx) {
                self.completion_state.completions = completions;
                self.completion_state.trigger_pos = trigger_pos;
                self.completion_state.selected_index = 0;
                self.completion_state.is_visible = !self.completion_state.completions.is_empty();
            } else {
                self.hide_completions();
            }
        } else {
            // If completer creation fails, hide completions
            self.hide_completions();
        }
    }

    pub fn hide_completions(&mut self) {
        self.completion_state.is_visible = false;
        self.completion_state.completions.clear();
        self.completion_state.selected_index = 0;
    }

    pub fn select_next_completion(&mut self) {
        if self.completion_state.is_visible && !self.completion_state.completions.is_empty() {
            self.completion_state.selected_index = 
                (self.completion_state.selected_index + 1) % self.completion_state.completions.len();
        }
    }

    pub fn select_previous_completion(&mut self) {
        if self.completion_state.is_visible && !self.completion_state.completions.is_empty() {
            self.completion_state.selected_index = if self.completion_state.selected_index == 0 {
                self.completion_state.completions.len() - 1
            } else {
                self.completion_state.selected_index - 1
            };
        }
    }

    pub fn apply_selected_completion(&mut self) -> bool {
        if self.completion_state.is_visible && 
           self.completion_state.selected_index < self.completion_state.completions.len() {
            
            let completion = &self.completion_state.completions[self.completion_state.selected_index];
            
            // Replace from trigger position to cursor
            let before = &self.conversation_input[..self.completion_state.trigger_pos];
            let after = &self.conversation_input[self.conversation_cursor..];
            
            self.conversation_input = format!("{}{}{}", before, completion.replacement, after);
            self.conversation_cursor = self.completion_state.trigger_pos + completion.replacement.len();
            
            self.hide_completions();
            return true;
        }
        false
    }

    pub fn get_selected_completion(&self) -> Option<&Pair> {
        if self.completion_state.is_visible && 
           self.completion_state.selected_index < self.completion_state.completions.len() {
            Some(&self.completion_state.completions[self.completion_state.selected_index])
        } else {
            None
        }
    }
}
