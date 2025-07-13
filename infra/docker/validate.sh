#!/bin/bash

# SQLTerm Docker Configuration Validation Script
# Validates that Docker configuration works with new directory structure

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() {
    echo -e "${GREEN}[$(date +'%H:%M:%S')] $1${NC}"
}

error() {
    echo -e "${RED}[$(date +'%H:%M:%S')] ERROR: $1${NC}"
    exit 1
}

warn() {
    echo -e "${YELLOW}[$(date +'%H:%M:%S')] WARNING: $1${NC}"
}

# Check if running from correct directory
check_directory() {
    if [ ! -f "go.mod" ] || [ ! -d "infra/docker" ]; then
        error "Please run this script from the SQLTerm project root directory"
    fi
    log "✅ Running from correct directory"
}

# Validate Dockerfile
validate_dockerfile() {
    if [ ! -f "infra/docker/Dockerfile" ]; then
        error "Dockerfile not found at infra/docker/Dockerfile"
    fi

    # Check Dockerfile syntax
    if ! docker run --rm -i hadolint/hadolint < infra/docker/Dockerfile 2>/dev/null; then
        warn "Dockerfile has linting warnings (non-critical)"
    fi

    log "✅ Dockerfile validation passed"
}

# Validate docker-compose
validate_compose() {
    if [ ! -f "infra/docker/docker-compose.yml" ]; then
        error "docker-compose.yml not found at infra/docker/docker-compose.yml"
    fi

    # Validate syntax
    if ! docker-compose -f infra/docker/docker-compose.yml config >/dev/null 2>&1; then
        error "docker-compose.yml has syntax errors"
    fi

    log "✅ docker-compose.yml validation passed"
}

# Test build context
test_build_context() {
    log "Testing Docker build context..."

    # Test if Dockerfile can access correct context
    if ! docker build -f infra/docker/Dockerfile . --target builder --quiet >/dev/null 2>&1; then
        error "Docker build context test failed"
    fi

    log "✅ Docker build context test passed"
}

# Test .dockerignore
test_dockerignore() {
    log "Testing .dockerignore..."

    # Check if .dockerignore exists and includes infra/docker
    if ! grep -q "infra/docker" .dockerignore 2>/dev/null; then
        warn ".dockerignore might not exclude infra/docker properly"
    fi

    # Check if .dockerignore excludes build artifacts
    if ! grep -q "dist/" .dockerignore 2>/dev/null; then
        warn ".dockerignore might not exclude dist/ directory"
    fi

    log "✅ .dockerignore validation passed"
}

# Test build script
test_build_script() {
    if [ ! -x "infra/docker/build.sh" ]; then
        error "build.sh is not executable"
    fi

    # Test build script syntax
    if ! bash -n infra/docker/build.sh; then
        error "build.sh has syntax errors"
    fi

    log "✅ build.sh validation passed"
}

# Test GitHub Actions integration
test_github_actions() {
    if [ ! -f ".github/workflows/build-release.yml" ]; then
        warn "GitHub Actions workflow not found"
        return 0
    fi

    # Check if workflow references correct Dockerfile path
    if ! grep -q "infra/docker/Dockerfile" .github/workflows/build-release.yml; then
        error "GitHub Actions workflow doesn't reference correct Dockerfile path"
    fi

    log "✅ GitHub Actions integration validated"
}

# Run all validations
main() {
    log "🔍 Starting Docker configuration validation..."

    check_directory
    validate_dockerfile
    validate_compose
    test_build_context
    test_dockerignore
    test_build_script
    test_github_actions

    log "🎉 All validations passed! Docker configuration is ready."
    log ""
    log "Next steps:"
    log "  1. Build development image: ./infra/docker/build.sh build-dev"
    log "  2. Run development container: ./infra/docker/build.sh run-dev"
    log "  3. Test with docker-compose: docker-compose -f infra/docker/docker-compose.yml up"
}

# Run main function
main "$@"
