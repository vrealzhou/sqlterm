# SQLTerm

A modern, terminal-based SQL database management tool built in Rust. SQLTerm provides a conversation-style CLI interface for managing multiple database connections and executing queries across different database systems.

## Features

### Core Functionality
- 🔌 **Multi-Database Support**: Connect to MySQL, PostgreSQL, and SQLite
- 💬 **Conversation Interface**: Claude Code-style `/` commands and `@` file references
- 🔍 **Schema Browser**: Explore databases, tables, columns, and indexes with `/tables`, `/describe`
- ⚡ **Smart Query Execution**: Execute SQL queries directly or from files with `@queries.sql`
- 📊 **Results Viewer**: Display query results in formatted tables with collapse/expand
- 💾 **Connection Management**: Save and manage multiple database connections

### Advanced Features
- 📁 **File-based Workflows**: Use your favorite text editor, reference SQL files with `@`
- 🔄 **Multi-Query Support**: Execute multiple queries from files with collapsible results
- 💾 **Session Management**: Save and restore conversation sessions with `/save-session`
- 🎯 **Auto-completion**: Tab completion for commands, SQL keywords, table names, and file paths
- 📜 **Command History**: Use ↑/↓ keys to navigate previous commands with persistent history
- 📋 **Clipboard Integration**: Copy queries and results to system clipboard with `/copy-*` commands
- 📂 **Organized Config**: Structured config directories for connections and sessions
- 🔐 **SSH Tunneling**: Connect to remote databases through SSH
- 🐳 **Docker Integration**: Test environment with containerized databases
- 🏔️ **Bastion Server**: Alpine Linux container with SSH access for remote testing
- 🔧 **Workspace Architecture**: Modular crate structure for extensibility

## Supported Database Systems

| Database | Status | Features |
|----------|--------|----------|
| MySQL | ✅ Implemented | Connection, Schema browsing, Query execution |
| PostgreSQL | 🚧 In Progress | Connection implemented, Schema/Query in development |
| SQLite | 🚧 In Progress | Connection implemented, Schema/Query in development |

## Architecture

SQLTerm is built as a Rust workspace with the following crates:

```
sqlterm/
├── crates/
│   ├── sqlterm-core/      # Core traits and common functionality
│   ├── sqlterm-mysql/     # MySQL-specific implementation
│   ├── sqlterm-postgres/  # PostgreSQL-specific implementation
│   ├── sqlterm-sqlite/    # SQLite-specific implementation
│   ├── sqlterm-ui/        # Terminal UI components (ratatui)
│   └── sqlterm-cli/       # Main CLI application
├── docker/                # Docker configuration for testing
└── target/                # Build artifacts
```

## Quick Start

### Prerequisites

- Rust 1.70+ (2021 edition)
- Podman and Podman Compose (or Docker and Docker Compose) for testing environment

### Installation

1. **Clone the repository:**
   ```bash
   git clone https://github.com/vrealzhou/sqlterm.git
   cd sqlterm
   ```

2. **Build the project:**
   ```bash
   make build
   # or
   cargo build --workspace
   ```

3. **Set up test environment:**
   ```bash
   make podman-up
   # or for Docker users:
   make docker-up
   ```

4. **Run SQLTerm:**
   ```bash
   make dev
   # or
   cargo run --bin sqlterm tui
   ```

### Using the Test Environment

The project includes a complete Podman/Docker Compose setup with:

- **MySQL 8.0** (port 3306)
  - Database: `testdb`
  - User: `testuser` / Password: `testpassword`

- **PostgreSQL 15** (port 5432)
  - Database: `testdb`
  - User: `testuser` / Password: `testpassword`

- **Alpine Bastion Server** (SSH port 2222)
  - User: `sqlterm` / Password: `sqlterm`
  - SQLTerm binary pre-installed

- **Adminer** (port 8080) - Web-based database administration

### Quick Test Commands

```bash
# Start all services
make podman-up

# Test database connections
make test-mysql
make test-postgres

# Test SSH access to bastion
make test-ssh

# Run the TUI
make dev-tui

# Connect directly to MySQL
make dev-connect-mysql

# Connect directly to PostgreSQL
make dev-connect-postgres
```

## Usage

### Conversation Mode (Default)

SQLTerm now defaults to a conversation-style interface similar to Claude Code. Simply run:

```bash
sqlterm
```

This starts an interactive session where you can use:

#### `/` Commands
```bash
/help                          # Show all available commands
/connect                       # Interactive connection setup wizard
/connect myconnection          # Connect to saved connection
/connect mysql://user:pass@localhost:3306/db  # Connect with connection string
/list-connections              # List all saved connections
/tables                        # List tables in current database
/describe users                # Show table structure
/status                        # Show connection status
/clear                         # Clear screen
/list-sessions                 # List saved conversation sessions
/save-session <name>           # Save current session
/load-session <name>           # Load a saved session
/delete-session <name>         # Delete a saved session
/copy-result <id>              # Copy query result to clipboard
/copy-query                    # Copy last query to clipboard
/paste                         # Show clipboard content
/quit                          # Exit SQLTerm
```

#### `@` File References
```bash
@queries.sql                   # Execute SQL file
@path/to/setup.sql            # Execute SQL file with path
@demo/seed-data.sql           # Execute relative path
```

#### Direct SQL Execution
```sql
SELECT * FROM users WHERE age > 25;
SELECT COUNT(*) FROM posts;
UPDATE users SET status = 'active' WHERE last_login > '2024-01-01';
```

#### Multi-Query Files
When you reference a file with `@`, SQLTerm will:
1. Parse and execute each query separated by semicolons
2. Display each query with its results
3. Allow you to collapse/expand results with `/toggle <result_id>`

#### Interactive Connection Setup

The `/connect` command without arguments starts an interactive wizard:

```bash
sqlterm > /connect
🔧 Interactive Connection Setup
Press Ctrl+C to cancel at any time.

📝 Enter connection name: local-mysql
📊 Select database type:
  1. MySQL
  2. PostgreSQL
  3. SQLite
Enter choice (1-3): 1
📝 Enter host [localhost]: 
📝 Enter port [3306]: 
📝 Enter database name: testdb
📝 Enter username: testuser
🔐 Enter password (leave empty for no password): [hidden]

📋 Connection Summary:
  Name: local-mysql
  Type: MySQL
  Host: localhost
  Port: 3306
  Database: testdb
  Username: testuser
  Password: [hidden]

🔌 Test connection and save? (Y/n): y

🔄 Testing connection...
✅ Connected to local-mysql (testdb)
💾 Connection 'local-mysql' saved successfully!
💡 Use '/connect local-mysql' to connect in the future

# Connection saved to: ~/.config/sqlterm/connections/local-mysql.toml
```

Example session:
```bash
sqlterm > /connect local-mysql
✅ Connected to local-mysql (testdb)

sqlterm (testdb) > /tables
📋 Tables in testdb:
  1. users
  2. posts
  3. categories

sqlterm (testdb) > /describe users
📊 Describing table 'users'...
📋 Table: users
   Type: Table
   Schema: testdb
   Rows: 150

📊 Columns:
   Name                 Type            Null     Key      Default
   ----------------------------------------------------------------------
   id                   int             NO       PRI      
   username             varchar(50)     NO       UNI      
   email                varchar(100)    NO               
   created_at           timestamp       NO               CURRENT_TIMESTAMP

sqlterm (testdb) > @setup.sql
📁 Executing SQL file: setup.sql
🔄 Found 3 queries in file

📝 Query 1 of 3:
```sql
CREATE TABLE temp_analysis AS SELECT * FROM users WHERE created_at > '2024-01-01'
```
✅ Query executed successfully (45 rows affected)
💡 Use /toggle 1 to collapse this result

📝 Query 2 of 3:
```sql
SELECT COUNT(*) as recent_users FROM temp_analysis
```
📊 Query Results (1 rows):
│ recent_users │
├──────────────┼
│ 45           │
└──────────────┴
💡 Use /toggle 2 to collapse this result

sqlterm (testdb) > SELECT username, email FROM users LIMIT 3;
📊 Query Results (3 rows):
│ username     │ email                    │
├──────────────┼──────────────────────────┼
│ john_doe     │ john@example.com         │
│ jane_smith   │ jane@example.com         │
│ bob_wilson   │ bob@example.com          │
└──────────────┴──────────────────────────┴

sqlterm (testdb) > /save-session analysis-work
💾 Session 'analysis-work' saved successfully!
📁 Location: ~/.config/sqlterm/sessions/analysis-work.txt

sqlterm (testdb) > /list-sessions
📋 Saved Sessions:
  1. analysis-work
  2. migration-queries
  3. performance-tests

💡 Use /load-session <name> to restore a session
💡 Use /delete-session <name> to remove a session
```

#### Enhanced Interaction Features

**Auto-completion (Tab key)**:
```bash
sqlterm > /con<TAB>          # Completes to /connect
sqlterm > /connect my<TAB>   # Shows saved connections starting with "my"
sqlterm > SELECT * FROM u<TAB>  # Shows tables starting with "u"
sqlterm > @demo<TAB>         # Completes to @demo.sql or shows files
```

**Command History (↑/↓ keys)**:
- Navigate through previous commands with up/down arrow keys
- History is persistent across sessions
- Stored in `~/.config/sqlterm/history.txt`

**Clipboard Integration**:
```bash
sqlterm > /copy-query        # Copy last executed query to clipboard
sqlterm > /paste             # Show current clipboard content
sqlterm > /copy-result 1     # Copy query result to clipboard (future feature)
```

**Smart File Completion**:
```bash
sqlterm > @<TAB>             # Shows all .sql files in current directory
sqlterm > @path/<TAB>        # Shows files in subdirectory
sqlterm > @demo/<TAB>        # Shows files in demo/ folder
```

### Legacy TUI Mode

For the traditional terminal UI interface:

```bash
sqlterm tui
```

### Command Line Interface

```bash
# Connect directly to a database
sqlterm connect --db-type mysql --host localhost --port 3306 --database testdb --username testuser

# List saved connections
sqlterm list

# Add a new connection
sqlterm add "My MySQL" --db-type mysql --host localhost --database mydb --username myuser
```

### Terminal User Interface

The TUI provides several screens:

1. **Connection Manager** - Manage and select database connections
2. **Database Browser** - Browse databases, tables, and schema
3. **Query Editor** - Write and execute SQL queries
4. **Results Viewer** - View query results in formatted tables

#### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `q` | Quit application |
| `Ctrl+C` | Force quit |
| `↑/↓` | Navigate lists |
| `Enter` | Select/Connect |
| `Tab` | Switch between panels |
| `Esc` | Go back/Cancel |

### SSH Tunneling

Connect to remote databases through SSH:

```bash
# SSH to bastion server
ssh -p 2222 sqlterm@localhost

# Run sqlterm on remote server
sqlterm tui
```

## Development

### Project Structure

```
crates/
├── sqlterm-core/          # Core abstractions
│   ├── connection.rs      # Database connection traits
│   ├── query.rs          # Query execution traits
│   ├── schema.rs         # Schema inspection traits
│   ├── result.rs         # Result handling
│   └── error.rs          # Error types
├── sqlterm-mysql/         # MySQL implementation
├── sqlterm-postgres/      # PostgreSQL implementation
├── sqlterm-sqlite/        # SQLite implementation
├── sqlterm-ui/           # Terminal UI
│   ├── app.rs           # Application state
│   ├── ui.rs            # UI rendering
│   ├── events.rs        # Event handling
│   └── components/      # Reusable UI components
└── sqlterm-cli/          # CLI application
```

### Building and Testing

```bash
# Build all crates
make build

# Run tests
make test

# Run with verbose output
make test-verbose

# Check code quality
make check
make clippy
make fmt

# Full development setup
make setup

# Prepare for release
make prepare-release
```

### Adding a New Database

To add support for a new database:

1. Create a new crate: `crates/sqlterm-newdb/`
2. Implement the core traits:
   - `DatabaseConnection`
   - `QueryExecutor`
   - `SchemaInspector`
3. Add the crate to workspace dependencies
4. Update the CLI to handle the new database type

### Docker Environment

The Docker environment includes:

```yaml
services:
  mysql:        # MySQL 8.0 with test data
  postgres:     # PostgreSQL 15 with test data
  bastion:      # Alpine Linux with SSH + sqlterm
  adminer:      # Web database admin interface
```

Sample data includes:
- Users table with sample users
- Posts table with blog posts
- Categories and relationships
- Indexes and foreign keys

## Configuration

### Configuration Structure

SQLTerm uses an organized configuration directory structure:

```
~/.config/sqlterm/
├── connections/          # Database connection configs
│   ├── local-mysql.toml
│   ├── prod-postgres.toml
│   └── dev-sqlite.toml
└── sessions/            # Saved conversation sessions
    ├── analysis-work.txt
    ├── migration-queries.txt
    └── performance-tests.txt
```

### Connection Configuration

Connections can be configured via:

1. **Interactive Wizard** - Use `/connect` for step-by-step setup
2. **Command Line** - Use `sqlterm add` command
3. **Configuration Files** - Stored in `~/.config/sqlterm/connections/`

Example configuration:

```toml
[[connections]]
name = "Local MySQL"
database_type = "MySQL"
host = "localhost"
port = 3306
database = "testdb"
username = "testuser"
ssl = false

[connections.ssh_tunnel]
host = "bastion.example.com"
port = 22
username = "sqlterm"
private_key_path = "~/.ssh/id_rsa"
```

### Session Management

SQLTerm can save and restore conversation sessions:

```bash
# Save current session
sqlterm > /save-session my-analysis

# List saved sessions  
sqlterm > /list-sessions

# Load a previous session
sqlterm > /load-session my-analysis

# Delete a session
sqlterm > /delete-session old-session
```

Sessions currently store:
- Session metadata (name, timestamp, connection info)
- Placeholder for future features like:
  - Conversation history
  - Query history
  - Active connection state
  - Collapsed result states

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make prepare-release`
6. Submit a pull request

### Development Guidelines

- Follow Rust best practices
- Add tests for new functionality
- Update documentation
- Use conventional commit messages
- Ensure all checks pass

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Roadmap

### Phase 1 (Current)
- ✅ Core architecture and traits
- ✅ MySQL basic implementation (connection, basic query execution, schema inspection)
- ✅ Terminal UI foundation with ratatui
- ✅ Podman/Docker test environment
- ✅ PostgreSQL connection implementation
- ✅ SQLite connection implementation
- ✅ CLI application with multiple commands
- ✅ Workspace-based crate structure
- ✅ Alpine bastion server for SSH testing

### Phase 2 (Next)
- 📋 Complete PostgreSQL and SQLite query execution
- 📋 Complete schema inspection for all databases
- 📋 Enhanced TUI with proper navigation and editing
- 📋 Connection configuration persistence
- 📋 Query history and bookmarks
- 📋 SSH tunnel implementation

### Phase 3 (Future)
- 📋 Advanced schema operations (DDL)
- 📋 Data export/import
- 📋 Query performance analysis
- 📋 Connection pooling
- 📋 Plugin system
- 📋 Custom themes
- 📋 Collaborative features
- 📋 Cloud database support

