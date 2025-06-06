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
            "↑/↓: Navigate | Enter: Connect | a: Add | d: Delete | q: Quit"
        }
        AppState::DatabaseBrowser => {
            "↑/↓: Navigate | Enter: Select | e: Query Editor | c: Connections | q: Quit"
        }
        AppState::QueryEditor => {
            "Ctrl+Enter: Execute | Ctrl+S: Save | Esc: Browser | q: Quit"
        }
        AppState::Results => {
            "↑/↓: Navigate | e: Query Editor | b: Browser | q: Quit"
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

    // Table details (placeholder)
    let details = Paragraph::new("Select a table to view details")
        .block(Block::default().title("Table Details").borders(Borders::ALL))
        .wrap(Wrap { trim: true });

    f.render_widget(details, chunks[1]);
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

    let query_input = Paragraph::new(app.query_input.as_str())
        .style(input_style)
        .block(Block::default().title("SQL Query").borders(Borders::ALL))
        .wrap(Wrap { trim: true });

    f.render_widget(query_input, chunks[0]);

    // Query history or help (placeholder)
    let help = Paragraph::new("Query history will appear here")
        .block(Block::default().title("History").borders(Borders::ALL));

    f.render_widget(help, chunks[1]);
}

fn render_results(f: &mut Frame, area: ratatui::layout::Rect, app: &App) {
    if let Some(results) = &app.query_results {
        let header_cells = results
            .columns
            .iter()
            .map(|col| Cell::from(col.name.clone()))
            .collect::<Vec<_>>();

        let header = Row::new(header_cells)
            .style(Style::default().bg(Color::Blue).fg(Color::White))
            .height(1);

        let rows = results.rows.iter().map(|row| {
            let cells = row
                .values
                .iter()
                .map(|value| Cell::from(value.to_string()))
                .collect::<Vec<_>>();
            Row::new(cells).height(1)
        });

        let num_columns = results.columns.len().min(5);
        let widths = vec![Constraint::Percentage(20); num_columns];

        let table = Table::new(rows)
            .header(header)
            .block(Block::default().title("Query Results").borders(Borders::ALL))
            .widths(&widths);

        f.render_widget(table, area);
    } else {
        let placeholder = Paragraph::new("No results to display")
            .block(Block::default().title("Query Results").borders(Borders::ALL))
            .alignment(Alignment::Center);

        f.render_widget(placeholder, area);
    }
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
