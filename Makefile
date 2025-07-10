.PHONY: build run clean test fmt vet mod-tidy

# Build the application
build:
	go build -o ./bin/sqlterm ./cmd/sqlterm

# Run the application
run:
	go run ./cmd/sqlterm

# Clean build artifacts
clean:
	rm -f ./bin/sqlterm

# Run tests
test:
	go test ./...

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Tidy module dependencies
mod-tidy:
	go mod tidy

# Run all checks
check: fmt vet test

# Development build and run
dev: build
	./bin/sqlterm

# Install dependencies
deps:
	go mod download

# Full clean and rebuild
rebuild: clean build
