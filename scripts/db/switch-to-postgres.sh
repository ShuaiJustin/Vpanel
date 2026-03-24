#!/bin/bash
# 快速切换到 PostgreSQL

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   切换到 PostgreSQL 数据库${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}错误: Docker 未安装${NC}"
    exit 1
fi

# 配置
DB_NAME="${1:-vpanel}"
DB_USER="${2:-vpanel}"
DB_PASS="${3:-vpanel123}"
DB_PORT="${4:-5432}"

echo -e "${YELLOW}数据库配置:${NC}"
echo "  数据库名: $DB_NAME"
echo "  用户名: $DB_USER"
echo "  密码: $DB_PASS"
echo "  端口: $DB_PORT"
echo ""

# 1. 备份 SQLite 数据库
if [ -f "data/v.db" ]; then
    echo -e "${YELLOW}1. 备份 SQLite 数据库...${NC}"
    cp data/v.db "data/v.db.backup.$(date +%Y%m%d_%H%M%S)"
    echo -e "${GREEN}✓ 备份完成${NC}"
else
    echo -e "${YELLOW}1. 未找到 SQLite 数据库，跳过备份${NC}"
fi
echo ""

# 2. 启动 PostgreSQL 容器
echo -e "${YELLOW}2. 启动 PostgreSQL 容器...${NC}"
if docker ps -a | grep -q v-panel-postgres; then
    echo -e "${BLUE}容器已存在，重启中...${NC}"
    docker start v-panel-postgres
else
    docker run -d \
      --name v-panel-postgres \
      -e POSTGRES_DB=$DB_NAME \
      -e POSTGRES_USER=$DB_USER \
      -e POSTGRES_PASSWORD=$DB_PASS \
      -p $DB_PORT:5432 \
      -v v-panel-pgdata:/var/lib/postgresql/data \
      postgres:16-alpine
fi

echo -e "${GREEN}✓ PostgreSQL 容器已启动${NC}"
echo ""

# 3. 等待 PostgreSQL 就绪
echo -e "${YELLOW}3. 等待 PostgreSQL 就绪...${NC}"
for i in {1..30}; do
    if docker exec v-panel-postgres pg_isready -U $DB_USER > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PostgreSQL 已就绪${NC}"
        break
    fi
    echo -n "."
    sleep 1
done
echo ""

# 4. 备份当前配置
echo -e "${YELLOW}4. 备份当前配置...${NC}"
if [ -f "configs/config.yaml" ]; then
    cp configs/config.yaml "configs/config.yaml.backup.$(date +%Y%m%d_%H%M%S)"
    echo -e "${GREEN}✓ 配置已备份${NC}"
fi
echo ""

# 5. 更新配置文件
echo -e "${YELLOW}5. 更新配置文件...${NC}"
cat > configs/config.yaml << EOF
# V Panel Configuration
server:
  host: 0.0.0.0
  port: 8080
  mode: release
  cors_origins:
    - "*"

database:
  driver: postgres
  dsn: "host=localhost port=$DB_PORT user=$DB_USER password=$DB_PASS dbname=$DB_NAME sslmode=disable"
  max_open_conns: 100
  max_idle_conns: 25
  conn_max_lifetime: 3600

jwt:
  secret: "your-secret-key-change-in-production"
  expiration: 86400

log:
  level: info
  file: logs/app.log
  max_size: 100
  max_backups: 10
  max_age: 30
EOF

echo -e "${GREEN}✓ 配置已更新${NC}"
echo ""

# 6. 显示连接信息
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}   PostgreSQL 配置完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${YELLOW}连接信息:${NC}"
echo "  主机: localhost"
echo "  端口: $DB_PORT"
echo "  数据库: $DB_NAME"
echo "  用户名: $DB_USER"
echo "  密码: $DB_PASS"
echo ""
echo -e "${YELLOW}DSN:${NC}"
echo "  host=localhost port=$DB_PORT user=$DB_USER password=$DB_PASS dbname=$DB_NAME sslmode=disable"
echo ""
echo -e "${YELLOW}下一步:${NC}"
echo "  1. 重新编译: go build -o v ./cmd/v/main.go"
echo "  2. 启动服务: ./v"
echo "  3. 或使用菜单: ./vpanel.sh"
echo ""
echo -e "${BLUE}提示: 首次启动会自动创建所有表${NC}"
