.PHONY: build run clean test fmt vet mod-tidy build-all build-windows build-linux build-darwin docker-build docker-run docker-dev docker-clean docker-validate

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

# Note: i18n files in internal/i18n/*.json are automatically embedded
# into the binary using Go's embed system (//go:embed directive)

# Build the application for current platform
build:
	go build -ldflags="$(LDFLAGS)" -o ./bin/sqlterm ./cmd/sqlterm

# Build for all platforms
build-all: build-windows build-linux build-darwin

# Build for Windows
build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o ./dist/sqlterm-windows-amd64.exe ./cmd/sqlterm
	GOOS=windows GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o ./dist/sqlterm-windows-arm64.exe ./cmd/sqlterm

# Build for Linux
build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o ./dist/sqlterm-linux-amd64 ./cmd/sqlterm
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o ./dist/sqlterm-linux-arm64 ./cmd/sqlterm

# Build for macOS
build-darwin:
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o ./dist/sqlterm-darwin-amd64 ./cmd/sqlterm
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o ./dist/sqlterm-darwin-arm64 ./cmd/sqlterm

# Run the application
run:
	go run ./cmd/sqlterm

# Clean build artifacts
clean:
	rm -rf ./bin/sqlterm ./dist/

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run golangci-lint (if installed)
lint:
	golangci-lint run

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

# Docker targets
docker-build:
	docker build -t sqlterm:latest -f infra/docker/Dockerfile .

docker-run:
	docker run -it --rm sqlterm:latest

docker-dev:
	./infra/docker/build.sh build-dev

docker-dev-run:
	./infra/docker/build.sh run-dev

docker-compose:
	docker-compose -f infra/docker/docker-compose.yml up

docker-compose-down:
	docker-compose -f infra/docker/docker-compose.yml down

docker-clean:
	docker image prune -f
	docker container prune -f

docker-validate:
	./infra/docker/validate.sh

docker-multi:
	./infra/docker/build.sh build-multi

docker-push:
	docker push ghcr.io/$(shell git config --get remote.origin.url | sed 's/.*github.com\///' | sed 's/\.git//'):latest

# Create release archives
release: clean build-all
	mkdir -p release
	cd dist && \
	for file in sqlterm-*; do \
		if [[ $$file == *.exe ]]; then \
			zip -r ../release/$$file.zip $$file; \
		else \
			tar -czf ../release/$$file.tar.gz $$file; \
		fi; \
	done

# Show help
help:
	@echo "Available targets:"
	@echo "  build        - Build for current platform"
	@echo "  build-all    - Build for all platforms (Windows, Linux, macOS)"
	@echo "  build-windows - Build for Windows (amd64, arm64)"
	@echo "  build-linux  - Build for Linux (amd64, arm64)"
	@echo "  build-darwin - Build for macOS (amd64, arm64)"
	@echo "  test         - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  fmt          - Format code"
	@echo "  vet          - Run go vet"
	@echo "  lint         - Run golangci-lint"
	@echo "  clean        - Clean build artifacts"
	@echo "  release      - Create release archives"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo "  docker-dev   - Build development Docker image"
	@echo "  docker-dev-run - Run development container"
	@echo "  docker-compose - Start development environment"
	@echo "  docker-validate - Validate Docker configuration"
	@echo "  help         - Show this help"
