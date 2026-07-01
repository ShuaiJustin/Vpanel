# 订阅链接地址配置指南

## 问题现象

订阅链接显示为 `http://localhost:8080/api/subscription/xxx` 或 `http://0.0.0.0:8080/api/subscription/xxx`，无法在外网访问。

## 解决方案

### 方法一：配置环境变量（推荐）

在启动 Panel 前设置环境变量：

```bash
export V_SERVER_BASE_URL="https://your-domain.com"
./bin/vpanel
```

或在 systemd service 文件中添加：

```ini
[Service]
Environment="V_SERVER_BASE_URL=https://your-domain.com"
ExecStart=/path/to/vpanel
```

### 方法二：配置 Nginx 反向代理

确保 Nginx 正确传递请求头：

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
    }
}
```

**关键配置：**
- `X-Forwarded-Proto` - 协议（http/https）
- `X-Forwarded-Host` - 域名

### 方法三：使用数据库配置

通过系统设置 API 配置（需要管理员权限）：

```bash
# 使用 PanelAPIDomain 字段
curl -X POST http://localhost:8080/api/admin/settings \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"panel_api_domain": "https://your-domain.com"}'
```

## 优先级

1. **环境变量 `V_SERVER_BASE_URL`** - 最高优先级
2. **数据库中的 `PanelAPIDomain`** 配置
3. **请求头** `X-Forwarded-Host` + `X-Forwarded-Proto`
4. **自动构建** - `http(s)://host:port`（最低优先级，可能不准确）

## 验证配置

访问订阅管理页面，检查订阅链接是否正确显示为你的域名。

正确示例：`https://your-domain.com/api/subscription/abc123`
错误示例：`http://localhost:8080/api/subscription/abc123`
