#!/bin/bash

set -Eeuo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

PANEL_CONTAINER_NAME=${PANEL_CONTAINER_NAME:-"v-panel"}
PANEL_HEALTH_URL=${PANEL_HEALTH_URL:-"http://127.0.0.1:8080/health"}
BACKUP_DIR=${BACKUP_DIR:-".reports"}
PANEL_BINARY=${PANEL_BINARY:-"vpanel.static"}
LOCAL_PANEL_BINARY=${LOCAL_PANEL_BINARY:-"vpanel"}
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
DEPLOY_ROLLBACK_ARMED=0
DEPLOY_BINARY_BACKUP=""
DEPLOY_WEB_BACKUP=""

log_info() {
    echo -e "${GREEN}$1${NC}"
}

log_warn() {
    echo -e "${YELLOW}$1${NC}"
}

log_error() {
    echo -e "${RED}$1${NC}"
}

docker_container_running() {
    command -v docker >/dev/null 2>&1 && docker ps --format '{{.Names}}' | grep -Fxq "$PANEL_CONTAINER_NAME"
}

auto_rollback_on_error() {
    local exit_code=$?

    if [ "${DEPLOY_ROLLBACK_ARMED:-0}" != "1" ]; then
        exit "$exit_code"
    fi

    DEPLOY_ROLLBACK_ARMED=0
    trap - ERR

    log_error "✗ 部署过程中发生错误，开始自动回滚"
    rollback_container_panel "$DEPLOY_BINARY_BACKUP" "$DEPLOY_WEB_BACKUP" || true
    log_error "✗ 已回滚，请检查服务日志"
    exit "$exit_code"
}

wait_for_health() {
    local url=$1
    local attempts=${2:-30}
    local interval=${3:-2}

    for _ in $(seq 1 "$attempts"); do
        if curl -fsS "$url" >/dev/null 2>&1; then
            return 0
        fi
        sleep "$interval"
    done

    return 1
}

build_frontend() {
    if [ ! -d "web" ]; then
        return 0
    fi

    log_warn "构建前端..."
    (
        cd web
        npm run build
    )
}

build_panel_binary() {
    log_warn "静态编译 Panel..."
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o "$PANEL_BINARY" ./cmd/v
}

backup_container_binary() {
    mkdir -p "$BACKUP_DIR"
    local backup_path="$BACKUP_DIR/v-panel-backup-${TIMESTAMP}"
    docker cp "${PANEL_CONTAINER_NAME}:/app/v-panel" "$backup_path"
    echo "$backup_path"
}

backup_container_web() {
    if [ ! -d "web/dist" ]; then
        return 0
    fi

    mkdir -p "$BACKUP_DIR"
    local backup_path="$BACKUP_DIR/v-panel-web-${TIMESTAMP}.tar.gz"
    local container_tmp="/tmp/v-panel-web-${TIMESTAMP}.tar.gz"

    docker exec "$PANEL_CONTAINER_NAME" sh -lc "tar -czf '$container_tmp' -C /app/web dist"
    docker cp "${PANEL_CONTAINER_NAME}:${container_tmp}" "$backup_path"
    docker exec "$PANEL_CONTAINER_NAME" rm -f "$container_tmp"
    echo "$backup_path"
}

restore_container_web() {
    local backup_path=$1

    if [ -z "$backup_path" ] || [ ! -f "$backup_path" ]; then
        return 0
    fi

    local container_tmp="/tmp/rollback-web-${TIMESTAMP}.tar.gz"
    docker cp "$backup_path" "${PANEL_CONTAINER_NAME}:${container_tmp}"
    docker exec "$PANEL_CONTAINER_NAME" sh -lc "rm -rf /app/web/dist && tar -xzf '$container_tmp' -C /app/web && rm -f '$container_tmp'"
}

rollback_container_panel() {
    local binary_backup=$1
    local web_backup=${2:-}

    log_warn "执行回滚..."
    docker cp "$binary_backup" "${PANEL_CONTAINER_NAME}:/app/v-panel"
    restore_container_web "$web_backup"
    docker restart "$PANEL_CONTAINER_NAME" >/dev/null
    wait_for_health "$PANEL_HEALTH_URL" 30 2
}

deploy_panel_container() {
    log_info "检测到 Docker 部署，开始容器内更新..."

    build_frontend
    build_panel_binary

    local binary_backup
    local web_backup
    binary_backup=$(backup_container_binary)
    web_backup=$(backup_container_web || true)
    DEPLOY_BINARY_BACKUP=$binary_backup
    DEPLOY_WEB_BACKUP=$web_backup
    DEPLOY_ROLLBACK_ARMED=1
    trap auto_rollback_on_error ERR

    log_info "二进制备份: $binary_backup"
    if [ -n "$web_backup" ]; then
        log_info "前端备份: $web_backup"
    fi

    if [ -d "web/dist" ]; then
        docker exec "$PANEL_CONTAINER_NAME" sh -lc "mkdir -p /app/web/dist"
        docker cp web/dist/. "${PANEL_CONTAINER_NAME}:/app/web/dist/"
    fi
    docker cp "$PANEL_BINARY" "${PANEL_CONTAINER_NAME}:/app/v-panel"
    docker restart "$PANEL_CONTAINER_NAME" >/dev/null

    if ! wait_for_health "$PANEL_HEALTH_URL" 30 2; then
        DEPLOY_ROLLBACK_ARMED=0
        trap - ERR
        log_error "✗ 部署后健康检查失败，开始自动回滚"
        rollback_container_panel "$binary_backup" "$web_backup" || true
        log_error "✗ 已回滚，请检查服务日志"
        return 1
    fi

    DEPLOY_ROLLBACK_ARMED=0
    trap - ERR
    log_info "✓ Panel 容器部署成功"
    log_info "健康检查: $PANEL_HEALTH_URL"
}

deploy_panel_local() {
    log_info "未检测到 Docker 容器，使用本地二进制部署..."

    local backup_path=""
    build_frontend

    if [ -f "$LOCAL_PANEL_BINARY" ]; then
        mkdir -p "$BACKUP_DIR"
        backup_path="$BACKUP_DIR/${LOCAL_PANEL_BINARY}.backup.${TIMESTAMP}"
        cp "$LOCAL_PANEL_BINARY" "$backup_path"
        log_info "本地二进制备份: $backup_path"
    fi

    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o "$LOCAL_PANEL_BINARY" ./cmd/v
    log_info "✓ 本地 Panel 已编译为 $LOCAL_PANEL_BINARY"
    log_warn "本地模式不会自动重启已有进程，请按需执行 ./scripts/deploy/start.sh restart"
}

rollback_panel() {
    local binary_backup=${2:-}
    local web_backup=${3:-}

    if [ -z "$binary_backup" ]; then
        log_error "错误: 需要提供二进制备份路径"
        echo "用法: $0 rollback-panel <binary-backup> [web-backup]"
        exit 1
    fi

    if docker_container_running; then
        rollback_container_panel "$binary_backup" "$web_backup"
        log_info "✓ 容器回滚完成"
        return
    fi

    cp "$binary_backup" "$LOCAL_PANEL_BINARY"
    log_info "✓ 本地二进制回滚完成，请手动重启服务"
}

deploy_panel() {
    if docker_container_running; then
        deploy_panel_container
    else
        deploy_panel_local
    fi
}

deploy_agent() {
    local panel_url=${1:-}
    local node_token=${2:-}
    local agent_port=${3:-${AGENT_HEALTH_PORT:-18443}}

    if [ -z "$panel_url" ] || [ -z "$node_token" ]; then
        log_error "错误: 需要提供 Panel URL 和 Token"
        echo "用法: $0 agent <panel-url> <token> [agent-port]"
        exit 1
    fi

    log_info "部署 Agent..."
    echo "Panel URL: $panel_url"
    echo "Agent Port: $agent_port"

    if [ ! -f "vpanel-agent" ]; then
        log_warn "编译 Agent..."
        go build -o vpanel-agent ./cmd/agent/main.go
    fi

    sudo mkdir -p /etc/vpanel
    sudo mkdir -p /var/log/vpanel

    log_warn "创建 Agent 配置..."
    sudo tee /etc/vpanel/agent.yaml >/dev/null <<EOF
panel:
  url: "$panel_url"
  token: "$node_token"

xray:
  binary_path: "/usr/local/bin/xray"
  config_path: "/usr/local/etc/xray/config.json"

sync:
  interval: 5m
  validate_before_apply: true
  backup_before_apply: true

health:
  port: $agent_port
EOF

    log_warn "安装 Agent 二进制..."
    sudo cp vpanel-agent /usr/local/bin/
    sudo chmod +x /usr/local/bin/vpanel-agent

    if ! command -v xray >/dev/null 2>&1; then
        log_warn "安装 Xray..."
        bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install
    else
        log_info "Xray 已安装"
    fi

    log_warn "创建 systemd 服务..."
    sudo tee /etc/systemd/system/vpanel-agent.service >/dev/null <<EOF
[Unit]
Description=V Panel Agent
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/vpanel-agent -config /etc/vpanel/agent.yaml
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

    log_warn "启动 Agent 服务..."
    sudo systemctl daemon-reload
    sudo systemctl enable vpanel-agent
    sudo systemctl restart vpanel-agent

    log_info "✓ Agent 部署完成"
    echo "查看状态: sudo systemctl status vpanel-agent"
    echo "查看日志: sudo journalctl -u vpanel-agent -f"
}

show_help() {
    echo "V Panel 快速部署脚本"
    echo ""
    echo "用法:"
    echo "  $0 panel                                部署 Panel"
    echo "  $0 rollback-panel <bin-backup> [web]    回滚 Panel"
    echo "  $0 agent <panel-url> <token> [agent-port] 部署 Agent"
    echo "  $0 all                                  部署 Panel，并提示 Agent 命令"
    echo ""
    echo "说明:"
    echo "  - Docker 模式下会自动备份 /app/v-panel 和 /app/web/dist"
    echo "  - 部署后会执行健康检查，失败时自动回滚"
}

case "${1:-}" in
    panel)
        deploy_panel
        ;;
    rollback-panel)
        rollback_panel "$@"
        ;;
    agent)
        deploy_agent "${2:-}" "${3:-}" "${4:-}"
        ;;
    all)
        deploy_panel
        echo ""
        log_warn "请在节点服务器执行:"
        echo "$0 agent <panel-url> <token> [agent-port]"
        ;;
    help|--help|-h|"")
        show_help
        ;;
    *)
        show_help
        exit 1
        ;;
esac
