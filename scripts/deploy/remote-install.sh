#!/bin/bash
# V Panel Agent 远程安装脚本
# 支持: Ubuntu, Debian, CentOS, AlmaLinux, Rocky Linux, Fedora

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 配置变量（由部署程序替换）
PANEL_URL="{{PANEL_URL}}"
NODE_TOKEN="{{NODE_TOKEN}}"
AGENT_VERSION="{{AGENT_VERSION}}"
AGENT_DOWNLOAD_URL="{{AGENT_DOWNLOAD_URL}}"

# 安装路径
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/vpanel"
LOG_DIR="/var/log/vpanel"
BACKUP_DIR="/var/backups/xray"

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# 检测操作系统
detect_os() {
    log_step "检测操作系统..."
    
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        OS_VERSION=$VERSION_ID
        OS_NAME=$NAME
    elif [ -f /etc/redhat-release ]; then
        OS="centos"
        OS_NAME=$(cat /etc/redhat-release)
    else
        log_error "无法检测操作系统"
        exit 1
    fi
    
    log_info "操作系统: $OS_NAME"
    log_info "版本: $OS_VERSION"
    
    # 标准化 OS 名称
    case "$OS" in
        ubuntu|debian)
            PKG_MANAGER="apt-get"
            PKG_UPDATE="apt-get update"
            PKG_INSTALL="apt-get install -y"
            ;;
        centos|rhel|almalinux|rocky|fedora)
            PKG_MANAGER="yum"
            PKG_UPDATE="yum makecache"
            PKG_INSTALL="yum install -y"
            # CentOS 8+ 和 Fedora 使用 dnf
            if command -v dnf &> /dev/null; then
                PKG_MANAGER="dnf"
                PKG_UPDATE="dnf makecache"
                PKG_INSTALL="dnf install -y"
            fi
            ;;
        *)
            log_error "不支持的操作系统: $OS"
            exit 1
            ;;
    esac
    
    log_info "包管理器: $PKG_MANAGER"
}

# 检查 root 权限
check_root() {
    if [ "$EUID" -ne 0 ]; then 
        log_error "请使用 root 权限运行此脚本"
        exit 1
    fi
}

# 检查系统要求
check_requirements() {
    log_step "检查系统要求..."
    
    # 检查内存
    total_mem=$(free -m | awk '/^Mem:/{print $2}')
    if [ "$total_mem" -lt 512 ]; then
        log_warn "内存不足 512MB，可能影响性能"
    fi
    
    # 检查磁盘空间
    available_space=$(df -m / | awk 'NR==2 {print $4}')
    if [ "$available_space" -lt 1024 ]; then
        log_warn "磁盘可用空间不足 1GB"
    fi
    
    # 检查 systemd
    if ! command -v systemctl &> /dev/null; then
        log_error "需要 systemd 支持"
        exit 1
    fi
    
    log_info "系统要求检查通过"
}

# 安装依赖
install_dependencies() {
    log_step "安装依赖包..."
    
    # 更新包索引
    log_info "更新包索引..."
    $PKG_UPDATE || log_warn "包索引更新失败，继续尝试安装"
    
    # 基础依赖
    local deps="curl wget unzip tar"
    
    # 根据不同系统添加特定依赖
    case "$OS" in
        ubuntu|debian)
            deps="$deps ca-certificates"
            ;;
        centos|rhel|almalinux|rocky|fedora)
            deps="$deps ca-certificates"
            ;;
    esac
    
    log_info "安装: $deps"
    $PKG_INSTALL $deps || {
        log_error "依赖安装失败"
        exit 1
    }
    
    log_info "依赖安装完成"
}

# 创建目录
create_directories() {
    log_step "创建安装目录..."
    
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$LOG_DIR"
    mkdir -p "$BACKUP_DIR"
    mkdir -p /etc/xray
    mkdir -p /var/log/xray
    
    log_info "目录创建完成"
}

# 安装 Xray
install_xray() {
    log_step "安装 Xray..."
    
    if command -v xray &> /dev/null; then
        log_info "Xray 已安装，跳过"
        return 0
    fi
    
    log_info "下载 Xray 安装脚本..."
    
    # 尝试使用官方安装脚本
    if curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh -o /tmp/install-xray.sh 2>&1; then
        bash /tmp/install-xray.sh install
        rm -f /tmp/install-xray.sh
    else
        log_warn "官方脚本下载失败，尝试备用方法..."
        
        # 手动安装 Xray
        XRAY_VERSION=$(curl -s https://api.github.com/repos/XTLS/Xray-core/releases/latest | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
        
        if [ -z "$XRAY_VERSION" ]; then
            log_error "无法获取 Xray 版本信息"
            exit 1
        fi
        
        # 检测系统架构
        ARCH=$(uname -m)
        case "$ARCH" in
            x86_64)
                XRAY_ARCH="linux-64"
                ;;
            aarch64|arm64)
                XRAY_ARCH="linux-arm64-v8a"
                ;;
            armv7l)
                XRAY_ARCH="linux-arm32-v7a"
                ;;
            *)
                log_error "不支持的架构: $ARCH"
                exit 1
                ;;
        esac
        
        XRAY_URL="https://github.com/XTLS/Xray-core/releases/download/${XRAY_VERSION}/Xray-${XRAY_ARCH}.zip"
        
        log_info "下载 Xray ${XRAY_VERSION} for ${XRAY_ARCH}..."
        wget -O /tmp/xray.zip "$XRAY_URL" || {
            log_error "Xray 下载失败"
            exit 1
        }
        
        log_info "解压并安装..."
        unzip -o /tmp/xray.zip -d /tmp/xray
        mv /tmp/xray/xray /usr/local/bin/
        chmod +x /usr/local/bin/xray
        rm -rf /tmp/xray /tmp/xray.zip
    fi
    
    # 验证安装
    if command -v xray &> /dev/null; then
        XRAY_VER=$(xray version | head -n 1)
        log_info "Xray 安装成功: $XRAY_VER"
    else
        log_error "Xray 安装失败"
        exit 1
    fi
}

# 下载并安装 Agent
install_agent() {
    log_step "安装 V Panel Agent..."
    
    if [ -z "$AGENT_DOWNLOAD_URL" ] || [ "$AGENT_DOWNLOAD_URL" = "{{AGENT_DOWNLOAD_URL}}" ]; then
        log_error "Agent 下载地址未配置"
        exit 1
    fi
    
    log_info "下载 Agent..."
    wget -O /tmp/vpanel-agent "$AGENT_DOWNLOAD_URL" || {
        log_error "Agent 下载失败"
        exit 1
    }
    
    log_info "安装 Agent..."
    mv /tmp/vpanel-agent "$INSTALL_DIR/vpanel-agent"
    chmod +x "$INSTALL_DIR/vpanel-agent"
    
    log_info "Agent 安装完成"
}

# 生成配置文件
generate_config() {
    log_step "生成配置文件..."
    
    if [ -z "$PANEL_URL" ] || [ "$PANEL_URL" = "{{PANEL_URL}}" ]; then
        log_error "Panel URL 未配置"
        exit 1
    fi
    
    if [ -z "$NODE_TOKEN" ] || [ "$NODE_TOKEN" = "{{NODE_TOKEN}}" ]; then
        log_error "Node Token 未配置"
        exit 1
    fi
    
    cat > "$CONFIG_DIR/agent.yaml" <<EOF
# V Panel Agent 配置文件
# 自动生成于: $(date)

panel:
  url: "$PANEL_URL"
  token: "$NODE_TOKEN"

xray:
  binary_path: "/usr/local/bin/xray"
  config_path: "/usr/local/etc/xray/config.json"

sync:
  interval: 5m

health:
  port: 18443

log:
  level: "info"
  file: "$LOG_DIR/agent.log"
EOF
    
    chmod 600 "$CONFIG_DIR/agent.yaml"
    log_info "配置文件已生成: $CONFIG_DIR/agent.yaml"
}

# 创建 systemd 服务
create_service() {
    log_step "创建 systemd 服务..."
    
    cat > /etc/systemd/system/vpanel-agent.service <<EOF
[Unit]
Description=V Panel Agent
Documentation=https://github.com/yourusername/vpanel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
ExecStart=$INSTALL_DIR/vpanel-agent -config $CONFIG_DIR/agent.yaml
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536

# 日志
StandardOutput=journal
StandardError=journal
SyslogIdentifier=vpanel-agent

[Install]
WantedBy=multi-user.target
EOF
    
    systemctl daemon-reload
    log_info "systemd 服务已创建"
}

# 启动服务
start_service() {
    log_step "启动 Agent 服务..."
    
    systemctl enable vpanel-agent
    systemctl start vpanel-agent
    
    sleep 2
    
    if systemctl is-active --quiet vpanel-agent; then
        log_info "Agent 服务启动成功"
    else
        log_error "Agent 服务启动失败"
        log_info "查看日志: journalctl -u vpanel-agent -n 50"
        exit 1
    fi
}

# 验证安装
verify_installation() {
    log_step "验证安装..."
    
    # 检查 Agent 二进制
    if [ ! -f "$INSTALL_DIR/vpanel-agent" ]; then
        log_error "Agent 二进制文件不存在"
        return 1
    fi
    
    # 检查配置文件
    if [ ! -f "$CONFIG_DIR/agent.yaml" ]; then
        log_error "配置文件不存在"
        return 1
    fi
    
    # 检查服务状态
    if ! systemctl is-active --quiet vpanel-agent; then
        log_error "Agent 服务未运行"
        return 1
    fi
    
    # 检查 Xray
    if ! command -v xray &> /dev/null; then
        log_warn "Xray 未安装"
    fi
    
    log_info "安装验证通过"
}

# 显示安装信息
show_info() {
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  V Panel Agent 安装完成！${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo -e "${YELLOW}安装信息:${NC}"
    echo "  操作系统: $OS_NAME"
    echo "  Agent 路径: $INSTALL_DIR/vpanel-agent"
    echo "  配置文件: $CONFIG_DIR/agent.yaml"
    echo "  日志目录: $LOG_DIR"
    echo ""
    echo -e "${YELLOW}常用命令:${NC}"
    echo "  查看状态: systemctl status vpanel-agent"
    echo "  查看日志: journalctl -u vpanel-agent -f"
    echo "  重启服务: systemctl restart vpanel-agent"
    echo "  停止服务: systemctl stop vpanel-agent"
    echo ""
    echo -e "${YELLOW}配置文件:${NC}"
    echo "  编辑配置: vim $CONFIG_DIR/agent.yaml"
    echo "  重载配置: systemctl restart vpanel-agent"
    echo ""
}

# 主函数
main() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}  V Panel Agent 自动安装脚本${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
    
    check_root
    detect_os
    check_requirements
    install_dependencies
    create_directories
    install_xray
    install_agent
    generate_config
    create_service
    start_service
    verify_installation
    show_info
    
    log_info "安装完成！"
}

# 执行主函数
main
