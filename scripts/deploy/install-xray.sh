#!/bin/bash

# Xray 自动安装脚本
# 用于 V Panel Agent 自动安装和配置 Xray

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

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

# 检查是否为 root 用户
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "请使用 root 用户运行此脚本"
        exit 1
    fi
}

# 检测操作系统
detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        VER=$VERSION_ID
    else
        log_error "无法检测操作系统"
        exit 1
    fi
    
    log_info "检测到操作系统: $OS $VER"
}

# 安装依赖
install_dependencies() {
    log_info "安装依赖包..."
    
    case $OS in
        ubuntu|debian)
            apt-get update
            apt-get install -y curl wget unzip
            ;;
        centos|rhel|fedora)
            yum install -y curl wget unzip
            ;;
        *)
            log_warn "未知操作系统，跳过依赖安装"
            ;;
    esac
}

# 检查 Xray 是否已安装
check_xray_installed() {
    if command -v xray &> /dev/null; then
        XRAY_VERSION=$(xray version 2>&1 | grep "Xray" | awk '{print $2}')
        log_info "Xray 已安装，版本: $XRAY_VERSION"
        return 0
    else
        log_info "Xray 未安装"
        return 1
    fi
}

# 安装 Xray
install_xray() {
    log_info "开始安装 Xray..."
    
    # 使用官方安装脚本
    bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install
    
    if [ $? -eq 0 ]; then
        log_info "Xray 安装成功"
    else
        log_error "Xray 安装失败"
        exit 1
    fi
}

# 配置 Xray
configure_xray() {
    log_info "配置 Xray..."
    
    # 创建配置目录
    mkdir -p /etc/xray
    mkdir -p /var/log/xray
    
    # 创建初始配置文件（如果不存在）
    if [ ! -f /etc/xray/config.json ]; then
        cat > /etc/xray/config.json <<EOF
{
  "log": {
    "loglevel": "warning",
    "access": "/var/log/xray/access.log",
    "error": "/var/log/xray/error.log"
  },
  "inbounds": [],
  "outbounds": [
    {
      "protocol": "freedom",
      "tag": "direct"
    }
  ]
}
EOF
        log_info "创建初始配置文件: /etc/xray/config.json"
    fi
    
    # 设置权限
    chown -R nobody:nogroup /etc/xray
    chmod 644 /etc/xray/config.json
    chown -R nobody:nogroup /var/log/xray
}

# 配置 systemd 服务
configure_systemd() {
    log_info "配置 systemd 服务..."
    
    # Xray 服务文件通常由安装脚本创建
    # 确保服务已启用
    systemctl daemon-reload
    systemctl enable xray
    
    log_info "Xray 服务已启用"
}

# 启动 Xray
start_xray() {
    log_info "启动 Xray 服务..."
    
    systemctl start xray
    
    if systemctl is-active --quiet xray; then
        log_info "Xray 服务启动成功"
    else
        log_error "Xray 服务启动失败"
        log_info "查看日志: journalctl -u xray -n 50"
        exit 1
    fi
}

# 验证安装
verify_installation() {
    log_info "验证安装..."
    
    # 检查 Xray 版本
    if command -v xray &> /dev/null; then
        XRAY_VERSION=$(xray version 2>&1 | grep "Xray" | awk '{print $2}')
        log_info "✓ Xray 版本: $XRAY_VERSION"
    else
        log_error "✗ Xray 命令未找到"
        return 1
    fi
    
    # 检查服务状态
    if systemctl is-active --quiet xray; then
        log_info "✓ Xray 服务运行中"
    else
        log_warn "✗ Xray 服务未运行"
    fi
    
    # 检查配置文件
    if [ -f /etc/xray/config.json ]; then
        log_info "✓ 配置文件存在: /etc/xray/config.json"
    else
        log_error "✗ 配置文件不存在"
        return 1
    fi
    
    # 测试配置
    if xray -test -config /etc/xray/config.json &> /dev/null; then
        log_info "✓ 配置文件语法正确"
    else
        log_warn "✗ 配置文件语法错误"
        xray -test -config /etc/xray/config.json
    fi
}

# 显示安装信息
show_info() {
    echo ""
    echo "========================================="
    echo "  Xray 安装完成"
    echo "========================================="
    echo ""
    echo "Xray 版本: $(xray version 2>&1 | grep "Xray" | awk '{print $2}')"
    echo "配置文件: /etc/xray/config.json"
    echo "日志目录: /var/log/xray/"
    echo ""
    echo "常用命令:"
    echo "  启动服务: systemctl start xray"
    echo "  停止服务: systemctl stop xray"
    echo "  重启服务: systemctl restart xray"
    echo "  查看状态: systemctl status xray"
    echo "  查看日志: journalctl -u xray -f"
    echo "  测试配置: xray -test -config /etc/xray/config.json"
    echo ""
    echo "========================================="
}

# 主函数
main() {
    log_info "开始安装 Xray..."
    
    check_root
    detect_os
    install_dependencies
    
    if check_xray_installed; then
        log_warn "Xray 已安装，跳过安装步骤"
    else
        install_xray
    fi
    
    configure_xray
    configure_systemd
    start_xray
    verify_installation
    show_info
    
    log_info "安装完成！"
}

# 运行主函数
main "$@"
