#!/bin/bash
# V Panel 生产环境部署检查脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$SCRIPT_DIR/common.sh"

echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}  V Panel 生产环境部署检查${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""

# 检查计数器
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0
WARNING_CHECKS=0

# 检查函数
check_item() {
    local name=$1
    local status=$2
    local message=$3
    
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    
    case $status in
        pass)
            echo -e "${GREEN}✓${NC} $name"
            [ -n "$message" ] && echo -e "  ${message}"
            PASSED_CHECKS=$((PASSED_CHECKS + 1))
            ;;
        fail)
            echo -e "${RED}✗${NC} $name"
            [ -n "$message" ] && echo -e "  ${RED}${message}${NC}"
            FAILED_CHECKS=$((FAILED_CHECKS + 1))
            ;;
        warn)
            echo -e "${YELLOW}!${NC} $name"
            [ -n "$message" ] && echo -e "  ${YELLOW}${message}${NC}"
            WARNING_CHECKS=$((WARNING_CHECKS + 1))
            ;;
    esac
}

# 1. 检查 Docker 环境
echo -e "${CYAN}[1/6] Docker 环境检查${NC}"
if command -v docker &> /dev/null; then
    check_item "Docker 已安装" "pass" "$(docker --version)"
else
    check_item "Docker 已安装" "fail" "请先安装 Docker"
fi

if compose_available; then
    check_item "Docker Compose 已安装" "pass"
else
    check_item "Docker Compose 已安装" "fail" "请先安装 Docker Compose"
fi

# 检查 Docker 是否运行
if docker info &> /dev/null; then
    check_item "Docker 服务运行中" "pass"
else
    check_item "Docker 服务运行中" "fail" "Docker 服务未启动"
fi

echo ""

# 2. 检查配置文件
echo -e "${CYAN}[2/6] 配置文件检查${NC}"
if [ -f "$DOCKER_DIR/.env" ]; then
    check_item ".env 文件存在" "pass"
    
    # 读取配置
    V_SERVER_MODE=$(read_env_var "V_SERVER_MODE" "$DOCKER_DIR/.env")
    V_JWT_SECRET=$(read_env_var "V_JWT_SECRET" "$DOCKER_DIR/.env")
    V_ADMIN_PASS=$(read_env_var "V_ADMIN_PASS" "$DOCKER_DIR/.env")
    V_SERVER_PORT=$(read_env_var "V_SERVER_PORT" "$DOCKER_DIR/.env")
    
    # 检查服务器模式
    if [ "$V_SERVER_MODE" = "release" ]; then
        check_item "服务器模式" "pass" "生产模式 (release)"
    else
        check_item "服务器模式" "warn" "当前模式: ${V_SERVER_MODE} (建议使用 release)"
    fi
    
else
    check_item ".env 文件存在" "fail" "请先创建 .env 文件"
fi

echo ""

# 3. 安全配置检查
echo -e "${CYAN}[3/6] 安全配置检查${NC}"

# 检查 JWT Secret
if [ -n "$V_JWT_SECRET" ]; then
    if [ ${#V_JWT_SECRET} -lt 32 ]; then
        check_item "JWT Secret 长度" "fail" "长度不足 32 字符 (当前: ${#V_JWT_SECRET})"
    elif is_default_jwt_secret "$V_JWT_SECRET"; then
        check_item "JWT Secret" "fail" "使用默认值，必须修改！"
    else
        check_item "JWT Secret" "pass" "已配置 (${#V_JWT_SECRET} 字符)"
    fi
else
    check_item "JWT Secret" "fail" "未配置"
fi

# 检查管理员密码
if [ -n "$V_ADMIN_PASS" ]; then
    if is_default_admin_password "$V_ADMIN_PASS"; then
        check_item "管理员密码" "fail" "使用默认值，必须修改！"
    elif [ ${#V_ADMIN_PASS} -lt 12 ]; then
        check_item "管理员密码强度" "fail" "长度不足 12 字符"
    elif ! validate_password "$V_ADMIN_PASS"; then
        check_item "管理员密码强度" "warn" "建议包含大小写字母、数字和特殊字符"
    else
        check_item "管理员密码" "pass" "强度良好"
    fi
else
    check_item "管理员密码" "fail" "未配置"
fi

echo ""

# 4. 网络配置检查
echo -e "${CYAN}[4/6] 网络配置检查${NC}"

if [ -n "$V_SERVER_PORT" ]; then
    check_item "服务端口配置" "pass" "端口: ${V_SERVER_PORT}"
    
    # 检查端口是否被占用
    if command -v lsof &> /dev/null; then
        if lsof -i :$V_SERVER_PORT &> /dev/null; then
            check_item "端口可用性" "fail" "端口 ${V_SERVER_PORT} 已被占用"
        else
            check_item "端口可用性" "pass" "端口 ${V_SERVER_PORT} 可用"
        fi
    elif command -v ss &> /dev/null; then
        if ss -ln | grep -E ":${V_SERVER_PORT}[[:space:]]" &> /dev/null; then
            check_item "端口可用性" "fail" "端口 ${V_SERVER_PORT} 已被占用"
        else
            check_item "端口可用性" "pass" "端口 ${V_SERVER_PORT} 可用"
        fi
    else
        check_item "端口可用性" "warn" "无法检查端口状态 (缺少 lsof/ss 工具)"
    fi
else
    check_item "服务端口配置" "fail" "端口未配置"
fi

echo ""

# 5. 系统资源检查
echo -e "${CYAN}[5/6] 系统资源检查${NC}"

# 检查磁盘空间（跨平台兼容）
if command -v df &> /dev/null; then
    # macOS 和 Linux 兼容的方式
    if df -h "$PROJECT_ROOT" &> /dev/null; then
        DISK_INFO=$(df -h "$PROJECT_ROOT" | tail -1)
        DISK_AVAIL=$(echo "$DISK_INFO" | awk '{print $4}' | sed 's/Gi\?//')
        
        # 尝试提取数字部分
        DISK_NUM=$(echo "$DISK_AVAIL" | grep -o '[0-9]*' | head -1)
        
        if [ -n "$DISK_NUM" ] && [ "$DISK_NUM" -gt 10 ]; then
            check_item "磁盘空间" "pass" "可用空间: ${DISK_AVAIL}"
        elif [ -n "$DISK_NUM" ] && [ "$DISK_NUM" -gt 5 ]; then
            check_item "磁盘空间" "warn" "可用空间: ${DISK_AVAIL} (建议至少 10GB)"
        elif [ -n "$DISK_NUM" ]; then
            check_item "磁盘空间" "fail" "可用空间不足: ${DISK_AVAIL}"
        else
            check_item "磁盘空间" "warn" "可用空间: ${DISK_AVAIL}"
        fi
    else
        check_item "磁盘空间" "warn" "无法检查磁盘空间"
    fi
fi

# 检查内存（Linux 系统）
if command -v free &> /dev/null; then
    MEM_AVAIL=$(free -g | grep Mem | awk '{print $7}')
    if [ -n "$MEM_AVAIL" ] && [ "$MEM_AVAIL" -gt 2 ]; then
        check_item "可用内存" "pass" "可用内存: ${MEM_AVAIL}GB"
    elif [ -n "$MEM_AVAIL" ] && [ "$MEM_AVAIL" -gt 1 ]; then
        check_item "可用内存" "warn" "可用内存: ${MEM_AVAIL}GB (建议至少 2GB)"
    elif [ -n "$MEM_AVAIL" ]; then
        check_item "可用内存" "fail" "可用内存不足: ${MEM_AVAIL}GB"
    fi
elif command -v vm_stat &> /dev/null; then
    # macOS 系统
    FREE_PAGES=$(vm_stat | grep "Pages free" | awk '{print $3}' | sed 's/\.//')
    if [ -n "$FREE_PAGES" ]; then
        # 页面大小通常是 4KB，转换为 GB
        FREE_GB=$((FREE_PAGES * 4 / 1024 / 1024))
        if [ "$FREE_GB" -gt 2 ]; then
            check_item "可用内存" "pass" "可用内存: ${FREE_GB}GB"
        elif [ "$FREE_GB" -gt 1 ]; then
            check_item "可用内存" "warn" "可用内存: ${FREE_GB}GB (建议至少 2GB)"
        else
            check_item "可用内存" "warn" "可用内存: ${FREE_GB}GB"
        fi
    fi
fi

echo ""

# 6. 文件权限检查
echo -e "${CYAN}[6/6] 文件权限检查${NC}"

if [ -w "$DOCKER_DIR" ]; then
    check_item "Docker 目录可写" "pass"
else
    check_item "Docker 目录可写" "fail" "没有写入权限"
fi

if [ -f "$PROJECT_ROOT/data/v.db" ]; then
    if [ -w "$PROJECT_ROOT/data/v.db" ]; then
        check_item "数据库文件可写" "pass"
    else
        check_item "数据库文件可写" "fail" "没有写入权限"
    fi
else
    check_item "数据库文件" "warn" "数据库文件不存在 (首次启动会自动创建)"
fi

echo ""
echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}  检查结果汇总${NC}"
echo -e "${CYAN}========================================${NC}"
echo -e "总检查项: ${TOTAL_CHECKS}"
echo -e "${GREEN}通过: ${PASSED_CHECKS}${NC}"
echo -e "${YELLOW}警告: ${WARNING_CHECKS}${NC}"
echo -e "${RED}失败: ${FAILED_CHECKS}${NC}"
echo ""

# 判断是否可以部署
if [ $FAILED_CHECKS -eq 0 ]; then
    if [ $WARNING_CHECKS -eq 0 ]; then
        echo -e "${GREEN}✓ 所有检查通过，可以安全部署！${NC}"
        exit 0
    else
        echo -e "${YELLOW}! 存在警告项，建议修复后再部署${NC}"
        echo -e "${YELLOW}  如果确认要继续，请手动运行部署命令${NC}"
        exit 0
    fi
else
    echo -e "${RED}✗ 存在严重问题，必须修复后才能部署！${NC}"
    echo ""
    echo -e "${CYAN}修复建议:${NC}"
    echo -e "1. 编辑 .env 文件: ${YELLOW}vi $DOCKER_DIR/.env${NC}"
    echo -e "2. 生成安全的 JWT Secret: ${YELLOW}openssl rand -base64 32${NC}"
    echo -e "3. 设置强密码 (至少12字符，包含大小写字母、数字和特殊字符)"
    echo -e "4. 再次运行检查: ${YELLOW}$0${NC}"
    exit 1
fi
