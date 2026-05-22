#!/usr/bin/env bash

# Mihomo CLI management script.
# Usage:
#   sudo bash mihomo-manager.sh install
#   sudo bash mihomo-manager.sh update "your Clash Meta/Mihomo subscription url"
#   sudo bash mihomo-manager.sh status

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
CONFIG_DIR="${CONFIG_DIR:-/etc/mihomo}"
SERVICE_NAME="${SERVICE_NAME:-mihomo}"
BINARY_NAME="${BINARY_NAME:-mihomo}"
GITHUB_REPO="${GITHUB_REPO:-MetaCubeX/mihomo}"
DEFAULT_VERSION="${DEFAULT_VERSION:-v1.19.24}"
GITHUB_PROXY_MODE="${GITHUB_PROXY_MODE:-cn}"
GITHUB_PROXY_LIST="${GITHUB_PROXY_LIST:-https://gh.llkk.cc/ https://ghfast.top/ https://gh-proxy.com/ https://gh.ddlc.top/ https://ghproxy.net/}"
GEODATA_MMDB_URLS="${GEODATA_MMDB_URLS:-https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/country.mmdb https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/country.mmdb}"
DASHBOARD_URL="${DASHBOARD_URL:-https://github.com/MetaCubeX/metacubexd/archive/refs/heads/gh-pages.zip}"
SUBSCRIPTION_FILE="$CONFIG_DIR/subscription.url"
BACKUP_DIR="$CONFIG_DIR/backups"
DASHBOARD_DIR="$CONFIG_DIR/ui"
DASHBOARD_SECRET_FILE="$CONFIG_DIR/dashboard.secret"
DASHBOARD_BIND="${DASHBOARD_BIND:-127.0.0.1}"
DASHBOARD_PORT="${DASHBOARD_PORT:-9090}"
SYSTEM_PROXY_FILE="/etc/profile.d/mihomo.sh"

CURL_CONNECT_TIMEOUT="${CURL_CONNECT_TIMEOUT:-8}"
CURL_METADATA_MAX_TIME="${CURL_METADATA_MAX_TIME:-20}"
CURL_MAX_TIME="${CURL_MAX_TIME:-300}"
CURL_RETRY="${CURL_RETRY:-3}"
CURL_LOW_SPEED_LIMIT="${CURL_LOW_SPEED_LIMIT:-10240}"
CURL_LOW_SPEED_TIME="${CURL_LOW_SPEED_TIME:-20}"
SUBSCRIPTION_USER_AGENT="${SUBSCRIPTION_USER_AGENT:-Clash.Meta}"
SUBSCRIPTION_CONNECT_TIMEOUT="${SUBSCRIPTION_CONNECT_TIMEOUT:-8}"
SUBSCRIPTION_MAX_TIME="${SUBSCRIPTION_MAX_TIME:-120}"
SUBSCRIPTION_RETRY="${SUBSCRIPTION_RETRY:-1}"
CONFIG_TEST_TIMEOUT="${CONFIG_TEST_TIMEOUT:-30}"
ENABLE_SYSTEM_PROXY="${ENABLE_SYSTEM_PROXY:-0}"
VERIFY_CONNECT_TIMEOUT="${VERIFY_CONNECT_TIMEOUT:-8}"
VERIFY_MAX_TIME="${VERIFY_MAX_TIME:-20}"
VERIFY_URLS="${VERIFY_URLS:-https://www.google.com/generate_204 https://www.gstatic.com/generate_204 https://cp.cloudflare.com/generate_204}"

print_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $*"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $*"; }
print_error() { echo -e "${RED}[ERROR]${NC} $*"; }

check_root() {
    if [ "${EUID:-$(id -u)}" -ne 0 ]; then
        print_error "此操作需要 root 权限"
        exit 1
    fi
}

require_commands() {
    local cmd missing=0
    for cmd in "$@"; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            print_error "缺少必要命令: $cmd"
            missing=1
        fi
    done
    [ "$missing" -eq 0 ] || exit 1
}

install_package_if_possible() {
    local package_name="$1"

    if command -v apt-get >/dev/null 2>&1; then
        apt-get update
        DEBIAN_FRONTEND=noninteractive apt-get install -y "$package_name"
        return
    fi

    if command -v dnf >/dev/null 2>&1; then
        dnf install -y "$package_name"
        return
    fi

    if command -v yum >/dev/null 2>&1; then
        yum install -y "$package_name"
        return
    fi

    if command -v apk >/dev/null 2>&1; then
        apk add --no-cache "$package_name"
        return
    fi

    return 1
}

ensure_unzip() {
    if command -v unzip >/dev/null 2>&1; then
        return 0
    fi

    print_warning "未找到 unzip，尝试自动安装..."
    install_package_if_possible unzip || return 1
}

check_systemd() {
    require_commands systemctl
    if ! systemctl >/dev/null 2>&1; then
        print_error "当前环境无法使用 systemd/systemctl"
        print_info "如是容器、OpenVZ 或很老的系统，请手动运行 mihomo 或改用其他 init 脚本"
        exit 1
    fi
}

check_installed() {
    if [ ! -x "$INSTALL_DIR/$BINARY_NAME" ]; then
        print_error "Mihomo 未安装"
        print_info "请先运行: $0 install"
        exit 1
    fi
}

detect_arch() {
    local arch
    arch=$(uname -m)
    DEFAULT_MIHOMO_VARIANT=""
    case "$arch" in
        x86_64)
            ARCH="amd64"
            DEFAULT_MIHOMO_VARIANT="compatible"
            ;;
        i386|i486|i586|i686) ARCH="386" ;;
        aarch64|arm64) ARCH="arm64" ;;
        armv7l) ARCH="armv7" ;;
        *)
            print_error "不支持的系统架构: $arch"
            exit 1
            ;;
    esac
}

curl_without_proxy() {
    env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY \
        -u all_proxy -u ALL_PROXY curl "$@"
}

github_download_urls() {
    local url="$1"
    local proxy="${CLASH_META_GITHUB_PROXY:-${GITHUB_PROXY:-}}"
    local item
    local seen=" "

    if [ -n "$proxy" ]; then
        echo "${proxy%/}/$url"
        return
    fi

    if [ "$GITHUB_PROXY_MODE" != "direct" ]; then
        for item in $GITHUB_PROXY_LIST; do
            [ -n "$item" ] || continue
            case "$seen" in
                *" $item "*) continue ;;
            esac
            seen="${seen}${item} "
            echo "${item%/}/$url"
        done
    fi

    if [ "$GITHUB_PROXY_MODE" != "proxy" ]; then
        echo "$url"
    fi

    if [ "$GITHUB_PROXY_MODE" = "direct" ]; then
        for item in $GITHUB_PROXY_LIST; do
            [ -n "$item" ] || continue
            case "$seen" in
                *" $item "*) continue ;;
            esac
            seen="${seen}${item} "
            echo "${item%/}/$url"
        done
    fi
}

curl_get_text() {
    local url="$1"
    curl -fsSL --connect-timeout "$CURL_CONNECT_TIMEOUT" --max-time "$CURL_METADATA_MAX_TIME" "$url" 2>/dev/null \
        || curl_without_proxy -fsSL --connect-timeout "$CURL_CONNECT_TIMEOUT" --max-time "$CURL_METADATA_MAX_TIME" "$url" 2>/dev/null
}

curl_download_file() {
    local url="$1"
    local output="$2"

    print_info "尝试下载: $url"
    if curl -fL --retry "$CURL_RETRY" --retry-delay 2 --progress-bar \
        --connect-timeout "$CURL_CONNECT_TIMEOUT" --max-time "$CURL_MAX_TIME" \
        --speed-limit "$CURL_LOW_SPEED_LIMIT" --speed-time "$CURL_LOW_SPEED_TIME" \
        -o "$output" "$url"; then
        return 0
    fi

    print_warning "当前网络或环境代理下载失败，自动绕过代理重试..."
    curl_without_proxy -fL --retry "$CURL_RETRY" --retry-delay 2 --progress-bar \
        --connect-timeout "$CURL_CONNECT_TIMEOUT" --max-time "$CURL_MAX_TIME" \
        --speed-limit "$CURL_LOW_SPEED_LIMIT" --speed-time "$CURL_LOW_SPEED_TIME" \
        -o "$output" "$url"
}

normalize_version() {
    if [ -n "${LATEST_VERSION:-}" ] && [[ "$LATEST_VERSION" != v* ]]; then
        LATEST_VERSION="v$LATEST_VERSION"
    fi
}

get_latest_version() {
    local url

    if [ -n "${MIHOMO_VERSION:-}" ]; then
        LATEST_VERSION="$MIHOMO_VERSION"
        normalize_version
        print_info "使用指定版本: $LATEST_VERSION"
        return
    fi

    print_info "获取 Mihomo 最新版本..."
    LATEST_VERSION=""
    for url in $(github_download_urls "https://api.github.com/repos/${GITHUB_REPO}/releases/latest"); do
        LATEST_VERSION=$(curl_get_text "$url" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/' || true)
        [ -n "$LATEST_VERSION" ] && break
    done

    if [ -z "$LATEST_VERSION" ]; then
        print_warning "GitHub API 访问失败，尝试解析 Releases 页面..."
        for url in $(github_download_urls "https://github.com/${GITHUB_REPO}/releases/latest"); do
            LATEST_VERSION=$(curl_get_text "$url" | grep -oE 'tag/v[0-9][^"]+' | sed 's#tag/##' | head -1 || true)
            [ -n "$LATEST_VERSION" ] && break
        done
    fi

    if [ -z "$LATEST_VERSION" ]; then
        LATEST_VERSION="$DEFAULT_VERSION"
        print_warning "无法自动获取版本，使用默认版本: $LATEST_VERSION"
    else
        normalize_version
        print_success "最新版本: $LATEST_VERSION"
    fi
}

detect_mihomo_variant() {
    if [ -n "${MIHOMO_VARIANT:-}" ]; then
        MIHOMO_VARIANT="${MIHOMO_VARIANT#-}"
        print_info "使用指定构建变体: $MIHOMO_VARIANT"
        return
    fi

    MIHOMO_VARIANT="${DEFAULT_MIHOMO_VARIANT:-}"
    if [ -n "$MIHOMO_VARIANT" ]; then
        print_info "使用兼容构建，适合老旧 AMD64 CPU"
    fi
}

build_asset_name() {
    local variant_part=""
    detect_mihomo_variant
    if [ -n "${MIHOMO_VARIANT:-}" ]; then
        variant_part="-${MIHOMO_VARIANT}"
    fi
    ASSET_NAME="mihomo-linux-${ARCH}${variant_part}-${LATEST_VERSION}.gz"
}

check_binary_runtime() {
    local binary_file="${1:-$INSTALL_DIR/$BINARY_NAME}"
    local output

    output=$("$binary_file" -v 2>&1) || {
        print_error "mihomo 二进制无法在当前机器运行"
        echo "$output"
        if echo "$output" | grep -qi "microarchitecture"; then
            print_info "这是 CPU 指令集不兼容。老旧 AMD64 CPU 应使用 compatible 或 v1 构建。"
        fi
        return 1
    }

    print_info "二进制版本: $(echo "$output" | head -1)"
}

create_sample_config() {
    mkdir -p "$CONFIG_DIR"
    local secret
    secret=$(ensure_dashboard_secret)
    cat > "$CONFIG_DIR/config.yaml" <<EOF
mixed-port: 7890
port: 7891
socks-port: 7892
# HTTP/SOCKS proxies and DNS resolver bind to localhost only by default
# so this machine doesn't become an open proxy / DNS amplifier for the LAN.
# Set allow-lan: true and bind-address: '*' if you want LAN devices to use
# this box as their proxy.
allow-lan: false
bind-address: '127.0.0.1'
mode: rule
log-level: info
ipv6: true
# Dashboard binds to ${DASHBOARD_BIND} only for safety. To reach it from
# outside the box, use an SSH tunnel:
#   ssh -L 9090:127.0.0.1:${DASHBOARD_PORT} user@server
# Then open http://127.0.0.1:${DASHBOARD_PORT}/ui locally.
external-controller: ${DASHBOARD_BIND}:${DASHBOARD_PORT}
external-ui: ui
external-ui-url: https://github.com/MetaCubeX/metacubexd/archive/refs/heads/gh-pages.zip
secret: "${secret}"
unified-delay: true
tcp-concurrent: true

dns:
  enable: true
  # Localhost only by default; expose to LAN by changing this to 0.0.0.0:1053
  # ONLY if other devices on a trusted network need this box as their resolver.
  listen: 127.0.0.1:1053
  ipv6: true
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  nameserver:
    - 223.5.5.5
    - 119.29.29.29
  fallback:
    - 8.8.8.8
    - 1.1.1.1

tun:
  enable: false
  stack: system
  dns-hijack:
    - any:53
  auto-route: true
  auto-detect-interface: true

proxies: []

proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - DIRECT

rules:
  - MATCH,DIRECT
EOF
    chmod 0644 "$CONFIG_DIR/config.yaml"
    print_success "示例配置已创建: $CONFIG_DIR/config.yaml"
    print_info "Dashboard 监听 ${DASHBOARD_BIND}:${DASHBOARD_PORT}（默认仅本机访问），secret 写入 $DASHBOARD_SECRET_FILE"
    print_info "HTTP/SOCKS/DNS 入口默认仅 127.0.0.1。如需让局域网设备使用，请改 allow-lan/bind-address/dns.listen"
}

# ensure_dashboard_secret returns (and persists on first call) a random
# secret used by Mihomo's external-controller. Stored at $DASHBOARD_SECRET_FILE
# with 600 perms so only root reads it. Idempotent: subsequent calls return
# the same value so the dashboard's saved password stays valid across
# subscription updates.
ensure_dashboard_secret() {
    if [ -s "$DASHBOARD_SECRET_FILE" ]; then
        cat "$DASHBOARD_SECRET_FILE"
        return
    fi
    mkdir -p "$CONFIG_DIR"
    local s
    if command -v openssl >/dev/null 2>&1; then
        s=$(openssl rand -hex 16)
    elif [ -r /dev/urandom ]; then
        s=$(head -c 32 /dev/urandom | base64 | tr -d '/+=\n' | head -c 24)
    else
        s="vpanel-$(date +%s)"
    fi
    local old_umask
    old_umask=$(umask)
    umask 077
    printf '%s\n' "$s" > "$DASHBOARD_SECRET_FILE"
    umask "$old_umask"
    chmod 600 "$DASHBOARD_SECRET_FILE" 2>/dev/null || true
    printf '%s' "$s"
}

ensure_dashboard_config() {
    local config_file="${1:-$CONFIG_DIR/config.yaml}"

    [ -f "$config_file" ] || return 0

    if ! grep -q '^external-controller:' "$config_file"; then
        # Bind to localhost only by default. Subscriptions that ship their
        # own external-controller (e.g. with 0.0.0.0) are left alone, on the
        # assumption that the subscription author meant it. Set
        # DASHBOARD_BIND=0.0.0.0 in env to opt back into LAN exposure.
        printf '\nexternal-controller: %s:%s\n' "$DASHBOARD_BIND" "$DASHBOARD_PORT" >> "$config_file"
    fi

    if ! grep -q '^external-ui:' "$config_file"; then
        printf 'external-ui: ui\n' >> "$config_file"
    fi

    if ! grep -q '^external-ui-url:' "$config_file"; then
        printf 'external-ui-url: %s\n' "$DASHBOARD_URL" >> "$config_file"
    fi

    if ! grep -q '^secret:' "$config_file"; then
        local secret
        secret=$(ensure_dashboard_secret)
        printf 'secret: "%s"\n' "$secret" >> "$config_file"
        print_info "Dashboard secret 已写入 $config_file（值见 $DASHBOARD_SECRET_FILE）"
    fi
}

create_systemd_service() {
    check_systemd
    cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Mihomo Service
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=$CONFIG_DIR
ExecStartPre=$INSTALL_DIR/$BINARY_NAME -t -d $CONFIG_DIR -f $CONFIG_DIR/config.yaml
ExecStart=$INSTALL_DIR/$BINARY_NAME -d $CONFIG_DIR -f $CONFIG_DIR/config.yaml
Restart=on-failure
RestartSec=5s
TimeoutStartSec=45s
LimitNOFILE=1048576

# Sandboxing — keep mihomo's blast radius small if a vulnerability lets an
# attacker execute code inside the daemon. Capabilities cover the TUN
# device + binding to <1024 ports; everything else is restricted.
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=read-only
PrivateTmp=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
ReadWritePaths=$CONFIG_DIR
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_NET_RAW
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX AF_NETLINK
RestrictNamespaces=true
RestrictSUIDSGID=true
LockPersonality=true
MemoryDenyWriteExecute=true
SystemCallArchitectures=native

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    print_success "systemd 服务已创建"
}

get_yaml_scalar() {
    local key="$1"
    local file="${2:-$CONFIG_DIR/config.yaml}"
    [ -f "$file" ] || return 0
    awk -v key="$key" '
        {
            if ($0 ~ /^[[:space:]]/) next
            line=$0
            split(line, parts, ":")
            name=parts[1]
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", name)
        }
        name == key {
            value=line
            sub(/^[^:]*:/, "", value)
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
            gsub(/^["'\''"]|["'\''"]$/, "", value)
            print value
            exit
        }
    ' "$file"
}

get_http_proxy_port() {
    local mixed_port http_port
    mixed_port=$(get_yaml_scalar "mixed-port")
    http_port=$(get_yaml_scalar "port")
    if [ -n "$mixed_port" ]; then
        echo "$mixed_port"
    elif [ -n "$http_port" ]; then
        echo "$http_port"
    else
        echo "7890"
    fi
}

get_socks_proxy_port() {
    local socks_port
    socks_port=$(get_yaml_scalar "socks-port")
    echo "${socks_port:-7892}"
}

configure_system_proxy() {
    local http_port socks_port

    if [ "$ENABLE_SYSTEM_PROXY" != "1" ]; then
        if [ -f "$SYSTEM_PROXY_FILE" ] && grep -q "Mihomo 代理配置" "$SYSTEM_PROXY_FILE"; then
            mv "$SYSTEM_PROXY_FILE" "${SYSTEM_PROXY_FILE}.disabled"
            print_warning "已停用旧的全局代理配置: ${SYSTEM_PROXY_FILE}.disabled"
        fi
        print_info "默认不写入全局系统代理，避免服务未运行时影响系统下载"
        print_info "如确实需要全局代理，请使用: ENABLE_SYSTEM_PROXY=1 $0 install"
        return
    fi

    http_port=$(get_http_proxy_port)
    socks_port=$(get_socks_proxy_port)
    cat > "$SYSTEM_PROXY_FILE" <<EOF
# Mihomo 代理配置
export http_proxy=http://127.0.0.1:$http_port
export https_proxy=http://127.0.0.1:$http_port
export all_proxy=socks5h://127.0.0.1:$socks_port
export no_proxy=localhost,127.0.0.1,::1
EOF
    chmod 0644 "$SYSTEM_PROXY_FILE"
    print_success "系统环境代理已写入: $SYSTEM_PROXY_FILE"
}

enable_system_proxy() {
    check_root
    check_installed
    ENABLE_SYSTEM_PROXY=1
    configure_system_proxy
    print_info "重新登录终端后生效；当前终端可执行: source $SYSTEM_PROXY_FILE"
}

disable_system_proxy_file() {
    check_root
    if [ -f "$SYSTEM_PROXY_FILE" ]; then
        mv "$SYSTEM_PROXY_FILE" "${SYSTEM_PROXY_FILE}.disabled"
        print_success "系统环境代理已停用: ${SYSTEM_PROXY_FILE}.disabled"
    else
        print_info "系统环境代理未启用"
    fi
}

ensure_country_mmdb() {
    local mmdb_file="$CONFIG_DIR/Country.mmdb"
    local temp_file url
    [ -s "$mmdb_file" ] && return 0
    command -v curl >/dev/null 2>&1 || return 0
    command -v mktemp >/dev/null 2>&1 || return 0

    mkdir -p "$CONFIG_DIR"
    temp_file=$(mktemp "$CONFIG_DIR/Country.mmdb.download.XXXXXX")
    print_info "下载 Country.mmdb，避免首次启动时卡在 geodata 下载..."

    for url in $GEODATA_MMDB_URLS $(github_download_urls "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/country.mmdb"); do
        rm -f "$temp_file"
        if curl_download_file "$url" "$temp_file" && [ -s "$temp_file" ]; then
            mv -f "$temp_file" "$mmdb_file"
            chmod 0644 "$mmdb_file"
            print_success "Country.mmdb 已准备好"
            return 0
        fi
        print_warning "Country.mmdb 下载失败，尝试下一个地址..."
    done

    rm -f "$temp_file"
    print_warning "Country.mmdb 预下载失败，首次启动可能会等待 mihomo 自动下载"
}

ensure_dashboard_ui() {
    local temp_file
    local temp_dir
    local index_file
    local source_dir
    local url

    if [ -s "$DASHBOARD_DIR/index.html" ]; then
        return 0
    fi

    command -v curl >/dev/null 2>&1 || {
        print_warning "缺少 curl，跳过 MetaCubeXD 面板预下载"
        return 0
    }

    command -v mktemp >/dev/null 2>&1 || {
        print_warning "缺少 mktemp，跳过 MetaCubeXD 面板预下载"
        return 0
    }

    if ! ensure_unzip; then
        print_warning "缺少 unzip，跳过 MetaCubeXD 面板预下载；mihomo 可能会按 external-ui-url 自动下载"
        return 0
    fi

    mkdir -p "$CONFIG_DIR"
    temp_file=$(mktemp "$CONFIG_DIR/metacubexd.XXXXXX.zip")
    temp_dir=$(mktemp -d "$CONFIG_DIR/metacubexd.XXXXXX")

    print_info "下载 MetaCubeXD 面板资源，确保 http://服务器IP:9090/ui 可用..."
    for url in $(github_download_urls "$DASHBOARD_URL"); do
        rm -f "$temp_file"
        if curl_download_file "$url" "$temp_file" && [ -s "$temp_file" ]; then
            rm -rf "$temp_dir"/*
            if unzip -oq "$temp_file" -d "$temp_dir"; then
                index_file=$(find "$temp_dir" -name index.html -type f | head -1 || true)
                if [ -n "$index_file" ]; then
                    source_dir=$(dirname "$index_file")
                    rm -rf "$DASHBOARD_DIR"
                    mkdir -p "$DASHBOARD_DIR"
                    cp -R "$source_dir"/. "$DASHBOARD_DIR"/
                    chmod -R u=rwX,go=rX "$DASHBOARD_DIR"
                    rm -rf "$temp_dir" "$temp_file"
                    print_success "MetaCubeXD 面板资源已准备好: $DASHBOARD_DIR"
                    return 0
                fi
            fi
        fi
        print_warning "MetaCubeXD 面板下载失败，尝试下一个地址..."
    done

    rm -rf "$temp_dir" "$temp_file"
    print_warning "MetaCubeXD 面板预下载失败；可稍后执行: $0 dashboard"
}

count_config_section_items() {
    local config_file="$1"
    local section="$2"
    local pattern="$3"
    awk -v target="$section" -v item_pattern="$pattern" '
        /^[^[:space:]#][^:]*:/ {
            current=$1
            sub(":$", "", current)
        }
        current == target && $0 ~ item_pattern {
            count++
        }
        END { print count + 0 }
    ' "$config_file"
}

show_config_summary() {
    local config_file="${1:-$CONFIG_DIR/config.yaml}"
    local proxy_count provider_count group_count rule_count
    [ -f "$config_file" ] || return 0
    proxy_count=$(count_config_section_items "$config_file" "proxies" "^[[:space:]]*-[[:space:]]+name:")
    provider_count=$(count_config_section_items "$config_file" "proxy-providers" "^[[:space:]][[:space:]][^[:space:]#][^:]*:")
    group_count=$(count_config_section_items "$config_file" "proxy-groups" "^[[:space:]]*-[[:space:]]+name:")
    rule_count=$(count_config_section_items "$config_file" "rules" "^[[:space:]]*-")
    print_info "配置摘要: 代理节点 ${proxy_count} 个，代理提供器 ${provider_count} 个，代理组 ${group_count} 个，规则 ${rule_count} 条"
    if [ "$proxy_count" -eq 0 ] && [ "$provider_count" -eq 0 ]; then
        print_warning "当前配置没有代理节点或代理提供器，只能直连；请使用: $0 update '订阅链接'"
    fi
}

validate_config_file() {
    local config_file="$1"
    local output status
    [ -s "$config_file" ] || {
        print_error "配置文件为空"
        return 1
    }

    ensure_country_mmdb
    set +e
    if command -v timeout >/dev/null 2>&1; then
        output=$(timeout "$CONFIG_TEST_TIMEOUT" "$INSTALL_DIR/$BINARY_NAME" -t -d "$CONFIG_DIR" -f "$config_file" 2>&1)
        status=$?
    else
        output=$("$INSTALL_DIR/$BINARY_NAME" -t -d "$CONFIG_DIR" -f "$config_file" 2>&1)
        status=$?
    fi
    set -e

    if [ "$status" -ne 0 ]; then
        print_error "配置校验失败"
        echo "$output"
        [ "$status" -eq 124 ] && print_info "配置校验超时，可重试: CONFIG_TEST_TIMEOUT=60 $0 test"
        return 1
    fi

    print_success "配置校验通过"
}

install_mihomo() {
    check_root
    require_commands curl gunzip uname mktemp date install ln
    check_systemd

    detect_arch
    print_info "系统架构: $ARCH"
    get_latest_version
    build_asset_name

    local origin_download_url="https://github.com/${GITHUB_REPO}/releases/download/${LATEST_VERSION}/${ASSET_NAME}"
    local temp_file="/tmp/${ASSET_NAME}"
    local output_file="${temp_file%.gz}"
    local download_url downloaded=0

    print_info "目标文件: $ASSET_NAME"
    rm -f "$temp_file" "$output_file"

    if [ -n "${MIHOMO_DOWNLOAD_URL:-}" ]; then
        if curl_download_file "$MIHOMO_DOWNLOAD_URL" "$temp_file"; then
            downloaded=1
        fi
    else
        for download_url in $(github_download_urls "$origin_download_url"); do
            rm -f "$temp_file"
            if curl_download_file "$download_url" "$temp_file"; then
                downloaded=1
                break
            fi
            print_warning "这个下载地址失败，自动尝试下一个地址..."
        done
    fi

    if [ "$downloaded" -ne 1 ] || [ ! -s "$temp_file" ]; then
        print_error "下载失败"
        print_info "手动下载地址: $origin_download_url"
        print_info "也可指定: MIHOMO_DOWNLOAD_URL=https://...gz $0 install"
        exit 1
    fi

    print_info "解压并安装..."
    gunzip -f "$temp_file"
    install -m 755 "$output_file" "$INSTALL_DIR/$BINARY_NAME"
    rm -f "$output_file"
    ln -sf "$INSTALL_DIR/$BINARY_NAME" "$INSTALL_DIR/clash-meta"

    check_binary_runtime "$INSTALL_DIR/$BINARY_NAME" || exit 1
    [ -f "$CONFIG_DIR/config.yaml" ] || create_sample_config
    ensure_dashboard_config "$CONFIG_DIR/config.yaml"
    ensure_country_mmdb
    ensure_dashboard_ui
    create_systemd_service
    configure_system_proxy

    print_success "Mihomo 安装完成"
    show_usage
}

start_service() {
    check_root
    check_installed
    check_systemd
    systemctl start "$SERVICE_NAME"
    sleep 1
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_success "服务已启动"
        systemctl status "$SERVICE_NAME" --no-pager || true
    else
        print_error "服务启动失败"
        systemctl status "$SERVICE_NAME" --no-pager || true
        exit 1
    fi
}

stop_service() {
    check_root
    check_installed
    check_systemd
    systemctl stop "$SERVICE_NAME"
    print_success "服务已停止"
}

restart_service() {
    check_root
    check_installed
    check_systemd
    systemctl restart "$SERVICE_NAME"
    sleep 1
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_success "服务已重启"
    else
        print_error "服务重启失败"
        systemctl status "$SERVICE_NAME" --no-pager || true
        exit 1
    fi
}

enable_service() {
    check_root
    check_installed
    check_systemd
    systemctl enable "$SERVICE_NAME"
    print_success "已启用开机自启"
}

disable_service() {
    check_root
    check_installed
    check_systemd
    systemctl disable "$SERVICE_NAME"
    print_success "已禁用开机自启"
}

show_status() {
    check_installed
    check_systemd
    echo "=========================================="
    echo "  Mihomo 状态"
    echo "=========================================="
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_success "服务状态: 运行中"
    else
        print_error "服务状态: 已停止"
    fi
    if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
        print_info "开机自启: 已启用"
    else
        print_info "开机自启: 未启用"
    fi
    print_info "版本: $("$INSTALL_DIR/$BINARY_NAME" -v 2>&1 | head -1)"
    print_info "配置文件: $CONFIG_DIR/config.yaml"
    if [ -s "$DASHBOARD_SECRET_FILE" ]; then
        print_info "Dashboard URL: http://${DASHBOARD_BIND}:${DASHBOARD_PORT}/ui"
        print_info "Dashboard secret 文件: $DASHBOARD_SECRET_FILE （仅 root 可读）"
        if [ "$DASHBOARD_BIND" = "127.0.0.1" ]; then
            print_info "Dashboard 仅本机访问，远程访问请用 SSH 隧道: ssh -L ${DASHBOARD_PORT}:127.0.0.1:${DASHBOARD_PORT} user@host"
        fi
    fi
    show_config_summary "$CONFIG_DIR/config.yaml"
    systemctl status "$SERVICE_NAME" --no-pager || true
}

show_logs() {
    check_installed
    check_systemd
    print_info "查看 Mihomo 日志，Ctrl+C 退出"
    journalctl -u "$SERVICE_NAME" -f
}

get_subscription_url() {
    local input_url="${1:-}"
    if [ -z "$input_url" ] && [ -n "${SUBSCRIPTION_URL:-}" ]; then
        input_url="$SUBSCRIPTION_URL"
    fi
    if [ -z "$input_url" ] && [ -s "$SUBSCRIPTION_FILE" ]; then
        input_url=$(sed -n '1p' "$SUBSCRIPTION_FILE")
        print_info "使用已保存订阅链接"
    fi
    if [ -z "$input_url" ]; then
        read -r -p "请输入订阅链接: " input_url
    fi
    [ -n "$input_url" ] || {
        print_error "订阅链接不能为空"
        exit 1
    }
    SUBSCRIPTION_URL="$input_url"
}

save_subscription_url() {
    local old_umask
    old_umask=$(umask)
    umask 077
    printf '%s\n' "$SUBSCRIPTION_URL" > "$SUBSCRIPTION_FILE"
    umask "$old_umask"
    chmod 600 "$SUBSCRIPTION_FILE" 2>/dev/null || true
}

is_tcp_port_open() {
    local host="$1"
    local port="$2"
    timeout 1 bash -c "</dev/tcp/$host/$port" >/dev/null 2>&1
}

subscription_proxy_urls() {
    local mixed_port http_port socks_port
    mixed_port=$(get_yaml_scalar "mixed-port")
    http_port=$(get_yaml_scalar "port")
    socks_port=$(get_yaml_scalar "socks-port")
    [ -n "$mixed_port" ] && echo "http://127.0.0.1:$mixed_port"
    [ -n "$http_port" ] && echo "http://127.0.0.1:$http_port"
    [ -n "$socks_port" ] && echo "socks5h://127.0.0.1:$socks_port"
    echo "http://127.0.0.1:7890"
    echo "socks5h://127.0.0.1:7892"
}

get_controller_base_url() {
    local controller host port

    controller=$(get_yaml_scalar "external-controller")
    controller="${controller:-127.0.0.1:9090}"
    controller="${controller#http://}"
    controller="${controller#https://}"

    case "$controller" in
        :*)
            host="127.0.0.1"
            port="${controller#:}"
            ;;
        *:*)
            host="${controller%:*}"
            port="${controller##*:}"
            ;;
        *)
            host="127.0.0.1"
            port="$controller"
            ;;
    esac

    case "$host" in
        ""|"*"|"0.0.0.0"|"::"|"[::]")
            host="127.0.0.1"
            ;;
    esac

    echo "http://$host:$port"
}

curl_controller_api() {
    local path="$1"
    local secret
    local base_url

    base_url=$(get_controller_base_url)
    secret=$(get_yaml_scalar "secret")

    if [ -n "$secret" ]; then
        curl_without_proxy -fsS \
            --connect-timeout "$VERIFY_CONNECT_TIMEOUT" \
            --max-time "$VERIFY_MAX_TIME" \
            -H "Authorization: Bearer $secret" \
            "$base_url$path" >/dev/null
    else
        curl_without_proxy -fsS \
            --connect-timeout "$VERIFY_CONNECT_TIMEOUT" \
            --max-time "$VERIFY_MAX_TIME" \
            "$base_url$path" >/dev/null
    fi
}

try_proxy_access() {
    local proxy_url="$1"
    local url
    local http_code

    for url in $VERIFY_URLS; do
        print_info "通过 $proxy_url 访问: $url"
        http_code=$(curl_without_proxy -sS -o /dev/null -w '%{http_code}' \
            -x "$proxy_url" \
            --connect-timeout "$VERIFY_CONNECT_TIMEOUT" \
            --max-time "$VERIFY_MAX_TIME" \
            "$url" 2>/dev/null || true)

        case "$http_code" in
            200|204|301|302|307|308)
                print_success "代理连通性正常: $url HTTP $http_code"
                return 0
                ;;
            *)
                print_info "该探测地址暂未返回有效响应，继续尝试下一个: $url HTTP ${http_code:-000}"
                ;;
        esac
    done

    return 1
}

curl_subscription() {
    curl -fL --retry "$SUBSCRIPTION_RETRY" --retry-delay 1 \
        --connect-timeout "$SUBSCRIPTION_CONNECT_TIMEOUT" \
        --max-time "$SUBSCRIPTION_MAX_TIME" \
        -A "$SUBSCRIPTION_USER_AGENT" "$@"
}

curl_subscription_without_proxy() {
    env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY \
        -u all_proxy -u ALL_PROXY curl -fL --retry "$SUBSCRIPTION_RETRY" --retry-delay 1 \
        --connect-timeout "$SUBSCRIPTION_CONNECT_TIMEOUT" \
        --max-time "$SUBSCRIPTION_MAX_TIME" \
        -A "$SUBSCRIPTION_USER_AGENT" "$@"
}

curl_download_subscription() {
    local url="$1"
    local output="$2"
    local proxy_url proxy_port seen_proxies=" "

    print_info "尝试下载订阅: 默认网络/环境代理"
    if curl_subscription -o "$output" "$url"; then
        return 0
    fi

    print_warning "默认网络失败，尝试直连..."
    if curl_subscription_without_proxy -o "$output" "$url"; then
        return 0
    fi

    for proxy_url in $(subscription_proxy_urls); do
        case "$seen_proxies" in
            *" $proxy_url "*) continue ;;
        esac
        seen_proxies="${seen_proxies}${proxy_url} "
        proxy_port=${proxy_url##*:}
        is_tcp_port_open 127.0.0.1 "$proxy_port" || continue
        print_info "尝试通过本机代理 $proxy_url 下载订阅..."
        if curl_subscription_without_proxy -x "$proxy_url" -o "$output" "$url"; then
            return 0
        fi
    done
    return 1
}

restart_service_with_rollback() {
    local backup_file="$1"
    if ! systemctl is-active --quiet "$SERVICE_NAME"; then
        print_info "服务当前未运行，配置已更新但未重启"
        return 0
    fi

    if systemctl restart "$SERVICE_NAME"; then
        print_success "服务已重启"
        return 0
    fi

    print_error "服务重启失败"
    if [ -n "$backup_file" ] && [ -f "$backup_file" ]; then
        print_warning "正在恢复更新前配置..."
        cp -p "$backup_file" "$CONFIG_DIR/config.yaml"
        systemctl restart "$SERVICE_NAME" || true
    fi
    return 1
}

update_subscription() {
    check_root
    check_installed
    require_commands curl mktemp date grep sed
    check_systemd

    local temp_config backup_file="" timestamp
    mkdir -p "$CONFIG_DIR"
    get_subscription_url "${1:-}"
    temp_config=$(mktemp "$CONFIG_DIR/config.yaml.download.XXXXXX")

    print_info "下载订阅配置..."
    if ! curl_download_subscription "$SUBSCRIPTION_URL" "$temp_config"; then
        rm -f "$temp_config"
        print_error "订阅配置下载失败，当前配置未变更"
        print_info "请确认订阅服务可访问，且订阅链接未失效。"
        exit 1
    fi

    ensure_dashboard_config "$temp_config"
    ensure_dashboard_ui
    validate_config_file "$temp_config" || {
        rm -f "$temp_config"
        exit 1
    }

    mkdir -p "$BACKUP_DIR"
    timestamp=$(date +%Y%m%d-%H%M%S)
    if [ -f "$CONFIG_DIR/config.yaml" ]; then
        backup_file="$BACKUP_DIR/config.yaml.$timestamp.backup"
        cp -p "$CONFIG_DIR/config.yaml" "$backup_file"
        print_info "已备份当前配置: $backup_file"
    fi

    chmod 0644 "$temp_config"
    mv -f "$temp_config" "$CONFIG_DIR/config.yaml"
    save_subscription_url
    print_success "订阅配置更新成功"
    show_config_summary "$CONFIG_DIR/config.yaml"
    restart_service_with_rollback "$backup_file"
}

test_config() {
    check_root
    check_installed
    [ -f "$CONFIG_DIR/config.yaml" ] || {
        print_error "配置文件不存在"
        exit 1
    }
    validate_config_file "$CONFIG_DIR/config.yaml"
    show_config_summary "$CONFIG_DIR/config.yaml"
}

verify_service() {
    check_root
    check_installed
    require_commands curl grep sed timeout
    check_systemd

    local failed=0
    local proxy_count
    local provider_count
    local proxy_url
    local proxy_port
    local seen_proxies=" "
    local controller_url

    echo "=========================================="
    echo "  Mihomo 使用验证"
    echo "=========================================="
    echo ""

    if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
        print_error "配置文件不存在: $CONFIG_DIR/config.yaml"
        exit 1
    fi

    if validate_config_file "$CONFIG_DIR/config.yaml"; then
        print_success "配置文件可被 Mihomo 正常加载"
    else
        failed=1
    fi

    show_config_summary "$CONFIG_DIR/config.yaml"
    proxy_count=$(count_config_section_items "$CONFIG_DIR/config.yaml" "proxies" "^[[:space:]]*-[[:space:]]+name:")
    provider_count=$(count_config_section_items "$CONFIG_DIR/config.yaml" "proxy-providers" "^[[:space:]][[:space:]][^[:space:]#][^:]*:")
    if [ "$proxy_count" -eq 0 ] && [ "$provider_count" -eq 0 ]; then
        print_warning "当前没有代理节点或代理提供器，连通性测试可能只能验证直连，不能代表代理节点可用"
        failed=1
    fi

    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_success "systemd 服务正在运行"
    else
        print_error "systemd 服务未运行，请先执行: $0 start"
        failed=1
    fi

    controller_url=$(get_controller_base_url)
    if curl_controller_api "/version"; then
        print_success "控制 API 正常: $controller_url"
    else
        print_warning "控制 API 无法访问: $controller_url"
        print_info "如需 Web 面板，请确认 external-controller、secret 和防火墙设置"
        failed=1
    fi

    for proxy_url in $(subscription_proxy_urls); do
        case "$seen_proxies" in
            *" $proxy_url "*) continue ;;
        esac
        seen_proxies="${seen_proxies}${proxy_url} "
        proxy_port=${proxy_url##*:}

        if is_tcp_port_open 127.0.0.1 "$proxy_port"; then
            print_success "本机代理端口已监听: $proxy_url"
            if try_proxy_access "$proxy_url"; then
                print_success "Mihomo 已可正常代理访问"
                [ "$failed" -eq 0 ] && return 0
                print_warning "代理可用，但上面的基础检查仍有异常，请按提示处理"
                return 1
            fi
        else
            print_warning "本机代理端口未监听: $proxy_url"
        fi
    done

    print_error "未通过代理连通性验证"
    print_info "可查看日志定位原因: journalctl -u $SERVICE_NAME -n 100 --no-pager"
    return 1
}

edit_config() {
    check_root
    check_installed
    require_commands date
    [ -f "$CONFIG_DIR/config.yaml" ] || {
        print_error "配置文件不存在"
        exit 1
    }
    mkdir -p "$BACKUP_DIR"
    cp -p "$CONFIG_DIR/config.yaml" "$BACKUP_DIR/config.yaml.edit.$(date +%Y%m%d-%H%M%S).backup"
    if command -v nano >/dev/null 2>&1; then
        nano "$CONFIG_DIR/config.yaml"
    elif command -v vim >/dev/null 2>&1; then
        vim "$CONFIG_DIR/config.yaml"
    elif command -v vi >/dev/null 2>&1; then
        vi "$CONFIG_DIR/config.yaml"
    else
        print_error "未找到 nano/vim/vi 编辑器"
        print_info "配置文件位置: $CONFIG_DIR/config.yaml"
        exit 1
    fi
    validate_config_file "$CONFIG_DIR/config.yaml"
    show_config_summary "$CONFIG_DIR/config.yaml"
}

install_dashboard() {
    check_root
    check_installed
    check_systemd

    [ -f "$CONFIG_DIR/config.yaml" ] || create_sample_config
    ensure_dashboard_config "$CONFIG_DIR/config.yaml"
    ensure_dashboard_ui
    validate_config_file "$CONFIG_DIR/config.yaml"

    if systemctl is-active --quiet "$SERVICE_NAME"; then
        systemctl restart "$SERVICE_NAME"
        print_success "面板资源已修复，服务已重启"
    else
        print_success "面板资源已修复，服务当前未运行"
    fi

    print_info "控制面板: http://服务器IP:9090/ui"
    print_warning "如果服务器有公网 IP，请在安全组/防火墙中限制 9090 访问来源"
}

uninstall_mihomo() {
    check_root
    check_systemd
    print_warning "即将卸载 Mihomo"
    read -r -p "确认继续？(y/n): " reply
    [[ "$reply" =~ ^[Yy]$ ]] || exit 0
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    systemctl disable "$SERVICE_NAME" 2>/dev/null || true
    rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
    systemctl daemon-reload
    rm -f "$INSTALL_DIR/$BINARY_NAME" "$INSTALL_DIR/clash-meta"
    read -r -p "是否删除配置文件？(y/n): " reply
    [[ "$reply" =~ ^[Yy]$ ]] && rm -rf "$CONFIG_DIR"
    rm -f "$SYSTEM_PROXY_FILE" "${SYSTEM_PROXY_FILE}.disabled"
    print_success "卸载完成"
}

show_menu() {
    clear 2>/dev/null || true
    echo "=========================================="
    echo "       Mihomo CLI 管理脚本"
    echo "=========================================="
    echo ""

    if [ -x "$INSTALL_DIR/$BINARY_NAME" ]; then
        echo -e "${GREEN}[已安装]${NC}"
        if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            echo -e "服务状态: ${GREEN}运行中${NC}"
        else
            echo -e "服务状态: ${RED}已停止${NC}"
        fi
        if command -v systemctl >/dev/null 2>&1 && systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
            echo -e "开机自启: ${GREEN}已启用${NC}"
        else
            echo -e "开机自启: ${YELLOW}未启用${NC}"
        fi
    else
        echo -e "${YELLOW}[未安装]${NC}"
    fi

    echo ""
    echo "  1) 安装 Mihomo"
    echo "  2) 更新订阅配置"
    echo "  3) 启动服务"
    echo "  4) 停止服务"
    echo "  5) 重启服务"
    echo "  6) 查看状态"
    echo "  7) 查看日志"
    echo "  8) 启用开机自启"
    echo "  9) 禁用开机自启"
    echo " 10) 校验配置"
    echo " 11) 验证是否可用"
    echo " 12) 修复/下载控制面板"
    echo " 13) 编辑配置"
    echo " 14) 启用系统环境代理"
    echo " 15) 停用系统环境代理"
    echo " 16) 卸载 Mihomo"
    echo " 17) 显示帮助"
    echo "  0) 退出"
    echo ""
    echo "=========================================="
    echo ""
}

pause_menu() {
    echo ""
    read -r -p "按回车键继续..." _
}

interactive_menu() {
    local choice input_url

    if [ ! -t 0 ]; then
        show_usage
        return
    fi

    while true; do
        show_menu
        read -r -p "请选择操作 [0-17]: " choice
        echo ""

        case "$choice" in
            1)
                install_mihomo
                pause_menu
                ;;
            2)
                read -r -p "请输入订阅链接（留空使用已保存链接）: " input_url
                update_subscription "$input_url"
                pause_menu
                ;;
            3)
                start_service
                pause_menu
                ;;
            4)
                stop_service
                pause_menu
                ;;
            5)
                restart_service
                pause_menu
                ;;
            6)
                show_status
                pause_menu
                ;;
            7)
                show_logs
                ;;
            8)
                enable_service
                pause_menu
                ;;
            9)
                disable_service
                pause_menu
                ;;
            10)
                test_config
                pause_menu
                ;;
            11)
                verify_service
                pause_menu
                ;;
            12)
                install_dashboard
                pause_menu
                ;;
            13)
                edit_config
                pause_menu
                ;;
            14)
                enable_system_proxy
                pause_menu
                ;;
            15)
                disable_system_proxy_file
                pause_menu
                ;;
            16)
                uninstall_mihomo
                pause_menu
                ;;
            17)
                show_usage
                pause_menu
                ;;
            0)
                print_info "退出脚本"
                exit 0
                ;;
            *)
                print_error "无效的选择"
                pause_menu
                ;;
        esac
    done
}

show_usage() {
    cat <<EOF

==========================================
  Mihomo CLI 管理脚本
==========================================

使用方法:
  $0 menu
  $0 install
  $0 update [订阅链接]
  $0 start|stop|restart|status|logs
  $0 enable|disable
  $0 test
  $0 verify
  $0 edit
  $0 dashboard
  $0 proxy on|off
  $0 uninstall

常用示例:
  sudo bash $0
  sudo bash $0 install
  sudo bash $0 update "https://example.com/sub?format=clashmeta"
  sudo bash $0 verify
  sudo systemctl status $SERVICE_NAME

下载环境变量:
  MIHOMO_VERSION=v1.19.24
  MIHOMO_VARIANT=compatible
  GITHUB_PROXY_MODE=cn|direct|proxy
  CLASH_META_GITHUB_PROXY=https://gh.llkk.cc/
  GITHUB_PROXY_LIST='https://gh.llkk.cc/ https://ghfast.top/ ...'
  MIHOMO_DOWNLOAD_URL=https://example.com/mihomo-linux-amd64-compatible-v1.19.24.gz
  DASHBOARD_URL=https://github.com/MetaCubeX/metacubexd/archive/refs/heads/gh-pages.zip
  CURL_METADATA_MAX_TIME=20
  CURL_LOW_SPEED_LIMIT=10240
  CURL_LOW_SPEED_TIME=20
  VERIFY_URLS='https://www.google.com/generate_204 ...'

配置文件: $CONFIG_DIR/config.yaml
订阅链接: $SUBSCRIPTION_FILE
配置备份: $BACKUP_DIR
面板目录: $DASHBOARD_DIR
控制面板: http://服务器IP:9090/ui
注意: 9090 监听外网时请限制访问来源，避免控制接口暴露。

EOF
}

main() {
    case "${1:-menu}" in
        menu) interactive_menu ;;
        install) install_mihomo ;;
        uninstall) uninstall_mihomo ;;
        start) start_service ;;
        stop) stop_service ;;
        restart) restart_service ;;
        status) show_status ;;
        logs) show_logs ;;
        enable) enable_service ;;
        disable) disable_service ;;
        update) update_subscription "${2:-}" ;;
        test) test_config ;;
        verify) verify_service ;;
        edit) edit_config ;;
        dashboard) install_dashboard ;;
        proxy)
            case "${2:-}" in
                on|enable) enable_system_proxy ;;
                off|disable) disable_system_proxy_file ;;
                *) print_error "用法: $0 proxy on|off"; exit 1 ;;
            esac
            ;;
        help|--help|-h|"") show_usage ;;
        *)
            print_error "未知命令: $1"
            show_usage
            exit 1
            ;;
    esac
}

main "$@"
