# SQLTerm Makefile

.PHONY: help build test clean docker-up docker-down docker-logs install dev check fmt clippy

# Default target
help:
	@echo "Available targets:"
	@echo "  build       - Build all crates"
	@echo "  test        - Run all tests"
	@echo "  clean       - Clean build artifacts"
	@echo "  install     - Install sqlterm binary"
	@echo "  dev         - Run in development mode"
	@echo "  check       - Check code without building"
	@echo "  fmt         - Format code"
	@echo "  clippy      - Run clippy lints"
	@echo "  podman-up   - Start test databases"
	@echo "  podman-down - Stop test databases"
	@echo "  podman-logs - Show database logs"
	@echo "  docker-*    - Docker compatibility aliases"

# Build targets
build:
	cargo build --workspace

build-release:
	cargo build --workspace --release

# Test targets
test:
	cargo test --workspace

test-verbose:
	cargo test --workspace -- --nocapture

# Development targets
dev:
	cargo run --bin sqlterm

dev-tui:
	cargo run --bin sqlterm tui

dev-connect-mysql:
	cargo run --bin sqlterm connect --db-type mysql --host localhost --port 3306 --database testdb --username testuser

dev-connect-postgres:
	cargo run --bin sqlterm connect --db-type postgres --host localhost --port 5432 --database testdb --username testuser

# Code quality targets
check:
	cargo check --workspace

fmt:
	cargo fmt --all

clippy:
	cargo clippy --workspace -- -D warnings

# Installation
install: build-release
	cargo install --path crates/sqlterm-cli

# Podman targets
podman-up:
	podman-compose -f podman-compose.yml up -d
	@echo "Waiting for databases to be ready..."
	@sleep 10
	@echo "Databases should be ready!"

podman-down:
	podman-compose -f podman-compose.yml down

podman-logs:
	podman-compose -f podman-compose.yml logs -f

podman-clean:
	podman-compose -f podman-compose.yml down -v
	podman system prune -f

# Docker targets (for compatibility)
docker-up: podman-up
docker-down: podman-down
docker-logs: podman-logs
docker-clean: podman-clean

# Database connection tests
test-mysql:
	@echo "Testing MySQL connection..."
	podman exec sqlterm-mysql mysql -u testuser -ptestpassword -e "SELECT 'MySQL connection successful' as status;"

test-postgres:
	@echo "Testing PostgreSQL connection..."
	podman exec sqlterm-postgres psql -U testuser -d testdb -c "SELECT 'PostgreSQL connection successful' as status;"

test-ssh:
	@echo "Testing SSH connection to bastion..."
	ssh -o StrictHostKeyChecking=no -p 2222 sqlterm@localhost "echo 'SSH connection successful'"

# Clean targets
clean:
	cargo clean
	podman-compose -f podman-compose.yml down -v 2>/dev/null || true

# Development workflow
setup: podman-up
	@echo "Setting up development environment..."
	@echo "Building project..."
	$(MAKE) build
	@echo "Running tests..."
	$(MAKE) test
	@echo "Setup complete!"

# Full test suite
test-all: podman-up
	@echo "Running full test suite..."
	$(MAKE) test
	$(MAKE) test-mysql
	$(MAKE) test-postgres
	@echo "All tests completed!"

# Release preparation
prepare-release: clean
	$(MAKE) fmt
	$(MAKE) clippy
	$(MAKE) test-all
	$(MAKE) build-release
	@echo "Release preparation complete!"
