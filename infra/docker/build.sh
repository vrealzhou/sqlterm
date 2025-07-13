#!/bin/bash

# SQLTerm Docker Build Script
# Builds Docker images for different purposes

set -e

# Configuration
IMAGE_NAME="sqlterm"
REGISTRY="ghcr.io"
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "latest")}
PLATFORMS="linux/amd64,linux/arm64"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}"
}

error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $1${NC}"
    exit 1
}

# Check dependencies
check_dependencies() {
    log "Checking dependencies..."

    if ! command -v docker &> /dev/null; then
        error "Docker is not installed"
    fi

    if ! docker buildx version &> /dev/null; then
        error "Docker Buildx is not available"
    fi

    log "Dependencies check passed"
}

# Build single platform image
build_single() {
    local platform=$1
    local tag_suffix=$2

    log "Building single platform image: $platform"

    docker build \
        --platform "$platform" \
        -t "${IMAGE_NAME}:${tag_suffix}" \
        -f infra/docker/Dockerfile \
        .

    log "✅ Single platform build completed: ${IMAGE_NAME}:${tag_suffix}"
}

# Build multi-platform image
build_multi() {
    log "Building multi-platform image: $PLATFORMS"

    docker buildx build \
        --platform "$PLATFORMS" \
        -t "${IMAGE_NAME}:${VERSION}" \
        -t "${IMAGE_NAME}:latest" \
        -f infra/docker/Dockerfile \
        --push=false \
        .

    log "✅ Multi-platform build completed"
}

# Push to registry
push_image() {
    local tag=$1

    log "Pushing image: ${REGISTRY}/${IMAGE_NAME}:${tag}"

    docker tag "${IMAGE_NAME}:${tag}" "${REGISTRY}/${IMAGE_NAME}:${tag}"
    docker push "${REGISTRY}/${IMAGE_NAME}:${tag}"

    log "✅ Image pushed: ${REGISTRY}/${IMAGE_NAME}:${tag}"
}

# Development build
build_dev() {
    log "Building development image..."

    docker build \
        -t "${IMAGE_NAME}:dev" \
        -f infra/docker/Dockerfile \
        .

    log "✅ Development image built: ${IMAGE_NAME}:dev"
}

# Run development container
run_dev() {
    log "Starting development container..."

    docker run -it --rm \
        --name sqlterm-dev \
        -v "$(pwd):/workspace" \
        -v "sqlterm-config:/home/sqlterm/.config/sqlterm" \
        -w /workspace \
        "${IMAGE_NAME}:dev"
}

# Show usage
usage() {
    echo "Usage: $0 [COMMAND] [OPTIONS]"
    echo ""
    echo "Commands:"
    echo "  build-single [platform] [tag]  Build single platform image"
    echo "  build-multi                    Build multi-platform image"
    echo "  build-dev                      Build development image"
    echo "  run-dev                        Run development container"
    echo "  push [tag]                     Push image to registry"
    echo "  help                           Show this help"
    echo ""
    echo "Examples:"
    echo "  $0 build-single linux/amd64 local"
    echo "  $0 build-multi"
    echo "  $0 build-dev"
    echo "  $0 run-dev"
    echo ""
    echo "Environment Variables:"
    echo "  VERSION    Image version (default: git tag or 'latest')"
    echo "  REGISTRY   Container registry (default: ghcr.io)"
    echo "  PLATFORMS  Build platforms (default: linux/amd64,linux/arm64)"
}

# Main execution
main() {
    check_dependencies

    case "${1:-help}" in
        "build-single")
            build_single "${2:-linux/amd64}" "${3:-local}"
            ;;
        "build-multi")
            build_multi
            ;;
        "build-dev")
            build_dev
            ;;
        "run-dev")
            run_dev
            ;;
        "push")
            push_image "${2:-latest}"
            ;;
        "help"|*)
            usage
            ;;
    esac
}

# Run main function
main "$@"
