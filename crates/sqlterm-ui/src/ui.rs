use crate::app::{App, AppState, InputMode};
use ratatui::{
    layout::{Alignment, Constraint, Direction, Layout},
    style::{Color, Modifier, Style},
    widgets::{
        Block, Borders, Clear, List, ListItem, Paragraph, Table, Row, Cell,
        Wrap,
    },
    Frame,
};

pub fn render(f: &mut Frame, app: &App) {
    let main_chunks = if app.show_logs {
        Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Length(3),  // Header
                Constraint::Min(0),     // Main content
                Constraint::Length(6),  // Log panel
                Constraint::Length(3),  // Footer
            ])
            .split(f.size())
    } else {
        Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Length(3), // Header
                Constraint::Min(0),    // Main content
                Constraint::Length(3), // Footer
            ])
            .split(f.size())
    };

    // Render header
    render_header(f, main_chunks[0], app);

    // Render main content based on current state
    let content_area = main_chunks[1];
    match app.state {
        AppState::ConnectionManager => render_connection_manager(f, content_area, app),
        AppState::DatabaseBrowser => render_database_browser(f, content_area, app),
        AppState::QueryEditor => render_query_editor(f, content_area, app),
        AppState::Results => render_results(f, content_area, app),
        AppState::AddConnection => render_connection_manager(f, content_area, app), // Render base, then popup
    }

    // Render logs panel if enabled
    if app.show_logs {
        render_logs_panel(f, main_chunks[2], app);
        render_footer(f, main_chunks[3], app);
    } else {
        render_footer(f, main_chunks[2], app);
    }

    // Render add connection popup if in add connection state
    if app.state == AppState::AddConnection {
        render_add_connection_popup(f, app);
    }

    // Render error popup if there's an error
    if app.error_message.is_some() {
        render_error_popup(f, app);
    }
}

fn render_header(f: &mut Frame, area: ratatui::layout::Rect, app: &App) {
    let title = match app.state {
        AppState::ConnectionManager => "Connection Manager",
        AppState::DatabaseBrowser => "Database Browser",
        AppState::QueryEditor => "Query Editor",
        AppState::Results => "Query Results",
        AppState::AddConnection => "Connection Manager",
    };

    let header = Paragraph::new(format!("SQLTerm - {}", title))
        .style(Style::default().fg(Color::Cyan).add_modifier(Modifier::BOLD))
        .alignment(Alignment::Center)
        .block(Block::default().borders(Borders::ALL));

    f.render_widget(header, area);
}

fn render_footer(f: &mut Frame, area: ratatui::layout::Rect, app: &App) {
    let help_text = match app.state {
        AppState::ConnectionManager => {
            "↑/↓: Navigate | Enter: Connect | a: Add | d: Delete | e: Query Editor | q/Esc/Ctrl+C: Quit"
        }
        AppState::DatabaseBrowser => {
            "↑/↓: Navigate | Enter: Show Details | d: Describe | e: Query Editor | c: Connections | q/Esc: Quit"
        }
        AppState::QueryEditor => {
            match app.input_mode {
                InputMode::Normal => "i: Insert | v: Visual | hjkl: Move | r: Execute | L: Logs | y: Copy | p: Paste",
                InputMode::Editing => "Esc: Normal mode | Arrow keys: Move | Enter: New line",
                InputMode::Visual => "hjkl: Move | y: Copy selection | Esc: Normal",
            }
        }
        AppState::Results => {
            "s: Export to file | f: Show full results | c: Copy | e: Query Editor | b: Browser | q: Quit"
        }
        AppState::AddConnection => {
            "Tab/↓: Next field | Shift+Tab/↑: Previous field | Enter: Connect | Esc: Cancel"
        }
    };

    let footer = Paragraph::new(help_text)
        .style(Style::default().fg(Color::Gray))
        .alignment(Alignment::Center)
        .block(Block::default().borders(Borders::ALL));

    f.render_widget(footer, area);
}

fn render_connection_manager(f: &mut Frame, area: ratatui::layout::Rect, app: &App) {
    let items: Vec<ListItem> = app
        .connections
        .iter()
        .enumerate()
        .map(|(i, conn)| {
            let style = if i == app.selected_connection {
                Style::default().bg(Color::Blue).fg(Color::White)
            } else {
                Style::default()
            };

            ListItem::new(format!(
                "{} ({}://{}:{})",
                conn.name, conn.database_type, conn.host, conn.port
            ))
            .style(style)
        })
        .collect();

    let list = List::new(items)
        .block(Block::default().title("Connections").borders(Borders::ALL))
        .highlight_style(Style::default().add_modifier(Modifier::BOLD));

    f.render_widget(list, area);
}

fn render_database_browser(f: &mut Frame, area: ratatui::layout::Rect, app: &App) {
    let chunks = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([Constraint::Percentage(30), Constraint::Percentage(70)])
        .split(area);

    // Tables list
    let items: Vec<ListItem> = app
        .tables
        .iter()
        .enumerate()
        .map(|(i, table)| {
            let style = if i == app.selected_table {
                Style::default().bg(Color::Blue).fg(Color::White)
            } else {
                Style::default()
            };

            ListItem::new(table.clone()).style(style)
        })
        .collect();

    let tables_list = List::new(items)
        .block(Block::default().title("Tables").borders(Borders::ALL))
        .highlight_style(Style::default().add_modifier(Modifier::BOLD));

    f.render_widget(tables_list, chunks[0]);

    // Table details
    render_table_details(f, chunks[1], app);
}

fn render_table_details(f: &mut Frame, area: ratatui::layout::Rect, app: &App) {
    if let Some(table_details) = app.get_table_details() {
        // Split the details area into sections
        let chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Length(6),  // Table info
                Constraint::Min(10),    // Columns
                Constraint::Length(6),  // Statistics
            ])
            .split(area);

        // Table information
        render_table_info(f, chunks[0], table_details);

        // Columns details
        render_table_columns(f, chunks[1], table_details);

        // Statistics
        render_table_statistics(f, chunks[2], table_details);
    } else {
        let placeholder = Paragraph::new("Select a table and press Enter to view details\n\nAvailable actions:\n• Enter: Load table details\n• ↑/↓: Navigate tables\n• e: Query Editor\n• c: Connections")
            .block(Block::default().title("Table Details").borders(Borders::ALL))
            .wrap(Wrap { trim: true });

        f.render_widget(placeholder, area);
    }
}

fn render_query_editor(f: &mut Frame, area: ratatui::layout::Rect, app: &App) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([Constraint::Percentage(70), Constraint::Percentage(30)])
        .split(area);

    // Enhanced query input with cursor and selection support
    let input_style = match app.input_mode {
        InputMode::Editing => Style::default().fg(Color::Yellow),
        InputMode::Visual => Style::default().fg(Color::Cyan),
        InputMode::Normal => Style::default(),
    };

    let title = match app.input_mode {
        InputMode::Normal => "SQL Query (Press 'i' to edit, 'v' for visual mode)",
        InputMode::Editing => "SQL Query (Editing - Press Esc to exit)",
        InputMode::Visual => "SQL Query (Visual mode - Press Esc to exit)",
    };

    // Prepare content with cursor and selection highlighting
    let query_content = render_editor_content(app);
    
    let query_input = Paragraph::new(query_content)
        .style(input_style)
        .block(Block::default().title(title).borders(Borders::ALL))
        .wrap(Wrap { trim: false })
        .scroll((app.query_editor.scroll_offset as u16, 0));

    f.render_widget(query_input, chunks[0]);

    // Show cursor position if in query editor
    if app.state == AppState::QueryEditor {
        let cursor_info = format!(
            "Line: {}, Col: {} | Mode: {:?} | r: Execute | L: Toggle Logs", 
            app.query_editor.cursor_line + 1, 
            app.query_editor.cursor_col + 1,
            app.input_mode
        );
        let help = Paragraph::new(cursor_info)
            .block(Block::default().title("Status").borders(Borders::ALL));
        f.render_widget(help, chunks[1]);
    } else {
        let help = Paragraph::new("Query history will appear here")
            .block(Block::default().title("History").borders(Borders::ALL));
        f.render_widget(help, chunks[1]);
    }
}

fn render_editor_content(app: &App) -> String {
    use crate::app::InputMode;
    
    let mut content = String::new();
    
    for (line_idx, line) in app.query_editor.content.iter().enumerate() {
        if line_idx > 0 {
            content.push('\n');
        }
        
        let is_cursor_line = line_idx == app.query_editor.cursor_line;
        let cursor_col = app.query_editor.cursor_col;
        
        if app.input_mode == InputMode::Visual {
            // Render with selection highlighting
            if let (Some(start), Some(end)) = (app.query_editor.visual_start, app.query_editor.visual_end) {
                // Calculate selection bounds properly
                let (selection_start_line, selection_start_col, selection_end_line, selection_end_col) = 
                    if start.0 < end.0 || (start.0 == end.0 && start.1 <= end.1) {
                        (start.0, start.1, end.0, end.1)
                    } else {
                        (end.0, end.1, start.0, start.1)
                    };
                
                if line_idx >= selection_start_line && line_idx <= selection_end_line {
                    let start_col = if line_idx == selection_start_line { 
                        selection_start_col.min(line.len()) 
                    } else { 
                        0 
                    };
                    let end_col = if line_idx == selection_end_line { 
                        selection_end_col.min(line.len()) 
                    } else { 
                        line.len() 
                    };
                    
                    render_line_with_cursor(&mut content, line, is_cursor_line, cursor_col, Some((start_col, end_col)));
                } else {
                    render_line_with_cursor(&mut content, line, is_cursor_line, cursor_col, None);
                }
            } else {
                render_line_with_cursor(&mut content, line, is_cursor_line, cursor_col, None);
            }
        } else {
            render_line_with_cursor(&mut content, line, is_cursor_line, cursor_col, None);
        }
    }
    
    content
}

fn render_line_with_cursor(content: &mut String, line: &str, is_cursor_line: bool, cursor_col: usize, selection: Option<(usize, usize)>) {
    if let Some((start_col, end_col)) = selection {
        // Render line with selection
        if start_col > 0 {
            let pre_selection = &line[..start_col];
            if is_cursor_line && cursor_col <= start_col {
                insert_cursor_in_text(content, pre_selection, cursor_col);
            } else {
                content.push_str(pre_selection);
            }
        }
        
        // Add selected part with visual markers
        if start_col < end_col {
            content.push_str("▶");
            let selected_text = &line[start_col..end_col];
            if is_cursor_line && cursor_col >= start_col && cursor_col <= end_col {
                insert_cursor_in_text(content, selected_text, cursor_col - start_col);
            } else {
                content.push_str(selected_text);
            }
            content.push_str("◀");
        }
        
        // Add unselected part after selection
        if end_col < line.len() {
            let post_selection = &line[end_col..];
            if is_cursor_line && cursor_col >= end_col {
                insert_cursor_in_text(content, post_selection, cursor_col - end_col);
            } else {
                content.push_str(post_selection);
            }
        }
        
        // Add cursor at end if needed
        if is_cursor_line && cursor_col >= line.len() {
            content.push('│');
        }
    } else {
        // Render line without selection
        if is_cursor_line {
            insert_cursor_in_text(content, line, cursor_col);
            if cursor_col >= line.len() {
                content.push('│');
            }
        } else {
            content.push_str(line);
        }
    }
}

fn insert_cursor_in_text(content: &mut String, text: &str, cursor_pos: usize) {
    if cursor_pos == 0 {
        content.push('│');
        content.push_str(text);
    } else if cursor_pos >= text.len() {
        content.push_str(text);
    } else {
        content.push_str(&text[..cursor_pos]);
        content.push('│');
        content.push_str(&text[cursor_pos..]);
    }
}

fn render_results(f: &mut Frame, area: ratatui::layout::Rect, app: &App) {
    if let Some(results) = &app.query_results {
        let chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Min(5),     // Results table
                Constraint::Length(3),  // Summary and export options
            ])
            .split(area);

        // Render results table
        render_results_table(f, chunks[0], results);

        // Render summary and export options
        render_results_summary(f, chunks[1], results);
    } else {
        let placeholder = Paragraph::new("No query results to display\n\nExecute a query from the Query Editor to see results here.\n\nKeyboard shortcuts:\n• e: Go to Query Editor\n• Ctrl+Enter: Execute query\n• s: Export results to file")
            .block(Block::default().title("Results").borders(Borders::ALL))
            .wrap(Wrap { trim: true });

        f.render_widget(placeholder, area);
    }
}

fn render_results_table(f: &mut Frame, area: ratatui::layout::Rect, query_results: &sqlterm_core::QueryResult) {
    if query_results.columns.is_empty() {
        let no_data = Paragraph::new("No data returned from query")
            .block(Block::default().title("Query Results").borders(Borders::ALL))
            .wrap(Wrap { trim: true });
        f.render_widget(no_data, area);
        return;
    }

    // Create header row
    let header_cells: Vec<Cell> = query_results.columns
        .iter()
        .map(|col| Cell::from(col.name.clone()))
        .collect();

    let header = Row::new(header_cells)
        .style(Style::default().bg(Color::Blue).fg(Color::White))
        .height(1);

    // Create data rows with value truncation for display
    let rows = query_results.rows.iter().map(|row| {
        let cells: Vec<Cell> = row.values.iter().map(|value| {
            let text = value.to_string();
            // Truncate long values for display
            let display_text = if text.len() > 50 {
                format!("{}...", &text[..47])
            } else {
                text
            };
            Cell::from(display_text)
        }).collect();
        Row::new(cells).height(1)
    });

    // Calculate column widths dynamically
    let num_columns = query_results.columns.len();
    let column_width = if num_columns > 0 {
        100 / num_columns as u16
    } else {
        100
    };

    let widths: Vec<Constraint> = (0..num_columns)
        .map(|_| Constraint::Percentage(column_width))
        .collect();

    let title = if query_results.is_truncated {
        format!("Query Results (showing {} of {} rows)",
                query_results.truncated_at.unwrap_or(0),
                query_results.total_rows)
    } else {
        format!("Query Results ({} rows)", query_results.total_rows)
    };

    let table = Table::new(rows)
        .header(header)
        .block(Block::default().title(title).borders(Borders::ALL))
        .widths(&widths);

    f.render_widget(table, area);
}

fn render_results_summary(f: &mut Frame, area: ratatui::layout::Rect, query_results: &sqlterm_core::QueryResult) {
    let summary_text = format!(
        "{} | Press 's' to export to file | 'f' for full results | 'c' to copy",
        query_results.get_summary()
    );

    let summary = Paragraph::new(summary_text)
        .block(Block::default().title("Summary & Export").borders(Borders::ALL))
        .wrap(Wrap { trim: true });

    f.render_widget(summary, area);
}

fn render_error_popup(f: &mut Frame, app: &App) {
    if let Some(error_msg) = &app.error_message {
        let area = centered_rect(60, 20, f.size());
        f.render_widget(Clear, area);

        let error_popup = Paragraph::new(error_msg.clone())
            .style(Style::default().fg(Color::Red))
            .block(
                Block::default()
                    .title("Error")
                    .borders(Borders::ALL)
                    .border_style(Style::default().fg(Color::Red)),
            )
            .wrap(Wrap { trim: true })
            .alignment(Alignment::Center);

        f.render_widget(error_popup, area);
    }
}

fn centered_rect(percent_x: u16, percent_y: u16, r: ratatui::layout::Rect) -> ratatui::layout::Rect {
    let popup_layout = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Percentage((100 - percent_y) / 2),
            Constraint::Percentage(percent_y),
            Constraint::Percentage((100 - percent_y) / 2),
        ])
        .split(r);

    Layout::default()
        .direction(Direction::Horizontal)
        .constraints([
            Constraint::Percentage((100 - percent_x) / 2),
            Constraint::Percentage(percent_x),
            Constraint::Percentage((100 - percent_x) / 2),
        ])
        .split(popup_layout[1])[1]
}

fn render_table_info(f: &mut Frame, area: ratatui::layout::Rect, table_details: &sqlterm_core::TableDetails) {
    let info_text = format!(
        "Table: {}\nType: {:?}\nSchema: {}\nRows: {}\nSize: {}",
        table_details.table.name,
        table_details.table.table_type,
        table_details.table.schema.as_deref().unwrap_or("default"),
        table_details.statistics.row_count,
        table_details.statistics.size_bytes
            .map(|s| format!("{} bytes", s))
            .unwrap_or_else(|| "Unknown".to_string())
    );

    let info_widget = Paragraph::new(info_text)
        .block(Block::default().title("Table Information").borders(Borders::ALL))
        .wrap(Wrap { trim: true });

    f.render_widget(info_widget, area);
}

fn render_table_columns(f: &mut Frame, area: ratatui::layout::Rect, table_details: &sqlterm_core::TableDetails) {
    let header = Row::new(vec![
        Cell::from("Column"),
        Cell::from("Type"),
        Cell::from("Null"),
        Cell::from("Key"),
        Cell::from("Default"),
    ])
    .style(Style::default().bg(Color::Blue).fg(Color::White))
    .height(1);

    let rows = table_details.columns.iter().map(|col| {
        let key_info = if col.is_primary_key {
            "PRI"
        } else if col.is_foreign_key {
            "FOR"
        } else if col.is_unique {
            "UNI"
        } else {
            ""
        };

        Row::new(vec![
            Cell::from(col.name.clone()),
            Cell::from(col.data_type.clone()),
            Cell::from(if col.nullable { "YES" } else { "NO" }),
            Cell::from(key_info),
            Cell::from(col.default_value.as_deref().unwrap_or("")),
        ])
        .height(1)
    });

    let widths = [
        Constraint::Percentage(25),
        Constraint::Percentage(25),
        Constraint::Percentage(10),
        Constraint::Percentage(10),
        Constraint::Percentage(30),
    ];

    let table = Table::new(rows)
        .header(header)
        .block(Block::default().title("Columns").borders(Borders::ALL))
        .widths(&widths);

    f.render_widget(table, area);
}

fn render_table_statistics(f: &mut Frame, area: ratatui::layout::Rect, table_details: &sqlterm_core::TableDetails) {
    let stats_text = format!(
        "Statistics:\n• Rows: {}\n• Indexes: {}\n• Foreign Keys: {}\n• Auto Increment: {}",
        table_details.statistics.row_count,
        table_details.indexes.len(),
        table_details.foreign_keys.len(),
        table_details.statistics.auto_increment_value
            .map(|v| v.to_string())
            .unwrap_or_else(|| "None".to_string())
    );

    let stats_widget = Paragraph::new(stats_text)
        .block(Block::default().title("Statistics").borders(Borders::ALL))
        .wrap(Wrap { trim: true });

    f.render_widget(stats_widget, area);
}

fn render_add_connection_popup(f: &mut Frame, app: &App) {
    use sqlterm_core::DatabaseType;

    // Create a centered popup area
    let area = centered_rect(70, 60, f.size());
    f.render_widget(Clear, area);

    // Create form layout
    let form_chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(3),  // Title
            Constraint::Length(3),  // Name
            Constraint::Length(3),  // Database Type
            Constraint::Length(3),  // Host
            Constraint::Length(3),  // Port
            Constraint::Length(3),  // Database
            Constraint::Length(3),  // Username
            Constraint::Length(3),  // Password
            Constraint::Length(3),  // Instructions
        ])
        .split(area);

    let form = &app.connection_form;

    // Title
    let title = Paragraph::new("Add New Connection")
        .style(Style::default().fg(Color::White).add_modifier(Modifier::BOLD))
        .alignment(Alignment::Center)
        .block(Block::default().borders(Borders::ALL).border_style(Style::default().fg(Color::Blue)));
    f.render_widget(title, form_chunks[0]);

    // Helper function to create field widget
    let create_field = |label: String, value: String, is_selected: bool| {
        let style = if is_selected {
            Style::default().fg(Color::Yellow).add_modifier(Modifier::BOLD)
        } else {
            Style::default().fg(Color::White)
        };
        let border_style = if is_selected {
            Style::default().fg(Color::Yellow)
        } else {
            Style::default().fg(Color::Gray)
        };
        
        Paragraph::new(value)
            .style(style)
            .block(
                Block::default()
                    .title(label)
                    .borders(Borders::ALL)
                    .border_style(border_style)
            )
    };

    // Name field
    f.render_widget(
        create_field("Name".to_string(), form.name.clone(), form.selected_field == 0),
        form_chunks[1]
    );

    // Database Type field
    let db_type_display = match form.database_type {
        DatabaseType::SQLite => "SQLite (s)",
        DatabaseType::MySQL => "MySQL (m)", 
        DatabaseType::PostgreSQL => "PostgreSQL (p)",
    };
    f.render_widget(
        create_field("Database Type (s/m/p)".to_string(), db_type_display.to_string(), form.selected_field == 1),
        form_chunks[2]
    );

    // Host field
    f.render_widget(
        create_field("Host".to_string(), form.host.clone(), form.selected_field == 2),
        form_chunks[3]
    );

    // Port field
    f.render_widget(
        create_field("Port".to_string(), form.port.clone(), form.selected_field == 3),
        form_chunks[4]
    );

    // Database field
    f.render_widget(
        create_field("Database".to_string(), form.database.clone(), form.selected_field == 4),
        form_chunks[5]
    );

    // Username field
    f.render_widget(
        create_field("Username".to_string(), form.username.clone(), form.selected_field == 5),
        form_chunks[6]
    );

    // Password field (masked)
    let masked_password = "*".repeat(form.password.len());
    f.render_widget(
        create_field("Password".to_string(), masked_password, form.selected_field == 6),
        form_chunks[7]
    );

    // Instructions
    let instructions = Paragraph::new("Tab/↓: Next field | Shift+Tab/↑: Previous field | Enter: Connect | Esc: Cancel")
        .style(Style::default().fg(Color::Gray))
        .alignment(Alignment::Center)
        .block(Block::default().borders(Borders::ALL));
    f.render_widget(instructions, form_chunks[8]);
}

fn render_logs_panel(f: &mut Frame, area: ratatui::layout::Rect, app: &App) {
    let logs_text = if app.logs.is_empty() {
        "No logs yet. Press 'L' to toggle this panel.".to_string()
    } else {
        app.logs
            .iter()
            .rev() // Show newest logs first
            .take(5) // Show last 5 logs
            .map(|log| format!("[{}] [{}] {}", log.timestamp, log.level, log.message))
            .collect::<Vec<_>>()
            .join("\n")
    };

    let logs_widget = Paragraph::new(logs_text)
        .style(Style::default().fg(Color::Gray))
        .block(
            Block::default()
                .title("Logs (Press 'L' to toggle)")
                .borders(Borders::ALL)
                .border_style(Style::default().fg(Color::DarkGray))
        )
        .wrap(Wrap { trim: true });

    f.render_widget(logs_widget, area);
}
