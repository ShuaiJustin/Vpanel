# Xray 配置生成指南

## 概述

V Panel 现在支持自动为节点生成 Xray 配置。系统会根据分配给节点的用户及其代理配置，自动生成完整的 Xray 配置文件。

## 工作原理

1. **用户-节点分配**: 管理员将用户分配到特定节点
2. **代理配置**: 用户创建代理配置（VLESS、VMess、Trojan、Shadowsocks）
3. **自动生成**: Panel 根据节点的用户和代理自动生成 Xray 配置
4. **自动同步**: Agent 定期从 Panel 获取最新配置并应用

## 架构流程

```
用户创建代理 → 管理员分配用户到节点 → Panel 生成 Xray 配置 → Agent 同步配置 → Xray 应用配置
```

## 代理配置示例

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
    "key_file": "/etc/ssl/private/key.pem",
    "alpn": ["h2", "http/1.1"]
  }
}
```

### VLESS + WebSocket + TLS

```json
{
  "protocol": "vless",
  "port": 443,
  "settings": {
    "uuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "network": "ws",
    "security": "tls",
    "server_name": "example.com",
    "ws_settings": {
      "path": "/ws",
      "headers": {
        "Host": "example.com"
      }
    }
  }
}
```

### VMess

```json
{
  "protocol": "vmess",
  "port": 10086,
  "settings": {
    "uuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "alter_id": 0,
    "network": "tcp",
    "security": "none"
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
    "server_name": "example.com",
    "cert_file": "/etc/ssl/certs/cert.pem",
    "key_file": "/etc/ssl/private/key.pem"
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

## API 端点

### 1. 节点获取配置（Agent 使用）

```
GET /api/node/:id/config
Header: X-Node-Token: <node-token>
```

响应：
```json
{
  "success": true,
  "node_id": 1,
  "version": "1.0",
  "timestamp": 1234567890,
  "config": "{...xray config json...}"
}
```

### 2. 预览节点配置（管理员测试）

```
GET /api/admin/nodes/:id/config/preview
Header: Authorization: Bearer <admin-token>
```

响应：
```json
{
  "success": true,
  "node_id": 1,
  "inbound_count": 5,
  "config": "{...xray config json...}"
}
```

## 使用步骤

### 1. 创建节点

```bash
# 在管理后台创建节点
POST /api/admin/nodes
{
  "name": "Node-1",
  "address": "node1.example.com",
  "port": 443,
  "enabled": true
}
```

### 2. 生成节点 Token

```bash
# 为节点生成认证 token
POST /api/admin/nodes/:id/token
```

### 3. 分配用户到节点

```bash
# 将用户分配到节点（通过管理后台）
# 系统会自动关联用户的所有代理到该节点
```

### 4. 创建代理配置

用户在后台创建代理配置，指定：
- 协议类型（VLESS/VMess/Trojan/Shadowsocks）
- 端口
- UUID/密码
- 传输方式（TCP/WebSocket/gRPC等）
- TLS 设置

### 5. 部署 Agent

在节点服务器上：

```bash
# 配置 agent
cat > /etc/vpanel/agent.yaml <<EOF
panel:
  url: "https://panel.example.com"
  token: "<node-token>"
  
xray:
  binary_path: "/usr/local/bin/xray"
  config_path: "/etc/xray/config.json"
  
sync:
  interval: 5m
  validate_before_apply: true
  backup_before_apply: true
EOF

# 启动 agent
systemctl start vpanel-agent
```

### 6. 验证配置

```bash
# 查看 agent 日志
journalctl -u vpanel-agent -f

# 测试配置生成（管理员）
curl -H "Authorization: Bearer <admin-token>" \
  https://panel.example.com/api/admin/nodes/1/config/preview
```

## 配置同步流程

1. **Agent 启动**: 连接到 Panel 并注册
2. **初始同步**: 获取初始配置并应用
3. **定期同步**: 每 5 分钟检查配置更新
4. **心跳**: 每 30 秒发送心跳，报告状态
5. **自动应用**: 检测到配置变化时自动应用

## 配置字段说明

### 通用字段

- `protocol`: 协议类型（vless/vmess/trojan/shadowsocks）
- `port`: 监听端口
- `enabled`: 是否启用

### VLESS/VMess 特有

- `uuid`: 用户 UUID（使用 `uuidgen` 生成）
- `alter_id`: VMess 的 alterId（推荐 0）

### Trojan/Shadowsocks 特有

- `password`: 密码
- `method`: Shadowsocks 加密方法

### 传输层设置

- `network`: 传输协议（tcp/ws/http/quic/grpc）
- `security`: 安全层（none/tls）
- `server_name`: TLS 服务器名称
- `cert_file`: TLS 证书文件路径
- `key_file`: TLS 密钥文件路径

### WebSocket 设置

```json
"ws_settings": {
  "path": "/ws",
  "headers": {
    "Host": "example.com"
  }
}
```

### gRPC 设置

```json
"grpc_settings": {
  "serviceName": "GunService"
}
```

## 故障排查

### 配置未同步

1. 检查 agent 日志：`journalctl -u vpanel-agent -f`
2. 验证 token 是否有效
3. 检查网络连接
4. 手动触发同步：重启 agent

### Xray 启动失败

1. 验证配置：`xray -test -config /etc/xray/config.json`
2. 检查端口冲突：`netstat -tlnp | grep <port>`
3. 检查证书路径和权限
4. 查看 Xray 日志

### 端口冲突

确保每个代理使用不同的端口，或者使用不同的监听地址。

## 安全建议

1. **使用 TLS**: 生产环境必须使用 TLS
2. **强密码**: Trojan 和 Shadowsocks 使用强随机密码
3. **证书管理**: 使用 Let's Encrypt 自动更新证书
4. **防火墙**: 只开放必要的端口
5. **Token 安全**: 妥善保管节点 token

## 高级功能

### 自定义路由规则

系统默认包含：
- API 路由（用于统计）
- 直连出站
- 阻断出站（BT 流量）

可以通过修改配置生成器添加自定义规则。

### 流量统计

所有代理自动启用流量统计，可通过 API 查询：
- 用户流量
- 节点流量
- 协议流量

### 节点月流量保护

节点除用户套餐流量外，还可以配置节点自身的月流量保护：
- `traffic_limit`：节点月总流量上限
- `alert_traffic_threshold`：停止新分配阈值（百分比）
- `traffic_reset_at`：当前月周期锚点

处理逻辑如下：
1. 节点流量达到阈值前：正常分配新用户
2. 达到阈值后：老用户仍可继续使用，但不再给新用户分配到该节点
3. 在仍可分配的新节点中：系统优先挑选流量压力更低、容量压力更低的节点
4. 达到硬上限后：节点自动停用，订阅和直连节点均不可继续使用
5. 首次跨过告警阈值或硬上限时：系统自动向管理员发送节点流量告警
6. 到达下个月周期后：系统自动重置节点累计流量

推荐配置：
- 节点商月流量为 `1TB` 时，可将 `traffic_limit` 设置为 `900-950 GiB`
- `alert_traffic_threshold` 建议设置为 `80%`
- 管理后台可通过“流量状态”筛选和“节点流量风险工作台”优先处理高风险节点
- 这样可以给月底突发流量、同步延迟和统计误差留出缓冲

### 多节点负载均衡

通过节点组功能实现：
1. 创建节点组
2. 添加多个节点到组
3. 系统自动分配用户到不同节点

## 参考资料

- [Xray 官方文档](https://xtls.github.io/)
- [配置示例](../configs/proxy-examples.json)
- [Agent 部署指南](./NODE-AGENT-GUIDE.md)
