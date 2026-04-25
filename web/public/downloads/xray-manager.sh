#!/usr/bin/env bash

# Xray-core management script.
# Usage:
#   sudo bash xray-manager.sh install
#   sudo bash xray-manager.sh edit
#   sudo bash xray-manager.sh status

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
CONFIG_DIR="${CONFIG_DIR:-/usr/local/etc/xray}"
ASSET_DIR="${ASSET_DIR:-/usr/local/share/xray}"
SERVICE_NAME="${SERVICE_NAME:-xray}"
BINARY_NAME="${BINARY_NAME:-xray}"
GITHUB_REPO="${GITHUB_REPO:-XTLS/Xray-core}"
DEFAULT_VERSION="${DEFAULT_VERSION:-v26.3.27}"
GITHUB_PROXY_MODE="${GITHUB_PROXY_MODE:-cn}"
GITHUB_PROXY_LIST="${GITHUB_PROXY_LIST:-https://gh.llkk.cc/ https://ghfast.top/ https://gh-proxy.com/ https://gh.ddlc.top/ https://ghproxy.net/}"
CONFIG_FILE="$CONFIG_DIR/config.json"
CONFIG_URL_FILE="$CONFIG_DIR/config.url"
BACKUP_DIR="$CONFIG_DIR/backups"
SYSTEM_PROXY_FILE="/etc/profile.d/xray.sh"

CURL_CONNECT_TIMEOUT="${CURL_CONNECT_TIMEOUT:-8}"
CURL_METADATA_MAX_TIME="${CURL_METADATA_MAX_TIME:-20}"
CURL_MAX_TIME="${CURL_MAX_TIME:-300}"
CURL_RETRY="${CURL_RETRY:-3}"
CURL_LOW_SPEED_LIMIT="${CURL_LOW_SPEED_LIMIT:-10240}"
CURL_LOW_SPEED_TIME="${CURL_LOW_SPEED_TIME:-20}"
CONFIG_CONNECT_TIMEOUT="${CONFIG_CONNECT_TIMEOUT:-8}"
CONFIG_MAX_TIME="${CONFIG_MAX_TIME:-120}"
CONFIG_RETRY="${CONFIG_RETRY:-1}"
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

check_systemd() {
    require_commands systemctl

    if ! systemctl >/dev/null 2>&1; then
        print_error "当前环境无法使用 systemd/systemctl"
        print_info "如是容器、OpenVZ 或很老的系统，请手动运行 xray 或改用其他 init 脚本"
        exit 1
    fi
}

check_installed() {
    if [ ! -x "$INSTALL_DIR/$BINARY_NAME" ]; then
        print_error "Xray-core 未安装"
        print_info "请先运行: $0 install"
        exit 1
    fi
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
        return
    fi

    print_warning "未找到 unzip，尝试自动安装..."
    if ! install_package_if_possible unzip; then
        print_error "无法自动安装 unzip，请手动安装后重试"
        exit 1
    fi
}

detect_arch() {
    local arch
    arch=$(uname -m)

    case "$arch" in
        x86_64|amd64) XRAY_ARCH="64" ;;
        i386|i486|i586|i686) XRAY_ARCH="32" ;;
        aarch64|arm64) XRAY_ARCH="arm64-v8a" ;;
        armv7l|armv7) XRAY_ARCH="arm32-v7a" ;;
        armv6l|armv6) XRAY_ARCH="arm32-v6" ;;
        armv5tel|armv5) XRAY_ARCH="arm32-v5" ;;
        mips64le) XRAY_ARCH="mips64le" ;;
        mips64) XRAY_ARCH="mips64" ;;
        mipsle) XRAY_ARCH="mips32le" ;;
        mips) XRAY_ARCH="mips32" ;;
        riscv64) XRAY_ARCH="riscv64" ;;
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
    local proxy="${XRAY_GITHUB_PROXY:-${GITHUB_PROXY:-}}"
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

is_tcp_port_open() {
    local host="$1"
    local port="$2"

    timeout 1 bash -c "</dev/tcp/$host/$port" >/dev/null 2>&1
}

extract_inbound_ports() {
    local config_file="${1:-$CONFIG_FILE}"

    [ -f "$config_file" ] || return 0

    awk '
        /"inbounds"[[:space:]]*:/ { in_inbounds=1 }
        in_inbounds && /"outbounds"[[:space:]]*:/ { in_inbounds=0 }
        in_inbounds {
            line=$0
            while (match(line, /"port"[[:space:]]*:[[:space:]]*"?[0-9]+/)) {
                token=substr(line, RSTART, RLENGTH)
                gsub(/[^0-9]/, "", token)
                if (token != "") print token
                line=substr(line, RSTART + RLENGTH)
            }
        }
    ' "$config_file" | sort -n -u
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

normalize_version() {
    if [ -n "${LATEST_VERSION:-}" ] && [[ "$LATEST_VERSION" != v* ]]; then
        LATEST_VERSION="v$LATEST_VERSION"
    fi
}

get_latest_version() {
    local url

    if [ -n "${XRAY_VERSION:-}" ]; then
        LATEST_VERSION="$XRAY_VERSION"
        normalize_version
        print_info "使用指定版本: $LATEST_VERSION"
        return
    fi

    print_info "获取 Xray-core 最新版本..."
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

build_asset_name() {
    ASSET_NAME="Xray-linux-${XRAY_ARCH}.zip"
}

check_binary_runtime() {
    local binary_file="${1:-$INSTALL_DIR/$BINARY_NAME}"
    local output

    output=$("$binary_file" version 2>&1) || {
        print_error "Xray-core 二进制无法在当前机器运行"
        echo "$output"
        return 1
    }

    print_info "二进制版本: $(echo "$output" | head -1)"
}

create_sample_config() {
    mkdir -p "$CONFIG_DIR"

    if [ -f "$CONFIG_FILE" ]; then
        return
    fi

    cat > "$CONFIG_FILE" <<'EOF'
{
  "log": {
    "loglevel": "warning"
  },
  "inbounds": [
    {
      "tag": "socks-in",
      "listen": "127.0.0.1",
      "port": 10808,
      "protocol": "socks",
      "settings": {
        "udp": true
      }
    },
    {
      "tag": "http-in",
      "listen": "127.0.0.1",
      "port": 10809,
      "protocol": "http"
    }
  ],
  "outbounds": [
    {
      "tag": "direct",
      "protocol": "freedom"
    },
    {
      "tag": "block",
      "protocol": "blackhole"
    }
  ],
  "routing": {
    "rules": [
      {
        "type": "field",
        "ip": [
          "geoip:private"
        ],
        "outboundTag": "direct"
      }
    ]
  }
}
EOF

    chmod 0644 "$CONFIG_FILE"
    print_success "示例配置已创建: $CONFIG_FILE"
    print_warning "示例配置仅提供本机 HTTP/SOCKS 入口和直连出口，请按实际节点修改 outbound"
}

create_systemd_service() {
    check_systemd

    cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Xray Service
Documentation=https://github.com/XTLS/Xray-core
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=$CONFIG_DIR
ExecStartPre=$INSTALL_DIR/$BINARY_NAME run -test -config $CONFIG_FILE
ExecStart=$INSTALL_DIR/$BINARY_NAME run -config $CONFIG_FILE
Restart=on-failure
RestartSec=5s
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    print_success "systemd 服务已创建: $SERVICE_NAME"
}

configure_system_proxy() {
    if [ "$ENABLE_SYSTEM_PROXY" != "1" ]; then
        if [ -f "$SYSTEM_PROXY_FILE" ] && grep -q "Xray 代理配置" "$SYSTEM_PROXY_FILE"; then
            mv "$SYSTEM_PROXY_FILE" "${SYSTEM_PROXY_FILE}.disabled"
            print_warning "已停用旧的全局代理配置: ${SYSTEM_PROXY_FILE}.disabled"
        fi

        print_info "默认不写入全局系统代理，避免服务未运行时 127.0.0.1 代理导致下载失败"
        print_info "如确实需要全局代理，请使用: ENABLE_SYSTEM_PROXY=1 $0 install"
        return
    fi

    cat > "$SYSTEM_PROXY_FILE" <<'EOF'
# Xray 代理配置
export http_proxy=http://127.0.0.1:10809
export https_proxy=http://127.0.0.1:10809
export all_proxy=socks5h://127.0.0.1:10808
export no_proxy=localhost,127.0.0.1,::1
EOF

    print_success "系统代理配置已添加: HTTP 10809, SOCKS 10808"
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
        print_success "系统代理配置已停用: ${SYSTEM_PROXY_FILE}.disabled"
    else
        print_info "系统代理配置未启用"
    fi
}

validate_config_file() {
    local config_file="$1"
    local output
    local status

    if [ ! -s "$config_file" ]; then
        print_error "配置文件为空"
        return 1
    fi

    if ! grep -q '"inbounds"\|"outbounds"' "$config_file"; then
        print_warning "配置内容不像完整 Xray JSON，将继续交给 Xray 校验"
    fi

    set +e
    if command -v timeout >/dev/null 2>&1; then
        output=$(timeout "$CONFIG_TEST_TIMEOUT" "$INSTALL_DIR/$BINARY_NAME" run -test -config "$config_file" 2>&1)
        status=$?
    else
        output=$("$INSTALL_DIR/$BINARY_NAME" run -test -config "$config_file" 2>&1)
        status=$?
    fi
    set -e

    if [ "$status" -ne 0 ]; then
        print_error "配置校验失败"
        echo "$output"

        if [ "$status" -eq 124 ]; then
            print_info "配置校验超时，可设置更长超时: CONFIG_TEST_TIMEOUT=60 $0 test"
        fi

        return 1
    fi

    print_success "配置校验通过"
}

print_xray_json_config_hint() {
    print_info "Xray-core 的 update 只接受完整 Xray config.json 配置，内容通常包含 inbounds/outbounds。"
    print_info "Clash/Mihomo 订阅请用: sudo bash mihomo-manager.sh update \"你的订阅链接\""
    print_info "V2Ray/v2rayN 分享链接订阅不能被 Xray-core 直接加载，需要先转换成 Xray JSON。"
}

check_xray_config_source_format() {
    local source_url="${1:-}"
    local config_file="${2:-}"
    local lower_url="${source_url,,}"
    local first_char

    case "$lower_url" in
        *format=clashmeta*|*format=clash*|*format=mihomo*)
            print_error "这个链接是 Clash/Mihomo 订阅，不是 Xray JSON 配置"
            print_xray_json_config_hint
            return 1
            ;;
        *format=singbox*|*format=sing-box*)
            print_error "这个链接是 sing-box 配置，不是 Xray JSON 配置"
            print_xray_json_config_hint
            return 1
            ;;
        *format=v2rayn*|*format=v2ray*|*format=base64*|*format=raw*)
            print_error "这个链接看起来是 V2Ray/v2rayN 订阅或分享链接列表，不是完整 Xray JSON 配置"
            print_xray_json_config_hint
            return 1
            ;;
    esac

    [ -n "$config_file" ] || return 0
    [ -s "$config_file" ] || return 0

    if grep -Eq '^[[:space:]]*(mixed-port|socks-port|proxy-groups|proxy-providers|proxies|rules):' "$config_file"; then
        print_error "下载内容是 Clash/Mihomo YAML 配置，不是 Xray JSON 配置"
        print_xray_json_config_hint
        return 1
    fi

    if grep -Eq '^[[:space:]]*(vmess|vless|trojan|ss|ssr)://' "$config_file"; then
        print_error "下载内容是节点分享链接列表，不是完整 Xray JSON 配置"
        print_xray_json_config_hint
        return 1
    fi

    first_char=$(sed -n '/^[[:space:]]*$/d; s/^[[:space:]]*//; s/^\(.\).*/\1/p; q' "$config_file")
    if [ -n "$first_char" ] && [ "$first_char" != "{" ]; then
        print_error "下载内容不是 JSON 对象，Xray-core 无法直接加载"
        print_xray_json_config_hint
        return 1
    fi
}

install_xray() {
    check_root
    require_commands curl uname mktemp date grep sed install
    ensure_unzip
    check_systemd

    local origin_download_url
    local download_url
    local temp_file
    local temp_dir
    local downloaded=0

    print_info "开始安装 Xray-core..."
    detect_arch
    print_info "系统架构: $XRAY_ARCH"
    get_latest_version
    build_asset_name

    origin_download_url="https://github.com/${GITHUB_REPO}/releases/download/${LATEST_VERSION}/${ASSET_NAME}"
    temp_file="/tmp/${ASSET_NAME}"
    temp_dir=$(mktemp -d /tmp/xray-install.XXXXXX)

    print_info "目标文件: $ASSET_NAME"
    rm -f "$temp_file"

    if [ -n "${XRAY_DOWNLOAD_URL:-}" ]; then
        if curl_download_file "$XRAY_DOWNLOAD_URL" "$temp_file"; then
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

    if [ "$downloaded" -ne 1 ]; then
        rm -rf "$temp_dir"
        print_error "下载失败"
        print_info "请检查网络连接或手动下载："
        print_info "$origin_download_url"
        print_info "如机器里有坏掉的代理变量，可先执行: unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY all_proxy ALL_PROXY"
        exit 1
    fi

    if [ ! -s "$temp_file" ]; then
        rm -rf "$temp_dir"
        print_error "下载文件为空"
        exit 1
    fi

    print_info "解压文件..."
    unzip -oq "$temp_file" -d "$temp_dir"

    if [ ! -f "$temp_dir/xray" ]; then
        rm -rf "$temp_dir" "$temp_file"
        print_error "压缩包内未找到 xray 二进制"
        exit 1
    fi

    print_info "安装到系统..."
    mkdir -p "$INSTALL_DIR" "$CONFIG_DIR" "$ASSET_DIR"
    install -m 0755 "$temp_dir/xray" "$INSTALL_DIR/$BINARY_NAME"

    if [ -f "$temp_dir/geoip.dat" ]; then
        install -m 0644 "$temp_dir/geoip.dat" "$ASSET_DIR/geoip.dat"
    fi

    if [ -f "$temp_dir/geosite.dat" ]; then
        install -m 0644 "$temp_dir/geosite.dat" "$ASSET_DIR/geosite.dat"
    fi

    rm -rf "$temp_dir" "$temp_file"

    if ! check_binary_runtime "$INSTALL_DIR/$BINARY_NAME"; then
        exit 1
    fi

    create_sample_config
    validate_config_file "$CONFIG_FILE"
    create_systemd_service
    configure_system_proxy

    print_success "安装完成！"
    show_usage
}

restart_service_with_rollback() {
    local backup_file="$1"

    if ! systemctl is-active --quiet "$SERVICE_NAME"; then
        print_info "服务当前未运行，配置已更新但未重启"
        return 0
    fi

    print_info "重启服务..."
    if systemctl restart "$SERVICE_NAME"; then
        print_success "服务已重启"
        return 0
    fi

    print_error "服务重启失败"

    if [ -n "$backup_file" ] && [ -f "$backup_file" ]; then
        print_warning "正在恢复更新前配置..."
        cp -p "$backup_file" "$CONFIG_FILE"

        if systemctl restart "$SERVICE_NAME"; then
            print_success "已恢复旧配置并重启服务"
        else
            print_error "旧配置恢复后服务仍启动失败，请查看日志: journalctl -u $SERVICE_NAME -n 100 --no-pager"
        fi
    fi

    return 1
}

get_config_url() {
    local input_url="${1:-}"

    if [ -z "$input_url" ] && [ -n "${XRAY_CONFIG_URL:-}" ]; then
        input_url="$XRAY_CONFIG_URL"
    fi

    if [ -z "$input_url" ] && [ -s "$CONFIG_URL_FILE" ]; then
        input_url=$(sed -n '1p' "$CONFIG_URL_FILE")
        print_info "使用已保存配置链接"
    fi

    if [ -z "$input_url" ]; then
        read -r -p "请输入完整 Xray JSON 配置链接（不是 Clash/V2Ray 订阅）: " input_url
    fi

    if [ -z "$input_url" ]; then
        print_error "配置链接不能为空"
        exit 1
    fi

    XRAY_CONFIG_URL="$input_url"
}

save_config_url() {
    local old_umask
    old_umask=$(umask)
    umask 077
    printf '%s\n' "$XRAY_CONFIG_URL" > "$CONFIG_URL_FILE"
    umask "$old_umask"
    chmod 600 "$CONFIG_URL_FILE" 2>/dev/null || true
}

curl_download_config() {
    local url="$1"
    local output="$2"

    curl -fL \
        --retry "$CONFIG_RETRY" \
        --retry-delay 1 \
        --connect-timeout "$CONFIG_CONNECT_TIMEOUT" \
        --max-time "$CONFIG_MAX_TIME" \
        -o "$output" \
        "$url" \
        || curl_without_proxy -fL \
            --retry "$CONFIG_RETRY" \
            --retry-delay 1 \
            --connect-timeout "$CONFIG_CONNECT_TIMEOUT" \
            --max-time "$CONFIG_MAX_TIME" \
            -o "$output" \
            "$url"
}

update_config() {
    check_root
    check_installed
    require_commands curl mktemp date sed
    check_systemd

    local input_url="${1:-}"
    local temp_config
    local backup_file=""
    local timestamp

    mkdir -p "$CONFIG_DIR"
    get_config_url "$input_url"

    if ! check_xray_config_source_format "$XRAY_CONFIG_URL"; then
        exit 1
    fi

    print_info "下载 Xray JSON 配置..."
    temp_config=$(mktemp "$CONFIG_DIR/config.json.download.XXXXXX")
    mv "$temp_config" "${temp_config}.json"
    temp_config="${temp_config}.json"

    if ! curl_download_config "$XRAY_CONFIG_URL" "$temp_config"; then
        rm -f "$temp_config"
        print_error "配置下载失败，当前配置未变更"
        print_xray_json_config_hint
        exit 1
    fi

    if ! check_xray_config_source_format "$XRAY_CONFIG_URL" "$temp_config"; then
        rm -f "$temp_config"
        exit 1
    fi

    if ! validate_config_file "$temp_config"; then
        rm -f "$temp_config"
        print_xray_json_config_hint
        exit 1
    fi

    mkdir -p "$BACKUP_DIR"
    timestamp=$(date +%Y%m%d-%H%M%S)

    if [ -f "$CONFIG_FILE" ]; then
        backup_file="$BACKUP_DIR/config.json.$timestamp.backup"
        cp -p "$CONFIG_FILE" "$backup_file"
        print_info "已备份当前配置: $backup_file"
    fi

    chmod 0644 "$temp_config"
    mv -f "$temp_config" "$CONFIG_FILE"
    save_config_url
    print_success "配置更新成功"

    restart_service_with_rollback "$backup_file"
}

test_config() {
    check_installed

    if [ ! -f "$CONFIG_FILE" ]; then
        print_error "配置文件不存在: $CONFIG_FILE"
        exit 1
    fi

    validate_config_file "$CONFIG_FILE"
}

verify_service() {
    check_root
    check_installed
    require_commands curl grep sed timeout
    check_systemd

    local failed=0
    local port
    local ports
    local proxy_url

    echo "=========================================="
    echo "  Xray-core 使用验证"
    echo "=========================================="
    echo ""

    if [ ! -f "$CONFIG_FILE" ]; then
        print_error "配置文件不存在: $CONFIG_FILE"
        exit 1
    fi

    if validate_config_file "$CONFIG_FILE"; then
        print_success "配置文件可被 Xray-core 正常加载"
    else
        failed=1
    fi

    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_success "systemd 服务正在运行"
    else
        print_error "systemd 服务未运行，请先执行: $0 start"
        failed=1
    fi

    ports=$(extract_inbound_ports "$CONFIG_FILE" | tr '\n' ' ')
    if [ -z "$ports" ]; then
        print_warning "没有从 inbounds 中解析到本地监听端口"
        failed=1
    fi

    for port in $ports; do
        if is_tcp_port_open 127.0.0.1 "$port"; then
            print_success "本机 inbound 端口已监听: 127.0.0.1:$port"
        else
            print_warning "本机 inbound 端口未监听或不是本机入口: 127.0.0.1:$port"
            failed=1
        fi
    done

    for port in $ports; do
        proxy_url="http://127.0.0.1:$port"
        if try_proxy_access "$proxy_url"; then
            print_success "Xray HTTP 代理入口可正常使用: $proxy_url"
            [ "$failed" -eq 0 ] && return 0
            print_warning "代理可用，但上面的基础检查仍有异常，请按提示处理"
            return 1
        fi
    done

    for port in $ports; do
        proxy_url="socks5h://127.0.0.1:$port"
        if try_proxy_access "$proxy_url"; then
            print_success "Xray SOCKS 代理入口可正常使用: $proxy_url"
            [ "$failed" -eq 0 ] && return 0
            print_warning "代理可用，但上面的基础检查仍有异常，请按提示处理"
            return 1
        fi
    done

    print_error "未检测到可用的 HTTP/SOCKS 本地代理入口"
    print_info "如果这台机器是服务端入站节点，HTTP/SOCKS 客户端验证失败是正常的；请用客户端连接该节点验证。"
    print_info "可查看日志定位原因: journalctl -u $SERVICE_NAME -n 100 --no-pager"
    return 1
}

edit_config() {
    check_root
    check_installed
    require_commands date

    if [ ! -f "$CONFIG_FILE" ]; then
        print_error "配置文件不存在: $CONFIG_FILE"
        exit 1
    fi

    local backup_file

    mkdir -p "$BACKUP_DIR"
    backup_file="$BACKUP_DIR/config.json.edit.$(date +%Y%m%d-%H%M%S).backup"
    cp -p "$CONFIG_FILE" "$backup_file"
    print_info "编辑前备份: $backup_file"

    if command -v nano >/dev/null 2>&1; then
        nano "$CONFIG_FILE"
    elif command -v vim >/dev/null 2>&1; then
        vim "$CONFIG_FILE"
    elif command -v vi >/dev/null 2>&1; then
        vi "$CONFIG_FILE"
    else
        print_error "未找到可用的编辑器"
        print_info "配置文件位置: $CONFIG_FILE"
        exit 1
    fi

    print_success "配置已保存"

    if ! validate_config_file "$CONFIG_FILE"; then
        print_warning "配置未通过校验，已跳过自动重启"
        return 1
    fi

    check_systemd
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        read -r -p "是否重启服务以应用配置？(y/n): " answer
        echo
        if [[ "$answer" =~ ^[Yy]$ ]]; then
            systemctl restart "$SERVICE_NAME"
            print_success "服务已重启"
        fi
    fi
}

start_service() {
    check_root
    check_installed
    check_systemd

    print_info "启动 Xray-core..."
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

    print_info "停止 Xray-core..."
    systemctl stop "$SERVICE_NAME"
    print_success "服务已停止"
}

restart_service() {
    check_root
    check_installed
    check_systemd

    print_info "重启 Xray-core..."
    systemctl restart "$SERVICE_NAME"
    sleep 1

    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_success "服务已重启"
        systemctl status "$SERVICE_NAME" --no-pager || true
    else
        print_error "服务重启失败"
        systemctl status "$SERVICE_NAME" --no-pager || true
        exit 1
    fi
}

show_status() {
    check_installed
    check_systemd

    echo "=========================================="
    echo "  Xray-core 状态"
    echo "=========================================="
    echo ""

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

    if [ -x "$INSTALL_DIR/$BINARY_NAME" ]; then
        print_info "版本: $("$INSTALL_DIR/$BINARY_NAME" version 2>&1 | head -1)"
    fi

    if [ -f "$CONFIG_FILE" ]; then
        print_info "配置文件: $CONFIG_FILE"
    else
        print_warning "配置文件不存在"
    fi

    echo ""
    echo "详细状态:"
    systemctl status "$SERVICE_NAME" --no-pager || true
}

show_logs() {
    check_installed
    check_systemd

    print_info "查看 Xray-core 日志 (Ctrl+C 退出)"
    journalctl -u "$SERVICE_NAME" -f
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

uninstall_xray() {
    check_root
    check_systemd

    print_warning "即将卸载 Xray-core"
    read -r -p "确认继续？(y/n): " answer
    echo
    if [[ ! "$answer" =~ ^[Yy]$ ]]; then
        print_info "已取消"
        exit 0
    fi

    if systemctl is-active --quiet "$SERVICE_NAME"; then
        systemctl stop "$SERVICE_NAME"
    fi

    if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
        systemctl disable "$SERVICE_NAME"
    fi

    rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
    systemctl daemon-reload
    rm -f "$INSTALL_DIR/$BINARY_NAME"
    rm -f "$SYSTEM_PROXY_FILE" "${SYSTEM_PROXY_FILE}.disabled"

    read -r -p "是否删除配置和 geo 数据？(y/n): " answer
    echo
    if [[ "$answer" =~ ^[Yy]$ ]]; then
        rm -rf "$CONFIG_DIR" "$ASSET_DIR"
        print_success "配置和 geo 数据已删除"
    fi

    print_success "卸载完成"
}

show_menu() {
    clear 2>/dev/null || true
    echo "=========================================="
    echo "       Xray-core 管理脚本"
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
    echo "  1) 安装 Xray-core"
    echo "  2) 更新 Xray JSON 配置"
    echo "  3) 启动服务"
    echo "  4) 停止服务"
    echo "  5) 重启服务"
    echo "  6) 查看状态"
    echo "  7) 查看日志"
    echo "  8) 启用开机自启"
    echo "  9) 禁用开机自启"
    echo " 10) 校验配置"
    echo " 11) 验证是否可用"
    echo " 12) 编辑配置"
    echo " 13) 启用系统环境代理"
    echo " 14) 停用系统环境代理"
    echo " 15) 卸载 Xray-core"
    echo " 16) 显示帮助"
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
        read -r -p "请选择操作 [0-16]: " choice
        echo ""

        case "$choice" in
            1)
                install_xray
                pause_menu
                ;;
            2)
                read -r -p "请输入完整 Xray JSON 配置链接（不是 Clash/V2Ray 订阅，留空使用已保存链接）: " input_url
                update_config "$input_url"
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
                edit_config
                pause_menu
                ;;
            13)
                enable_system_proxy
                pause_menu
                ;;
            14)
                disable_system_proxy_file
                pause_menu
                ;;
            15)
                uninstall_xray
                pause_menu
                ;;
            16)
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
    echo ""
    echo "=========================================="
    echo "  Xray-core 管理脚本"
    echo "=========================================="
    echo ""
    echo "使用方法: $0 [命令]"
    echo "JSON 配置更新: $0 update [完整 Xray JSON 配置链接]"
    echo ""
    echo "命令列表:"
    echo "  menu             打开交互式菜单"
    echo "  install          安装 Xray-core"
    echo "  uninstall        卸载 Xray-core"
    echo "  start            启动服务"
    echo "  stop             停止服务"
    echo "  restart          重启服务"
    echo "  status           查看状态"
    echo "  logs             查看日志"
    echo "  enable           启用开机自启"
    echo "  disable          禁用开机自启"
    echo "  update           下载并应用完整 Xray JSON 配置（不是订阅链接）"
    echo "  test             校验当前配置"
    echo "  verify           验证服务、端口和本地代理连通性"
    echo "  edit             编辑配置文件"
    echo "  proxy on/off     启用/停用系统环境代理"
    echo "  help             显示帮助信息"
    echo ""
    echo "下载相关环境变量:"
    echo "  XRAY_VERSION=v26.3.27                  指定版本，跳过自动获取最新版"
    echo "  GITHUB_PROXY_MODE=cn|direct|proxy      cn 默认镜像优先，direct 直连优先，proxy 只走镜像"
    echo "  XRAY_GITHUB_PROXY=https://.../         高级: 指定单个 GitHub 加速地址"
    echo "  GITHUB_PROXY_LIST='https://.../'       高级: 覆盖自动加速地址列表"
    echo "  XRAY_DOWNLOAD_URL=https://...zip       直接指定完整二进制下载地址"
    echo "  XRAY_CONFIG_URL=https://...json        非交互传入 Xray JSON 配置链接"
    echo "  ENABLE_SYSTEM_PROXY=1                  安装后写入 /etc/profile.d 全局代理"
    echo "  CURL_RETRY=3                           下载失败重试次数"
    echo "  CURL_METADATA_MAX_TIME=20              版本查询超时时间(秒)"
    echo "  CURL_LOW_SPEED_LIMIT=10240             低于该速度视为慢速下载并切换地址"
    echo "  CURL_LOW_SPEED_TIME=20                 慢速持续秒数"
    echo "  CONFIG_TEST_TIMEOUT=30                 配置校验超时时间(秒)"
    echo "  VERIFY_URLS='https://www.google.com/generate_204 ...'"
    echo ""
    echo "配置文件: $CONFIG_FILE"
    echo "配置备份: $BACKUP_DIR"
    echo "geo 数据: $ASSET_DIR"
    echo ""
    echo "常用示例:"
    echo "  sudo bash $0"
    echo "  sudo bash $0 install"
    echo "  sudo bash $0 edit"
    echo "  sudo bash $0 verify"
    echo ""
}

main() {
    case "${1:-menu}" in
        menu)
            interactive_menu
            ;;
        install)
            install_xray
            ;;
        uninstall)
            uninstall_xray
            ;;
        start)
            start_service
            ;;
        stop)
            stop_service
            ;;
        restart)
            restart_service
            ;;
        status)
            show_status
            ;;
        logs)
            show_logs
            ;;
        enable)
            enable_service
            ;;
        disable)
            disable_service
            ;;
        update)
            update_config "${2:-}"
            ;;
        test)
            test_config
            ;;
        verify)
            verify_service
            ;;
        edit)
            edit_config
            ;;
        proxy)
            case "${2:-}" in
                on|enable)
                    enable_system_proxy
                    ;;
                off|disable)
                    disable_system_proxy_file
                    ;;
                *)
                    print_error "用法: $0 proxy on|off"
                    exit 1
                    ;;
            esac
            ;;
        help|--help|-h)
            show_usage
            ;;
        *)
            print_error "未知命令: $1"
            show_usage
            exit 1
            ;;
    esac
}

main "$@"
