#!/usr/bin/env bash

# sing-box CLI management script.
# Usage:
#   sudo bash sing-box-manager.sh install
#   sudo bash sing-box-manager.sh update "your Sing-box subscription url"
#   sudo bash sing-box-manager.sh status

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
CONFIG_DIR="${CONFIG_DIR:-/etc/sing-box}"
SERVICE_NAME="${SERVICE_NAME:-sing-box}"
BINARY_NAME="${BINARY_NAME:-sing-box}"
GITHUB_REPO="${GITHUB_REPO:-SagerNet/sing-box}"
DEFAULT_VERSION="${DEFAULT_VERSION:-v1.13.13}"
GITHUB_PROXY_MODE="${GITHUB_PROXY_MODE:-cn}"
GITHUB_PROXY_LIST="${GITHUB_PROXY_LIST:-https://gh.llkk.cc/ https://ghfast.top/ https://gh-proxy.com/ https://gh.ddlc.top/ https://ghproxy.net/}"
SUBSCRIPTION_FILE="$CONFIG_DIR/subscription.url"
CONFIG_FILE="$CONFIG_DIR/config.json"
BACKUP_DIR="$CONFIG_DIR/backups"
SYSTEM_PROXY_FILE="/etc/profile.d/sing-box.sh"

CURL_CONNECT_TIMEOUT="${CURL_CONNECT_TIMEOUT:-8}"
CURL_METADATA_MAX_TIME="${CURL_METADATA_MAX_TIME:-20}"
CURL_MAX_TIME="${CURL_MAX_TIME:-300}"
CURL_RETRY="${CURL_RETRY:-3}"
CURL_LOW_SPEED_LIMIT="${CURL_LOW_SPEED_LIMIT:-10240}"
CURL_LOW_SPEED_TIME="${CURL_LOW_SPEED_TIME:-20}"
SUBSCRIPTION_USER_AGENT="${SUBSCRIPTION_USER_AGENT:-sing-box/1.0}"
SUBSCRIPTION_CONNECT_TIMEOUT="${SUBSCRIPTION_CONNECT_TIMEOUT:-8}"
SUBSCRIPTION_MAX_TIME="${SUBSCRIPTION_MAX_TIME:-120}"
SUBSCRIPTION_RETRY="${SUBSCRIPTION_RETRY:-1}"
CONFIG_TEST_TIMEOUT="${CONFIG_TEST_TIMEOUT:-30}"
ENABLE_SYSTEM_PROXY="${ENABLE_SYSTEM_PROXY:-0}"
DEFAULT_MIXED_PORT="${DEFAULT_MIXED_PORT:-2080}"
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

ensure_python3() {
    if command -v python3 >/dev/null 2>&1; then
        return 0
    fi

    print_warning "未找到 python3，尝试自动安装..."
    if ! install_package_if_possible python3; then
        print_error "无法自动安装 python3。sing-box 订阅需要用 python3 安全合并 JSON 配置。"
        exit 1
    fi
}

check_systemd() {
    require_commands systemctl

    if ! systemctl >/dev/null 2>&1; then
        print_error "当前环境无法使用 systemd/systemctl"
        print_info "如是容器、OpenVZ 或很老的系统，请手动运行 sing-box 或改用其他 init 脚本"
        exit 1
    fi
}

check_installed() {
    if [ ! -x "$INSTALL_DIR/$BINARY_NAME" ]; then
        print_error "sing-box 未安装"
        print_info "请先运行: $0 install"
        exit 1
    fi
}

detect_arch() {
    local arch
    arch=$(uname -m)

    case "$arch" in
        x86_64|amd64) SING_BOX_ARCH="amd64" ;;
        i386|i486|i586|i686) SING_BOX_ARCH="386" ;;
        aarch64|arm64) SING_BOX_ARCH="arm64" ;;
        armv7l|armv7) SING_BOX_ARCH="armv7" ;;
        armv6l|armv6) SING_BOX_ARCH="armv6" ;;
        armv5tel|armv5) SING_BOX_ARCH="armv5" ;;
        mips64le) SING_BOX_ARCH="mips64le" ;;
        mipsle) SING_BOX_ARCH="mipsle" ;;
        mips) SING_BOX_ARCH="mips" ;;
        riscv64) SING_BOX_ARCH="riscv64" ;;
        loongarch64|loong64) SING_BOX_ARCH="loong64" ;;
        ppc64le) SING_BOX_ARCH="ppc64le" ;;
        s390x) SING_BOX_ARCH="s390x" ;;
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
    local proxy="${SING_BOX_GITHUB_PROXY:-${GITHUB_PROXY:-}}"
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

    if [ -n "${SING_BOX_VERSION:-}" ]; then
        LATEST_VERSION="$SING_BOX_VERSION"
        normalize_version
        print_info "使用指定版本: $LATEST_VERSION"
        return
    fi

    print_info "获取 sing-box 最新版本..."
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
    local version_number
    local variant_part=""

    version_number="${LATEST_VERSION#v}"
    if [ -n "${SING_BOX_VARIANT:-}" ]; then
        SING_BOX_VARIANT="${SING_BOX_VARIANT#-}"
        variant_part="-$SING_BOX_VARIANT"
        print_info "使用构建变体: $SING_BOX_VARIANT"
    fi

    ASSET_NAME="sing-box-${version_number}-linux-${SING_BOX_ARCH}${variant_part}.tar.gz"
}

check_binary_runtime() {
    local binary_file="${1:-$INSTALL_DIR/$BINARY_NAME}"
    local output

    output=$("$binary_file" version 2>&1) || {
        print_error "sing-box 二进制无法在当前机器运行"
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

    cat > "$CONFIG_FILE" <<EOF
{
  "log": {
    "level": "info",
    "timestamp": true
  },
  "inbounds": [
    {
      "type": "mixed",
      "tag": "mixed-in",
      "listen": "127.0.0.1",
      "listen_port": $DEFAULT_MIXED_PORT,
      "sniff": true,
      "sniff_override_destination": true
    }
  ],
  "outbounds": [
    {
      "type": "direct",
      "tag": "direct"
    },
    {
      "type": "block",
      "tag": "block"
    }
  ],
  "route": {
    "final": "direct"
  }
}
EOF

    chmod 0644 "$CONFIG_FILE"
    print_success "示例配置已创建: $CONFIG_FILE"
    print_info "默认本机 mixed HTTP/SOCKS 入口: 127.0.0.1:$DEFAULT_MIXED_PORT"
}

create_systemd_service() {
    check_systemd

    cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=sing-box Service
Documentation=https://sing-box.sagernet.org
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=$CONFIG_DIR
ExecStartPre=$INSTALL_DIR/$BINARY_NAME check -c $CONFIG_FILE
ExecStart=$INSTALL_DIR/$BINARY_NAME run -c $CONFIG_FILE
Restart=on-failure
RestartSec=5s
LimitNOFILE=1048576

# Sandboxing. Capabilities allow TUN and privileged ports when the config
# enables them; localhost-only mixed proxy configs work without exposing the
# machine as an open proxy.
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
SystemCallArchitectures=native

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    print_success "systemd 服务已创建: $SERVICE_NAME"
}

extract_inbound_ports() {
    local config_file="${1:-$CONFIG_FILE}"
    [ -f "$config_file" ] || return 0

    if ! command -v python3 >/dev/null 2>&1; then
        echo "$DEFAULT_MIXED_PORT"
        return 0
    fi

    python3 - "$config_file" <<'PY'
import json
import sys

try:
    with open(sys.argv[1], "r", encoding="utf-8") as fh:
        data = json.load(fh)
except Exception:
    sys.exit(0)

seen = set()
for inbound in data.get("inbounds", []) or []:
    if not isinstance(inbound, dict):
        continue
    port = inbound.get("listen_port")
    if isinstance(port, int) and 0 < port <= 65535 and port not in seen:
        seen.add(port)
        print(port)
PY
}

get_first_inbound_port() {
    local port
    port=$(extract_inbound_ports "$CONFIG_FILE" | sed -n '1p')
    echo "${port:-$DEFAULT_MIXED_PORT}"
}

configure_system_proxy() {
    local port

    if [ "$ENABLE_SYSTEM_PROXY" != "1" ]; then
        if [ -f "$SYSTEM_PROXY_FILE" ] && grep -q "sing-box 代理配置" "$SYSTEM_PROXY_FILE"; then
            mv "$SYSTEM_PROXY_FILE" "${SYSTEM_PROXY_FILE}.disabled"
            print_warning "已停用旧的全局代理配置: ${SYSTEM_PROXY_FILE}.disabled"
        fi

        print_info "默认不写入全局系统代理，避免服务未运行时 127.0.0.1 代理导致下载失败"
        print_info "如确实需要全局代理，请使用: ENABLE_SYSTEM_PROXY=1 $0 install"
        return
    fi

    port=$(get_first_inbound_port)
    cat > "$SYSTEM_PROXY_FILE" <<EOF
# sing-box 代理配置
export http_proxy=http://127.0.0.1:$port
export https_proxy=http://127.0.0.1:$port
export all_proxy=socks5h://127.0.0.1:$port
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

validate_config_file() {
    local config_file="$1"
    local output
    local status

    if [ ! -s "$config_file" ]; then
        print_error "配置文件为空"
        return 1
    fi

    set +e
    if command -v timeout >/dev/null 2>&1; then
        output=$(timeout "$CONFIG_TEST_TIMEOUT" "$INSTALL_DIR/$BINARY_NAME" check -c "$config_file" 2>&1)
        status=$?
    else
        output=$("$INSTALL_DIR/$BINARY_NAME" check -c "$config_file" 2>&1)
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

print_singbox_config_hint() {
    print_info "sing-box CLI 的 update 需要 Sing-box JSON 配置，通常订阅链接包含 format=singbox。"
    print_info "Clash/Mihomo 订阅请用: sudo bash mihomo-manager.sh update \"你的订阅链接\""
    print_info "Xray JSON 配置请用: sudo bash xray-manager.sh update \"你的 Xray JSON 配置链接\""
}

check_singbox_config_source_format() {
    local source_url="${1:-}"
    local config_file="${2:-}"
    local lower_url="${source_url,,}"
    local first_char

    case "$lower_url" in
        *format=clashmeta*|*format=clash*|*format=mihomo*)
            print_error "这个链接是 Clash/Mihomo 订阅，不是 sing-box JSON 配置"
            print_singbox_config_hint
            return 1
            ;;
        *format=v2rayn*|*format=v2ray*|*format=base64*|*format=raw*)
            print_error "这个链接看起来是 V2Ray/v2rayN 订阅或分享链接列表，不是 sing-box JSON 配置"
            print_singbox_config_hint
            return 1
            ;;
        *format=xray*)
            print_error "这个链接看起来是 Xray JSON 配置，不是 sing-box JSON 配置"
            print_singbox_config_hint
            return 1
            ;;
    esac

    [ -n "$config_file" ] || return 0
    [ -s "$config_file" ] || return 0

    if grep -Eq '^[[:space:]]*(mixed-port|socks-port|proxy-groups|proxy-providers|proxies|rules):' "$config_file"; then
        print_error "下载内容是 Clash/Mihomo YAML 配置，不是 sing-box JSON 配置"
        print_singbox_config_hint
        return 1
    fi

    if grep -Eq '^[[:space:]]*(vmess|vless|trojan|ss|ssr)://' "$config_file"; then
        print_error "下载内容是节点分享链接列表，不是 sing-box JSON 配置"
        print_singbox_config_hint
        return 1
    fi

    first_char=$(sed -n '/^[[:space:]]*$/d; s/^[[:space:]]*//; s/^\(.\).*/\1/p; q' "$config_file")
    if [ -n "$first_char" ] && [ "$first_char" != "{" ] && [ "$first_char" != "[" ]; then
        print_error "下载内容不是 JSON 对象或数组，sing-box 无法直接加载"
        print_singbox_config_hint
        return 1
    fi
}

normalize_singbox_config() {
    local input_file="$1"
    local output_file="$2"

    ensure_python3

    python3 - "$input_file" "$output_file" "$DEFAULT_MIXED_PORT" <<'PY'
import json
import sys

input_file, output_file, default_port = sys.argv[1], sys.argv[2], int(sys.argv[3])

with open(input_file, "r", encoding="utf-8") as fh:
    data = json.load(fh)

if isinstance(data, list):
    data = {"outbounds": data}

if not isinstance(data, dict):
    raise SystemExit("sing-box 配置必须是 JSON 对象，或 outbound 数组")

outbounds = data.get("outbounds")
if not isinstance(outbounds, list):
    outbounds = []
    data["outbounds"] = outbounds

tags = {
    outbound.get("tag")
    for outbound in outbounds
    if isinstance(outbound, dict) and isinstance(outbound.get("tag"), str)
}

first_proxy_tag = ""
for outbound in outbounds:
    if not isinstance(outbound, dict):
        continue
    tag = outbound.get("tag")
    outbound_type = outbound.get("type")
    if not isinstance(tag, str) or not tag:
        continue
    if outbound_type not in ("direct", "block", "dns"):
        first_proxy_tag = tag
        break

if "direct" not in tags:
    outbounds.append({"type": "direct", "tag": "direct"})
if "block" not in tags:
    outbounds.append({"type": "block", "tag": "block"})

inbounds = data.get("inbounds")
if not isinstance(inbounds, list) or not inbounds:
    data["inbounds"] = [
        {
            "type": "mixed",
            "tag": "mixed-in",
            "listen": "127.0.0.1",
            "listen_port": default_port,
            "sniff": True,
            "sniff_override_destination": True,
        }
    ]

route = data.get("route")
if not isinstance(route, dict):
    route = {}
if "final" not in route:
    route["final"] = first_proxy_tag or "direct"
data["route"] = route

log = data.get("log")
if not isinstance(log, dict):
    data["log"] = {"level": "info", "timestamp": True}

with open(output_file, "w", encoding="utf-8") as fh:
    json.dump(data, fh, ensure_ascii=False, indent=2)
    fh.write("\n")
PY
}

install_sing_box() {
    check_root
    require_commands curl tar uname mktemp date grep sed install find
    check_systemd

    local origin_download_url
    local download_url
    local temp_file
    local temp_dir
    local binary_path
    local downloaded=0

    print_info "开始安装 sing-box..."
    detect_arch
    print_info "系统架构: $SING_BOX_ARCH"
    get_latest_version
    build_asset_name

    origin_download_url="https://github.com/${GITHUB_REPO}/releases/download/${LATEST_VERSION}/${ASSET_NAME}"
    temp_file="/tmp/${ASSET_NAME}"
    temp_dir=$(mktemp -d /tmp/sing-box-install.XXXXXX)

    print_info "目标文件: $ASSET_NAME"
    rm -f "$temp_file"

    if [ -n "${SING_BOX_DOWNLOAD_URL:-}" ]; then
        if curl_download_file "$SING_BOX_DOWNLOAD_URL" "$temp_file"; then
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
        rm -rf "$temp_dir"
        print_error "下载失败"
        print_info "请检查网络连接或手动下载："
        print_info "$origin_download_url"
        print_info "如机器里有坏掉的代理变量，可先执行: unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY all_proxy ALL_PROXY"
        exit 1
    fi

    print_info "解压文件..."
    tar -xzf "$temp_file" -C "$temp_dir"
    binary_path=$(find "$temp_dir" -type f -name "$BINARY_NAME" | sed -n '1p')

    if [ -z "$binary_path" ] || [ ! -f "$binary_path" ]; then
        rm -rf "$temp_dir" "$temp_file"
        print_error "压缩包内未找到 sing-box 二进制"
        exit 1
    fi

    print_info "安装到系统..."
    mkdir -p "$INSTALL_DIR" "$CONFIG_DIR"
    install -m 0755 "$binary_path" "$INSTALL_DIR/$BINARY_NAME"
    rm -rf "$temp_dir" "$temp_file"

    if ! check_binary_runtime "$INSTALL_DIR/$BINARY_NAME"; then
        exit 1
    fi

    create_sample_config
    validate_config_file "$CONFIG_FILE"
    create_systemd_service
    configure_system_proxy

    print_success "sing-box 安装完成"
    show_usage
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
        read -r -p "请输入 Sing-box 订阅链接: " input_url
    fi

    if [ -z "$input_url" ]; then
        print_error "订阅链接不能为空"
        exit 1
    fi

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

is_tcp_port_open() {
    local host="$1"
    local port="$2"

    timeout 1 bash -c "</dev/tcp/$host/$port" >/dev/null 2>&1
}

subscription_proxy_urls() {
    local port
    for port in $(extract_inbound_ports "$CONFIG_FILE"); do
        echo "http://127.0.0.1:$port"
        echo "socks5h://127.0.0.1:$port"
    done
    echo "http://127.0.0.1:$DEFAULT_MIXED_PORT"
    echo "socks5h://127.0.0.1:$DEFAULT_MIXED_PORT"
}

curl_download_subscription() {
    local url="$1"
    local output="$2"
    local proxy_url
    local proxy_port
    local seen_proxies=" "

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

update_subscription() {
    check_root
    check_installed
    require_commands curl mktemp date grep sed
    check_systemd

    local raw_config
    local temp_config
    local backup_file=""
    local timestamp

    mkdir -p "$CONFIG_DIR"
    get_subscription_url "${1:-}"

    if ! check_singbox_config_source_format "$SUBSCRIPTION_URL"; then
        exit 1
    fi

    raw_config=$(mktemp "$CONFIG_DIR/config.json.download.XXXXXX")
    temp_config=$(mktemp "$CONFIG_DIR/config.json.normalized.XXXXXX")

    print_info "下载 Sing-box 订阅配置..."
    if ! curl_download_subscription "$SUBSCRIPTION_URL" "$raw_config"; then
        rm -f "$raw_config" "$temp_config"
        print_error "订阅配置下载失败，当前配置未变更"
        print_info "请确认订阅服务可访问、订阅链接未失效，并且格式选择为 Sing-box。"
        exit 1
    fi

    if ! check_singbox_config_source_format "$SUBSCRIPTION_URL" "$raw_config"; then
        rm -f "$raw_config" "$temp_config"
        exit 1
    fi

    if ! normalize_singbox_config "$raw_config" "$temp_config"; then
        rm -f "$raw_config" "$temp_config"
        print_error "Sing-box JSON 配置合并失败"
        print_singbox_config_hint
        exit 1
    fi

    rm -f "$raw_config"

    if ! validate_config_file "$temp_config"; then
        rm -f "$temp_config"
        print_singbox_config_hint
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
    save_subscription_url
    print_success "订阅配置更新成功"

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

verify_service() {
    check_root
    check_installed
    require_commands curl grep sed timeout
    check_systemd

    local failed=0
    local port
    local ports
    local proxy_url
    local proxy_port
    local seen_proxies=" "

    echo "=========================================="
    echo "  sing-box 使用验证"
    echo "=========================================="
    echo ""

    if [ ! -f "$CONFIG_FILE" ]; then
        print_error "配置文件不存在: $CONFIG_FILE"
        exit 1
    fi

    if validate_config_file "$CONFIG_FILE"; then
        print_success "配置文件可被 sing-box 正常加载"
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
            print_warning "本机 inbound 端口未监听: 127.0.0.1:$port"
            failed=1
        fi
    done

    for proxy_url in $(subscription_proxy_urls); do
        case "$seen_proxies" in
            *" $proxy_url "*) continue ;;
        esac
        seen_proxies="${seen_proxies}${proxy_url} "
        proxy_port=${proxy_url##*:}
        is_tcp_port_open 127.0.0.1 "$proxy_port" || continue

        if try_proxy_access "$proxy_url"; then
            print_success "sing-box 已可正常代理访问: $proxy_url"
            [ "$failed" -eq 0 ] && return 0
            print_warning "代理可用，但上面的基础检查仍有异常，请按提示处理"
            return 1
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
        print_error "未找到 nano/vim/vi 编辑器"
        print_info "配置文件位置: $CONFIG_FILE"
        exit 1
    fi

    validate_config_file "$CONFIG_FILE"
}

start_service() {
    check_root
    check_installed
    check_systemd

    print_info "启动 sing-box..."
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

    print_info "停止 sing-box..."
    systemctl stop "$SERVICE_NAME"
    print_success "服务已停止"
}

restart_service() {
    check_root
    check_installed
    check_systemd

    print_info "重启 sing-box..."
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
    echo "  sing-box 状态"
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
        print_info "本机 mixed 入口端口: $(extract_inbound_ports "$CONFIG_FILE" | tr '\n' ' ')"
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

    print_info "查看 sing-box 日志 (Ctrl+C 退出)"
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

uninstall_sing_box() {
    check_root
    check_systemd

    print_warning "即将卸载 sing-box"
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

    read -r -p "是否删除配置文件？(y/n): " answer
    echo
    if [[ "$answer" =~ ^[Yy]$ ]]; then
        rm -rf "$CONFIG_DIR"
        print_success "配置文件已删除"
    fi

    print_success "卸载完成"
}

show_menu() {
    clear 2>/dev/null || true
    echo "=========================================="
    echo "       sing-box CLI 管理脚本"
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
    echo "  1) 安装 sing-box"
    echo "  2) 更新 Sing-box 订阅配置"
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
    echo " 15) 卸载 sing-box"
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
                install_sing_box
                pause_menu
                ;;
            2)
                read -r -p "请输入 Sing-box 订阅链接（留空使用已保存链接）: " input_url
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
                uninstall_sing_box
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
    cat <<EOF

==========================================
  sing-box CLI 管理脚本
==========================================

使用方法:
  $0 menu
  $0 install
  $0 update [Sing-box 订阅链接]
  $0 start|stop|restart|status|logs
  $0 enable|disable
  $0 test
  $0 verify
  $0 edit
  $0 proxy on|off
  $0 uninstall

常用示例:
  sudo bash $0
  sudo bash $0 install
  sudo bash $0 update "https://example.com/sub?format=singbox"
  sudo bash $0 verify
  sudo systemctl status $SERVICE_NAME

下载环境变量:
  SING_BOX_VERSION=v1.13.13
  SING_BOX_VARIANT=glibc|musl
  GITHUB_PROXY_MODE=cn|direct|proxy
  SING_BOX_GITHUB_PROXY=https://gh.llkk.cc/
  GITHUB_PROXY_LIST='https://gh.llkk.cc/ https://ghfast.top/ ...'
  SING_BOX_DOWNLOAD_URL=https://example.com/sing-box-1.13.13-linux-amd64.tar.gz
  DEFAULT_MIXED_PORT=2080
  ENABLE_SYSTEM_PROXY=1
  CURL_METADATA_MAX_TIME=20
  CURL_LOW_SPEED_LIMIT=10240
  CURL_LOW_SPEED_TIME=20
  VERIFY_URLS='https://www.google.com/generate_204 ...'

配置文件: $CONFIG_FILE
订阅链接: $SUBSCRIPTION_FILE
配置备份: $BACKUP_DIR
默认本机 mixed HTTP/SOCKS 入口: 127.0.0.1:$DEFAULT_MIXED_PORT

EOF
}

main() {
    case "${1:-menu}" in
        menu)
            interactive_menu
            ;;
        install)
            install_sing_box
            ;;
        uninstall)
            uninstall_sing_box
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
            update_subscription "${2:-}"
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
        help|--help|-h|"")
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
