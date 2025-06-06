# SQLTerm

A modern, terminal-based SQL database management tool built in Rust. SQLTerm provides an intuitive TUI (Terminal User Interface) for managing multiple database connections and executing queries across different database systems.

## Features

### Core Functionality
- 🔌 **Multi-Database Support**: Connect to MySQL, PostgreSQL, and SQLite
- 🖥️ **Terminal UI**: Beautiful, responsive interface built with ratatui
- 🔍 **Schema Browser**: Explore databases, tables, columns, and indexes
- ⚡ **Query Editor**: Execute SQL queries with syntax highlighting
- 📊 **Results Viewer**: Display query results in formatted tables
- 💾 **Connection Management**: Save and manage multiple database connections

### Advanced Features
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

### Command Line Interface

```bash
# Start interactive TUI
sqlterm tui

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

### Connection Configuration

Connections can be configured via:

1. **Interactive TUI** - Add connections through the interface
2. **Command Line** - Use `sqlterm add` command
3. **Configuration File** - `~/.config/sqlterm/connections.toml`

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

