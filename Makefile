.PHONY: help build agent run dev test lint clean migrate install docker-build docker-up docker-down docker-logs

# 默认目标
.DEFAULT_GOAL := help

# 变量
VERSION ?= 1.0.0
BUILD_DIR = build
PANEL_BINARY = vpanel
AGENT_BINARY = vpanel-agent

help: ## 显示帮助信息
	@echo "V Panel - 可用命令:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## 编译 Panel
	@echo "编译 Panel..."
	@go build -ldflags "-X main.version=$(VERSION) -s -w" -o $(PANEL_BINARY) ./cmd/v/main.go
	@echo "✓ 编译完成: $(PANEL_BINARY)"

agent: ## 编译 Agent
	@echo "编译 Agent..."
	@mkdir -p bin
	@go build -ldflags "-X main.version=$(VERSION) -s -w" -o bin/$(AGENT_BINARY)-amd64 ./cmd/agent/main.go
	@echo "✓ 编译完成: bin/$(AGENT_BINARY)-amd64"

agent-linux-amd64: ## 编译 Linux amd64 Agent
	@echo "编译 Linux amd64 Agent..."
	@mkdir -p bin
	@GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION) -s -w" -o bin/$(AGENT_BINARY)-amd64 ./cmd/agent/main.go
	@echo "✓ 编译完成: bin/$(AGENT_BINARY)-amd64"

agent-linux-arm64: ## 编译 Linux arm64 Agent
	@echo "编译 Linux arm64 Agent..."
	@mkdir -p bin
	@GOOS=linux GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION) -s -w" -o bin/$(AGENT_BINARY)-arm64 ./cmd/agent/main.go
	@echo "✓ 编译完成: bin/$(AGENT_BINARY)-arm64"

agent-linux-arm: ## 编译 Linux arm Agent
	@echo "编译 Linux arm Agent..."
	@mkdir -p bin
	@GOOS=linux GOARCH=arm go build -ldflags "-X main.version=$(VERSION) -s -w" -o bin/$(AGENT_BINARY)-arm ./cmd/agent/main.go
	@echo "✓ 编译完成: bin/$(AGENT_BINARY)-arm"

agent-all: agent-linux-amd64 agent-linux-arm64 agent-linux-arm ## 编译所有平台 Agent

build-all: build agent ## 编译 Panel 和 Agent

agent-multi: ## 编译多平台 Agent
	@./scripts/build/build-agent.sh

run: build ## 运行 Panel
	@./$(PANEL_BINARY)

dev: ## 开发模式（热重载）
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "air 未安装，使用普通模式..."; \
		go run ./cmd/v/main.go; \
	fi

test: ## 运行测试
	@echo "运行测试..."
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "✓ 测试完成"

test-coverage: test ## 查看测试覆盖率
	@go tool cover -html=coverage.out

lint: ## 代码检查
	@echo "运行代码检查..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint 未安装，跳过检查"; \
	fi

fmt: ## 格式化代码
	@echo "格式化代码..."
	@go fmt ./...
	@echo "✓ 格式化完成"

clean: ## 清理构建文件
	@echo "清理构建文件..."
	@rm -f $(PANEL_BINARY) $(AGENT_BINARY)
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out
	@echo "✓ 清理完成"

migrate: ## 运行数据库迁移
	@echo "运行数据库迁移..."
	@./$(PANEL_BINARY) migrate || (make build && ./$(PANEL_BINARY) migrate)

migrate-verify: ## 验证数据库迁移
	@echo "迁移验证脚本已移除，请使用 make migrate 直接运行迁移"

install: ## 安装开发工具
	@echo "安装开发工具..."
	@go install github.com/cosmtrek/air@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "✓ 安装完成"

deps: ## 下载依赖
	@echo "下载依赖..."
	@go mod download
	@go mod tidy
	@echo "✓ 依赖下载完成"

docker-build: ## 构建 Docker 镜像
	@echo "构建 Docker 镜像..."
	@docker build -t v-panel:$(VERSION) -f deployments/docker/Dockerfile .
	@echo "✓ Docker 镜像构建完成"

docker-up: ## 使用 Docker Compose 启动服务
	@cd deployments/docker && if docker compose version > /dev/null 2>&1; then docker compose up -d; else docker-compose up -d; fi

docker-down: ## 使用 Docker Compose 停止服务
	@cd deployments/docker && if docker compose version > /dev/null 2>&1; then docker compose down; else docker-compose down; fi

docker-logs: ## 查看 Docker Compose 日志
	@cd deployments/docker && if docker compose version > /dev/null 2>&1; then docker compose logs -f; else docker-compose logs -f; fi

deploy-panel: build ## 部署 Panel
	@./scripts/deploy/quick-deploy.sh panel

deploy-agent: agent ## 部署 Agent (需要参数: PANEL_URL, NODE_TOKEN)
	@if [ -z "$(PANEL_URL)" ] || [ -z "$(NODE_TOKEN)" ]; then \
		echo "错误: 需要设置 PANEL_URL 和 NODE_TOKEN"; \
		echo "示例: make deploy-agent PANEL_URL=https://panel.example.com NODE_TOKEN=token"; \
		exit 1; \
	fi
	@./scripts/deploy/quick-deploy.sh agent $(PANEL_URL) $(NODE_TOKEN)

api-test: ## 测试 API (需要 ADMIN_TOKEN)
	@echo "API 测试脚本已移除，请使用 go test ./internal/... 运行集成测试"

setup: deps install ## 设置开发环境
	@./scripts/dev/dev-setup.sh

all: clean deps build agent ## 完整构建流程
