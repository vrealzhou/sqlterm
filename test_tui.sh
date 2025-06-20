#!/bin/bash

echo "Testing SQLTerm TUI..."
echo "The application should start with a connection manager."
echo "You should see:"
echo "1. A header showing 'SQLTerm - Connection Manager'"
echo "2. A list of sample connections (MySQL, PostgreSQL, SQLite)" 
echo "3. A footer with keyboard shortcuts"
echo "4. Press 'q' to quit, arrow keys to navigate"
echo ""
echo "Starting TUI in 3 seconds..."
sleep 3

cargo run --package ui