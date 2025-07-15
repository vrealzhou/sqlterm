# SQLTerm

A modern, AI-powered terminal-based SQL database management tool built in Go. SQLTerm provides an intuitive conversation-style interface with intelligent AI assistance for managing database connections and executing queries across MySQL, PostgreSQL, and SQLite. Each database connection maintains its own isolated session with command history, vector-based AI context, and organized query results.

## Features

- 🔌 **Multi-Database Support**: Connect to MySQL, PostgreSQL, and SQLite
- 💬 **Conversation Interface**: Intuitive chat-like interface with `/` commands
- 🤖 **AI Integration**: Multi-provider AI support (OpenRouter, Ollama, LM Studio) with intelligent context selection
- 🧠 **Vector Database**: SQLite-based semantic search for intelligent table discovery
- 📁 **File-based Workflows**: Execute SQL files with `@filename.sql`
- 💾 **Connection Management**: Save and manage multiple database connections
- 🔍 **Schema Exploration**: Browse tables, columns, and indexes with AI-powered relevance scoring
- 📊 **Rich Results Display**: View query results in formatted tables (top 20 rows)
- ✨ **SQL Auto-formatting**: Automatic SQL formatting in markdown output for better readability
- 📜 **Session-specific History**: Command history stored separately for each database connection
- 📄 **Markdown Export**: Auto-save results as formatted markdown with glow preview
- 📈 **CSV Export**: Export complete results to CSV with `> filename.csv`
- 🎯 **Auto-completion**: Tab completion for commands and files
- 💻 **Session Management**: Organized per-connection storage in `~/.config/sqlterm/sessions/{connection}/`
- 🌐 **Internationalization**: Full i18n support with English and Chinese localizations (780+ translated strings)

## Installation

### Prerequisites

- Go 1.24+
- Git
- [Glow](https://github.com/charmbracelet/glow) for markdown preview (optional): `go install github.com/charmbracelet/glow@latest`

### Build from Source

```bash
# Clone the repository
git clone <repository-url>
cd sqlterm

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
/exec                    # Enter multi-line SQL mode (end with ;)
/exec SELECT * FROM users # Execute a query directly
/quit                    # Exit SQLTerm

# AI Commands (when configured)
<question>           # Ask AI about your database or SQL
/config                  # Configure AI providers and settings
/usage                   # Show AI usage statistics
/prompts                 # View recent AI prompt history
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

## AI Integration

### Multi-Provider Support

SQLTerm supports multiple AI providers for intelligent database assistance:

- **OpenRouter**: Access to multiple models including Claude, GPT-4, and more
- **Ollama**: Local AI models for privacy and offline usage
- **LM Studio**: Local model serving with OpenAI-compatible API

### AI Configuration

Set up AI providers with the interactive configuration:

```bash
sqlterm > /config
🤖 AI Configuration
📊 Select provider:
  1. OpenRouter (Cloud)
  2. Ollama (Local)
  3. LM Studio (Local)
Enter choice (1-3): 1
🔑 Enter OpenRouter API key: sk-or-...
📝 Enter model [anthropic/claude-3.5-sonnet]:
✅ AI configured successfully!
```

### Intelligent Context Selection

SQLTerm uses vector databases to provide AI with the most relevant context:

- **Semantic Search**: Finds tables most relevant to your question
- **Access Patterns**: Learns from your query history
- **Smart Context**: Provides AI with column details, sample data, and relationships
- **Per-Connection Learning**: Each database has its own knowledge base

### Example AI Usage

```bash
sqlterm (ecommerce) > How do I find customers who haven't placed orders?

🤖 AI Response:
To find customers who haven't placed orders, you can use a LEFT JOIN with a WHERE clause:

```sql
SELECT c.customer_id, c.name, c.email
FROM customers c
LEFT JOIN orders o ON c.customer_id = o.customer_id
WHERE o.customer_id IS NULL;
```

This query joins the customers table with orders and filters for customers where no matching order exists.
```

### Language Support

SQLTerm supports multiple languages with automatic detection based on system locale:

- **English (en_AU)**: Full feature support with comprehensive help text
- **Chinese (zh_CN)**: Complete translation of all user interface elements
- **Automatic Detection**: Uses system locale to determine display language
- **Fallback System**: Gracefully falls back to English if translations are missing

780+ interface strings are fully internationalized, including CLI help, error messages, status updates, and AI responses.

## Key Features

### Multi-line SQL Execution

Execute complex queries with the improved `/exec` command:

```bash
sqlterm (mydb) > /exec
📝 Multi-line SQL mode. Enter your query:
   • Paste multiple lines
   • End with ; to execute
   • Or press Ctrl+C to cancel

  1│ SELECT u.name,
  2│   COUNT(o.order_id) as order_count,
  3│   SUM(o.total) as total_spent
  4│ FROM users u
  5│ LEFT JOIN orders o ON u.id = o.user_id
  6│ GROUP BY u.id, u.name
  7│ ORDER BY total_spent DESC;

🔍 Executing query...
```

### SQL Auto-formatting

All SQL queries in markdown output are automatically formatted for better readability:

- Keyword capitalization and proper indentation
- Clean JOIN formatting
- SELECT column alignment
- Consistent spacing and line breaks

### Session Management

Each database connection maintains its own isolated session:

- **Command History**: Separate history per connection with ↑/↓ navigation
- **Vector Database**: Connection-specific table embeddings and learning
- **Query Results**: Organized markdown exports per connection
- **Configuration**: Per-session settings and preferences

### CSV Export

Export complete query results to CSV using the `>` operator:

```sql
sqlterm (mydb) > SELECT * FROM users > users.csv
Executing query and exporting to users.csv...
✅ Exported 25 rows to users.csv
```

### Auto-completion

Tab completion for:
- Commands (`/help`, `/connect`, `/tables`, etc.)
- File paths for `@filename.sql`
- Connection names
- AI model names during configuration

## Configuration

SQLTerm stores configuration in your system's config directory:

- **Linux/macOS**: `~/.config/sqlterm/`
- **Windows**: `%APPDATA%\sqlterm\`

### Directory Structure

```
~/.config/sqlterm/
├── ai.yaml               # AI provider configuration
├── usage.yaml            # AI usage statistics
├── connections/          # Saved database connections
│   ├── my-local-db.yaml
│   └── production.yaml
└── sessions/             # Per-connection session data
    ├── global_history.txt # Global command history (when not connected)
    ├── my-local-db/       # Session data for "my-local-db" connection
    │   ├── vectors.db     # Vector database for AI context
    │   ├── history.txt    # Command history for this connection
    │   ├── session.yaml   # Session configuration
    │   ├── query_result_20250715_143022.md
    │   └── query_result_20250715_143105.md
    └── production/        # Session data for "production" connection
        ├── vectors.db
        ├── history.txt
        ├── session.yaml
        └── [query results...]
```

## Database Support

| Database   | Status | Connection | Queries | Schema |
|------------|--------|------------|---------|--------|
| MySQL      | ✅     | ✅         | ✅      | ✅     |
| PostgreSQL | ✅     | ✅         | ✅      | ✅     |
| SQLite     | ✅     | ✅         | ✅      | ✅     |


## License

This project is licensed under the MIT License.
