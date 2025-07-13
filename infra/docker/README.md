# SQLTerm Docker Configuration

This directory contains Docker configuration files for building and running SQLTerm in containerized environments.

## Directory Structure
```
infra/docker/
├── Dockerfile          # Main production Dockerfile
├── docker-compose.yml  # Development environment with databases
├── build.sh           # Build script for various scenarios
└── README.md          # This documentation
```

## Quick Start

### Production Build
```bash
# Build single platform
./infra/docker/build.sh build-single linux/amd64 local

# Build multi-platform
./infra/docker/build.sh build-multi

# Build development image
./infra/docker/build.sh build-dev
```

### Using Docker Compose
```bash
# Start SQLTerm with PostgreSQL and MySQL for testing
docker-compose -f infra/docker/docker-compose.yml up

# Run SQLTerm only
docker-compose -f infra/docker/docker-compose.yml run sqlterm

# Start with specific database
docker-compose -f infra/docker/docker-compose.yml up postgres sqlterm
```

## Manual Docker Commands

### Build Production Image
```bash
# From project root
docker build -t sqlterm:latest -f infra/docker/Dockerfile .

# With specific version
docker build -t sqlterm:v1.0.0 -f infra/docker/Dockerfile .
```

### Run Container
```bash
# Interactive mode
docker run -it --rm sqlterm:latest

# With volume for SQL files
docker run -it --rm \
  -v $(pwd):/workspace \
  -v sqlterm-config:/home/sqlterm/.config/sqlterm \
  sqlterm:latest

# With mounted database
docker run -it --rm \
  --network host \
  sqlterm:latest
```

## Development Setup

### Prerequisites
- Docker and Docker Compose
- Make (optional, for convenience)

### Environment Variables
- `VERSION`: Image version (default: git tag or 'latest')
- `REGISTRY`: Container registry (default: 'ghcr.io')
- `PLATFORMS`: Build platforms (default: 'linux/amd64,linux/arm64')

### Example Workflows

#### 1. Development with PostgreSQL
```bash
# Start full development environment
docker-compose -f infra/docker/docker-compose.yml up postgres sqlterm

# SQLTerm will be available in the sqlterm container
# Connect to PostgreSQL: host=postgres, user=testuser, password=testpass, db=testdb
```

#### 2. Testing with MySQL
```bash
# Start MySQL and SQLTerm
docker-compose -f infra/docker/docker-compose.yml up mysql sqlterm

# Connect to MySQL: host=mysql, user=testuser, password=testpass, db=testdb
```

#### 3. SQLite Development
```bash
# Start SQLTerm with SQLite volume
docker-compose -f infra/docker/docker-compose.yml up sqlterm

# SQLite files will be stored in the sqlite-data volume
```

## Build Script Usage

The `build.sh` script provides convenient commands:

```bash
# Show help
./infra/docker/build.sh help

# Build development image
./infra/docker/build.sh build-dev

# Run development container
./infra/docker/build.sh run-dev

# Build for specific platform
./infra/docker/build.sh build-single linux/amd64 my-tag

# Build multi-platform
./infra/docker/build.sh build-multi
```

## Volume Mounts

### Persistent Configuration
```bash
# Mount config directory for persistent settings
docker run -it --rm \
  -v sqlterm-config:/home/sqlterm/.config/sqlterm \
  sqlterm:latest
```

### Working with SQL Files
```bash
# Mount current directory for SQL files
docker run -it --rm \
  -v $(pwd):/workspace \
  -w /workspace \
  sqlterm:latest
```

## Networking

### Docker Compose Network
The `docker-compose.yml` creates a custom network `sqlterm-network` for inter-container communication.

### Database Connections
- **PostgreSQL**: `host=postgres, port=5432`
- **MySQL**: `host=mysql, port=3306`
- **SQLite**: Use mounted volumes

## Troubleshooting

### Build Issues
```bash
# Clean build cache
docker build --no-cache -f infra/docker/Dockerfile .

# Check build context
docker build -f infra/docker/Dockerfile . --progress=plain
```

### Runtime Issues
```bash
# Check logs
docker logs sqlterm-dev

# Interactive debugging
docker run -it --rm --entrypoint /bin/sh sqlterm:latest
```

### Permission Issues
```bash
# Ensure build script is executable
chmod +x infra/docker/build.sh

# Fix file permissions
docker run -it --rm \
  -u root \
  sqlterm:latest \
  chown -R sqlterm:sqlterm /home/sqlterm
```

## CI/CD Integration

### GitHub Actions
The GitHub Actions workflow automatically builds and pushes Docker images to GitHub Container Registry:
- `ghcr.io/username/sqlterm:latest`
- `ghcr.io/username/sqlterm:v1.0.0`

### Manual Registry Push
```bash
# Build and push to registry
./infra/docker/build.sh build-multi
docker tag sqlterm:latest ghcr.io/yourusername/sqlterm:latest
docker push ghcr.io/yourusername/sqlterm:latest
```

## Security Notes

- Runs as non-root user (sqlterm:sqlterm)
- Uses minimal Alpine base image
- Includes only necessary dependencies
- No secrets or sensitive data in image

## Performance Tips

- Use multi-stage builds to reduce image size
- Leverage build cache for faster builds
- Use specific tags instead of latest for reproducibility
- Consider using BuildKit for improved performance