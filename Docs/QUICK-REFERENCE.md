# V Panel 快速参考

## 常用命令速查

### 开发

```bash
make help          # 查看所有命令
make build         # 编译 Panel
make agent         # 编译 Agent
make dev           # 开发模式（热重载）
make test          # 运行测试
make lint          # 代码检查
make clean         # 清理构建文件
```

### 部署

```bash
# 部署 Panel
make deploy-panel

# 部署 Agent
make deploy-agent PANEL_URL=https://panel.example.com NODE_TOKEN=xxx

# 或使用脚本
./scripts/quick-deploy.sh panel
./scripts/quick-deploy.sh agent https://panel.example.com token
```

### 运维

```bash
# 健康检查
./scripts/health-check.sh all

# 备份
./scripts/backup.sh all

# 恢复
./scripts/backup.sh restore backups/database/xxx.sql.gz

# 日志管理
./scripts/log-rotate.sh analyze
./scripts/log-rotate.sh rotate
```

### 测试

```bash
# API 测试
make api-test ADMIN_TOKEN=xxx

# 数据库验证
./scripts/verify-migration.sh

# 单元测试
make test
```

---

## 目录结构

```
V/
├── Makefile                    # 开发命令
├── vpanel                      # Panel 二进制
├── vpanel-agent                # Agent 二进制
├── configs/                    # 配置文件
│   ├── config.yaml            # Panel 配置
│   └── xray.json.example      # Xray 配置示例
├── scripts/                    # 脚本工具
│   ├── dev-setup.sh           # 开发环境设置
│   ├── build-agent.sh         # 编译 Agent
│   ├── quick-deploy.sh        # 快速部署
│   ├── health-check.sh        # 健康检查
│   ├── backup.sh              # 备份恢复
│   └── log-rotate.sh          # 日志轮转
├── Docs/                       # 文档
│   ├── OPERATIONS-GUIDE.md    # 运维指南
│   ├── SCRIPTS-GUIDE.md       # 脚本指南
│   ├── KNOWN-ISSUES.md        # 已知问题
│   └── quick-start-xray.md    # 快速开始
└── logs/                       # 日志目录
```

---

## 配置文件

### Panel 配置

**位置**: `configs/config.yaml`

```yaml
server:
  host: 0.0.0.0
  port: 8080

database:
  host: localhost
  port: 5432
  name: vpanel
  user: vpanel
  password: password
```

### Agent 配置

**位置**: `/etc/vpanel/agent.yaml`

```yaml
panel:
  url: "https://panel.example.com"
  token: "node-token"

xray:
  binary_path: "/usr/local/bin/xray"
  config_path: "/usr/local/etc/xray/config.json"  # 托管标准路径；历史手工节点仍兼容 /etc/xray/config.json

sync:
  interval: 5m
```

---

## 服务管理

### Panel

```bash
# 启动
./vpanel

# 后台运行
nohup ./vpanel > logs/vpanel.log 2>&1 &

# 查看日志
tail -f logs/vpanel.log
```

### Agent

```bash
# 启动
systemctl start vpanel-agent

# 停止
systemctl stop vpanel-agent

# 重启
systemctl restart vpanel-agent

# 状态
systemctl status vpanel-agent

# 日志
journalctl -u vpanel-agent -f
```

### Xray

```bash
# 启动
systemctl start xray

# 停止
systemctl stop xray

# 重启
systemctl restart xray

# 状态
systemctl status xray

# 测试配置
xray -test -config /etc/xray/config.json
```

---

## 故障排查

### 历史手工节点流量不上报

```bash
# 1. 查看 Agent 配置中的 Xray 路径
grep config_path /etc/vpanel/agent.yaml

# 2. 查看 Xray 实际运行路径
ps -eo pid=,comm=,args= | grep xray

# 3. 查看 Agent 流量诊断
curl http://127.0.0.1:18443/traffic/status
```

### Panel 无法启动

```bash
# 1. 检查配置
cat configs/config.yaml

# 2. 检查数据库
psql -h localhost -U vpanel -d vpanel

# 3. 检查端口
lsof -i :8080

# 4. 查看日志
tail -100 logs/vpanel.log
```

### Agent 无法连接

```bash
# 1. 检查状态
systemctl status vpanel-agent

# 2. 查看日志
journalctl -u vpanel-agent -n 50

# 3. 测试连接
curl -v https://panel.example.com/health

# 4. 检查配置
cat /etc/vpanel/agent.yaml
```

### Xray 无法启动

```bash
# 1. 验证配置
xray -test -config /etc/xray/config.json

# 2. 查看日志
tail -100 /var/log/xray/error.log

# 3. 检查端口
netstat -tlnp | grep xray

# 4. 手动启动
xray -config /etc/xray/config.json
```

---

## API 端点

### 健康检查

```bash
curl http://localhost:8080/health
```

### 节点管理

```bash
# 获取节点列表
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/admin/nodes

# 获取节点配置
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/admin/nodes/1/config/preview
```

### 代理管理

```bash
# 获取代理列表
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/proxies

# 创建代理
curl -X POST -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test","protocol":"vless","port":443,"node_id":1}' \
  http://localhost:8080/api/proxies
```

---

## 环境变量

### 开发

```bash
export VERSION=1.0.0
export BUILD_DIR=build
```

### 数据库

```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=vpanel
export DB_USER=vpanel
export DB_PASSWORD=password
```

### 备份

```bash
export BACKUP_DIR=backups
export KEEP_DAYS=7
```

### 日志

```bash
export LOG_DIR=logs
export MAX_SIZE=100M
export KEEP_DAYS=30
```

---

## 定时任务

```bash
# 编辑 crontab
crontab -e

# 推荐任务
*/5 * * * * cd /path/to/vpanel && ./scripts/health-check.sh all >> /var/log/vpanel-health.log 2>&1
0 2 * * * cd /path/to/vpanel && ./scripts/log-rotate.sh rotate
0 3 * * * cd /path/to/vpanel && ./scripts/backup.sh all
0 4 * * 0 cd /path/to/vpanel && ./scripts/backup.sh clean
```

---

## 端口说明

| 端口 | 服务 | 说明 |
|------|------|------|
| 8080 | Panel | Web 管理界面 |
| 18443 | Agent | 健康检查端点 |
| 62789 | Xray API | Xray 管理 API |
| 443+ | 代理 | 用户代理端口 |

---

## 日志位置

| 服务 | 日志位置 |
|------|----------|
| Panel | `logs/vpanel.log` |
| Agent | `journalctl -u vpanel-agent` |
| Xray | `/var/log/xray/error.log` |
| Xray Access | `/var/log/xray/access.log` |

---

## 重要文档

| 文档 | 说明 |
|------|------|
| [快速开始](Docs/quick-start-xray.md) | 新用户入门 |
| [脚本指南](Docs/SCRIPTS-GUIDE.md) | 脚本使用 |
| [运维指南](Docs/OPERATIONS-GUIDE.md) | 运维手册 |
| [已知问题](Docs/KNOWN-ISSUES.md) | 问题和解决方案 |

---

## 获取帮助

```bash
# 查看 Makefile 命令
make help

# 查看脚本帮助
./scripts/health-check.sh
./scripts/backup.sh
./scripts/log-rotate.sh
```

---

**最后更新**: 2026-01-19
