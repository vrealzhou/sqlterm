#!/bin/bash
# SQLTerm Demo Script

echo "🚀 SQLTerm Demo - Multi-Database SQL Terminal"
echo "=============================================="
echo

# Build the project
echo "📦 Building SQLTerm..."
cargo build --release --workspace
echo "✅ Build complete!"
echo

# Show help
echo "📖 SQLTerm Help:"
./target/release/sqlterm --help
echo

# List connections (should be empty initially)
echo "📋 Current connections:"
./target/release/sqlterm list
echo

# Add some sample connections
echo "➕ Adding sample connections..."

echo "  Adding MySQL connection..."
./target/release/sqlterm add "Production MySQL" \
    --db-type mysql \
    --host localhost \
    --port 3306 \
    --database production \
    --username admin

echo "  Adding PostgreSQL connection..."
./target/release/sqlterm add "Analytics PostgreSQL" \
    --db-type postgres \
    --host analytics.example.com \
    --port 5432 \
    --database analytics \
    --username analyst

echo "  Adding SQLite connection..."
./target/release/sqlterm add "Local SQLite" \
    --db-type sqlite \
    --database ./local.db \
    --username sqlite

echo "  Adding Development MySQL..."
./target/release/sqlterm add "Dev MySQL" \
    --db-type mysql \
    --host dev.example.com \
    --port 3306 \
    --database devdb \
    --username developer

echo

# List all connections
echo "📋 All saved connections:"
./target/release/sqlterm list
echo

# Show configuration file location
echo "⚙️  Configuration saved to:"
echo "   ~/.config/sqlterm/config.toml"
echo

# Show TUI demo
echo "🖥️  Starting Terminal UI Demo..."
echo "   (Press 'q' to quit, use arrow keys to navigate)"
echo "   Available screens:"
echo "   - Connection Manager (current)"
echo "   - Database Browser (press 'e' then 'b')"
echo "   - Query Editor (press 'e')"
echo "   - Results Viewer (press 'e' then 'r')"
echo
echo "   Press Enter to start the TUI..."
read -p ""

# Start the TUI
./target/release/sqlterm tui

echo
echo "🎉 Demo complete!"
echo
echo "📚 Next steps:"
echo "   1. Set up database containers: make docker-up"
echo "   2. Test database connections: make test-mysql"
echo "   3. Connect via SSH: ssh -p 2222 sqlterm@localhost"
echo "   4. Explore the codebase in crates/"
echo
echo "🔗 Key features demonstrated:"
echo "   ✅ Multi-database support (MySQL, PostgreSQL, SQLite)"
echo "   ✅ Configuration management"
echo "   ✅ Terminal UI with ratatui"
echo "   ✅ Workspace-based architecture"
echo "   ✅ CLI with multiple commands"
echo "   ✅ Connection validation"
echo "   ✅ Trait-based abstractions"
echo
