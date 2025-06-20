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
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(3), // Header
            Constraint::Min(0),    // Main content
            Constraint::Length(3), // Footer
        ])
        .split(f.size());

    // Render header
    render_header(f, chunks[0], app);

    // Render main content based on current state
    match app.state {
        AppState::ConnectionManager => render_connection_manager(f, chunks[1], app),
        AppState::DatabaseBrowser => render_database_browser(f, chunks[1], app),
        AppState::QueryEditor => render_query_editor(f, chunks[1], app),
        AppState::Results => render_results(f, chunks[1], app),
    }

    // Render footer
    render_footer(f, chunks[2], app);

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
                InputMode::Normal => "i: Insert | Ctrl+Enter/Ctrl+R: Execute | b: Browser | c: Connections | q/Esc: Quit",
                InputMode::Editing => "Esc: Normal mode | Ctrl+Enter/Ctrl+R: Execute | Enter: New line | Ctrl+C: Quit",
            }
        }
        AppState::Results => {
            "s: Export to file | f: Show full results | c: Copy | e: Query Editor | b: Browser | q: Quit"
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

    // Query input
    let input_style = match app.input_mode {
        InputMode::Editing => Style::default().fg(Color::Yellow),
        InputMode::Normal => Style::default(),
    };

    let title = match app.input_mode {
        InputMode::Normal => "SQL Query (Press 'i' to edit)",
        InputMode::Editing => "SQL Query (Editing - Press Esc to exit)",
    };

    let query_input = Paragraph::new(app.query_input.as_str())
        .style(input_style)
        .block(Block::default().title(title).borders(Borders::ALL))
        .wrap(Wrap { trim: true });

    f.render_widget(query_input, chunks[0]);

    // Query history or help (placeholder)
    let help = Paragraph::new("Query history will appear here")
        .block(Block::default().title("History").borders(Borders::ALL));

    f.render_widget(help, chunks[1]);
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
