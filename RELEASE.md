# SQLTerm Release Guide

## Overview
This document provides instructions for building and releasing SQLTerm across multiple platforms.

## Supported Platforms
- **Linux**: amd64, arm64
- **macOS**: amd64 (Intel), arm64 (Apple Silicon)
- **Windows**: amd64, arm64

## Building from Source

### Prerequisites
- Go 1.24 or later
- Git

### Local Build
```bash
# Clone the repository
git clone <repository-url>
cd sqlterm

# Build for current platform
make build

# Build for all platforms
make build-all

# Create release archives
make release
```

### Platform-Specific Builds
```bash
# Linux
make build-linux

# macOS
make build-darwin

# Windows
make build-windows
```

## Using GitHub Actions (Recommended)

### Automatic Releases
1. Create and push a new tag:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. GitHub Actions will automatically:
   - Run tests
   - Build for all platforms
   - Create GitHub release
   - Upload release artifacts

### Manual Workflow Dispatch
Go to Actions → Build and Release → Run workflow

## Downloading Releases

### From GitHub Releases
1. Visit [Releases](../../releases)
2. Download the appropriate binary for your platform:
   - **Linux**: `sqlterm-linux-amd64.tar.gz` or `sqlterm-linux-arm64.tar.gz`
   - **macOS**: `sqlterm-darwin-amd64.tar.gz` or `sqlterm-darwin-arm64.tar.gz`
   - **Windows**: `sqlterm-windows-amd64.zip` or `sqlterm-windows-arm64.zip`

### Installation
```bash
# Linux/macOS
tar -xzf sqlterm-linux-amd64.tar.gz
sudo mv sqlterm /usr/local/bin/

# Windows
# Extract zip file and add to PATH
```

## Docker Usage

### Pull from GitHub Container Registry
```bash
# Pull latest
docker pull ghcr.io/your-username/sqlterm:latest

# Run
docker run -it --rm ghcr.io/your-username/sqlterm:latest
```

### Build Locally
```bash
docker build -t sqlterm .
docker run -it --rm sqlterm
```

## Release Checklist

### Before Release
- [ ] All tests pass (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] No linting issues (`make lint`)
- [ ] Update version in code if needed
- [ ] Update README with new features
- [ ] Update CHANGELOG.md

### During Release
- [ ] Create release tag
- [ ] Verify GitHub Actions complete successfully
- [ ] Check release artifacts are uploaded
- [ ] Test downloaded binaries on target platforms

### After Release
- [ ] Update documentation links
- [ ] Announce release
- [ ] Monitor for any issues

## File Structure
```
sqlterm/
├── .github/workflows/build-release.yml  # CI/CD configuration
├── Dockerfile                           # Docker build configuration
├── Makefile                            # Build automation
├── dist/                               # Built binaries (local)
├── release/                            # Release archives (local)
└── bin/                                # Current platform binary
```

## Troubleshooting

### Build Issues
```bash
# Clean and rebuild
make clean && make build

# Update dependencies
go mod tidy
```

### Cross-Compilation Issues
- Ensure CGO is disabled for static binaries
- Check Go version compatibility
- Verify target platform support

### Docker Issues
```bash
# Build with no cache
docker build --no-cache -t sqlterm .

# Check image layers
docker history sqlterm
```

## Versioning
We use [Semantic Versioning](https://semver.org/):
- **MAJOR**: Incompatible API changes
- **MINOR**: New functionality (backwards compatible)
- **PATCH**: Bug fixes (backwards compatible)

Tags format: `v1.0.0`, `v1.1.0`, `v1.0.1`, etc.