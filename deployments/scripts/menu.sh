#!/bin/bash
# V Panel 菜单管理脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$SCRIPT_DIR/common.sh"

# 清屏函数
clear_screen() {
    clear
}

# 显示标题
show_header() {
    clear_screen
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}       V Panel 管理菜单${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""
}

# 暂停函数
pause() {
    echo ""
    read -p "按回车键继续..."
}

# Docker 相关操作
docker_menu() {
    while true; do
        show_header
        echo -e "${GREEN}Docker 部署管理${NC}"
        echo ""
        
        # 显示当前状态
        cd "$DOCKER_DIR" || { echo -e "${RED}错误: 无法进入 Docker 目录${NC}"; pause; continue; }
        if check_container_status; then
            echo -e "当前状态: ${GREEN}运行中${NC}"
        else
            echo -e "当前状态: ${YELLOW}已停止${NC}"
        fi
        echo ""
        
        echo "  1) 启动服务"
        echo "  2) 停止服务"
        echo "  3) 重启服务"
        echo "  4) 重新构建并启动"
        echo "  5) 查看日志 (实时)"
        echo "  6) 查看日志 (最近 100 行)"
        echo "  7) 查看状态"
        echo "  8) 进入容器 Shell"
        echo "  9) 清理数据 (危险)"
        echo "  0) 返回主菜单"
        echo ""
        read -p "请选择操作 [0-9]: " choice

        case $choice in
            1)
                if check_container_status; then
                    echo -e "${YELLOW}服务已在运行中${NC}"
                else
                    echo -e "${GREEN}启动 V Panel...${NC}"
                    # 使用 start.sh 脚本来处理安全检查
                    if "$SCRIPT_DIR/start.sh" start; then
                        echo -e "${GREEN}启动成功${NC}"
                    else
                        echo -e "${RED}启动失败${NC}"
                    fi
                fi
                pause
                ;;
            2)
                if ! check_container_status; then
                    echo -e "${YELLOW}服务未运行${NC}"
                else
                    echo -e "${YELLOW}停止 V Panel...${NC}"
                    cd "$DOCKER_DIR" || { echo -e "${RED}错误: 无法进入 Docker 目录${NC}"; pause; continue; }
                    if docker_compose_cmd down; then
                        echo -e "${GREEN}V Panel 已停止${NC}"
                    else
                        echo -e "${RED}停止失败${NC}"
                    fi
                fi
                pause
                ;;
            3)
                echo -e "${YELLOW}重启 V Panel...${NC}"
                cd "$DOCKER_DIR" || { echo -e "${RED}错误: 无法进入 Docker 目录${NC}"; pause; continue; }
                if docker_compose_cmd restart; then
                    echo -e "${GREEN}V Panel 已重启${NC}"
                else
                    echo -e "${RED}重启失败${NC}"
                fi
                pause
                ;;
            4)
                echo -e "${YELLOW}重新构建并启动...${NC}"
                # 先停止服务
                cd "$DOCKER_DIR" || { echo -e "${RED}错误: 无法进入 Docker 目录${NC}"; pause; continue; }
                docker_compose_cmd down
                # 清理构建缓存
                echo -e "${CYAN}清理构建缓存...${NC}"
                docker_compose_cmd build --no-cache
                # 使用 start.sh 启动（包含安全检查和密码生成）
                echo -e "${CYAN}启动服务...${NC}"
                cd "$PROJECT_ROOT" || { echo -e "${RED}错误: 无法返回项目根目录${NC}"; pause; continue; }
                if bash "$SCRIPT_DIR/start.sh" start; then
                    echo -e "${GREEN}重新构建并启动完成！${NC}"
                else
                    echo -e "${RED}启动失败${NC}"
                fi
                pause
                ;;
            5)
                echo -e "${CYAN}查看实时日志 (Ctrl+C 退出)${NC}"
                cd "$DOCKER_DIR" || { echo -e "${RED}错误: 无法进入 Docker 目录${NC}"; pause; continue; }
                docker_compose_cmd logs -f
                ;;
            6)
                echo -e "${CYAN}最近 100 行日志:${NC}"
                cd "$DOCKER_DIR" || { echo -e "${RED}错误: 无法进入 Docker 目录${NC}"; pause; continue; }
                docker_compose_cmd logs --tail=100
                pause
                ;;
            7)
                cd "$DOCKER_DIR" || { echo -e "${RED}错误: 无法进入 Docker 目录${NC}"; pause; continue; }
                echo -e "${CYAN}容器状态:${NC}"
                docker_compose_cmd ps
                echo ""
                echo -e "${CYAN}资源使用:${NC}"
                docker stats --no-stream v-panel 2>/dev/null || echo "容器未运行"
                pause
                ;;
            8)
                if ! check_container_status; then
                    echo -e "${RED}错误: 容器未运行${NC}"
                    pause
                else
                    echo -e "${CYAN}进入容器 Shell (输入 exit 退出)${NC}"
                    docker exec -it v-panel sh
                fi
                ;;
            9)
                echo ""
                echo -e "${RED}========================================${NC}"
                echo -e "${RED}        警告: 危险操作！${NC}"
                echo -e "${RED}========================================${NC}"
                echo -e "${RED}这将删除:${NC}"
                echo -e "  - 所有容器"
                echo -e "  - 所有数据卷 (配置、数据库、日志、Xray)"
                echo ""
                echo -e "${YELLOW}建议: 在删除前先备份数据${NC}"
                show_volume_backup_hint
                echo ""
                read -p "确认删除所有数据? 输入 'DELETE' 确认: " confirm
                if [ "$confirm" = "DELETE" ]; then
                    cd "$DOCKER_DIR" || { echo -e "${RED}错误: 无法进入 Docker 目录${NC}"; pause; continue; }
                    if docker_compose_cmd down -v; then
                        echo -e "${GREEN}已清理所有容器和数据卷${NC}"
                    else
                        echo -e "${RED}清理失败${NC}"
                    fi
                else
                    echo -e "${YELLOW}已取消操作${NC}"
                fi
                pause
                ;;
            0)
                break
                ;;
            *)
                echo -e "${RED}无效选择，请重试${NC}"
                pause
                ;;
        esac
    done
}

# 开发环境操作
dev_menu() {
    while true; do
        show_header
        echo -e "${GREEN}本地开发环境${NC}"
        echo ""
        
        # 检查编译文件
        if [ -f "$PROJECT_ROOT/vpanel" ]; then
            echo -e "编译状态: ${GREEN}已编译${NC}"
        else
            echo -e "编译状态: ${YELLOW}未编译${NC}"
        fi
        echo ""
        
        echo "  1) 编译 Panel"
        echo "  2) 编译 Agent (所有平台)"
        echo "  3) 编译 Panel + Agent"
        echo "  4) 运行 Panel (go run)"
        echo "  5) 运行已编译的 Panel"
        echo "  6) 运行测试"
        echo "  7) 代码格式化"
        echo "  8) 前端开发服务器"
        echo "  9) 编译前端"
        echo " 10) 安装所有依赖"
        echo "  0) 返回主菜单"
        echo ""
        read -p "请选择操作 [0-10]: " choice

        cd "$PROJECT_ROOT"

        case $choice in
            1)
                echo -e "${GREEN}编译 Panel...${NC}"
                if make build; then
                    echo -e "${GREEN}编译完成: ./vpanel${NC}"
                else
                    echo -e "${RED}编译失败${NC}"
                fi
                pause
                ;;
            2)
                echo -e "${GREEN}编译 Agent (所有平台)...${NC}"
                if make agent-all; then
                    echo -e "${GREEN}编译完成，文件位于 bin/ 目录${NC}"
                    ls -lh bin/
                else
                    echo -e "${RED}编译失败${NC}"
                fi
                pause
                ;;
            3)
                echo -e "${GREEN}编译 Panel + Agent...${NC}"
                if make build-all; then
                    echo -e "${GREEN}编译完成${NC}"
                else
                    echo -e "${RED}编译失败${NC}"
                fi
                pause
                ;;
            4)
                echo -e "${GREEN}运行 Panel (开发模式)...${NC}"
                go run ./cmd/v/main.go
                ;;
            5)
                if [ ! -f "$PROJECT_ROOT/vpanel" ]; then
                    echo -e "${RED}错误: 未找到编译文件，请先编译${NC}"
                    pause
                else
                    echo -e "${GREEN}运行 Panel...${NC}"
                    ./vpanel
                fi
                ;;
            6)
                echo -e "${GREEN}运行测试...${NC}"
                if make test; then
                    echo -e "${GREEN}测试通过${NC}"
                else
                    echo -e "${RED}测试失败${NC}"
                fi
                pause
                ;;
            7)
                echo -e "${GREEN}格式化代码...${NC}"
                if make fmt; then
                    echo -e "${GREEN}格式化完成${NC}"
                else
                    echo -e "${RED}格式化失败${NC}"
                fi
                pause
                ;;
            8)
                echo -e "${GREEN}启动前端开发服务器...${NC}"
                cd web && npm run dev
                ;;
            9)
                echo -e "${GREEN}编译前端...${NC}"
                if cd web && npm run build; then
                    echo -e "${GREEN}前端编译完成${NC}"
                else
                    echo -e "${RED}前端编译失败${NC}"
                fi
                pause
                ;;
            10)
                echo -e "${GREEN}安装依赖...${NC}"
                echo -e "${CYAN}安装 Go 依赖...${NC}"
                if go mod download; then
                    echo -e "${GREEN}Go 依赖安装完成${NC}"
                else
                    echo -e "${RED}Go 依赖安装失败${NC}"
                fi
                echo -e "${CYAN}安装前端依赖...${NC}"
                if cd web && npm install; then
                    echo -e "${GREEN}前端依赖安装完成${NC}"
                else
                    echo -e "${RED}前端依赖安装失败${NC}"
                fi
                pause
                ;;
            0)
                break
                ;;
            *)
                echo -e "${RED}无效选择，请重试${NC}"
                pause
                ;;
        esac
    done
}

# 配置管理
config_menu() {
    while true; do
        show_header
        echo -e "${GREEN}配置管理${NC}"
        echo ""
        echo "  1) 创建/重置配置文件 (本地部署)"
        echo "  2) 编辑配置文件 (本地部署)"
        echo "  3) 查看配置文件"
        echo "  4) 创建/编辑 Docker .env 文件"
        echo "  5) 生产环境部署检查"
        echo "  0) 返回主菜单"
        echo ""
        read -p "请选择操作 [0-5]: " choice

        case $choice in
            1)
                if [ -f "$PROJECT_ROOT/configs/config.yaml.example" ]; then
                    cp "$PROJECT_ROOT/configs/config.yaml.example" "$PROJECT_ROOT/configs/config.yaml"
                    echo -e "${GREEN}配置文件已创建/重置${NC}"
                else
                    echo -e "${RED}错误: 找不到示例配置文件${NC}"
                fi
                pause
                ;;
            2)
                if [ -f "$PROJECT_ROOT/configs/config.yaml" ]; then
                    ${EDITOR:-vi} "$PROJECT_ROOT/configs/config.yaml"
                else
                    echo -e "${RED}错误: 配置文件不存在，请先创建${NC}"
                    pause
                fi
                ;;
            3)
                echo -e "${CYAN}配置文件查看${NC}"
                echo ""
                echo "1) 查看 configs/config.yaml (本地部署)"
                echo "2) 查看 Docker .env (Docker 部署)"
                echo "0) 返回"
                echo ""
                read -p "请选择 [0-2]: " view_choice
                
                case $view_choice in
                    1)
                        if [ -f "$PROJECT_ROOT/configs/config.yaml" ]; then
                            cat "$PROJECT_ROOT/configs/config.yaml"
                        else
                            echo -e "${YELLOW}配置文件不存在 (本地部署使用)${NC}"
                        fi
                        ;;
                    2)
                        if [ -f "$DOCKER_DIR/.env" ]; then
                            echo -e "${CYAN}Docker 环境配置:${NC}"
                            echo ""
                            # 隐藏敏感信息
                            while IFS= read -r line; do
                                if echo "$line" | grep -q "^V_JWT_SECRET="; then
                                    echo "V_JWT_SECRET=********** (已隐藏)"
                                elif echo "$line" | grep -q "^V_ADMIN_PASS="; then
                                    echo "V_ADMIN_PASS=********** (已隐藏)"
                                else
                                    echo "$line"
                                fi
                            done < "$DOCKER_DIR/.env"
                        else
                            echo -e "${RED}.env 文件不存在${NC}"
                        fi
                        ;;
                    0)
                        ;;
                    *)
                        echo -e "${RED}无效选择${NC}"
                        ;;
                esac
                pause
                ;;
            4)
                if [ -f "$DOCKER_DIR/.env" ]; then
                    echo -e "${CYAN}Docker .env 文件管理${NC}"
                    echo ""
                    echo "1) 编辑 .env 文件"
                    echo "2) 从示例重新创建 (会覆盖现有配置)"
                    echo "0) 返回"
                    echo ""
                    read -p "请选择 [0-2]: " env_choice
                    
                    case $env_choice in
                        1)
                            ${EDITOR:-vi} "$DOCKER_DIR/.env"
                            ;;
                        2)
                            echo -e "${YELLOW}警告: 这将覆盖现有 .env 文件${NC}"
                            read -p "确认覆盖? (y/N): " confirm
                            if [ "$confirm" = "y" ] || [ "$confirm" = "Y" ]; then
                                cp "$DOCKER_DIR/.env.example" "$DOCKER_DIR/.env"
                                echo -e "${GREEN}Docker .env 文件已重新创建${NC}"
                                echo -e "${YELLOW}首次启动时会自动生成密码和 JWT Secret${NC}"
                            else
                                echo -e "${YELLOW}已取消${NC}"
                            fi
                            ;;
                        0)
                            ;;
                        *)
                            echo -e "${RED}无效选择${NC}"
                            ;;
                    esac
                else
                    echo -e "${YELLOW}创建 Docker .env 文件...${NC}"
                    if [ -f "$DOCKER_DIR/.env.example" ]; then
                        cp "$DOCKER_DIR/.env.example" "$DOCKER_DIR/.env"
                        echo -e "${GREEN}Docker .env 文件已创建${NC}"
                        echo -e "${YELLOW}首次启动时会自动生成密码和 JWT Secret${NC}"
                    else
                        echo -e "${RED}错误: 找不到 .env.example 文件${NC}"
                    fi
                fi
                pause
                ;;
            5)
                echo -e "${CYAN}执行生产环境部署检查...${NC}"
                echo ""
                if "$SCRIPT_DIR/production-check.sh"; then
                    echo ""
                    echo -e "${GREEN}检查完成！${NC}"
                else
                    echo ""
                    echo -e "${RED}检查发现问题，请修复后重试${NC}"
                fi
                pause
                ;;
            0)
                break
                ;;
            *)
                echo -e "${RED}无效选择，请重试${NC}"
                pause
                ;;
        esac
    done
}

# 系统检查
system_check() {
    show_header
    echo -e "${GREEN}系统环境检查${NC}"
    echo ""
    
    # 检查 Go
    if command -v go &> /dev/null; then
        echo -e "${GREEN}✓${NC} Go: $(go version)"
    else
        echo -e "${RED}✗${NC} Go: 未安装"
    fi
    
    # 检查 Docker
    if command -v docker &> /dev/null; then
        echo -e "${GREEN}✓${NC} Docker: $(docker --version)"
    else
        echo -e "${RED}✗${NC} Docker: 未安装"
    fi
    
    # 检查 Docker Compose
    if compose_available; then
        if docker compose version &> /dev/null; then
            echo -e "${GREEN}✓${NC} Docker Compose: $(docker compose version)"
        else
            echo -e "${GREEN}✓${NC} Docker Compose: $(docker-compose --version)"
        fi
    else
        echo -e "${RED}✗${NC} Docker Compose: 未安装"
    fi
    
    # 检查 Node.js
    if command -v node &> /dev/null; then
        echo -e "${GREEN}✓${NC} Node.js: $(node --version)"
    else
        echo -e "${RED}✗${NC} Node.js: 未安装"
    fi
    
    # 检查 npm
    if command -v npm &> /dev/null; then
        echo -e "${GREEN}✓${NC} npm: $(npm --version)"
    else
        echo -e "${RED}✗${NC} npm: 未安装"
    fi
    
    echo ""
    echo -e "${BLUE}项目路径:${NC} $PROJECT_ROOT"
    
    # 检查配置文件
    if [ -f "$PROJECT_ROOT/configs/config.yaml" ]; then
        echo -e "${GREEN}✓${NC} 配置文件存在"
    else
        echo -e "${YELLOW}!${NC} 配置文件不存在"
    fi
    
    # 检查数据库
    if [ -f "$PROJECT_ROOT/data/v.db" ]; then
        echo -e "${GREEN}✓${NC} 数据库文件存在"
    else
        echo -e "${YELLOW}!${NC} 数据库文件不存在"
    fi
    
    pause
}

# 主菜单
main_menu() {
    while true; do
        show_header
        
        # 显示快速状态
        echo -e "${CYAN}快速状态:${NC}"
        if check_container_status 2>/dev/null; then
            # 读取端口
            if [ -f "$DOCKER_DIR/.env" ]; then
                PORT=$(read_env_var "V_SERVER_PORT" "$DOCKER_DIR/.env")
                PORT=${PORT:-8080}
            else
                PORT=8080
            fi
            echo -e "  Docker: ${GREEN}运行中${NC} | 访问: ${YELLOW}http://localhost:${PORT}${NC}"
        else
            echo -e "  Docker: ${YELLOW}已停止${NC}"
        fi
        echo ""
        
        echo -e "${BLUE}请选择操作:${NC}"
        echo ""
        echo "  1) Docker 部署管理"
        echo "  2) 本地开发环境"
        echo "  3) 配置管理"
        echo "  4) 系统环境检查"
        echo "  5) 快速启动 (Docker)"
        echo "  6) 快速停止 (Docker)"
        echo "  0) 退出"
        echo ""
        read -p "请选择 [0-6]: " choice

        case $choice in
            1)
                docker_menu
                ;;
            2)
                dev_menu
                ;;
            3)
                config_menu
                ;;
            4)
                system_check
                ;;
            5)
                echo -e "${GREEN}快速启动 Docker 服务...${NC}"
                if "$SCRIPT_DIR/start.sh" start; then
                    echo -e "${GREEN}启动成功${NC}"
                else
                    echo -e "${RED}启动失败${NC}"
                fi
                pause
                ;;
            6)
                echo -e "${YELLOW}快速停止 Docker 服务...${NC}"
                cd "$DOCKER_DIR" || { echo -e "${RED}错误: 无法进入 Docker 目录${NC}"; pause; continue; }
                if docker_compose_cmd down; then
                    echo -e "${GREEN}已停止${NC}"
                else
                    echo -e "${RED}停止失败${NC}"
                fi
                pause
                ;;
            0)
                echo -e "${GREEN}再见！${NC}"
                exit 0
                ;;
            *)
                echo -e "${RED}无效选择，请重试${NC}"
                pause
                ;;
        esac
    done
}

# 启动主菜单
main_menu
