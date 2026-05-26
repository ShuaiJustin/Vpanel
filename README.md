# V Panel

多协议代理面板：在一台机器上集中管理多个远程代理节点的下发、用户、流量、订阅、计费和 TLS 证书。

后端 Go + Gin，前端 Vue 3 + Element Plus，存储 SQLite，容器化部署。

---

## 功能

- **多协议节点**：VLESS / VMess / Trojan / Shadowsocks，含 Reality、XTLS、WebSocket、gRPC
- **远程节点管理**：一台主面板，SSH 下发 Xray 配置到多个出口节点
- **用户与流量**：用户隔离、流量统计、配额、到期、订阅链接（Clash / Shadowrocket / V2rayN 等）
- **商业化**：套餐、订单、优惠券、礼品卡、试用、邀请返佣、支付宝/微信支付
- **HTTPS 证书**：界面里申请 Let's Encrypt 证书（含泛域名，dns_cf / dns_dp / dns_ali 等），一键应用到面板自身
- **审计与监控**：操作日志、登录日志、节点心跳、流量曲线
- **IP 限制**：单个订阅最多并发的客户端 IP 上限

---

## 快速部署

要求：Docker 20+ 和 Docker Compose v2。

```bash
git clone https://github.com/chengchnegcheng/Vpanel.git
cd Vpanel
./vpanel.sh
```

`vpanel.sh` 是交互菜单：选 **"Docker 一键部署"**。首次运行会：

1. 复制 `deployments/docker/.env.example` 到 `deployments/docker/.env`
2. **自动生成随机管理员密码和 JWT Secret** 写入 `.env`
3. 随机分配宿主机端口（10000–65000，避免冲突）
4. 构建镜像并启动容器

启动完成后菜单会显示访问地址。

### 不想用菜单，纯命令行

```bash
cd deployments/docker
cp .env.example .env
# 编辑 .env：把 V_ADMIN_PASS 和 V_JWT_SECRET 改成真实值，VPANEL_PUBLISH_PORT 改成你想用的端口
docker compose up -d --build
```

---

## 首次登录

打开 `http://<your-host>:<端口>/`，账号默认 `admin`。

**密码不是 admin123**。密码来自这里，按顺序找：

1. 看 `deployments/docker/.env` 里的 `V_ADMIN_PASS` —— 菜单首次启动时随机生成的强密码
2. 容器首次启动日志里也会打印一次（之后启动不再打）：`docker logs vpanel-v-panel-1 | grep "admin user created"`

记得**首次登录后立刻改密码**（右上角头像 → 修改密码）。

---

## 启用 HTTPS

不需要改 `.env`，全在面板里操作：

1. **证书管理 → 申请证书** → 选 DNS 验证 → 选 DNS 提供商（如 Cloudflare `dns_cf`）→ 填 API 凭证 → 提交
2. 等 1–2 分钟，证书状态变 "有效"
3. **系统设置 → 服务器配置 → HTTPS/TLS** → 下拉选刚签的证书 → "应用并保存"
4. 顶部 **重启面板**

重启后浏览器访问 `https://...`，应用会自动把生成链接（订阅、邮件）也升级到 https。

---

## 常用操作

```bash
./vpanel.sh                                 # 进交互菜单
./deployments/scripts/start.sh start        # 启动
./deployments/scripts/start.sh stop         # 停止
./deployments/scripts/start.sh restart      # 重启
./deployments/scripts/start.sh logs         # 跟踪日志
./deployments/scripts/start.sh status       # 状态
docker exec -it vpanel-v-panel-1 sh         # 进容器
```

**升级**：

```bash
git pull
./vpanel.sh   # 菜单里选 "重新构建并启动"
# 或
cd deployments/docker && docker compose up -d --build
```

数据卷 (`v-panel-data` / `v-panel-config` / `v-panel-logs` / `v-panel-xray`) 不会被 rebuild 影响，升级是安全的。

**完整清理**（**会删数据**）：

```bash
cd deployments/docker
docker compose down -v   # -v 会删数据卷
```

---

## 配置（`deployments/docker/.env` 关键项）

| 变量 | 说明 | 默认 |
|---|---|---|
| `VPANEL_PUBLISH_PORT` | 宿主机映射端口 | 首次随机分配 |
| `V_ADMIN_USER` | 管理员用户名 | `admin` |
| `V_ADMIN_PASS` | 管理员密码 | 首次随机生成 |
| `V_JWT_SECRET` | JWT 签名密钥 | 首次随机生成 |
| `V_SERVER_PUBLIC_URL` | 对外完整 URL（订阅/邮件链接用） | 留空走 host:port |
| `V_SERVER_CORS_ORIGINS` | CORS 白名单 | 同上 |
| `V_LOG_LEVEL` | 日志级别 | `info` |
| `TZ` | 时区 | `UTC` |
| `VPANEL_BACKUP_ENABLED` | 自动备份 | `1` |
| `VPANEL_BACKUP_RETENTION_DAYS` | 备份保留天数 | `14` |

启用 HTTPS 后 `V_SERVER_PUBLIC_URL` / `V_SERVER_CORS_ORIGINS` 的 `http://` 在启动时会自动升级到 `https://`，无需手动改。

---

## 远程节点

主面板自己**不**跑 Xray —— 各个出口节点跑 Xray，由主面板通过 SSH 下发配置和 Node Agent 反向上报心跳。

部署方法看 [Docs/NODE-AGENT-GUIDE.md](Docs/NODE-AGENT-GUIDE.md)。

---

## 故障排查速查

| 现象 | 先查 |
|---|---|
| 容器 unhealthy | `docker logs vpanel-v-panel-1` 看启动错误 |
| 浏览器 "不安全"（HTTPS 启用后） | 证书 SAN 是否覆盖访问的域名（泛域名不匹配根域，需要同时签发） |
| acme.sh 申请超时 / `signal: killed` | 容器能否访问 DNS 提供商的 API（Cloudflare 等） |
| 证书申请后下拉里看不到 | 证书状态在"证书管理"里看——只有 "valid/expiring" 才会进 HTTPS 下拉 |
| 节点心跳一直 unhealthy | 节点的 panel URL 用对协议（启用 HTTPS 后必须 https://） |
| 改了 `.env` 不生效 | 必须 `docker compose up -d` 重建容器，重启容器不读 .env |

更详细见 [Docs/OPERATIONS-GUIDE.md](Docs/OPERATIONS-GUIDE.md)。

---

## 项目文档

| 文档 | 内容 |
|---|---|
| [Docs/QUICK-REFERENCE.md](Docs/QUICK-REFERENCE.md) | 常用命令速查 |
| [Docs/OPERATIONS-GUIDE.md](Docs/OPERATIONS-GUIDE.md) | 运维手册 |
| [Docs/NODE-AGENT-GUIDE.md](Docs/NODE-AGENT-GUIDE.md) | 节点 Agent 部署 |
| [Docs/certificate-guide.md](Docs/certificate-guide.md) | 证书申请详细流程 |
| [Docs/remote-deploy-guide.md](Docs/remote-deploy-guide.md) | 远程节点部署 |

---

## 开发

```bash
# 本地后端
go run ./cmd/v

# 前端 dev server（另一终端）
cd web && npm install && npm run dev

# 完整测试
go test ./...
cd web && npm test
```

提交前跑 `go vet ./... && go test ./...`，前端 `npm run lint`。

---

## License

[MIT](LICENSE)
