# V Panel - 高性能代理服务器管理面板

<div align="center">

[![Build Status](https://github.com/chengchnegcheng/Vpanel/workflows/Build%20and%20Release/badge.svg)](https://github.com/chengchnegcheng/Vpanel/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/chengchnegcheng/Vpanel)](https://goreportcard.com/report/github.com/chengchnegcheng/Vpanel)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![GitHub release](https://img.shields.io/github/release/chengchnegcheng/Vpanel.svg)](https://github.com/chengchnegcheng/Vpanel/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/chengchnegcheng/Vpanel)](https://github.com/chengchnegcheng/Vpanel)

  <p>
    <a href="#功能特点">功能特点</a> •
    <a href="#快速开始">快速开始</a> •
    <a href="#docker-部署">Docker 部署</a> •
    <a href="#功能状态">功能状态</a> •
    <a href="#客户端推荐">客户端推荐</a> •
    <a href="#开发指南">开发指南</a>
  </p>
</div>

V Panel 是一个用 Go 语言编写的高性能代理服务器管理面板，基于 Xray-core，支持多种代理协议。提供完整的用户管理、流量统计、证书管理等功能，以及直观的 Web 管理界面。

## 功能特点

### 核心功能
- **多协议支持**: VMess, VLESS, Trojan, Shadowsocks
- **用户管理**: 认证授权、流量限制、状态监控、多级权限
- **流量管理**: 实时统计、每日统计、流量限制和警告
- **证书管理**: 自动 SSL 证书申请和更新、多域名支持
- **Xray 管理**: 版本切换、远程更新、运行状态监控
- **系统管理**: 完整日志系统、系统状态监控、配置管理
- **角色权限**: 角色管理、权限分配
- **代理控制**: 启停控制、批量操作

### 多服务器管理
- **节点管理**: 注册、编辑、删除远程 Xray 节点，支持 IP/域名、标签、地区分类
- **节点分组**: 按地区或用途组织节点，支持分组级别配置
- **健康检查**: 自动定期检测节点状态（TCP、API、Xray 进程），可配置检查间隔
- **负载均衡**: 支持轮询、最少连接、加权、地理位置等多种策略
- **故障转移**: 节点故障时自动迁移用户到健康节点，支持同组优先和跨组转移
- **配置同步**: 代理配置自动同步到所有相关节点，支持手动触发
- **Node Agent**: 轻量级代理程序，部署在节点上执行命令和上报指标

## 快速开始

### 系统要求

- Docker 20+ (推荐，一键部署)
- 或 Go 1.23+ (源码构建)
- 或 Node.js 20+ (前端开发)

### 方式一：Docker 一键部署（推荐）

最简单的部署方式，使用交互式菜单：

```bash
# 1. 克隆仓库
git clone https://github.com/chengchnegcheng/Vpanel.git
cd Vpanel

# 2. 启动菜单管理脚本
./vpanel.sh
```

**菜单功能：**
- 🐳 Docker 部署管理（启动/停止/重启/日志/状态/清理）
- 💻 本地开发环境（编译/运行/测试/前端开发）
- ⚙️ 配置管理（创建/编辑配置文件）
- 🔍 系统环境检查（检查依赖安装状态）

**启动成功后：**
- 🌐 访问地址：终端会显示随机生成的端口
- 👤 默认用户名：`admin`
- 🔑 默认密码：`admin123`（或查看 `deployments/docker/.env` 文件）

> 💡 首次启动会自动生成随机端口（10000-65000），端口号保存在 `.env` 文件中

**传统命令方式（仍然支持）：**
```bash
./deployments/scripts/start.sh start    # 启动服务
./deployments/scripts/start.sh stop     # 停止服务
./deployments/scripts/start.sh restart  # 重启服务
./deployments/scripts/start.sh logs     # 查看日志
./deployments/scripts/start.sh status   # 查看状态
```

### 方式二：从源码构建

```bash
# 克隆仓库
git clone https://github.com/chengchnegcheng/Vpanel.git
cd Vpanel

# 构建（前端 + 后端）
./scripts/build.sh all

# 启动服务
./scripts/start.sh start
```

**启动成功后：**
- 🌐 访问地址：终端会显示随机生成的端口
- 👤 默认用户名：`admin`
- 🔑 默认密码：`admin123`

**其他命令：**
```bash
./scripts/start.sh stop     # 停止服务
./scripts/start.sh restart  # 重启服务
./scripts/start.sh logs     # 查看日志
./scripts/start.sh status   # 查看状态
./scripts/start.sh run      # 前台运行（开发调试）
```

### 方式三：本地开发

**推荐使用菜单方式：**
```bash
# 启动菜单，选择"本地开发环境"
./vpanel.sh
```

**传统命令方式：**
```bash
# 安装依赖
./deployments/scripts/dev.sh install

# 启动后端
./deployments/scripts/dev.sh start

# 启动前端（另开终端）
./deployments/scripts/dev.sh frontend
```

### 访问地址说明

V Panel 提供两套独立的界面，管理后台和用户门户完全分离：

| 界面 | 访问地址 | 说明 |
|------|---------|------|
| **用户门户** | `http://localhost:端口/` | 默认入口，普通用户访问 |
| **管理后台** | `http://localhost:端口/login` | 管理员登录入口（隐藏） |

> 💡 端口号在首次启动时随机生成，可在终端输出或 `.env` 文件中查看

#### 用户门户 (User Portal)
- 访问 `http://localhost:端口/` 自动跳转到 `/user/login`（用户门户登录页）
- 访问 `http://localhost:端口/user/register` 进行注册
- 登录后进入用户仪表板 `/user/dashboard`
- 用户可查看节点、订阅、订单、工单等

#### 管理后台 (Admin)
- 访问 `http://localhost:端口/login` 使用管理员账号登录（注意：不是 `/user/login`）
- 登录后自动进入管理仪表盘 `/admin/dashboard`
- 可管理用户、节点、套餐、订单、系统设置等
- **管理后台地址相对隐藏，提高安全性**

### ⚠️ 首次登录注意事项

1. **管理员**：访问 `http://localhost:端口/login`，使用 `admin` / `admin123` 登录
2. **普通用户**：访问 `http://localhost:端口/` 或 `/user/register` 注册账号
3. **首次登录后请立即修改密码！**
4. 建议修改 JWT 密钥（生产环境）

> 💡 **提示**: 管理员登录地址是 `/login`，用户登录地址是 `/user/login`，两者不同！

## Docker 部署详细说明

上面的一键部署已经是最简单的方式。如果需要更多控制，可以使用以下方式：

### 使用 Docker Compose (手动方式)

```bash
# 进入部署目录
cd deployments/docker

# 复制环境变量配置
cp .env.example .env

# 编辑配置 (修改密码等)
vim .env

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f
```

### 使用 Docker 命令

```bash
# 构建镜像
./scripts/docker-build.sh build

# 运行容器（端口会自动分配）
docker run -d \
  --name v-panel \
  -p 随机端口:8080 \
  -v v-panel-data:/app/data \
  -e V_JWT_SECRET=your-secret \
  -e V_ADMIN_PASS=your-password \
  v-panel:latest
```

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `V_SERVER_PORT` | 服务端口（留空自动生成） | 随机 10000-65000 |
| `V_SERVER_MODE` | 运行模式 (debug/release) | release |
| `V_SERVER_PUBLIC_URL` | 面板公网地址，用于支付回调等外部地址生成 | - |
| `V_JWT_SECRET` | JWT 密钥 | - |
| `V_ADMIN_USER` | 管理员用户名 | admin |
| `V_ADMIN_PASS` | 管理员密码 | admin123 |
| `V_LOG_LEVEL` | 日志级别 | info |
| `V_DB_PATH` | 数据库路径 | /app/data/v.db |
| `V_PAYMENT_ALIPAY_ENABLED` | 是否启用支付宝 | false |
| `V_PAYMENT_ALIPAY_APP_ID` | 支付宝应用 ID | - |
| `V_PAYMENT_ALIPAY_PRIVATE_KEY` | 支付宝商户私钥 | - |
| `V_PAYMENT_ALIPAY_PUBLIC_KEY` | 支付宝公钥 | - |
| `V_PAYMENT_WECHAT_ENABLED` | 是否启用微信支付 | false |
| `V_PAYMENT_WECHAT_APP_ID` | 微信应用 ID | - |
| `V_PAYMENT_WECHAT_MCH_ID` | 微信商户号 | - |
| `V_PAYMENT_WECHAT_API_KEY` | 微信 API Key | - |

## Node Agent 部署

Node Agent 是部署在远程节点服务器上的轻量级代理程序，负责与 Panel 通信、执行命令和上报指标。

### 部署步骤

1. 在 Panel 管理界面添加节点，获取节点 Token
2. 在节点服务器上部署 Agent：

```bash
# 下载 Agent
wget https://github.com/chengchnegcheng/Vpanel/releases/latest/download/v-agent-linux-amd64

# 添加执行权限
chmod +x v-agent-linux-amd64

# 创建配置文件
cat > agent.yaml << EOF
panel:
  address: "https://your-panel-address:端口"
  token: "your-node-token"
  
agent:
  port: 18443
  
xray:
  binary: "/usr/local/bin/xray"
  config: "/etc/xray/config.json"
EOF

# 运行 Agent
./v-agent-linux-amd64 -config agent.yaml
```

### Agent 配置说明

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `panel.address` | Panel 服务器地址 | - |
| `panel.token` | 节点认证 Token | - |
| `agent.port` | Agent 监听端口 | 18443 |
| `xray.binary` | Xray 可执行文件路径 | /usr/local/bin/xray |
| `xray.config` | Xray 配置文件路径 | /etc/xray/config.json |

### 使用 systemd 管理

```bash
# 创建 systemd 服务文件
cat > /etc/systemd/system/v-agent.service << EOF
[Unit]
Description=V Panel Node Agent
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/v-agent -config /etc/v-agent/agent.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 启用并启动服务
systemctl daemon-reload
systemctl enable v-agent
systemctl start v-agent
```

## 订阅系统

V Panel 提供完整的订阅链接系统，支持多种客户端格式自动检测和生成。

### 支持的订阅格式

| 格式 | 客户端 | 说明 |
|------|--------|------|
| V2rayN | v2rayN, v2rayNG | Base64 编码链接列表 |
| Clash | Clash, ClashX | YAML 配置文件 |
| Clash Meta | Clash Meta | 支持 Reality、XTLS |
| Sing-box | Sing-box | JSON 配置文件 |
| Shadowrocket | Shadowrocket | Base64 编码链接 |
| Surge | Surge | 配置文件格式 |
| Quantumult X | Quantumult X | 配置文件格式 |

### 订阅链接特性

- **自动格式检测**: 根据 User-Agent 自动返回对应格式
- **短链接支持**: 生成 8 字符短链接便于分享
- **令牌重新生成**: 支持重新生成订阅令牌，旧令牌立即失效
- **访问统计**: 记录订阅访问次数和最后访问时间
- **响应头信息**: 包含流量使用、到期时间等信息

### 订阅响应头

| 响应头 | 说明 |
|--------|------|
| `Subscription-Userinfo` | 流量使用信息 (upload, download, total, expire) |
| `Profile-Update-Interval` | 建议更新间隔（小时） |
| `Content-Disposition` | 配置文件名 |

## IP 限制系统

V Panel 提供完善的 IP 限制功能，防止账号共享和滥用。

### 功能特性

- **并发 IP 限制**: 限制同时在线的设备数量
- **白名单/黑名单**: 支持 IP 和 CIDR 范围
- **地理位置限制**: 基于 IP 地理位置的访问控制
- **自动黑名单**: 多次失败尝试自动封禁
- **设备管理**: 用户可查看和踢出在线设备
- **订阅 IP 限制**: 限制订阅链接的访问 IP 数量

### 配置选项

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `max_concurrent_ips` | 最大并发 IP 数 | 3 |
| `ip_inactive_timeout` | 不活跃 IP 超时时间 | 30 分钟 |
| `auto_blacklist_threshold` | 自动黑名单阈值 | 10 次 |
| `subscription_ip_limit` | 订阅链接 IP 限制 | 5 |

## 商业化系统

V Panel 提供完整的商业化功能，支持套餐销售、支付处理和用户管理。

### 套餐管理

- **套餐类型**: 支持按月、按季、按年等多种周期
- **流量配置**: 可设置流量限制或无限流量
- **价格设置**: 支持多币种定价
- **试用功能**: 支持套餐试用，可配置试用时长

### 支付系统

| 支付方式 | 状态 | 说明 |
|---------|------|------|
| 支付宝 | ✅ 支持 | 扫码支付、网页支付 |
| 微信支付 | ✅ 支持 | 扫码支付、JSAPI |
| 余额支付 | ✅ 支持 | 使用账户余额 |
| 礼品卡 | ✅ 支持 | 兑换码充值 |

### 营销功能

- **优惠券**: 支持折扣和固定金额优惠
- **邀请系统**: 邀请码、推荐奖励、佣金计算
- **礼品卡**: 批量生成、购买、兑换

### 订阅管理

- **套餐升降级**: 支持按比例计算差价
- **订阅暂停**: 用户可暂停订阅，到期自动恢复
- **自动续费**: 支持余额自动扣款

## 功能状态

### 后端管理功能

| 功能模块 | 状态 | 说明 |
|---------|------|------|
| 用户认证 (JWT) | ✅ 已完成 | 登录、登出、令牌刷新 |
| 用户管理 | ✅ 已完成 | CRUD、启用/禁用、密码重置 |
| 角色权限 | ✅ 已完成 | 角色管理、权限分配 |
| 代理管理 | ✅ 已完成 | 多协议、启停控制、批量操作 |
| 流量统计 | ✅ 已完成 | Dashboard、协议统计、用户统计 |
| 证书管理 | ✅ 已完成 | SSL 申请、续期、验证 |
| Xray 管理 | ✅ 已完成 | 启停、配置、版本管理 |
| 日志系统 | ✅ 已完成 | 查询、导出、清理 |
| 系统设置 | ✅ 已完成 | 配置管理、备份恢复 |
| 节点管理 | ✅ 已完成 | 节点注册、编辑、删除、状态监控 |
| 节点分组 | ✅ 已完成 | 分组 CRUD、成员管理、分组统计 |
| 健康检查 | ✅ 已完成 | 自动检测、状态转换、历史记录 |
| 负载均衡 | ✅ 已完成 | 多策略支持、容量限制、用户分配 |
| 故障转移 | ✅ 已完成 | 自动迁移、同组优先、跨组转移 |
| 配置同步 | ✅ 已完成 | 自动同步、手动触发、状态追踪 |
| Node Agent | ✅ 已完成 | 节点代理程序、指标上报、命令执行 |
| 订阅链接 | ✅ 已完成 | 多格式订阅、自动检测、短链接 |
| IP 限制 | ✅ 已完成 | 并发限制、白名单/黑名单、地理位置限制 |
| 数据库备份 | 🔜 待规划 | 自动备份恢复 |

### 商业化功能

| 功能模块 | 状态 | 说明 |
|---------|------|------|
| 套餐管理 | ✅ 已完成 | 套餐 CRUD、价格设置、流量/时长配置 |
| 订单系统 | ✅ 已完成 | 订单创建、状态管理、过期处理 |
| 支付网关 | ✅ 已完成 | 支付宝、微信支付、回调处理 |
| 余额系统 | ✅ 已完成 | 充值、扣款、交易记录 |
| 优惠券 | ✅ 已完成 | 折扣/固定金额、使用限制、批量生成 |
| 邀请系统 | ✅ 已完成 | 邀请码、推荐关系、佣金计算 |
| 发票管理 | ✅ 已完成 | 发票生成、PDF 下载 |
| 财务报表 | ✅ 已完成 | 收入统计、订单分析 |
| 套餐试用 | ✅ 已完成 | 试用激活、过期检查、转化统计 |
| 套餐升降级 | ✅ 已完成 | 升级价格计算、降级调度 |
| 支付重试 | ✅ 已完成 | 失败重试、支付方式切换 |
| 多币种 | ✅ 已完成 | 汇率转换、货币自动检测 |
| 订阅暂停 | ✅ 已完成 | 暂停/恢复、时长限制、自动恢复 |
| 礼品卡 | ✅ 已完成 | 创建、购买、兑换、批量生成 |

### 用户前台功能

| 功能模块 | 状态 | 说明 |
|---------|------|------|
| 用户注册/登录 | ✅ 已完成 | 邮箱验证、邀请码、2FA |
| 用户仪表板 | ✅ 已完成 | 流量使用、到期时间、快捷操作 |
| 节点列表 | ✅ 已完成 | 过滤排序、延迟测试 |
| 订阅管理 | ✅ 已完成 | 订阅链接、QR码、格式选择 |
| 客户端下载 | ✅ 已完成 | 按平台分组、推荐标记 |
| 个人设置 | ✅ 已完成 | 资料修改、安全设置、2FA |
| 使用统计 | ✅ 已完成 | 流量图表、数据导出 |
| 公告中心 | ✅ 已完成 | 公告列表、已读状态 |
| 工单系统 | ✅ 已完成 | 创建、回复、状态跟踪 |
| 帮助中心 | ✅ 已完成 | 文章列表、搜索、分类 |
| 设备管理 | ✅ 已完成 | 在线设备、踢出设备 |
| 移动端适配 | ✅ 已完成 | 响应式布局、底部导航 |
| 主题切换 | ✅ 已完成 | 深色/浅色主题 |

## 客户端推荐

### Windows
| 客户端 | 下载地址 | 特点 |
|--------|---------|------|
| **v2rayN** (推荐) | [GitHub](https://github.com/2dust/v2rayN/releases) | 界面简洁、支持订阅、自动更新 |
| Clash for Windows | [GitHub](https://github.com/Fndroid/clash_for_windows_pkg/releases) | 规则分流、订阅转换 |

### macOS
| 客户端 | 下载地址 | 特点 |
|--------|---------|------|
| **V2rayU** (推荐) | [GitHub](https://github.com/yanue/V2rayU/releases) | 原生应用、菜单栏快速切换 |
| ClashX | [GitHub](https://github.com/yichengchen/clashX/releases) | 规则分流、简洁易用 |

### Linux
| 客户端 | 下载地址 | 特点 |
|--------|---------|------|
| **v2rayA** (推荐) | [GitHub](https://github.com/v2rayA/v2rayA/releases) | Web 界面、透明代理 |
| Qv2ray | [GitHub](https://github.com/Qv2ray/Qv2ray/releases) | 图形化界面、插件支持 |

### Android
| 客户端 | 下载地址 | 特点 |
|--------|---------|------|
| **v2rayNG** (推荐) | [GitHub](https://github.com/2dust/v2rayNG/releases) | 开源免费、分应用代理 |
| Clash for Android | [GitHub](https://github.com/Kr328/ClashForAndroid/releases) | 规则分流 |

### iOS
| 客户端 | 下载地址 | 特点 |
|--------|---------|------|
| **Shadowrocket** (推荐) | App Store ($2.99) | 稳定可靠、规则分流 |
| Quantumult X | App Store ($7.99) | 功能强大、脚本支持 |
| Surge | App Store ($49.99) | 专业级、强大规则引擎 |

## 开发指南

### 开发环境

```bash
# 后端开发
go run ./cmd/v/main.go -config configs/config.yaml

# 前端开发
cd web
npm install
npm run dev
```

### 构建命令

```bash
# 完整构建
./scripts/build.sh all

# 仅构建后端
./scripts/build.sh backend

# 仅构建前端
./scripts/build.sh frontend

# 多平台构建
./scripts/build.sh platforms

# 运行测试
./scripts/build.sh test
```

### 测试

```bash
# 运行后端测试
go test -v ./...

# 运行前端测试
cd web
npm run test:unit

# 查看测试覆盖率
go test -cover ./...
```

## 故障排除

### 数据库和 API 问题

如果遇到 API 返回 500 或 503 错误，可能是数据库迁移未正确执行。

#### 诊断步骤

1. **检查数据库状态**
```bash
./scripts/check-db.sh
```

2. **测试 API 端点**
```bash
# 不需要认证的测试
./scripts/test-api.sh

# 需要 admin token 的完整测试
./scripts/test-api.sh http://localhost:你的端口 YOUR_ADMIN_TOKEN
```

#### 常见问题

**问题 1: IP 限制 API 返回 503/500**

原因：IP 限制相关的数据库表未创建

解决方案：
```bash
# 方案 A: 手动执行迁移（快速）
./scripts/fix-migrations.sh

# 方案 B: 重启服务（自动执行迁移）
./vpanel.sh  # 选择停止然后启动

# 方案 C: Docker 重新部署
./deployments/scripts/start.sh restart
```

**问题 2: 数据库文件损坏**

检查数据库完整性：
```bash
sqlite3 data/v.db "PRAGMA integrity_check;"
```

如果损坏，恢复备份：
```bash
cp data/v.db.backup data/v.db
```

**问题 3: 迁移失败**

查看详细错误：
```bash
# 查看应用日志
tail -f logs/app.log

# 或 Docker 日志
docker logs v-panel
```

#### 诊断工具

| 工具 | 用途 | 命令 |
|------|------|------|
| 数据库检查 | 检查表结构和迁移状态 | `./scripts/check-db.sh` |
| 迁移修复 | 手动执行 SQL 迁移 | `./scripts/fix-migrations.sh` |
| API 测试 | 测试所有 API 端点 | `./scripts/test-api.sh` |

### 日志查看

```bash
# 应用日志
tail -f logs/app.log

# Docker 日志
docker logs -f v-panel

# 查看特定错误
grep -i "error" logs/app.log
```

### 性能问题

如果遇到性能问题：

1. 检查数据库大小和索引
2. 查看慢查询日志
3. 监控系统资源使用

```bash
# 数据库大小
du -h data/v.db

# 系统资源
top
```

## 项目结构

```
v/
├── cmd/
│   ├── v/                 # 主程序入口
│   └── agent/             # Node Agent 入口
├── internal/               # 私有包
│   ├── api/               # API 层 (handlers, middleware, routes)
│   ├── agent/             # Node Agent 实现
│   ├── auth/              # 认证模块
│   ├── cache/             # 缓存层 (内存、Redis)
│   ├── commercial/        # 商业化模块
│   │   ├── balance/       # 余额服务
│   │   ├── commission/    # 佣金服务
│   │   ├── coupon/        # 优惠券服务
│   │   ├── currency/      # 多币种服务
│   │   ├── giftcard/      # 礼品卡服务
│   │   ├── invite/        # 邀请服务
│   │   ├── invoice/       # 发票服务
│   │   ├── order/         # 订单服务
│   │   ├── pause/         # 订阅暂停服务
│   │   ├── payment/       # 支付服务
│   │   ├── plan/          # 套餐服务
│   │   ├── planchange/    # 套餐升降级服务
│   │   ├── refund/        # 退款服务
│   │   └── trial/         # 试用服务
│   ├── config/            # 配置管理
│   ├── database/          # 数据库层 (migrations, repository)
│   ├── ip/                # IP 限制模块
│   ├── log/               # 日志服务
│   ├── logger/            # 日志模块
│   ├── monitor/           # 监控模块
│   ├── node/              # 节点管理 (服务、健康检查、负载均衡、故障转移)
│   ├── notification/      # 通知服务
│   ├── portal/            # 用户门户
│   │   ├── announcement/  # 公告服务
│   │   ├── auth/          # 门户认证
│   │   ├── help/          # 帮助中心
│   │   ├── node/          # 门户节点服务
│   │   ├── stats/         # 统计服务
│   │   └── ticket/        # 工单服务
│   ├── proxy/             # 代理协议 (vmess, vless, trojan, shadowsocks)
│   ├── server/            # HTTP 服务器
│   ├── settings/          # 系统设置
│   ├── subscription/      # 订阅系统
│   │   └── generators/    # 格式生成器 (v2rayn, clash, singbox 等)
│   └── xray/              # Xray 管理
├── pkg/                    # 公共包
│   ├── errors/            # 错误处理
│   └── sanitizer/         # 数据清理
├── configs/               # 配置模板
├── deployments/           # 部署文件 (docker, scripts)
├── scripts/               # 构建脚本
├── web/                   # 前端代码
│   └── src/
│       ├── api/           # API 模块
│       ├── components/    # 组件
│       ├── stores/        # Pinia 状态管理
│       ├── views/         # 页面视图
│       └── router/        # 路由配置
└── data/                  # 数据目录
```

## API 文档

### 认证
- `POST /api/auth/login` - 用户登录
- `POST /api/auth/refresh` - 刷新令牌
- `POST /api/auth/logout` - 用户登出
- `GET /api/auth/me` - 获取当前用户

### 代理管理
- `GET /api/proxies` - 获取代理列表
- `POST /api/proxies` - 创建代理
- `GET /api/proxies/:id` - 获取代理详情
- `PUT /api/proxies/:id` - 更新代理
- `DELETE /api/proxies/:id` - 删除代理
- `GET /api/proxies/:id/link` - 获取分享链接

### 系统
- `GET /api/system/info` - 系统信息
- `GET /api/system/status` - 系统状态
- `GET /health` - 健康检查
- `GET /ready` - 就绪检查

### 节点管理
- `GET /api/nodes` - 获取节点列表
- `POST /api/nodes` - 创建节点
- `GET /api/nodes/:id` - 获取节点详情
- `PUT /api/nodes/:id` - 更新节点
- `DELETE /api/nodes/:id` - 删除节点
- `POST /api/nodes/:id/token` - 生成/轮换节点 Token
- `DELETE /api/nodes/:id/token` - 撤销节点 Token
- `POST /api/nodes/:id/sync` - 同步节点配置
- `GET /api/nodes/:id/health` - 获取节点健康状态
- `GET /api/nodes/:id/stats` - 获取节点统计信息

### 节点分组
- `GET /api/node-groups` - 获取分组列表
- `POST /api/node-groups` - 创建分组
- `GET /api/node-groups/:id` - 获取分组详情
- `PUT /api/node-groups/:id` - 更新分组
- `DELETE /api/node-groups/:id` - 删除分组
- `POST /api/node-groups/:id/members` - 添加分组成员
- `DELETE /api/node-groups/:id/members/:nodeId` - 移除分组成员

### Node Agent
- `POST /api/agent/register` - Agent 注册
- `POST /api/agent/heartbeat` - Agent 心跳
- `POST /api/agent/metrics` - 上报指标
- `GET /api/agent/config` - 获取配置

### 订阅管理
- `GET /api/subscription/link` - 获取订阅链接
- `GET /api/subscription/info` - 获取订阅信息
- `POST /api/subscription/regenerate` - 重新生成订阅令牌
- `GET /api/subscription/:token` - 获取订阅内容（公开）
- `GET /s/:short_code` - 短链接访问订阅（公开）

### IP 限制
- `GET /api/admin/ip-restrictions/stats` - IP 限制统计
- `GET /api/admin/users/:id/online-ips` - 用户在线 IP
- `POST /api/admin/users/:id/kick-ip` - 踢出用户 IP
- `GET /api/admin/ip-whitelist` - 获取白名单
- `POST /api/admin/ip-whitelist` - 添加白名单
- `DELETE /api/admin/ip-whitelist/:id` - 删除白名单
- `GET /api/admin/ip-blacklist` - 获取黑名单
- `POST /api/admin/ip-blacklist` - 添加黑名单
- `DELETE /api/admin/ip-blacklist/:id` - 删除黑名单
- `GET /api/user/devices` - 用户设备列表
- `POST /api/user/devices/:ip/kick` - 用户踢出设备

### 商业化系统
- `GET /api/plans` - 获取套餐列表
- `GET /api/plans/:id` - 获取套餐详情
- `POST /api/orders` - 创建订单
- `GET /api/orders` - 获取订单列表
- `GET /api/orders/:id` - 获取订单详情
- `POST /api/orders/:id/cancel` - 取消订单
- `POST /api/payments/create` - 创建支付
- `POST /api/payments/callback/:gateway` - 支付回调
- `GET /api/balance` - 获取余额
- `GET /api/balance/transactions` - 交易记录
- `POST /api/coupons/validate` - 验证优惠券
- `GET /api/invite/code` - 获取邀请码
- `GET /api/invite/referrals` - 获取推荐列表
- `GET /api/commissions` - 获取佣金列表
- `GET /api/invoices` - 获取发票列表
- `GET /api/invoices/:id/download` - 下载发票
- `GET /api/trial` - 获取试用状态
- `POST /api/trial` - 激活试用
- `POST /api/plan-change/calculate` - 计算升降级价格
- `POST /api/plan-change/upgrade` - 升级套餐
- `POST /api/plan-change/downgrade` - 降级套餐
- `POST /api/subscription/pause` - 暂停订阅
- `POST /api/subscription/resume` - 恢复订阅
- `POST /api/gift-cards/redeem` - 兑换礼品卡

### 用户门户
- `POST /api/portal/auth/register` - 用户注册
- `POST /api/portal/auth/login` - 用户登录
- `POST /api/portal/auth/logout` - 用户登出
- `POST /api/portal/auth/forgot-password` - 忘记密码
- `POST /api/portal/auth/reset-password` - 重置密码
- `GET /api/portal/dashboard` - 仪表板数据
- `GET /api/portal/nodes` - 节点列表
- `GET /api/portal/nodes/:id/ping` - 节点延迟测试
- `GET /api/portal/tickets` - 工单列表
- `POST /api/portal/tickets` - 创建工单
- `GET /api/portal/tickets/:id` - 工单详情
- `POST /api/portal/tickets/:id/reply` - 回复工单
- `GET /api/portal/announcements` - 公告列表
- `GET /api/portal/announcements/:id` - 公告详情
- `POST /api/portal/announcements/:id/read` - 标记已读
- `GET /api/portal/stats` - 使用统计
- `GET /api/portal/help` - 帮助文章列表
- `GET /api/portal/help/:slug` - 帮助文章详情

## 贡献

欢迎贡献！请查看 [贡献指南](.github/CONTRIBUTING.md) 了解详情。

## 问题反馈

- 使用 [Bug Report](https://github.com/chengchnegcheng/Vpanel/issues/new?template=bug_report.yml) 报告 Bug
- 使用 [Feature Request](https://github.com/chengchnegcheng/Vpanel/issues/new?template=feature_request.yml) 提出新功能

## 特别鸣谢

- [Xray-core](https://github.com/XTLS/Xray-core) - 核心代理引擎
- [Vue.js](https://vuejs.org/) - 前端框架
- [Gin](https://gin-gonic.com/) - Web 框架
- [GORM](https://gorm.io/) - ORM 框架

## 许可证

[MIT License](LICENSE)
