#!/bin/bash
# V Panel Agent Start Script

set -e

# Configuration
APP_NAME="v-agent"
CONFIG_FILE="${CONFIG_FILE:-configs/agent.yaml}"
PID_FILE="./data/v-agent.pid"
LOG_FILE="./data/v-agent.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Find the binary
find_binary() {
    if [ -f "./build/$APP_NAME" ]; then
        echo "./build/$APP_NAME"
    elif [ -f "./$APP_NAME" ]; then
        echo "./$APP_NAME"
    elif [ -f "./agent" ]; then
        echo "./agent"
    else
        echo ""
    fi
}

# Check if process is running
is_running() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null 2>&1; then
            return 0
        fi
    fi
    return 1
}

# Build agent
build() {
    log_info "Building V Agent..."
    
    LDFLAGS="-s -w"
    
    CGO_ENABLED=0 go build \
        -ldflags="$LDFLAGS" \
        -o "./build/$APP_NAME" \
        ./cmd/agent/main.go
    
    log_info "Agent built: ./build/$APP_NAME"
}

# Start the agent
start() {
    log_info "Starting V Agent..."
    
    if is_running; then
        log_warn "V Agent is already running (PID: $(cat $PID_FILE))"
        return 1
    fi
    
    BINARY=$(find_binary)
    if [ -z "$BINARY" ]; then
        log_warn "Binary not found, building..."
        build
        BINARY="./build/$APP_NAME"
    fi
    
    if [ ! -f "$CONFIG_FILE" ]; then
        log_error "Config file not found: $CONFIG_FILE"
        log_info "Creating default config from example..."
        if [ -f "configs/agent.yaml.example" ]; then
            cp configs/agent.yaml.example "$CONFIG_FILE"
            log_warn "Please edit $CONFIG_FILE with your Panel address and token"
            exit 1
        else
            log_error "No config example found"
            exit 1
        fi
    fi
    
    # Create data directory
    mkdir -p ./data
    
    # Start in background
    nohup "$BINARY" -config "$CONFIG_FILE" > "$LOG_FILE" 2>&1 &
    echo $! > "$PID_FILE"
    
    sleep 2
    
    if is_running; then
        log_info "V Agent started successfully (PID: $(cat $PID_FILE))"
        log_info "Log file: $LOG_FILE"
    else
        log_error "Failed to start V Agent. Check log: $LOG_FILE"
        exit 1
    fi
}

# Stop the agent
stop() {
    log_info "Stopping V Agent..."
    
    if ! is_running; then
        log_warn "V Agent is not running"
        return 0
    fi
    
    PID=$(cat "$PID_FILE")
    kill "$PID"
    
    # Wait for process to stop
    for i in {1..10}; do
        if ! is_running; then
            rm -f "$PID_FILE"
            log_info "V Agent stopped"
            return 0
        fi
        sleep 1
    done
    
    # Force kill if still running
    kill -9 "$PID" 2>/dev/null || true
    rm -f "$PID_FILE"
    log_info "V Agent stopped (forced)"
}

# Restart the agent
restart() {
    stop
    sleep 2
    start
}

# Show status
status() {
    if is_running; then
        PID=$(cat "$PID_FILE")
        log_info "V Agent is running (PID: $PID)"
    else
        log_info "V Agent is not running"
    fi
}

# Run in foreground (for development)
run() {
    log_info "Running V Agent in foreground..."
    
    BINARY=$(find_binary)
    if [ -z "$BINARY" ]; then
        log_info "Binary not found, running with go run..."
        go run ./cmd/agent/main.go -config "$CONFIG_FILE"
    else
        "$BINARY" -config "$CONFIG_FILE"
    fi
}

# Show logs
logs() {
    if [ -f "$LOG_FILE" ]; then
        tail -f "$LOG_FILE"
    else
        log_warn "Log file not found: $LOG_FILE"
    fi
}

# Show help
help() {
    echo "V Panel Agent Start Script"
    echo ""
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  build     Build the agent binary"
    echo "  start     Start V Agent in background"
    echo "  stop      Stop V Agent"
    echo "  restart   Restart V Agent"
    echo "  status    Show running status"
    echo "  run       Run in foreground (for development)"
    echo "  logs      Show and follow logs"
    echo "  help      Show this help"
    echo ""
    echo "Environment variables:"
    echo "  CONFIG_FILE   Config file path (default: configs/agent.yaml)"
}

# Main
case "${1:-help}" in
    build)
        build
        ;;
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    status)
        status
        ;;
    run)
        run
        ;;
    logs)
        logs
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
