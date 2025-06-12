#!/bin/bash
# Create SQLite test database

DB_PATH="/data/testdb.sqlite"

# Create the database and run initialization script
sqlite3 "$DB_PATH" < /init/init.sql

echo "SQLite database created at $DB_PATH"
echo "Database size: $(du -h $DB_PATH | cut -f1)"
echo "Tables created:"
sqlite3 "$DB_PATH" ".tables"
