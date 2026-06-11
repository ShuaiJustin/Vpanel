# 生产部署检查清单

在部署到生产环境前，请确保完成以下检查项。

## ✅ 必需项

### 1. 编译构建
- [ ] 后端编译完成：`go build -o bin/vpanel cmd/v/main.go`
- [ ] Agent 编译完成：`go build -o bin/vpanel-agent cmd/agent/main.go`
- [ ] 前端构建完成：`cd web && npm run build`

### 2. 配置文件
- [ ] 复制配置模板：`cp deployments/docker/.env.example deployments/docker/.env`
- [ ] **必须配置** `V_SERVER_PUBLIC_URL` 为公网地址（用于 Agent 远程部署）
- [ ] **建议配置** `V_SERVER_BASE_URL` 为公网地址（用于订阅链接，否则可能显示 localhost）
- [ ] 配置 `V_JWT_SECRET`（建议使用 `openssl rand -base64 48` 生成）
- [ ] 配置 `V_ADMIN_PASS`（至少 12 字符，包含大小写字母、数字、特殊字符）
- [ ] 配置 `V_SERVER_CORS_ORIGINS`（生产环境必须设置，防止 CSRF 攻击）

### 3. 反向代理（使用 Nginx）
- [ ] 配置 Nginx（参考 `deployments/nginx/vpanel.conf.example`）
- [ ] **必须设置**以下请求头：
  ```nginx
  proxy_set_header Host $host;
  proxy_set_header X-Real-IP $remote_addr;
  proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
  proxy_set_header X-Forwarded-Proto $scheme;
  proxy_set_header X-Forwarded-Host $host;
  ```
- [ ] 配置 SSL 证书（推荐使用 Let's Encrypt）
- [ ] 启用 HSTS 头：`add_header Strict-Transport-Security "max-age=31536000"`

### 4. 安全设置
- [ ] 修改默认管理员密码
- [ ] 启用防火墙，只开放必要端口（80, 443）
- [ ] Panel 端口（默认 8080）不要直接暴露到公网
- [ ] 配置备份（默认启用，保留 14 天）
- [ ] 启用操作审计日志（系统设置中开启）

### 5. 数据持久化
- [ ] 确认数据目录挂载：`/app/data`（包含数据库）
- [ ] 确认配置目录挂载：`/app/configs`
- [ ] 确认日志目录挂载：`/app/logs`
- [ ] 确认 Xray 目录挂载：`/app/xray`

### 6. 资源限制
- [ ] 设置容器内存限制（建议至少 512MB）
- [ ] 设置容器 CPU 限制（建议至少 1 核）
- [ ] 检查磁盘空间（建议至少 10GB）

## 🔍 推荐项

### 1. 订阅链接配置
如果订阅链接显示为 `http://localhost:8080`，选择以下方法之一：

**方法 A - 环境变量（最简单）**
```bash
V_SERVER_BASE_URL=https://panel.example.com
```

**方法 B - Nginx 反向代理（推荐）**
```nginx
proxy_set_header X-Forwarded-Proto $scheme;
proxy_set_header X-Forwarded-Host $host;
```

详见：[订阅链接配置指南](./subscription-url-config.md)

### 2. 监控和日志
- [ ] 配置日志收集（建议使用 Loki 或 ELK）
- [ ] 设置资源监控（CPU、内存、磁盘）
- [ ] 配置告警通知（SMTP 或 Telegram Bot）

### 3. 备份策略
- [ ] 配置自动备份保留天数（默认 14 天）
- [ ] 定期测试备份恢复流程
- [ ] 异地备份数据库文件

### 4. 性能优化
- [ ] 启用 HTTP/2（Nginx 配置中已启用）
- [ ] 配置静态资源缓存（Nginx 配置中已启用）
- [ ] 根据节点数量调整数据库连接池

## 🚀 部署步骤

### Docker Compose（推荐）

```bash
# 1. 进入部署目录
cd /path/to/Vpanel

# 2. 配置环境变量
cp deployments/docker/.env.example deployments/docker/.env
nano deployments/docker/.env  # 修改必要配置

# 3. 启动服务
./deployments/scripts/start.sh start

# 4. 查看日志
./deployments/scripts/start.sh logs

# 5. 查看状态
./deployments/scripts/start.sh status
```

### 手动部署

```bash
# 1. 编译
go build -o bin/vpanel cmd/v/main.go
cd web && npm run build && cd ..

# 2. 配置
cp configs/config.yaml.example configs/config.yaml
nano configs/config.yaml  # 修改配置

# 3. 启动
./bin/vpanel

# 或使用 systemd
cp deployments/systemd/vpanel.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable vpanel
systemctl start vpanel
```

## 🧪 部署后验证

```bash
# 1. 检查服务状态
curl http://localhost:8080/health

# 2. 检查前端
curl http://localhost:8080/

# 3. 测试登录
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"YOUR_PASSWORD"}'

# 4. 检查订阅链接（登录后在用户订阅页面查看）
# 确认链接不是 localhost，而是你的公网域名
```

## 📋 常见问题

### 订阅链接显示 localhost
- 检查 `V_SERVER_BASE_URL` 是否配置
- 检查 Nginx 是否正确传递 `X-Forwarded-Host` 头
- 参考：[订阅链接配置指南](./subscription-url-config.md)

### Agent 无法连接 Panel
- 检查 `V_SERVER_PUBLIC_URL` 是否为公网地址
- 检查防火墙是否允许 Agent 端口访问
- 检查 Agent token 是否正确

### 数据库连接失败
- 检查 `/app/data` 目录权限
- 检查磁盘空间是否充足
- 查看日志：`./deployments/scripts/start.sh logs`

### 证书问题
- 检查证书文件路径是否正确
- 检查证书是否过期
- 使用 `openssl` 验证证书：`openssl x509 -in cert.pem -text -noout`

## 📚 相关文档

- [部署说明](../deployments/README.md)
- [订阅链接配置](./subscription-url-config.md)
- [Nginx 配置示例](../deployments/nginx/vpanel.conf.example)
- [环境变量说明](../deployments/docker/.env.example)

## 🆘 获取帮助

如遇到问题：
1. 查看日志：`/app/logs/` 或 `./deployments/scripts/start.sh logs`
2. 检查配置：`/app/configs/config.yaml` 或 `.env`
3. 提交 Issue：https://github.com/chengchnegcheng/Vpanel/issues
