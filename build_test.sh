#!/bin/bash
set -e
cd "$(dirname "$0")"
echo "Building UI package..."
cargo build --package ui --verbose
echo "Build completed successfully!"