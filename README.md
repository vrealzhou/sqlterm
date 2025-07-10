# SQLTerm-Go

A modern, terminal-based SQL database management tool built in Go. SQLTerm provides an intuitive conversation-style interface for managing database connections and executing queries across MySQL, PostgreSQL, and SQLite.

## Features

- 🔌 **Multi-Database Support**: Connect to MySQL, PostgreSQL, and SQLite
- 💬 **Conversation Interface**: Intuitive chat-like interface with `/` commands
- 📁 **File-based Workflows**: Execute SQL files with `@filename.sql`
- 💾 **Connection Management**: Save and manage multiple database connections
- 🔍 **Schema Exploration**: Browse tables, columns, and indexes
- 📊 **Rich Results Display**: View query results in formatted tables (top 20 rows)
- 📜 **Command History**: Persistent command history with ↑/↓ navigation
- 📄 **Markdown Export**: Auto-save results as markdown with glow preview
- 📈 **CSV Export**: Export complete results to CSV with `> filename.csv`
- 🎯 **Auto-completion**: Tab completion for commands and files
- 💻 **Session Management**: Organized result storage in `~/.config/sqlterm/sessions/`

## Installation

### Prerequisites

- Go 1.24+
- Git
- [Glow](https://github.com/charmbracelet/glow) for markdown preview (optional): `go install github.com/charmbracelet/glow@latest`

### Build from Source

```bash
# Clone the repository
git clone <repository-url>
cd sqlterm-go

# Build the project
go build -o sqlterm ./cmd/sqlterm

# Run SQLTerm
./sqlterm
```

### Quick Development Setup

```bash
# Run in development mode
go run ./cmd/sqlterm

# Or build and run
make build && ./sqlterm
```

## Usage

### Starting SQLTerm

Simply run `sqlterm` to start the conversation interface:

```bash
sqlterm
```

### Basic Commands

SQLTerm uses a conversation-style interface with the following command types:

#### `/` Commands - System Operations

```bash
/help                    # Show all available commands
/connect                 # Interactive connection setup
/connect mydb            # Connect to saved connection "mydb"
/list-connections        # List all saved connections
/tables                  # List tables in current database
/describe users          # Show table structure for "users"
/status                  # Show current connection status
/exec SELECT * FROM users # Execute a query directly
/quit                    # Exit SQLTerm
```

#### `@` File References - Execute SQL Files

```bash
@setup.sql               # Execute all queries in setup.sql
@queries/analysis.sql    # Execute file with path
@migration.sql 1         # Execute only the first query
@seed-data.sql 2-5       # Execute queries 2 through 5
```

#### Direct SQL Execution

```sql
SELECT * FROM users WHERE age > 25;
INSERT INTO posts (title, content) VALUES ('Hello', 'World');
UPDATE users SET status = 'active' WHERE last_login > '2024-01-01';
```

#### CSV Export

```sql
SELECT * FROM users > users.csv              # Export all users to CSV
SELECT * FROM orders WHERE date > '2024-01-01' > recent_orders.csv
```

### Getting Started

1. **Start SQLTerm**:
   ```bash
   sqlterm
   ```

2. **Set up your first connection** using the interactive wizard:
   ```bash
   sqlterm > /connect
   ```
   
   Follow the prompts to enter your database details.

3. **List tables** in your database:
   ```bash
   sqlterm (mydb) > /tables
   ```

4. **Explore a table structure**:
   ```bash
   sqlterm (mydb) > /describe users
   ```

5. **Run SQL queries**:
   ```bash
   sqlterm (mydb) > SELECT * FROM users LIMIT 5;
   ```

### Connection Management

#### Interactive Setup

The easiest way to add a connection is using the interactive wizard:

```bash
sqlterm > /connect
🔧 Interactive Connection Setup
📝 Enter connection name: my-local-db
📊 Select database type:
  1. MySQL
  2. PostgreSQL  
  3. SQLite
Enter choice (1-3): 1
📝 Enter host [localhost]: 
📝 Enter port [3306]: 
📝 Enter database name: testdb
📝 Enter username: myuser
🔐 Enter password: [hidden]

✅ Connected to my-local-db (testdb)
💾 Connection saved!
```

#### Command Line Setup

You can also add connections via command line:

```bash
# Add a new connection
sqlterm add "My Database" --db-type mysql --host localhost --database mydb --username myuser

# List saved connections
sqlterm list

# Connect directly
sqlterm connect --db-type mysql --host localhost --database mydb --username myuser
```

## New Features

### Markdown Export & Preview

Every query execution automatically saves results as markdown files with:
- Query metadata (connection, timestamp)
- Top 20 results formatted as markdown tables
- Automatic glow preview (press Enter to view, ESC to quit)
- Files stored in `~/.config/sqlterm/sessions/<connection>/`

### CSV Export

Export complete query results to CSV using the `>` operator:

```sql
sqlterm (mydb) > SELECT * FROM users > users.csv
Executing query and exporting to users.csv...
✅ Exported 25 rows to users.csv
```

### Enhanced Display

- Console output limited to top 20 rows for readability
- Truncation message shows remaining row count
- Suggestion to use CSV export for complete results

### Auto-completion

Tab completion for:
- Commands (`/help`, `/connect`, `/tables`, etc.)
- File paths for `@filename.sql`
- Connection names

## Configuration

SQLTerm stores configuration in your system's config directory:

- **Linux/macOS**: `~/.config/sqlterm/`
- **Windows**: `%APPDATA%\sqlterm\`

### Directory Structure

```
~/.config/sqlterm/
├── connections/          # Saved database connections
│   ├── my-local-db.yaml
│   └── production.yaml
├── sessions/             # Query result sessions
│   └── <connection>/
│       └── query_results_20241210_143022.md
└── history.txt          # Command history
```

## Database Support

| Database   | Status | Connection | Queries | Schema |
|------------|--------|------------|---------|--------|
| MySQL      | ✅     | ✅         | ✅      | ✅     |
| PostgreSQL | ✅     | ✅         | ✅      | ✅     |
| SQLite     | ✅     | ✅         | ✅      | ✅     |

## Project Structure

```
sqlterm-go/
├── cmd/
│   └── sqlterm/          # Main application entry point
├── internal/
│   ├── cli/              # Command line interface
│   ├── config/           # Configuration management
│   ├── conversation/     # Interactive conversation mode
│   └── core/            # Core database functionality
├── go.mod
├── go.sum
└── README.md
```

## License

This project is licensed under the MIT License.