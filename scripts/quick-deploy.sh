#!/bin/bash

# 快速部署脚本
# 用于快速部署 Panel 和 Agent

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

show_help() {
    echo "V Panel 快速部署脚本"
    echo ""
    echo "用法:"
    echo "  $0 panel              部署 Panel"
    echo "  $0 agent <panel-url> <token>  部署 Agent"
    echo "  $0 all                部署 Panel 和 Agent"
    echo ""
    echo "示例:"
    echo "  $0 panel"
    echo "  $0 agent https://panel.example.com node-token-here"
    echo ""
}

deploy_panel() {
    echo -e "${GREEN}部署 Panel...${NC}"
    
    # 检查是否已编译
    if [ ! -f "vpanel" ]; then
        echo -e "${YELLOW}编译 Panel...${NC}"
        go build -o vpanel ./cmd/v/main.go
    fi
    
    # 创建必要目录
    mkdir -p logs data
    
    # 检查配置文件
    if [ ! -f "configs/config.yaml" ]; then
        echo -e "${YELLOW}创建配置文件...${NC}"
        cp configs/config.yaml.example configs/config.yaml
        echo -e "${RED}请编辑 configs/config.yaml 配置数据库等信息${NC}"
        exit 1
    fi
    
    # 运行数据库迁移
    echo -e "${YELLOW}运行数据库迁移...${NC}"
    ./vpanel migrate || true
    
    # 启动 Panel
    echo -e "${GREEN}启动 Panel...${NC}"
    ./vpanel &
    
    echo ""
    echo -e "${GREEN}Panel 部署完成！${NC}"
    echo "访问: http://localhost:8080"
    echo "日志: tail -f logs/vpanel.log"
}

deploy_agent() {
    local panel_url=$1
    local node_token=$2
    
    if [ -z "$panel_url" ] || [ -z "$node_token" ]; then
        echo -e "${RED}错误: 需要提供 Panel URL 和 Token${NC}"
        echo "用法: $0 agent <panel-url> <token>"
        exit 1
    fi
    
    echo -e "${GREEN}部署 Agent...${NC}"
    echo "Panel URL: $panel_url"
    
    # 检查是否已编译
    if [ ! -f "vpanel-agent" ]; then
        echo -e "${YELLOW}编译 Agent...${NC}"
        go build -o vpanel-agent ./cmd/agent/main.go
    fi
    
    # 创建配置目录
    sudo mkdir -p /etc/vpanel
    sudo mkdir -p /var/log/vpanel
    
    # 创建配置文件
    echo -e "${YELLOW}创建配置文件...${NC}"
    sudo tee /etc/vpanel/agent.yaml > /dev/null <<EOF
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
  port: 18443
EOF
    
    # 安装 Agent
    echo -e "${YELLOW}安装 Agent...${NC}"
    sudo cp vpanel-agent /usr/local/bin/
    sudo chmod +x /usr/local/bin/vpanel-agent
    
    # 安装 Xray
    if ! command -v xray &> /dev/null; then
        echo -e "${YELLOW}安装 Xray...${NC}"
        bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install
    else
        echo -e "${GREEN}Xray 已安装${NC}"
    fi
    
    # 创建 systemd 服务
    echo -e "${YELLOW}创建 systemd 服务...${NC}"
    sudo tee /etc/systemd/system/vpanel-agent.service > /dev/null <<EOF
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
    
    # 启动服务
    echo -e "${YELLOW}启动服务...${NC}"
    sudo systemctl daemon-reload
    sudo systemctl enable vpanel-agent
    sudo systemctl start vpanel-agent
    
    echo ""
    echo -e "${GREEN}Agent 部署完成！${NC}"
    echo "查看状态: sudo systemctl status vpanel-agent"
    echo "查看日志: sudo journalctl -u vpanel-agent -f"
}

# 主逻辑
case "$1" in
    panel)
        deploy_panel
        ;;
    agent)
        deploy_agent "$2" "$3"
        ;;
    all)
        deploy_panel
        echo ""
        echo -e "${YELLOW}请在另一台服务器上运行:${NC}"
        echo "$0 agent <panel-url> <token>"
        ;;
    *)
        show_help
        exit 1
        ;;
esac
