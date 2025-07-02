#!/bin/bash

# Test script to debug integer type issues in SQLTerm

echo "=== Building SQLTerm ==="
cargo build

if [ $? -eq 0 ]; then
    echo "Build successful!"
else
    echo "Build failed!"
    exit 1
fi

echo ""
echo "=== Build complete ==="
echo "The UI can be started with: cargo run --package ui"
echo "Binary is available at: target/debug/sqlterm"