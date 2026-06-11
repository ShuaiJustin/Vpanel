# V Panel

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/Vue-3.x-4FC08D?style=flat&logo=vue.js" alt="Vue Version">
  <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License">
  <img src="https://img.shields.io/badge/Docker-20+-2496ED?style=flat&logo=docker" alt="Docker">
</p>

<p align="center">
  现代化的多协议代理管理面板，支持集中管理远程节点、用户订阅、流量统计和商业化运营
</p>

---

## ✨ 特性

### 核心功能
- 🚀 **多协议支持** - VLESS、VMess、Trojan、Shadowsocks，支持 Reality、XTLS、WebSocket、gRPC
- 🌐 **远程节点管理** - 集中管理多个远程 Xray 节点，自动配置下发
- 👥 **用户管理** - 用户隔离、流量统计、配额管理、订阅链接生成
- 📊 **流量监控** - 实时流量统计、历史数据分析、节点健康检查
- 🔒 **HTTPS 证书** - 集成 Let's Encrypt，支持泛域名证书自动申请和续期

### 商业化功能
- 💰 **套餐系统** - 灵活的套餐配置、订单管理
- 🎫 **营销工具** - 优惠券、礼品卡、试用、邀请返佣
- 💳 **支付集成** - 支付宝、微信支付
- 📈 **数据分析** - 用户统计、收入报表

### 运维功能
- 📝 **操作审计** - 完整的操作日志记录
- 🔐 **安全控制** - IP 限制、JWT 认证、CSRF 保护
- 🔄 **自动备份** - 数据库定期备份，可配置保留策略
- 📡 **Agent 管理** - 远程 Agent 自动更新、SSH 配置管理

---

## 🚀 快速开始

### 环境要求
- Docker 20+
- Docker Compose v2

### 一键部署

```bash
git clone https://github.com/chengchnegcheng/Vpanel.git
cd Vpanel
./vpanel.sh
```

选择 **"Docker 一键部署"**，脚本会自动：
1. 生成随机管理员密码和 JWT Secret
2. 分配随机宿主机端口（10000-65000）
3. 构建并启动容器

### 手动部署

```bash
cd deployments/docker
cp .env.example .env
nano .env  # 配置必要参数
docker compose up -d --build
```

### 首次登录

1. 访问 `http://<your-host>:<端口>/`
2. 用户名：`admin`
3. 密码：查看 `deployments/docker/.env` 中的 `V_ADMIN_PASS`

⚠️ **登录后请立即修改密码**（右上角头像 → 修改密码）

---

## 📖 文档

| 文档 | 说明 |
|------|------|
| [生产部署检查清单](docs/production-checklist.md) | 生产环境部署完整指南 |
| [订阅链接配置](docs/subscription-url-config.md) | 订阅链接地址配置方法 |
| [运维指南](Docs/OPERATIONS-GUIDE.md) | 日常运维操作手册 |
| [节点部署](Docs/NODE-AGENT-GUIDE.md) | 远程节点 Agent 部署 |
| [证书申请](Docs/certificate-guide.md) | Let's Encrypt 证书申请流程 |
| [快速参考](Docs/QUICK-REFERENCE.md) | 常用命令速查 |

---

## 🔧 配置

### 核心配置（`.env` 文件）

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `V_SERVER_PUBLIC_URL` | 公网访问地址（用于 Agent 部署） | 必须配置 |
| `V_SERVER_BASE_URL` | 订阅链接基础地址 | 自动检测 |
| `V_ADMIN_USER` | 管理员用户名 | `admin` |
| `V_ADMIN_PASS` | 管理员密码 | 自动生成 |
| `V_JWT_SECRET` | JWT 签名密钥 | 自动生成 |
| `V_SERVER_CORS_ORIGINS` | CORS 白名单 | 生产必须配置 |
| `VPANEL_PUBLISH_PORT` | 宿主机端口 | 自动分配 |
| `VPANEL_BACKUP_ENABLED` | 自动备份 | `1` |
| `VPANEL_BACKUP_RETENTION_DAYS` | 备份保留天数 | `14` |

完整配置说明：[deployments/docker/.env.example](deployments/docker/.env.example)

---

## 🛠️ 常用命令

```bash
# 使用交互菜单
./vpanel.sh

# 直接操作
./deployments/scripts/start.sh start      # 启动
./deployments/scripts/start.sh stop       # 停止
./deployments/scripts/start.sh restart    # 重启
./deployments/scripts/start.sh logs       # 查看日志
./deployments/scripts/start.sh status     # 查看状态

# 进入容器
docker exec -it vpanel-v-panel-1 sh

# 升级
git pull
./vpanel.sh  # 选择 "重新构建并启动"
```

---

## 🔒 启用 HTTPS

无需修改配置文件，在面板界面操作：

1. **证书管理** → 申请证书 → 选择 DNS 验证 → 填写 DNS API 凭证
2. 等待证书签发完成（1-2 分钟）
3. **系统设置** → HTTPS/TLS → 选择证书 → 应用
4. 重启面板

详见：[证书申请指南](Docs/certificate-guide.md)

---

## 🌐 部署远程节点

主面板负责管理，各个出口节点运行 Xray。部署方法：

1. 在面板中添加节点，获取 Token
2. 在远程服务器上安装 Agent
3. Agent 自动连接主面板并接收配置

详见：[节点 Agent 部署指南](Docs/NODE-AGENT-GUIDE.md)

---

## 🐛 故障排查

| 问题 | 解决方法 |
|------|----------|
| 订阅链接显示 localhost | 配置 `V_SERVER_BASE_URL` 或使用 Nginx 反向代理 |
| 容器 unhealthy | 查看日志：`docker logs vpanel-v-panel-1` |
| Agent 无法连接 | 检查 `V_SERVER_PUBLIC_URL` 是否为公网地址 |
| 证书申请失败 | 检查 DNS API 凭证和网络连接 |
| 配置修改不生效 | 需要重建容器：`docker compose up -d` |

更多问题：[运维指南](Docs/OPERATIONS-GUIDE.md)

---

## 🏗️ 架构

```
┌─────────────┐
│  Web Panel  │  ← Vue 3 + Element Plus
│   (主面板)   │
└──────┬──────┘
       │
       │ HTTP/HTTPS
       │
┌──────┴──────┐
│  Go Backend │  ← Gin + SQLite
│  API Server │
└──────┬──────┘
       │
       │ SSH / Agent
       │
┌──────┴──────┬──────────┬──────────┐
│   Node 1    │  Node 2  │  Node 3  │
│  Xray +     │  Xray +  │  Xray +  │
│  Agent      │  Agent   │  Agent   │
└─────────────┴──────────┴──────────┘
```

---

## 🔄 API 功能

### 新增接口（v1.1.0）
- `POST /api/admin/nodes/:id/agent/update` - Agent 远程更新
- `GET/POST /api/admin/nodes/:id/ssh-metadata` - SSH 配置管理

### 订阅链接优化
- 支持更多反向代理头（`X-Forwarded-Host`, `X-Forwarded-Proto`）
- 自动过滤本地地址（localhost/127.0.0.1）
- 智能 URL 检测

---

## 👨‍💻 开发

### 本地开发

```bash
# 后端
go run ./cmd/v

# 前端（新终端）
cd web
npm install
npm run dev
```

### 测试

```bash
# 后端测试
go vet ./...
go test ./...

# 前端测试
cd web
npm run lint
npm test
```

### 代码提交

提交前确保通过所有测试和代码检查。

---

## 📊 技术栈

### 后端
- **语言**: Go 1.21+
- **框架**: Gin
- **数据库**: SQLite
- **认证**: JWT

### 前端
- **框架**: Vue 3
- **UI**: Element Plus
- **构建**: Vite
- **状态管理**: Pinia

### 基础设施
- **容器**: Docker + Docker Compose
- **反向代理**: Nginx
- **证书**: Let's Encrypt (acme.sh)

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交改动 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 提交 Pull Request

---

## 📄 许可证

本项目采用 [MIT](LICENSE) 许可证。

---

## 🙏 致谢

感谢所有贡献者和使用者的支持！

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/ShuaiJustin/">/ShuaiJustin</a>
</p>
