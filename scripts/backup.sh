#!/bin/bash

set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

BACKUP_DIR=${BACKUP_DIR:-"backups"}
DB_TYPE=${DB_TYPE:-"auto"}
DB_HOST=${DB_HOST:-"localhost"}
DB_PORT=${DB_PORT:-"5432"}
DB_NAME=${DB_NAME:-"vpanel"}
DB_USER=${DB_USER:-"vpanel"}
DB_PASSWORD=${DB_PASSWORD:-""}
DB_PATH=${DB_PATH:-""}
KEEP_DAYS=${KEEP_DAYS:-7}
CONTAINER_NAME=${CONTAINER_NAME:-"v-panel"}
DOCKER_DATA_VOLUME=${DOCKER_DATA_VOLUME:-"v-panel-data"}
DOCKER_CONFIG_VOLUME=${DOCKER_CONFIG_VOLUME:-"v-panel-config"}
RESTORE_FORCE=${RESTORE_FORCE:-"0"}
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

docker_container_running() {
    command_exists docker && docker ps --format '{{.Names}}' | grep -Fxq "$CONTAINER_NAME"
}

docker_volume_mountpoint() {
    docker volume inspect "$1" --format '{{.Mountpoint}}' 2>/dev/null
}

confirm_action() {
    local prompt=$1

    if [ "$RESTORE_FORCE" = "1" ]; then
        return 0
    fi

    read -r -p "$prompt (yes/no): " confirm
    [ "$confirm" = "yes" ]
}

show_help() {
    echo "V Panel 备份脚本"
    echo ""
    echo "用法:"
    echo "  $0 database           备份数据库"
    echo "  $0 config             备份配置文件"
    echo "  $0 agent              备份 Agent 配置"
    echo "  $0 all                备份所有内容"
    echo "  $0 restore <file>     恢复备份"
    echo "  $0 clean              清理旧备份"
    echo "  $0 list               列出备份"
    echo ""
    echo "环境变量:"
    echo "  BACKUP_DIR            备份目录 (默认: backups)"
    echo "  DB_TYPE               数据库类型: auto/sqlite/postgres"
    echo "  DB_PATH               SQLite 数据库路径"
    echo "  DB_HOST               PostgreSQL 主机 (默认: localhost)"
    echo "  DB_PORT               PostgreSQL 端口 (默认: 5432)"
    echo "  DB_NAME               PostgreSQL 数据库名 (默认: vpanel)"
    echo "  DB_USER               PostgreSQL 用户名 (默认: vpanel)"
    echo "  DB_PASSWORD           PostgreSQL 密码"
    echo "  CONTAINER_NAME        运行中的容器名 (默认: v-panel)"
    echo "  RESTORE_FORCE         设为 1 时跳过恢复确认"
    echo ""
}

log_info() {
    echo -e "${GREEN}$1${NC}"
}

log_warn() {
    echo -e "${YELLOW}$1${NC}"
}

log_error() {
    echo -e "${RED}$1${NC}"
}

detect_db_type() {
    if [ "$DB_TYPE" != "auto" ]; then
        echo "$DB_TYPE"
        return
    fi

    if [ -n "$DB_PATH" ] && [ -f "$DB_PATH" ]; then
        echo "sqlite"
        return
    fi

    if [ -f "./data/v.db" ]; then
        echo "sqlite"
        return
    fi

    if docker_container_running; then
        local container_db_path
        container_db_path=$(docker exec "$CONTAINER_NAME" sh -lc 'printf "%s" "${V_DB_PATH:-/app/data/v.db}"')
        if docker exec "$CONTAINER_NAME" sh -lc "[ -f \"$container_db_path\" ]"; then
            echo "sqlite"
            return
        fi
    fi

    echo "postgres"
}

detect_sqlite_source() {
    if [ -n "$DB_PATH" ] && [ -f "$DB_PATH" ]; then
        echo "local:$DB_PATH"
        return
    fi

    if [ -f "./data/v.db" ]; then
        echo "local:./data/v.db"
        return
    fi

    if docker_container_running; then
        local container_db_path
        container_db_path=$(docker exec "$CONTAINER_NAME" sh -lc 'printf "%s" "${V_DB_PATH:-/app/data/v.db}"')
        if docker exec "$CONTAINER_NAME" sh -lc "[ -f \"$container_db_path\" ]"; then
            echo "container:$container_db_path"
            return
        fi
    fi

    return 1
}

backup_database_postgres() {
    log_warn "备份 PostgreSQL 数据库..."
    mkdir -p "$BACKUP_DIR/database"

    local backup_file="$BACKUP_DIR/database/vpanel_db_${TIMESTAMP}.sql"

    if [ -n "$DB_PASSWORD" ]; then
        export PGPASSWORD="$DB_PASSWORD"
    fi

    echo "备份到: $backup_file"
    if pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -F p -f "$backup_file"; then
        gzip -f "$backup_file"
        gzip -t "${backup_file}.gz"
        log_info "✓ PostgreSQL 数据库备份成功: ${backup_file}.gz"
    else
        log_error "✗ PostgreSQL 数据库备份失败"
        return 1
    fi

    unset PGPASSWORD
}

backup_database_sqlite() {
    log_warn "备份 SQLite 数据库..."
    mkdir -p "$BACKUP_DIR/database"

    local source
    source=$(detect_sqlite_source) || {
        log_error "✗ 未找到 SQLite 数据库"
        return 1
    }

    local source_type=${source%%:*}
    local source_path=${source#*:}
    local backup_file="$BACKUP_DIR/database/vpanel_db_${TIMESTAMP}.sqlite3"

    echo "备份到: $backup_file"

    if [ "$source_type" = "local" ]; then
        if command_exists sqlite3; then
            sqlite3 "$source_path" ".backup '$backup_file'"
        else
            cp "$source_path" "$backup_file"
        fi
    else
        local container_tmp="/tmp/vpanel_db_${TIMESTAMP}.sqlite3"
        if docker exec "$CONTAINER_NAME" sh -lc "command -v sqlite3 >/dev/null 2>&1"; then
            docker exec "$CONTAINER_NAME" sqlite3 "$source_path" ".backup '$container_tmp'"
            docker cp "$CONTAINER_NAME:$container_tmp" "$backup_file"
            docker exec "$CONTAINER_NAME" rm -f "$container_tmp"
        else
            docker cp "$CONTAINER_NAME:$source_path" "$backup_file"
        fi
    fi

    gzip -f "$backup_file"
    gzip -t "${backup_file}.gz"
    log_info "✓ SQLite 数据库备份成功: ${backup_file}.gz"
}

backup_database() {
    case "$(detect_db_type)" in
        sqlite)
            backup_database_sqlite
            ;;
        postgres)
            backup_database_postgres
            ;;
        *)
            log_error "✗ 不支持的数据库类型: $(detect_db_type)"
            return 1
            ;;
    esac
}

backup_config_local() {
    mkdir -p "$BACKUP_DIR/config"

    local backup_file="$BACKUP_DIR/config/vpanel_config_${TIMESTAMP}.tar.gz"
    local existing_files=()
    local candidates=("configs" ".env" "deployments/docker/.env")

    for file in "${candidates[@]}"; do
        if [ -e "$file" ]; then
            existing_files+=("$file")
        fi
    done

    if [ ${#existing_files[@]} -eq 0 ]; then
        log_warn "⚠ 没有找到可备份的配置文件"
        return 0
    fi

    echo "备份到: $backup_file"
    tar -czf "$backup_file" "${existing_files[@]}"
    log_info "✓ 配置文件备份成功: $backup_file"
}

backup_config_container() {
    mkdir -p "$BACKUP_DIR/config"

    local backup_file="$BACKUP_DIR/config/vpanel_config_${TIMESTAMP}.tar.gz"
    local volume_root
    volume_root=$(docker_volume_mountpoint "$DOCKER_CONFIG_VOLUME")

    if [ -z "$volume_root" ] || [ ! -d "$volume_root" ]; then
        log_warn "⚠ 未找到 Docker 配置卷，回退为本地配置备份"
        backup_config_local
        return
    fi

    local tmp_dir
    tmp_dir=$(mktemp -d)
    cp -a "$volume_root" "$tmp_dir/configs"

    if [ -f "deployments/docker/.env" ]; then
        cp "deployments/docker/.env" "$tmp_dir/.env"
    elif [ -f ".env" ]; then
        cp ".env" "$tmp_dir/.env"
    fi

    echo "备份到: $backup_file"
    tar -czf "$backup_file" -C "$tmp_dir" .
    rm -rf "$tmp_dir"
    log_info "✓ Docker 配置备份成功: $backup_file"
}

backup_config() {
    log_warn "备份配置文件..."
    if docker_container_running; then
        backup_config_container
    else
        backup_config_local
    fi
}

backup_agent_config() {
    log_warn "备份 Agent 配置..."

    mkdir -p "$BACKUP_DIR/agent"

    local backup_file="$BACKUP_DIR/agent/agent_config_${TIMESTAMP}.tar.gz"
    local files_to_backup=(
        "/etc/vpanel/agent.yaml"
        "/etc/xray/config.json"
    )
    local existing_files=()

    for file in "${files_to_backup[@]}"; do
        if [ -f "$file" ]; then
            existing_files+=("$file")
        fi
    done

    if [ ${#existing_files[@]} -eq 0 ]; then
        log_warn "⚠ 没有找到 Agent 配置文件"
        return 0
    fi

    echo "备份到: $backup_file"
    tar -czf "$backup_file" "${existing_files[@]}"
    log_info "✓ Agent 配置备份成功: $backup_file"
}

restore_postgres_database() {
    local backup_file=$1
    local temp_file="/tmp/vpanel_restore_${TIMESTAMP}.sql"

    if [[ "$backup_file" == *.gz ]]; then
        gunzip -c "$backup_file" > "$temp_file"
    else
        cp "$backup_file" "$temp_file"
    fi

    if [ -n "$DB_PASSWORD" ]; then
        export PGPASSWORD="$DB_PASSWORD"
    fi

    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$temp_file"
    rm -f "$temp_file"
    unset PGPASSWORD
}

restore_sqlite_database() {
    local backup_file=$1
    local source
    source=$(detect_sqlite_source) || {
        if docker_container_running; then
            source="container:/app/data/v.db"
        elif [ -n "$DB_PATH" ]; then
            source="local:$DB_PATH"
        else
            source="local:./data/v.db"
        fi
    }

    local source_type=${source%%:*}
    local source_path=${source#*:}
    local temp_file="/tmp/vpanel_restore_${TIMESTAMP}.sqlite3"

    if [[ "$backup_file" == *.gz ]]; then
        gunzip -c "$backup_file" > "$temp_file"
    else
        cp "$backup_file" "$temp_file"
    fi

    if [ "$source_type" = "local" ]; then
        mkdir -p "$(dirname "$source_path")"
        if [ -f "$source_path" ]; then
            cp "$source_path" "${source_path}.backup.${TIMESTAMP}"
        fi
        mv "$temp_file" "$source_path"
        log_info "✓ SQLite 数据库已恢复到 $source_path"
        return
    fi

    local volume_root
    volume_root=$(docker_volume_mountpoint "$DOCKER_DATA_VOLUME")
    if [ -z "$volume_root" ] || [ ! -d "$volume_root" ]; then
        log_error "✗ 未找到 Docker 数据卷: $DOCKER_DATA_VOLUME"
        rm -f "$temp_file"
        return 1
    fi

    local target_path="$volume_root/$(basename "$source_path")"
    if docker_container_running; then
        docker stop "$CONTAINER_NAME" >/dev/null
    fi

    if [ -f "$target_path" ]; then
        cp "$target_path" "${target_path}.backup.${TIMESTAMP}"
    fi
    mv "$temp_file" "$target_path"

    if command_exists docker; then
        docker start "$CONTAINER_NAME" >/dev/null 2>&1 || true
    fi

    log_info "✓ SQLite 数据库已恢复到 $target_path"
}

restore_database() {
    local backup_file=$1

    if [ ! -f "$backup_file" ]; then
        log_error "✗ 备份文件不存在: $backup_file"
        return 1
    fi

    log_warn "恢复数据库..."
    log_warn "警告: 这将覆盖当前数据库"
    confirm_action "确认恢复数据库?" || {
        echo "取消恢复"
        return 0
    }

    case "$backup_file" in
        *.sqlite3|*.sqlite3.gz|*.db|*.db.gz)
            restore_sqlite_database "$backup_file"
            ;;
        *.sql|*.sql.gz)
            restore_postgres_database "$backup_file"
            ;;
        *)
            log_error "✗ 无法识别数据库备份格式: $backup_file"
            return 1
            ;;
    esac
}

restore_config() {
    local backup_file=$1

    if [ ! -f "$backup_file" ]; then
        log_error "✗ 备份文件不存在: $backup_file"
        return 1
    fi

    log_warn "恢复配置文件..."
    log_warn "警告: 这将覆盖当前配置"
    confirm_action "确认恢复配置?" || {
        echo "取消恢复"
        return 0
    }

    local tmp_dir
    tmp_dir=$(mktemp -d)
    tar -xzf "$backup_file" -C "$tmp_dir"

    local root_env_backup="$tmp_dir/.env"
    local docker_env_backup="$tmp_dir/deployments/docker/.env"

    if docker_container_running; then
        local volume_root
        volume_root=$(docker_volume_mountpoint "$DOCKER_CONFIG_VOLUME")
        if [ -n "$volume_root" ] && [ -d "$volume_root" ] && [ -d "$tmp_dir/configs" ]; then
            find "$volume_root" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
            cp -a "$tmp_dir/configs/." "$volume_root/"
        fi
        if [ -f "$root_env_backup" ]; then
            mkdir -p "deployments/docker"
            cp "$root_env_backup" "deployments/docker/.env"
        elif [ -f "$docker_env_backup" ]; then
            mkdir -p "deployments/docker"
            cp "$docker_env_backup" "deployments/docker/.env"
        fi
    else
        if [ -d "$tmp_dir/configs" ]; then
            mkdir -p ./configs
            find ./configs -mindepth 1 -maxdepth 1 -exec rm -rf {} +
            cp -a "$tmp_dir/configs/." ./configs/
        fi
        if [ -f "$root_env_backup" ]; then
            cp "$root_env_backup" ./.env
        fi
        if [ -f "$docker_env_backup" ]; then
            mkdir -p ./deployments/docker
            cp "$docker_env_backup" ./deployments/docker/.env
        fi
    fi

    rm -rf "$tmp_dir"
    log_info "✓ 配置文件恢复成功"
}

clean_old_backups() {
    log_warn "清理旧备份..."
    echo "保留最近 $KEEP_DAYS 天的备份"

    if [ ! -d "$BACKUP_DIR" ]; then
        echo "备份目录不存在"
        return 0
    fi

    local deleted_count=0
    while IFS= read -r -d '' file; do
        echo "删除: $file"
        rm -f "$file"
        deleted_count=$((deleted_count + 1))
    done < <(find "$BACKUP_DIR" -type f -mtime +"$KEEP_DAYS" -print0)

    if [ "$deleted_count" -eq 0 ]; then
        log_info "✓ 没有需要清理的旧备份"
    else
        log_info "✓ 已删除 $deleted_count 个旧备份"
    fi
}

list_backups() {
    log_warn "可用备份:"
    echo ""

    if [ ! -d "$BACKUP_DIR" ]; then
        echo "没有找到备份"
        return 0
    fi

    echo "数据库备份:"
    if [ -d "$BACKUP_DIR/database" ]; then
        ls -lh "$BACKUP_DIR/database" | tail -n +2 || echo "  无"
    else
        echo "  无"
    fi

    echo ""
    echo "配置备份:"
    if [ -d "$BACKUP_DIR/config" ]; then
        ls -lh "$BACKUP_DIR/config" | tail -n +2 || echo "  无"
    else
        echo "  无"
    fi

    echo ""
    echo "Agent 配置备份:"
    if [ -d "$BACKUP_DIR/agent" ]; then
        ls -lh "$BACKUP_DIR/agent" | tail -n +2 || echo "  无"
    else
        echo "  无"
    fi
}

case "${1:-}" in
    database)
        backup_database
        ;;
    config)
        backup_config
        ;;
    agent)
        backup_agent_config
        ;;
    all)
        backup_database
        echo ""
        backup_config
        echo ""
        backup_agent_config
        ;;
    restore)
        if [ -z "${2:-}" ]; then
            log_error "错误: 需要指定备份文件"
            echo "用法: $0 restore <backup-file>"
            exit 1
        fi

        case "$2" in
            *"_db_"*|*.sqlite3|*.sqlite3.gz|*.sql|*.sql.gz|*.db|*.db.gz)
                restore_database "$2"
                ;;
            *"_config_"*|*.tar.gz)
                restore_config "$2"
                ;;
            *)
                log_error "错误: 无法识别备份文件类型"
                exit 1
                ;;
        esac
        ;;
    clean)
        clean_old_backups
        ;;
    list)
        list_backups
        ;;
    help|--help|-h|"")
        show_help
        ;;
    *)
        show_help
        exit 1
        ;;
esac

echo ""
log_info "完成！"
