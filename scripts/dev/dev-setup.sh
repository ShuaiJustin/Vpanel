#!/bin/bash

# 开发环境设置脚本

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}设置 V Panel 开发环境${NC}"
echo ""

# 检查 Go 版本
echo -e "${YELLOW}检查 Go 版本...${NC}"
if ! command -v go &> /dev/null; then
    echo -e "${RED}错误: Go 未安装${NC}"
    echo "请安装 Go 1.21 或更高版本"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo -e "${GREEN}✓ Go $GO_VERSION${NC}"

# 检查 Node.js
echo ""
echo -e "${YELLOW}检查 Node.js...${NC}"
if command -v node &> /dev/null; then
    NODE_VERSION=$(node --version)
    echo -e "${GREEN}✓ Node.js $NODE_VERSION${NC}"
else
    echo -e "${YELLOW}⚠ Node.js 未安装（前端开发需要）${NC}"
fi

# 检查 PostgreSQL
echo ""
echo -e "${YELLOW}检查 PostgreSQL...${NC}"
if command -v psql &> /dev/null; then
    PG_VERSION=$(psql --version | awk '{print $3}')
    echo -e "${GREEN}✓ PostgreSQL $PG_VERSION${NC}"
else
    echo -e "${YELLOW}⚠ PostgreSQL 未安装${NC}"
fi

# 安装 Go 依赖
echo ""
echo -e "${YELLOW}安装 Go 依赖...${NC}"
go mod download
echo -e "${GREEN}✓ Go 依赖安装完成${NC}"

# 安装开发工具
echo ""
echo -e "${YELLOW}安装开发工具...${NC}"

# air (热重载)
if ! command -v air &> /dev/null; then
    echo "安装 air..."
    go install github.com/cosmtrek/air@latest
    echo -e "${GREEN}✓ air 安装完成${NC}"
else
    echo -e "${GREEN}✓ air 已安装${NC}"
fi

# golangci-lint (代码检查)
if ! command -v golangci-lint &> /dev/null; then
    echo "安装 golangci-lint..."
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
    echo -e "${GREEN}✓ golangci-lint 安装完成${NC}"
else
    echo -e "${GREEN}✓ golangci-lint 已安装${NC}"
fi

# 创建必要目录
echo ""
echo -e "${YELLOW}创建目录结构...${NC}"
mkdir -p logs data build
echo -e "${GREEN}✓ 目录创建完成${NC}"

# 创建示例配置
echo ""
echo -e "${YELLOW}创建示例配置...${NC}"
if [ ! -f "configs/config.yaml" ]; then
    if [ -f "configs/config.yaml.example" ]; then
        cp configs/config.yaml.example configs/config.yaml
        echo -e "${GREEN}✓ 配置文件创建完成${NC}"
        echo -e "${YELLOW}请编辑 configs/config.yaml 配置数据库等信息${NC}"
    fi
else
    echo -e "${GREEN}✓ 配置文件已存在${NC}"
fi

# 创建 .air.toml
echo ""
echo -e "${YELLOW}创建 air 配置...${NC}"
if [ ! -f ".air.toml" ]; then
    cat > .air.toml <<'EOF'
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main ./cmd/v/main.go"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "web/node_modules"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  include_file = []
  kill_delay = "0s"
  log = "build-errors.log"
  poll = false
  poll_interval = 0
  rerun = false
  rerun_delay = 500
  send_interrupt = false
  stop_on_error = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  main_only = false
  time = false

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
  keep_scroll = true
EOF
    echo -e "${GREEN}✓ air 配置创建完成${NC}"
else
    echo -e "${GREEN}✓ air 配置已存在${NC}"
fi

# 创建 Makefile
echo ""
echo -e "${YELLOW}创建 Makefile...${NC}"
if [ ! -f "Makefile" ]; then
    cat > Makefile <<'EOF'
.PHONY: help build run dev test clean agent

help:
	@echo "V Panel 开发命令"
	@echo ""
	@echo "make build       - 编译 Panel"
	@echo "make agent       - 编译 Agent"
	@echo "make run         - 运行 Panel"
	@echo "make dev         - 开发模式（热重载）"
	@echo "make test        - 运行测试"
	@echo "make lint        - 代码检查"
	@echo "make clean       - 清理构建文件"

build:
	go build -o vpanel ./cmd/v/main.go

agent:
	go build -o vpanel-agent ./cmd/agent/main.go

run: build
	./vpanel

dev:
	air

test:
	go test -v ./...

lint:
	golangci-lint run

clean:
	rm -f vpanel vpanel-agent
	rm -rf build tmp
EOF
    echo -e "${GREEN}✓ Makefile 创建完成${NC}"
else
    echo -e "${GREEN}✓ Makefile 已存在${NC}"
fi

# 显示下一步
echo ""
echo -e "${GREEN}开发环境设置完成！${NC}"
echo ""
echo "下一步:"
echo "1. 配置数据库: 编辑 configs/config.yaml"
echo "2. 运行迁移: ./vpanel migrate"
echo "3. 启动开发: make dev"
echo ""
echo "常用命令:"
echo "  make build    - 编译"
echo "  make dev      - 开发模式"
echo "  make test     - 测试"
echo "  make lint     - 代码检查"
echo ""
