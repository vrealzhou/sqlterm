# Build stage
FROM golang:1.24-alpine AS builder

# Install git and ca-certificates for fetching dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'docker')" \
    -a -installsuffix cgo \
    -o sqlterm ./cmd/sqlterm

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS connections
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 sqlterm && \
    adduser -D -s /bin/sh -u 1000 -G sqlterm sqlterm

# Set working directory
WORKDIR /home/sqlterm

# Copy binary from builder stage
COPY --from=builder /app/sqlterm /usr/local/bin/sqlterm

# Change ownership to non-root user
RUN chown -R sqlterm:sqlterm /home/sqlterm

# Switch to non-root user
USER sqlterm

# Expose any necessary ports (if needed for future web interface)
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["sqlterm"]
