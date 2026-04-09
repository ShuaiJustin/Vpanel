# Xray 配置快速开始

## 5 分钟快速部署

### 1. 启动 Panel (1 分钟)

```bash
# 编译（如果还没编译）
go build -o vpanel ./cmd/v/main.go

# 启动
./vpanel
```

### 2. 创建节点 (1 分钟)

```bash
# 登录管理后台
# http://localhost:8080

# 创建节点
POST /api/admin/nodes
{
  "name": "Node-1",
  "address": "node1.example.com",
  "port": 443,
  "enabled": true
}

# 生成节点 Token
POST /api/admin/nodes/1/token
# 保存返回的 token
```

### 3. 创建代理 (1 分钟)

```bash
# 创建 VLESS 代理
POST /api/proxies
{
  "name": "VLESS-443",
  "protocol": "vless",
  "port": 443,
  "settings": {
    "uuid": "$(uuidgen)",
    "network": "tcp"
  }
}
```

### 4. 分配用户到节点 (30 秒)

在管理后台：
1. 进入"节点管理"
2. 选择节点
3. 点击"分配用户"
4. 选择用户并保存

### 5. 预览配置 (30 秒)

```bash
# 查看生成的配置
curl http://localhost:8080/api/admin/nodes/1/config/preview \
  -H "Authorization: Bearer <your-admin-token>"
```

## 部署 Agent

### 方法 1: 使用安装脚本

```bash
# 下载安装脚本
curl -O https://your-panel.com/scripts/install-agent.sh

# 运行安装
sudo bash install-agent.sh \
  --panel-url "https://panel.example.com" \
  --token "<node-token>"
```

### 方法 2: 手动部署

```bash
# 1. 安装 Xray
bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install

# 2. 配置 Agent
cat > /etc/vpanel/agent.yaml <<EOF
panel:
  url: "https://panel.example.com"
  token: "<node-token>"
  
xray:
  binary_path: "/usr/local/bin/xray"
  config_path: "/usr/local/etc/xray/config.json"
  
sync:
  interval: 5m
EOF

# 3. 启动 Agent
systemctl start vpanel-agent
systemctl enable vpanel-agent

# 4. 查看日志
journalctl -u vpanel-agent -f
```

## 常见代理配置

### VLESS + TLS

```json
{
  "protocol": "vless",
  "port": 443,
  "settings": {
    "uuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "network": "tcp",
    "security": "tls",
    "server_name": "example.com",
    "cert_file": "/etc/ssl/certs/cert.pem",
    "key_file": "/etc/ssl/private/key.pem"
  }
}
```

### VMess + WebSocket

```json
{
  "protocol": "vmess",
  "port": 443,
  "settings": {
    "uuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "alter_id": 0,
    "network": "ws",
    "security": "tls",
    "ws_settings": {
      "path": "/vmess"
    }
  }
}
```

### Trojan

```json
{
  "protocol": "trojan",
  "port": 443,
  "settings": {
    "password": "your-strong-password",
    "network": "tcp",
    "security": "tls",
    "server_name": "example.com"
  }
}
```

### Shadowsocks

```json
{
  "protocol": "shadowsocks",
  "port": 8388,
  "settings": {
    "method": "aes-256-gcm",
    "password": "your-strong-password"
  }
}
```

## 验证部署

### 1. 检查 Panel 日志

```bash
# 查看配置生成日志
tail -f logs/vpanel.log | grep "config"
```

### 2. 检查 Agent 状态

```bash
# Agent 状态
systemctl status vpanel-agent

# Agent 日志
journalctl -u vpanel-agent -n 50
```

### 3. 检查 Xray 状态

```bash
# Xray 状态
systemctl status xray

# 测试配置
xray -test -config /etc/xray/config.json

# 查看监听端口
netstat -tlnp | grep xray
```

### 4. 测试连接

```bash
# 使用客户端连接测试
# 或使用 curl 测试 HTTP 代理
curl -x socks5://127.0.0.1:1080 https://www.google.com
```

## 故障排查

### 配置未生成

**问题**: 预览配置返回空

**解决**:
1. 检查用户是否分配到节点
2. 检查代理是否启用
3. 检查代理配置是否完整

```bash
# 查看节点分配
GET /api/admin/nodes/1

# 查看用户代理
GET /api/proxies
```

### Agent 无法连接

**问题**: Agent 日志显示连接失败

**解决**:
1. 检查 Panel URL 是否正确
2. 检查 Token 是否有效
3. 检查防火墙规则

```bash
# 测试连接
curl https://panel.example.com/health

# 验证 Token
curl -H "X-Node-Token: <token>" \
  https://panel.example.com/api/node/1/config
```

### Xray 启动失败

**问题**: Xray 无法启动

**解决**:
1. 验证配置语法
2. 检查端口占用
3. 检查证书路径

```bash
# 验证配置
xray -test -config /etc/xray/config.json

# 检查端口
netstat -tlnp | grep 443

# 检查证书
ls -la /etc/ssl/certs/cert.pem
```

### 端口冲突

**问题**: 端口已被占用

**解决**:
1. 修改代理端口
2. 停止占用端口的服务

```bash
# 查找占用端口的进程
lsof -i :443

# 修改代理配置
PUT /api/proxies/1
{
  "port": 10443
}
```

## 生产环境建议

### 1. 使用 TLS

```bash
# 安装 certbot
apt install certbot

# 获取证书
certbot certonly --standalone -d example.com

# 配置代理使用证书
{
  "security": "tls",
  "cert_file": "/etc/letsencrypt/live/example.com/fullchain.pem",
  "key_file": "/etc/letsencrypt/live/example.com/privkey.pem"
}
```

### 2. 配置防火墙

```bash
# 开放必要端口
ufw allow 443/tcp
ufw allow 80/tcp
ufw enable
```

### 3. 启用自动更新

```bash
# 配置 certbot 自动续期
certbot renew --dry-run

# 添加 cron 任务
0 0 * * * certbot renew --quiet
```

### 4. 监控和告警

```bash
# 监控 Agent 状态
systemctl status vpanel-agent

# 监控 Xray 状态
systemctl status xray

# 查看流量统计
GET /api/admin/nodes/1/traffic
```

## 下一步

1. 📖 阅读[完整配置指南](./xray-config-guide.md)
2. 🔧 查看[实现文档](./xray-config-implementation.md)
3. 📝 参考[配置示例](../configs/proxy-examples.json)
4. 🚀 部署[多节点集群](./NODE-AGENT-GUIDE.md)

## 获取帮助

- 查看日志: `journalctl -u vpanel-agent -f`
- 测试配置: `xray -test -config /etc/xray/config.json`
- 预览配置: `GET /api/admin/nodes/:id/config/preview`
- 查看文档: `Docs/` 目录
