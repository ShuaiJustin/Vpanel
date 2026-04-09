# Xray 配置生成功能实现总结

## 实现的功能

### 1. 自动配置生成
- ✅ Panel 根据节点分配的用户和代理自动生成 Xray 配置
- ✅ 支持多种协议：VLESS、VMess、Trojan、Shadowsocks
- ✅ 支持多种传输方式：TCP、WebSocket、HTTP/2、gRPC、QUIC
- ✅ 支持 TLS 加密配置
- ✅ 自动包含流量统计配置

### 2. 配置同步
- ✅ Agent 定期从 Panel 获取最新配置（默认 5 分钟）
- ✅ 配置变化时自动应用
- ✅ 应用前自动验证配置
- ✅ 应用前自动备份旧配置

### 3. API 端点

#### Agent 端点
```
GET /api/node/:id/config
Header: X-Node-Token: <token>
```
返回节点的完整 Xray 配置

#### 管理端点
```
GET /api/admin/nodes/:id/config/preview
Header: Authorization: Bearer <admin-token>
```
预览节点配置（用于测试和调试）

## 文件变更

### 新增文件

1. **internal/xray/config_generator.go**
   - Xray 配置生成器核心逻辑
   - 支持所有主流协议和传输方式
   - 自动生成 inbound、outbound、routing 配置

2. **internal/api/handlers/node_config_preview.go**
   - 管理员配置预览端点
   - 用于测试和调试配置生成

3. **configs/proxy-examples.json**
   - 代理配置示例
   - 包含各种协议的配置模板

4. **Docs/xray-config-guide.md**
   - 完整的配置指南
   - 包含使用示例和故障排查

### 修改文件

1. **internal/database/repository/repository.go**
   - 添加 `GetByNodeID` 方法到 ProxyRepository 接口

2. **internal/database/repository/proxy_repository.go**
   - 实现 `GetByNodeID` 方法
   - 通过 JOIN user_node_assignments 获取节点的所有代理

3. **internal/api/handlers/node_agent.go**
   - 更新 `GetConfig` 方法使用配置生成器
   - 添加配置生成和序列化逻辑

4. **internal/agent/panel_client.go**
   - 更新 `SyncConfig` 方法解析新的响应格式
   - 从响应中提取 config 字段

5. **internal/api/routes.go**
   - 初始化配置生成器
   - 添加配置预览路由

## 数据流程

```
1. 管理员创建节点
   ↓
2. 管理员分配用户到节点
   ↓
3. 用户创建代理配置
   ↓
4. Panel 生成 Xray 配置
   - 查询节点分配的用户
   - 获取用户的所有启用代理
   - 生成 inbound 配置
   - 生成 outbound 和 routing
   ↓
5. Agent 同步配置
   - 定期请求 /api/node/:id/config
   - 检查配置版本
   - 验证新配置
   - 备份旧配置
   - 应用新配置
   - 重启 Xray
```

## 配置生成逻辑

### Inbound 生成

每个启用的代理生成一个 inbound：

```go
{
  "tag": "inbound-{proxy_id}",
  "port": {proxy.port},
  "protocol": "{proxy.protocol}",
  "settings": {...},
  "streamSettings": {...},
  "sniffing": {
    "enabled": true,
    "destOverride": ["http", "tls"]
  }
}
```

### 协议特定设置

**VLESS:**
```json
{
  "clients": [{
    "id": "{uuid}",
    "email": "user-{user_id}-proxy-{proxy_id}",
    "level": 0
  }],
  "decryption": "none"
}
```

**VMess:**
```json
{
  "clients": [{
    "id": "{uuid}",
    "email": "user-{user_id}-proxy-{proxy_id}",
    "level": 0,
    "alterId": 0
  }]
}
```

**Trojan:**
```json
{
  "clients": [{
    "password": "{password}",
    "email": "user-{user_id}-proxy-{proxy_id}",
    "level": 0
  }]
}
```

**Shadowsocks:**
```json
{
  "method": "{method}",
  "password": "{password}",
  "network": "tcp,udp"
}
```

### 传输层设置

根据 `settings.network` 生成对应的传输配置：

- **tcp**: TCPSettings
- **ws**: WSSettings (path, headers)
- **http**: HTTPSettings
- **quic**: QUICSettings
- **grpc**: GRPCSettings

### TLS 设置

当 `settings.security = "tls"` 时：

```json
{
  "security": "tls",
  "tlsSettings": {
    "serverName": "{server_name}",
    "certificates": [{
      "certificateFile": "{cert_file}",
      "keyFile": "{key_file}"
    }],
    "alpn": ["h2", "http/1.1"]
  }
}
```

## 使用示例

### 1. 创建 VLESS 代理

```bash
POST /api/proxies
{
  "name": "VLESS-443",
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

### 2. 分配用户到节点

通过管理后台将用户分配到节点，系统会自动关联用户的所有代理。

### 3. 预览配置

```bash
curl -H "Authorization: Bearer <admin-token>" \
  https://panel.example.com/api/admin/nodes/1/config/preview
```

### 4. Agent 自动同步

Agent 会自动：
- 每 5 分钟检查配置更新
- 检测到变化时应用新配置
- 验证配置有效性
- 备份旧配置
- 重启 Xray

## 测试步骤

### 1. 编译并启动 Panel

```bash
go build -o vpanel ./cmd/v/main.go
./vpanel
```

### 2. 创建测试数据

```bash
# 创建节点
curl -X POST http://localhost:8080/api/admin/nodes \
  -H "Authorization: Bearer <token>" \
  -d '{"name":"Test-Node","address":"127.0.0.1","port":443}'

# 生成节点 token
curl -X POST http://localhost:8080/api/admin/nodes/1/token \
  -H "Authorization: Bearer <token>"

# 创建用户代理
curl -X POST http://localhost:8080/api/proxies \
  -H "Authorization: Bearer <token>" \
  -d '{
    "name": "Test-VLESS",
    "protocol": "vless",
    "port": 10443,
    "settings": {
      "uuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
      "network": "tcp"
    }
  }'
```

### 3. 预览配置

```bash
curl http://localhost:8080/api/admin/nodes/1/config/preview \
  -H "Authorization: Bearer <token>"
```

### 4. 部署 Agent

```bash
# 配置 agent
cat > /etc/vpanel/agent.yaml <<EOF
panel:
  url: "http://localhost:8080"
  token: "<node-token>"
xray:
  binary_path: "/usr/local/bin/xray"
  config_path: "/usr/local/etc/xray/config.json"
EOF

# 启动 agent
./agent
```

## 故障排查

### 配置生成失败

检查：
1. 用户是否分配到节点
2. 代理是否启用
3. 代理配置是否完整（uuid/password）
4. 端口是否冲突

### Agent 同步失败

检查：
1. 网络连接
2. Token 是否有效
3. Panel 日志
4. Agent 日志

### Xray 启动失败

检查：
1. 配置语法：`xray -test -config /etc/xray/config.json`
2. 端口占用：`netstat -tlnp | grep <port>`
3. 证书路径和权限
4. Xray 日志

## 下一步改进

1. **配置模板**: 支持自定义配置模板
2. **批量操作**: 批量更新节点配置
3. **配置版本**: 配置版本管理和回滚
4. **实时推送**: WebSocket 实时推送配置更新
5. **配置验证**: 更严格的配置验证规则
6. **性能优化**: 缓存配置减少数据库查询

## 相关文档

- [Xray 配置指南](./xray-config-guide.md)
- [Agent 部署指南](./NODE-AGENT-GUIDE.md)
- [配置示例](../configs/proxy-examples.json)
