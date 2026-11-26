#!/bin/bash
# V Panel 本地开发启动脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 获取项目根目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}     V Panel 本地开发启动脚本${NC}"
echo -e "${GREEN}========================================${NC}"

cd "$PROJECT_ROOT"

# 检查 Go 是否安装
if ! command -v go &> /dev/null; then
    echo -e "${RED}错误: Go 未安装，请先安装 Go${NC}"
    exit 1
fi

# 确保数据目录存在
mkdir -p data

# 确保配置文件存在
if [ ! -f "configs/config.yaml" ]; then
    if [ -f "configs/config.yaml.example" ]; then
        echo -e "${YELLOW}创建配置文件...${NC}"
        cp configs/config.yaml.example configs/config.yaml
    fi
fi

case "${1:-start}" in
    start)
        echo -e "${GREEN}编译并启动 V Panel...${NC}"
        go build -o v ./cmd/v/main.go
        echo -e "${GREEN}启动服务...${NC}"
        ./v
        ;;
    build)
        echo -e "${GREEN}编译 V Panel...${NC}"
        go build -o v ./cmd/v/main.go
        echo -e "${GREEN}编译完成: ./v${NC}"
        ;;
    run)
        echo -e "${GREEN}直接运行 (不编译)...${NC}"
        go run ./cmd/v/main.go
        ;;
    test)
        echo -e "${GREEN}运行测试...${NC}"
        go test ./... -v
        ;;
    frontend)
        echo -e "${GREEN}启动前端开发服务器...${NC}"
        cd web && npm run dev
        ;;
    install)
        echo -e "${GREEN}安装依赖...${NC}"
        go mod download
        cd web && npm install
        echo -e "${GREEN}依赖安装完成${NC}"
        ;;
    *)
        echo "用法: $0 {start|build|run|test|frontend|install}"
        echo ""
        echo "  start    - 编译并启动服务 (默认)"
        echo "  build    - 仅编译"
        echo "  run      - 直接运行 (go run)"
        echo "  test     - 运行测试"
        echo "  frontend - 启动前端开发服务器"
        echo "  install  - 安装所有依赖"
        exit 1
        ;;
esac
