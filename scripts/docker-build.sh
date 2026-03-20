#!/bin/bash
# V Panel Docker Build Script

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$SCRIPT_DIR/../deployments/scripts/common.sh"

IMAGE_NAME="${IMAGE_NAME:-v-panel}"
VERSION="${VERSION:-latest}"
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Build Docker image
build() {
    require_docker
    log_info "Building Docker image: ${IMAGE_NAME}:${VERSION}"
    
    docker build \
        --build-arg VERSION="$VERSION" \
        --build-arg BUILD_TIME="$BUILD_TIME" \
        --build-arg GIT_COMMIT="$GIT_COMMIT" \
        -t "${IMAGE_NAME}:${VERSION}" \
        -t "${IMAGE_NAME}:latest" \
        -f deployments/docker/Dockerfile \
        .
    
    log_info "Docker image built successfully"
}

# Build multi-platform image
build_multiplatform() {
    require_docker
    log_info "Building multi-platform Docker image..."
    
    docker buildx build \
        --platform linux/amd64,linux/arm64 \
        --build-arg VERSION="$VERSION" \
        --build-arg BUILD_TIME="$BUILD_TIME" \
        --build-arg GIT_COMMIT="$GIT_COMMIT" \
        -t "${IMAGE_NAME}:${VERSION}" \
        -t "${IMAGE_NAME}:latest" \
        -f deployments/docker/Dockerfile \
        --push \
        .
    
    log_info "Multi-platform Docker image built and pushed"
}

# Run with Docker deployment scripts
run() {
    require_docker
    require_compose
    log_info "Starting V Panel with deployment start script..."
    "$PROJECT_ROOT/deployments/scripts/start.sh" start
}

# Stop Docker deployment
stop() {
    require_docker
    require_compose
    log_info "Stopping V Panel..."
    "$PROJECT_ROOT/deployments/scripts/start.sh" stop
    log_info "V Panel stopped"
}

# View logs
logs() {
    require_docker
    require_compose
    "$PROJECT_ROOT/deployments/scripts/start.sh" logs
}

# Clean up
clean() {
    require_docker
    require_compose
    log_info "Cleaning up Docker resources..."
    
    cd "$DOCKER_DIR"
    docker_compose_cmd down -v 2>/dev/null || true
    
    # Remove images
    docker rmi "${IMAGE_NAME}:${VERSION}" 2>/dev/null || true
    docker rmi "${IMAGE_NAME}:latest" 2>/dev/null || true
    
    log_info "Cleanup complete"
}

# Show help
help() {
    echo "V Panel Docker Build Script"
    echo ""
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  build           Build Docker image"
    echo "  multiplatform   Build multi-platform image (requires buildx)"
    echo "  run             Start via deployments/scripts/start.sh"
    echo "  stop            Stop deployed containers"
    echo "  logs            View container logs"
    echo "  clean           Clean up Docker resources"
    echo "  help            Show this help"
    echo ""
    echo "Environment variables:"
    echo "  VERSION         Image version tag (default: latest)"
    echo "  IMAGE_NAME      Image name (default: v-panel)"
}

# Main
case "${1:-build}" in
    build)
        build
        ;;
    multiplatform)
        build_multiplatform
        ;;
    run)
        run
        ;;
    stop)
        stop
        ;;
    logs)
        logs
        ;;
    clean)
        clean
        ;;
    help|--help|-h)
        help
        ;;
    *)
        log_error "Unknown command: $1"
        help
        exit 1
        ;;
esac
