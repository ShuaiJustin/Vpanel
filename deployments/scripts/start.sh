#!/bin/bash
# V Panel 一键启动脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$SCRIPT_DIR/common.sh"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}       V Panel 一键启动脚本${NC}"
echo -e "${GREEN}========================================${NC}"

require_docker
require_compose

# 生产环境安全检查
production_security_check() {
    local env_file="$1"
    local mode=$(read_env_var "V_SERVER_MODE" "$env_file")
    
    # 只在 release 模式下进行严格检查
    if [ "$mode" != "release" ]; then
        echo -e "${YELLOW}警告: 非生产模式 (${mode})，跳过安全检查${NC}"
        return 0
    fi
    
    echo -e "${CYAN}执行生产环境安全检查...${NC}"
    
    local has_error=0
    
    # 检查 JWT Secret
    local jwt_secret=$(read_env_var "V_JWT_SECRET" "$env_file")
    if ! validate_jwt_secret "$jwt_secret"; then
        echo -e "${RED}✗ JWT Secret 不安全！${NC}"
        echo -e "  必须至少 32 字符且不能使用默认值"
        echo -e "  生成方法: ${YELLOW}openssl rand -base64 32${NC}"
        has_error=1
    else
        echo -e "${GREEN}✓ JWT Secret 验证通过${NC}"
    fi
    
    # 检查管理员密码
    local admin_pass=$(read_env_var "V_ADMIN_PASS" "$env_file")
    if is_default_admin_password "$admin_pass"; then
        echo -e "${RED}✗ 管理员密码使用默认值！${NC}"
        echo -e "  必须修改为强密码"
        has_error=1
    elif ! validate_password "$admin_pass"; then
        echo -e "${RED}✗ 管理员密码强度不足！${NC}"
        echo -e "  要求: 至少12字符，包含大小写字母、数字和特殊字符"
        has_error=1
    else
        echo -e "${GREEN}✓ 管理员密码验证通过${NC}"
    fi
    
    # 检查端口配置
    local server_port=$(read_env_var "V_SERVER_PORT" "$env_file")
    if [ -z "$server_port" ]; then
        echo -e "${RED}✗ 服务端口未配置！${NC}"
        echo -e "  生产环境必须设置固定端口"
        has_error=1
    else
        echo -e "${GREEN}✓ 服务端口: ${server_port}${NC}"
    fi
    
    if [ $has_error -eq 1 ]; then
        echo ""
        echo -e "${RED}========================================${NC}"
        echo -e "${RED}  生产环境安全检查失败！${NC}"
        echo -e "${RED}  请修改 .env 文件后重试${NC}"
        echo -e "${RED}========================================${NC}"
        return 1
    fi
    
    echo -e "${GREEN}✓ 生产环境安全检查通过${NC}"
    return 0
}

# 初始化或更新 .env 文件
init_env_file() {
    local env_file="$1"
    local needs_update=0
    
    # 读取当前值
    local jwt_secret=$(read_env_var "V_JWT_SECRET" "$env_file")
    local admin_pass=$(read_env_var "V_ADMIN_PASS" "$env_file")
    
    # 检查是否需要生成 JWT Secret
    if is_default_jwt_secret "$jwt_secret"; then
        jwt_secret=$(generate_jwt_secret)
        needs_update=1
        echo -e "${GREEN}✓ 已生成 JWT Secret${NC}"
    fi
    
    # 检查是否需要生成管理员密码
    if is_default_admin_password "$admin_pass"; then
        admin_pass=$(generate_strong_password)
        needs_update=1
        echo -e "${GREEN}✓ 已生成管理员密码${NC}"
    fi
    
    # 如果需要更新，写入文件
    if [ $needs_update -eq 1 ]; then
        # 创建临时文件
        local temp_file="${env_file}.tmp"
        
        # 复制原文件，替换相关行
        while IFS= read -r line; do
            if echo "$line" | grep -q "^V_JWT_SECRET="; then
                echo "V_JWT_SECRET=$jwt_secret"
            elif echo "$line" | grep -q "^V_ADMIN_PASS="; then
                echo "V_ADMIN_PASS=$admin_pass"
            else
                echo "$line"
            fi
        done < "$env_file" > "$temp_file"
        
        # 替换原文件
        mv "$temp_file" "$env_file"
        
        echo ""
        echo -e "${CYAN}========================================${NC}"
        echo -e "${CYAN}  自动生成的凭据信息${NC}"
        echo -e "${CYAN}========================================${NC}"
        echo -e "${YELLOW}管理员密码: ${admin_pass}${NC}"
        echo -e "${GREEN}JWT Secret 已保存到 .env 文件${NC}"
        echo -e "${CYAN}========================================${NC}"
        echo -e "${RED}请妥善保存管理员密码！${NC}"
        echo ""
        
        # 等待用户确认
        read -p "按回车键继续启动服务..."
    fi
}

# 检查 .env 文件
if [ ! -f "$DOCKER_DIR/.env" ]; then
    echo -e "${YELLOW}创建 .env 配置文件...${NC}"
    cp "$DOCKER_DIR/.env.example" "$DOCKER_DIR/.env"
    echo -e "${GREEN}.env 文件已创建${NC}"
    echo ""
fi

# 初始化 .env 文件（自动生成密码）
echo -e "${CYAN}检查配置文件...${NC}"
init_env_file "$DOCKER_DIR/.env"

if ! remove_stale_vpanel_container; then
    echo -e "${RED}错误: 清理遗留容器失败${NC}"
    exit 1
fi

# 切换到 Docker 目录
cd "$DOCKER_DIR" || {
    echo -e "${RED}错误: 无法进入 Docker 目录${NC}"
    exit 1
}

# 读取配置
V_SERVER_PORT=$(read_env_var "V_SERVER_PORT" ".env")
V_SERVER_MODE=$(read_env_var "V_SERVER_MODE" ".env")

# 端口处理逻辑
if [ -z "$V_SERVER_PORT" ]; then
    # 端口为空
    if [ "$V_SERVER_MODE" = "release" ]; then
        # 生产模式必须配置端口
        echo -e "${RED}错误: 生产模式必须配置固定端口！${NC}"
        echo -e "${YELLOW}请编辑 .env 文件，设置 V_SERVER_PORT${NC}"
        exit 1
    else
        # 开发/测试模式可以使用随机端口
        V_SERVER_PORT=8080
        echo -e "${YELLOW}开发模式: 使用默认端口 8080${NC}"
    fi
fi

# 解析命令行参数
case "${1:-start}" in
    start)
        # 执行生产环境安全检查
        if ! production_security_check ".env"; then
            exit 1
        fi
        
        echo ""
        echo -e "${GREEN}启动 V Panel...${NC}"
        if [ "${VPANEL_SKIP_BUILD:-0}" = "1" ]; then
            if docker_compose_cmd up -d; then
                start_success=1
            else
                start_success=0
            fi
        else
            if docker_compose_cmd up -d --build; then
                start_success=1
            else
                start_success=0
            fi
        fi

        if [ "$start_success" -eq 1 ]; then
            # 读取实际的管理员密码
            admin_pass=$(read_env_var "V_ADMIN_PASS" ".env")
            
            echo ""
            echo -e "${GREEN}========================================${NC}"
            echo -e "${GREEN}V Panel 启动成功！${NC}"
            echo -e "${GREEN}========================================${NC}"
            echo -e "访问地址: ${YELLOW}http://localhost:${V_SERVER_PORT}${NC}"
            echo -e "用户名:   ${YELLOW}admin${NC}"
            echo -e "密码:     ${YELLOW}${admin_pass}${NC}"
            echo -e "模式:     ${YELLOW}${V_SERVER_MODE}${NC}"
            echo ""
        else
            echo -e "${RED}启动失败！${NC}"
            exit 1
        fi
        ;;
    stop)
        echo -e "${YELLOW}停止 V Panel...${NC}"
        if docker_compose_cmd down; then
            echo -e "${GREEN}V Panel 已停止${NC}"
        else
            echo -e "${RED}停止失败！${NC}"
            exit 1
        fi
        ;;
    restart)
        echo -e "${YELLOW}重启 V Panel...${NC}"
        if docker_compose_cmd down && docker_compose_cmd up -d --build; then
            echo -e "${GREEN}V Panel 已重启${NC}"
            echo -e "访问地址: ${YELLOW}http://localhost:${V_SERVER_PORT}${NC}"
        else
            echo -e "${RED}重启失败！${NC}"
            exit 1
        fi
        ;;
    logs)
        docker_compose_cmd logs -f
        ;;
    status)
        docker_compose_cmd ps
        echo ""
        echo -e "访问地址: ${YELLOW}http://localhost:${V_SERVER_PORT}${NC}"
        echo -e "模式:     ${YELLOW}${V_SERVER_MODE}${NC}"
        ;;
    clean)
        echo ""
        echo -e "${RED}========================================${NC}"
        echo -e "${RED}        警告: 危险操作！${NC}"
        echo -e "${RED}========================================${NC}"
        echo -e "${RED}这将删除所有数据，包括:${NC}"
        echo -e "  - 面板配置"
        echo -e "  - 数据库文件"
        echo -e "  - 日志文件"
        echo -e "  - Xray 配置"
        echo ""
        echo -e "${YELLOW}建议: 在删除前先备份数据${NC}"
        show_volume_backup_hint
        echo ""
        read -p "确认删除所有数据? 输入 'DELETE' 确认: " confirm
        if [ "$confirm" = "DELETE" ]; then
            if docker_compose_cmd down -v; then
                echo -e "${GREEN}已清理所有容器和数据卷${NC}"
            else
                echo -e "${RED}清理失败！${NC}"
                exit 1
            fi
        else
            echo -e "${YELLOW}已取消操作${NC}"
        fi
        ;;
    *)
        echo "用法: $0 {start|stop|restart|logs|status|clean}"
        echo ""
        echo "  start   - 启动服务 (默认)"
        echo "  stop    - 停止服务"
        echo "  restart - 重启服务"
        echo "  logs    - 查看日志"
        echo "  status  - 查看状态"
        echo "  clean   - 清理所有数据 (危险)"
        exit 1
        ;;
esac
