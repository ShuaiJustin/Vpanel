#!/bin/bash

# 健康检查脚本
# 用于检查 Panel 和 Agent 的运行状态

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

PANEL_URL=${PANEL_URL:-"http://localhost:8080"}
AGENT_PORT=${AGENT_PORT:-18443}

show_help() {
    echo "V Panel 健康检查脚本"
    echo ""
    echo "用法:"
    echo "  $0 panel              检查 Panel 状态"
    echo "  $0 agent              检查 Agent 状态"
    echo "  $0 all                检查所有组件"
    echo ""
    echo "环境变量:"
    echo "  PANEL_URL             Panel 地址 (默认: http://localhost:8080)"
    echo "  AGENT_PORT            Agent 端口 (默认: 18443)"
    echo ""
}

check_panel() {
    echo -e "${YELLOW}检查 Panel 状态...${NC}"
    echo ""
    
    # 检查进程
    if pgrep -x "vpanel" > /dev/null; then
        echo -e "${GREEN}✓ Panel 进程运行中${NC}"
    else
        echo -e "${RED}✗ Panel 进程未运行${NC}"
        return 1
    fi
    
    # 检查 HTTP 端点
    if curl -s -f "${PANEL_URL}/health" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Panel HTTP 响应正常${NC}"
    else
        echo -e "${RED}✗ Panel HTTP 无响应${NC}"
        return 1
    fi
    
    # 检查数据库连接
    response=$(curl -s "${PANEL_URL}/health" 2>/dev/null || echo "{}")
    if echo "$response" | grep -q "ok"; then
        echo -e "${GREEN}✓ Panel 健康检查通过${NC}"
    else
        echo -e "${RED}✗ Panel 健康检查失败${NC}"
        echo "响应: $response"
        return 1
    fi
    
    # 检查日志文件
    if [ -f "logs/vpanel.log" ]; then
        error_count=$(tail -100 logs/vpanel.log | grep -c "ERROR" || echo "0")
        if [ "$error_count" -gt 10 ]; then
            echo -e "${YELLOW}⚠ 最近有 $error_count 个错误日志${NC}"
        else
            echo -e "${GREEN}✓ 日志状态正常${NC}"
        fi
    fi
    
    echo ""
    echo -e "${GREEN}Panel 状态: 正常${NC}"
}

check_agent() {
    echo -e "${YELLOW}检查 Agent 状态...${NC}"
    echo ""
    
    # 检查进程
    if pgrep -x "vpanel-agent" > /dev/null; then
        echo -e "${GREEN}✓ Agent 进程运行中${NC}"
    else
        echo -e "${RED}✗ Agent 进程未运行${NC}"
        return 1
    fi
    
    # 检查 systemd 服务
    if systemctl is-active --quiet vpanel-agent 2>/dev/null; then
        echo -e "${GREEN}✓ Agent 服务运行中${NC}"
    else
        echo -e "${YELLOW}⚠ Agent 服务状态未知或未使用 systemd${NC}"
    fi
    
    # 检查健康端点
    if curl -s -f "http://localhost:${AGENT_PORT}/health" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Agent 健康端点响应正常${NC}"
    else
        echo -e "${YELLOW}⚠ Agent 健康端点无响应${NC}"
    fi
    
    # 检查 Xray
    if command -v xray &> /dev/null; then
        echo -e "${GREEN}✓ Xray 已安装${NC}"
        xray_version=$(xray version 2>/dev/null | head -1 || echo "unknown")
        echo "  版本: $xray_version"
    else
        echo -e "${RED}✗ Xray 未安装${NC}"
        return 1
    fi
    
    # 检查 Xray 进程
    if pgrep -x "xray" > /dev/null; then
        echo -e "${GREEN}✓ Xray 进程运行中${NC}"
    else
        echo -e "${YELLOW}⚠ Xray 进程未运行${NC}"
    fi
    
    # 检查配置文件
    if [ -f "/etc/vpanel/agent.yaml" ]; then
        echo -e "${GREEN}✓ Agent 配置文件存在${NC}"
    else
        echo -e "${RED}✗ Agent 配置文件不存在${NC}"
        return 1
    fi
    
    if [ -f "/etc/xray/config.json" ]; then
        echo -e "${GREEN}✓ Xray 配置文件存在${NC}"
    else
        echo -e "${YELLOW}⚠ Xray 配置文件不存在${NC}"
    fi
    
    # 检查日志
    if journalctl -u vpanel-agent -n 10 --no-pager 2>/dev/null | grep -q "ERROR"; then
        echo -e "${YELLOW}⚠ Agent 日志中有错误${NC}"
        echo "最近的错误:"
        journalctl -u vpanel-agent -n 5 --no-pager 2>/dev/null | grep "ERROR" || true
    else
        echo -e "${GREEN}✓ Agent 日志状态正常${NC}"
    fi
    
    echo ""
    echo -e "${GREEN}Agent 状态: 正常${NC}"
}

check_system() {
    echo -e "${YELLOW}检查系统资源...${NC}"
    echo ""
    
    # CPU 使用率
    cpu_usage=$(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1 || echo "0")
    echo "CPU 使用率: ${cpu_usage}%"
    
    # 内存使用
    mem_info=$(free -h | grep "Mem:")
    echo "内存: $mem_info"
    
    # 磁盘使用
    disk_usage=$(df -h / | tail -1 | awk '{print $5}')
    echo "磁盘使用: $disk_usage"
    
    # 网络连接
    conn_count=$(netstat -an 2>/dev/null | grep ESTABLISHED | wc -l || echo "0")
    echo "活动连接数: $conn_count"
    
    echo ""
}

# 主逻辑
case "$1" in
    panel)
        check_panel
        ;;
    agent)
        check_agent
        ;;
    all)
        check_panel
        echo ""
        check_agent
        echo ""
        check_system
        ;;
    system)
        check_system
        ;;
    *)
        show_help
        exit 1
        ;;
esac
