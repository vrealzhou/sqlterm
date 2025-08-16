# CRUSH.md - Developer Guide for SQLTerm

## Build Commands
- `make build` - Build for current platform
- `make build-all` - Build for all platforms (Windows, Linux, macOS)
- `make run` - Run the application directly
- `make dev` - Build and run in development mode
- `go run ./cmd/sqlterm` - Run in development mode

## Test Commands
- `make test` - Run all tests
- `make test-coverage` - Run tests with coverage report
- `go test ./internal/database/...` - Run tests for specific package
- `go test -v ./internal/database/types_test.go` - Run a single test file
- `go test -run TestParseDatabaseType` - Run a specific test function

## Lint/Format Commands
- `make fmt` - Format all Go code
- `make vet` - Run go vet
- `make lint` - Run golangci-lint (if installed)
- `make check` - Run fmt, vet, and tests together

## Code Style Guidelines

### General Structure
- Follow idiomatic Go patterns
- Use clear, descriptive names for variables, functions, and types
- All code should be properly tested with comprehensive test cases
- Error handling follows Go best practices with error wrapping

### Imports
- Standard library imports first, separated by blank line
- Third-party imports next, separated by blank line
- Local imports last
- Group imports alphabetically within each section

### Types and Naming
- Use PascalCase for exported identifiers
- Use camelCase for unexported identifiers
- Constants should be UPPER_SNAKE_CASE
- Prefer descriptive names over abbreviations
- Use typed constants when possible

### Error Handling
- Always check errors and handle appropriately
- Use fmt.Errorf with %w verb for wrapping errors
- Return errors early (avoid deep nesting)
- Provide context in error messages

### Testing
- Use table-driven tests where appropriate
- Name test cases descriptively
- Test both success and error cases
- Use t.Run for subtests with meaningful names
- Follow the pattern: arrange, act, assert

### Comments and Documentation
- All exported functions, types, and variables should have comments
- Comments should explain "why" not just "what"
- Use Godoc style comments

## Project Structure
- `/cmd/sqlterm` - Main application entry point
- `/internal/ai` - AI integration (OpenRouter, Ollama, LM Studio)
- `/internal/database` - Core database functionality
- `/internal/config` - Configuration management
- `/internal/cli` - CLI command handling
- `/internal/conversation` - Conversation interface
- `/internal/session` - Session management
- `/internal/i18n` - Internationalization support

## Makefile Targets
- `make help` - Show all available targets
- `make clean` - Clean build artifacts
- `make release` - Create release archives
- `make mod-tidy` - Tidy Go modules
- `make deps` - Download dependencies