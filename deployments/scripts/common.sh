#!/bin/bash

SCRIPT_DIR_COMMON="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR_COMMON/../.." && pwd)"
DOCKER_DIR="$PROJECT_ROOT/deployments/docker"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

compose_available() {
    docker compose version >/dev/null 2>&1 || command -v docker-compose >/dev/null 2>&1
}

docker_compose_cmd() {
    if docker compose version >/dev/null 2>&1; then
        docker compose "$@"
    else
        docker-compose "$@"
    fi
}

compose_has_vpanel_service() {
    local current_dir
    current_dir="$(pwd)"

    cd "$DOCKER_DIR" 2>/dev/null || return 1
    local result=1

    if docker_compose_cmd ps -a --services 2>/dev/null | grep -qx "v-panel"; then
        result=0
    fi

    cd "$current_dir" 2>/dev/null || true
    return $result
}

remove_stale_vpanel_container() {
    if ! docker ps -a --format '{{.Names}}' 2>/dev/null | grep -qx 'v-panel'; then
        return 0
    fi

    if compose_has_vpanel_service; then
        return 0
    fi

    echo -e "${YELLOW}检测到遗留的同名容器 v-panel，正在自动清理...${NC}"
    docker rm -f v-panel >/dev/null 2>&1 || return 1
    return 0
}

require_docker() {
    if ! command -v docker >/dev/null 2>&1; then
        echo -e "${RED}错误: Docker 未安装，请先安装 Docker${NC}"
        exit 1
    fi
}

require_compose() {
    if ! compose_available; then
        echo -e "${RED}错误: Docker Compose 未安装${NC}"
        exit 1
    fi
}

read_env_var() {
    local var_name=$1
    local env_file=$2
    grep "^${var_name}=" "$env_file" 2>/dev/null | head -n1 | cut -d'=' -f2- | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

panel_access_url() {
    local env_file=$1
    local public_url
    local publish_port

    public_url=$(read_env_var "V_SERVER_PUBLIC_URL" "$env_file")
    publish_port=$(read_env_var "VPANEL_PUBLISH_PORT" "$env_file")
    publish_port=${publish_port:-8080}

    if [ -n "$public_url" ]; then
        echo "$public_url"
    else
        echo "http://localhost:${publish_port}"
    fi
}

is_default_jwt_secret() {
    case "$1" in
        ""|"CHANGE_ME_OR_AUTO_GENERATE_ON_FIRST_START"|"CHANGE_ME_OR_SYSTEM_WILL_REFUSE_TO_START"|"your-secure-jwt-secret-change-me"|"change-me-in-production")
            return 0
            ;;
    esac
    return 1
}

is_default_admin_password() {
    case "$1" in
        ""|"CHANGE_ME_OR_AUTO_GENERATE_ON_FIRST_START"|"CHANGE_ME_OR_SYSTEM_WILL_REFUSE_TO_START"|"admin123"|"your-secure-admin-password")
            return 0
            ;;
    esac
    return 1
}

validate_password() {
    local password=$1
    local min_length=12

    [ "${#password}" -ge "${min_length}" ] || return 1
    echo "$password" | grep -q '[A-Z]' || return 1
    echo "$password" | grep -q '[a-z]' || return 1
    echo "$password" | grep -q '[0-9]' || return 1
    echo "$password" | grep -q '[^A-Za-z0-9]' || return 1
}

validate_jwt_secret() {
    local secret=$1
    local min_length=32

    [ "${#secret}" -ge "${min_length}" ] || return 1
    ! is_default_jwt_secret "$secret"
}

generate_strong_password() {
    if command -v openssl >/dev/null 2>&1; then
        local password
        password="$(openssl rand -base64 24 | tr -d '=+/' | cut -c1-16)"
        echo "${password}@$(openssl rand -hex 2 | cut -c1-2)!"
        return
    fi

    LC_ALL=C tr -dc 'A-Za-z0-9!@#$%^&*' < /dev/urandom | head -c 16
    echo "!@"
}

generate_jwt_secret() {
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -base64 48 | tr -d "\n"
        return
    fi

    LC_ALL=C tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 48
}

check_container_status() {
    local current_dir
    current_dir="$(pwd)"

    cd "$DOCKER_DIR" 2>/dev/null || return 1
    local result=1

    if docker_compose_cmd ps 2>/dev/null | grep -q "v-panel.*Up"; then
        result=0
    fi

    cd "$current_dir" 2>/dev/null || true
    return $result
}

vpanel_container_id() {
    local current_dir
    current_dir="$(pwd)"

    cd "$DOCKER_DIR" 2>/dev/null || return 1
    local container_id
    container_id="$(docker_compose_cmd ps -q v-panel 2>/dev/null | head -n1)"
    cd "$current_dir" 2>/dev/null || true

    [ -n "$container_id" ] || return 1
    echo "$container_id"
}

show_volume_backup_hint() {
    cat <<'EOF'
  配置卷: docker run --rm -v v-panel-config:/source -v "$(pwd)":/backup alpine tar czf /backup/v-panel-config-$(date +%Y%m%d-%H%M%S).tar.gz -C /source .
  数据卷: docker run --rm -v v-panel-data:/source -v "$(pwd)":/backup alpine tar czf /backup/v-panel-data-$(date +%Y%m%d-%H%M%S).tar.gz -C /source .
  日志卷: docker run --rm -v v-panel-logs:/source -v "$(pwd)":/backup alpine tar czf /backup/v-panel-logs-$(date +%Y%m%d-%H%M%S).tar.gz -C /source .
  Xray卷: docker run --rm -v v-panel-xray:/source -v "$(pwd)":/backup alpine tar czf /backup/v-panel-xray-$(date +%Y%m%d-%H%M%S).tar.gz -C /source .
EOF
}
