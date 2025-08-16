# SQLTerm TUI Application

This is a terminal-based user interface application built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) that provides a two-view interface for managing database connections and exploring tables.

## Features

1. **Connection View** (First View):
   - Left panel: List all database connections
   - Right panel: Display selected connection details with editable text inputs
   - Navigation: Use ↑/↓ or Tab to navigate between input fields
   - Save: Press Ctrl+S to save changes
   - Connect: Press Enter to connect to the selected connection and switch to table view

2. **Table View** (Second View):
   - Left panel: List all tables in the connected database.
   - Each table can be expanded to view its fields.
   - Right panel: Display selected table details (columns, types, etc.)
   - Navigation: Press Esc to return to connection view

3. **Query View** (Third View):
   - Left panel: List all tables in the connected database
   - Each table can be expanded to view its fields.
   - Middle Top panel: Query editor with syntax highlighting and autocompletion.
   - Middle Bottom panel: Query results displayed in a table format.
   - Right panel: A chat panel for user interactions with LLM models and generate SQL queries or answer questions about the schema.

## Building and Running

To build and run the TUI application:

```bash
# Navigate to the project root
cd /path/to/sqlterm

# Build the TUI application
go build -o sqlterm-tui ./cmd/tui

# Run the TUI application
./sqlterm-tui
```

Or run directly without building:

```bash
go run ./cmd/tui
```

## Controls

### Connection View
- ↑/↓: Navigate within focused panel
- Tab: Switch between connection list and connection details
- Enter: Connect to selected database and switch to table view
- Ctrl+S: Save connection changes
- Ctrl+C: Exit application

### Table View
- Esc: Return to connection view
- Ctrl+C: Exit application

## Implementation Details

The application follows the Bubble Tea architecture with a state-based model:

1. **State Management**: The application has two main states (`connectionState` and `tableState`) that determine which view is displayed.

2. **Views**:
   - `connectionView()`: Displays a split-screen view with connections list on the left and connection details on the right
   - `tableView()`: Displays a split-screen view with tables list on the left and table details on the right

3. **Updates**:
   - `updateConnections()`: Handles updates for the connection view including list navigation and text input handling
   - `updateTables()`: Handles updates for the table view

4. **Components**:
   - Uses `list.Model` for displaying connections and tables
   - Uses `textinput.Model` for editable connection details
   - Uses `lipgloss` for styling and layout

## Customization

You can customize the appearance by modifying the styling variables at the top of the file:
- `docStyle`: Overall document styling
- `colors`: Color palette for the application
- `activeClr`: Active element color

## Dependencies

The application requires the following Go modules:
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/bubbles`
- `github.com/charmbracelet/lipgloss`

These are already included in the project's `go.mod` file.
