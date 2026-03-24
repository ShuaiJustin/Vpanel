#!/bin/bash
# V Panel Node Agent 安装脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== V Panel Node Agent 安装脚本 ===${NC}"
echo ""

# 检查是否为root
if [ "$EUID" -ne 0 ]; then 
    echo -e "${RED}错误: 请使用root权限运行此脚本${NC}"
    exit 1
fi

# 检查Go是否安装
if ! command -v go &> /dev/null; then
    echo -e "${RED}错误: 未检测到Go，请先安装Go 1.21+${NC}"
    exit 1
fi

# 安装目录
INSTALL_DIR="/opt/vpanel-agent"
LOG_DIR="/var/log/vpanel-agent"
BACKUP_DIR="/var/backups/xray"

echo -e "${YELLOW}1. 创建安装目录...${NC}"
mkdir -p "$INSTALL_DIR"
mkdir -p "$LOG_DIR"
mkdir -p "$BACKUP_DIR"

echo -e "${YELLOW}2. 编译Agent...${NC}"
go build -o "$INSTALL_DIR/vpanel-agent" ./cmd/agent/main.go

echo -e "${YELLOW}3. 复制配置文件...${NC}"
mkdir -p "$INSTALL_DIR/configs"
if [ ! -f "$INSTALL_DIR/configs/agent.yaml" ]; then
    cp configs/agent.yaml.example "$INSTALL_DIR/configs/agent.yaml"
    echo -e "${GREEN}已创建配置文件: $INSTALL_DIR/configs/agent.yaml${NC}"
    echo -e "${YELLOW}请编辑配置文件并填入Panel URL和Token${NC}"
else
    echo -e "${GREEN}配置文件已存在，跳过${NC}"
fi

echo -e "${YELLOW}4. 安装systemd服务...${NC}"
cp deployments/systemd/vpanel-agent.service /etc/systemd/system/
systemctl daemon-reload

echo -e "${YELLOW}5. 设置权限...${NC}"
chmod +x "$INSTALL_DIR/vpanel-agent"
chown -R root:root "$INSTALL_DIR"
chmod 600 "$INSTALL_DIR/configs/agent.yaml"

echo ""
echo -e "${GREEN}=== 安装完成 ===${NC}"
echo ""
echo -e "${YELLOW}下一步操作：${NC}"
echo "1. 编辑配置文件："
echo "   vim $INSTALL_DIR/configs/agent.yaml"
echo ""
echo "2. 启动服务："
echo "   systemctl start vpanel-agent"
echo ""
echo "3. 设置开机自启："
echo "   systemctl enable vpanel-agent"
echo ""
echo "4. 查看状态："
echo "   systemctl status vpanel-agent"
echo ""
echo "5. 查看日志："
echo "   journalctl -u vpanel-agent -f"
echo ""
