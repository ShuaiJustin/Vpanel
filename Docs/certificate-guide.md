# 证书申请配置指南

本指南介绍如何使用系统的证书申请功能，支持 Let's Encrypt 和 ZeroSSL 两种证书提供商。

## 目录

- [前置要求](#前置要求)
- [HTTP 验证方式](#http-验证方式)
- [DNS 验证方式](#dns-验证方式)
- [常见 DNS 提供商配置](#常见-dns-提供商配置)
- [故障排查](#故障排查)

---

## 前置要求

### 1. 安装 acme.sh

系统使用 `acme.sh` 作为 ACME 客户端。

- 默认情况下，系统会在**首次申请证书时自动安装** `acme.sh`
- 如果服务器无法访问 `https://get.acme.sh`，建议提前手动安装
- 首次申请通常会比后续申请多等待约 1~2 分钟

如需手动安装，可执行：

```bash
curl https://get.acme.sh | sh
```

安装完成后，重新加载 shell 配置：

```bash
source ~/.bashrc
# 或
source ~/.zshrc
```

### 2. 域名要求

- 域名必须是有效的格式（如 `example.com` 或 `*.example.com`）
- 对于 HTTP 验证，域名必须解析到本服务器
- 对于 DNS 验证，需要有 DNS 提供商的 API 访问权限

---

## HTTP 验证方式

HTTP 验证适用于可以直接访问的域名，Let's Encrypt 会通过 HTTP 访问你的域名来验证所有权。

### 配置要求

1. **端口 80 必须开放**
2. **域名已解析到服务器 IP**
3. **Webroot 目录存在且可写**

### 申请示例

```go
req := &certificate.ApplyRequest{
    Domain:   "example.com",
    Email:    "admin@example.com",
    Provider: "letsencrypt",
    Method:   "http",
    Webroot:  "/var/www/html",  // 网站根目录
}

cert, err := certService.Apply(ctx, req)
```

### Webroot 配置

常见的 webroot 路径：

- Nginx: `/var/www/html` 或 `/usr/share/nginx/html`
- Apache: `/var/www/html`
- 自定义: 确保目录存在且 acme.sh 有写入权限

---

## DNS 验证方式

DNS 验证通过在 DNS 记录中添加 TXT 记录来验证域名所有权，适用于：

- 无法开放 80 端口的服务器
- 申请通配符证书（`*.example.com`）
- 内网服务器

### 配置要求

1. **DNS 提供商支持 API**
2. **获取 API 凭证**
3. **配置 DNS API 环境变量**

---

## 常见 DNS 提供商配置

### Cloudflare（推荐）

Cloudflare 提供三种认证方式，推荐使用 API Token。

#### 方法 1: API Token（推荐）

**步骤：**

1. 登录 Cloudflare Dashboard
2. 进入 "My Profile" → "API Tokens"
3. 点击 "Create Token"
4. 选择 "Edit zone DNS" 模板
5. 配置权限：
   - Permissions: `Zone` → `DNS` → `Edit`
   - Zone Resources: 选择你的域名
6. 创建并复制 Token

**获取 Account ID：**

1. 进入域名的 "Overview" 页面
2. 右侧 "API" 部分可以看到 "Account ID"

**申请证书：**

```go
req := &certificate.ApplyRequest{
    Domain:      "example.com",
    Email:       "admin@example.com",
    Provider:    "letsencrypt",
    Method:      "dns",
    DNSProvider: "dns_cf",
    DNSEnv: map[string]string{
        "CF_Token":      "your-api-token-here",
        "CF_Account_ID": "your-account-id-here",
    },
}

cert, err := certService.Apply(ctx, req)
```

#### 方法 2: Global API Key（不推荐）

**获取 Global API Key：**

1. 登录 Cloudflare Dashboard
2. 进入 "My Profile" → "API Tokens"
3. 在 "API Keys" 部分，点击 "Global API Key" 的 "View"
4. 输入密码后复制 Key

**申请证书：**

```go
req := &certificate.ApplyRequest{
    Domain:      "example.com",
    Email:       "admin@example.com",
    Provider:    "letsencrypt",
    Method:      "dns",
    DNSProvider: "dns_cf",
    DNSEnv: map[string]string{
        "CF_Key":   "your-global-api-key",
        "CF_Email": "your-cloudflare-email",
    },
}
```

⚠️ **安全提示**: Global API Key 拥有账户的完全权限，泄露后果严重，建议使用 API Token。

---

### 阿里云 DNS

**获取 API 凭证：**

1. 登录阿里云控制台
2. 进入 "访问控制" → "用户" → "创建用户"
3. 勾选 "OpenAPI 调用访问"
4. 创建后获取 AccessKey ID 和 AccessKey Secret
5. 授予 DNS 管理权限

**申请证书：**

```go
req := &certificate.ApplyRequest{
    Domain:      "example.com",
    Email:       "admin@example.com",
    Provider:    "letsencrypt",
    Method:      "dns",
    DNSProvider: "dns_ali",
    DNSEnv: map[string]string{
        "Ali_Key":    "your-access-key-id",
        "Ali_Secret": "your-access-key-secret",
    },
}
```

---

### DNSPod

**获取 API 凭证：**

1. 登录 DNSPod 控制台
2. 进入 "用户中心" → "安全设置" → "API Token"
3. 创建 Token 并记录 ID 和 Token

**申请证书：**

```go
req := &certificate.ApplyRequest{
    Domain:      "example.com",
    Email:       "admin@example.com",
    Provider:    "letsencrypt",
    Method:      "dns",
    DNSProvider: "dns_dp",
    DNSEnv: map[string]string{
        "DP_Id":  "your-token-id",
        "DP_Key": "your-token-key",
    },
}
```

---

### AWS Route53

**获取 API 凭证：**

1. 登录 AWS Console
2. 进入 IAM → Users → 创建用户
3. 附加策略: `AmazonRoute53FullAccess`
4. 创建访问密钥

**申请证书：**

```go
req := &certificate.ApplyRequest{
    Domain:      "example.com",
    Email:       "admin@example.com",
    Provider:    "letsencrypt",
    Method:      "dns",
    DNSProvider: "dns_aws",
    DNSEnv: map[string]string{
        "AWS_ACCESS_KEY_ID":     "your-access-key-id",
        "AWS_SECRET_ACCESS_KEY": "your-secret-access-key",
    },
}
```

---

### 腾讯云 DNS

**获取 API 凭证：**

1. 登录腾讯云控制台
2. 进入 "访问管理" → "API 密钥管理"
3. 创建密钥并记录 SecretId 和 SecretKey

**申请证书：**

```go
req := &certificate.ApplyRequest{
    Domain:      "example.com",
    Email:       "admin@example.com",
    Provider:    "letsencrypt",
    Method:      "dns",
    DNSProvider: "dns_tencent",
    DNSEnv: map[string]string{
        "Tencent_SecretId":  "your-secret-id",
        "Tencent_SecretKey": "your-secret-key",
    },
}
```

---

## 通配符证书

通配符证书（`*.example.com`）只能使用 DNS 验证方式申请。

**示例：**

```go
req := &certificate.ApplyRequest{
    Domain:      "*.example.com",
    Email:       "admin@example.com",
    Provider:    "letsencrypt",
    Method:      "dns",
    DNSProvider: "dns_cf",
    DNSEnv: map[string]string{
        "CF_Token":      "your-api-token",
        "CF_Account_ID": "your-account-id",
    },
}
```

⚠️ **注意**: 通配符证书不包含根域名，如需同时支持 `example.com` 和 `*.example.com`，需要申请多域名证书。

---

## 证书提供商选择

### Let's Encrypt（默认）

- **免费**
- 证书有效期 90 天
- 支持自动续期
- 频率限制：每周每域名最多 50 个证书

```go
Provider: "letsencrypt"
```

### ZeroSSL

- **免费**
- 证书有效期 90 天
- 需要 Email 验证
- 部分功能需要 EAB 凭证

```go
Provider: "zerossl"
```

---

## 故障排查

### 1. 域名解析失败

**错误信息**: `域名解析失败` 或 `DNS problem`

**解决方法**:
- 检查域名是否正确解析到服务器 IP
- 使用 `nslookup example.com` 或 `dig example.com` 验证
- 等待 DNS 传播（可能需要几分钟到几小时）

### 2. 端口无法访问

**错误信息**: `Connection refused` 或 `连接被拒绝`

**解决方法**:
- 确保端口 80 已开放（HTTP 验证）
- 检查防火墙规则: `sudo ufw status`
- 检查 Nginx/Apache 是否运行

### 3. Webroot 目录不存在

**错误信息**: `webroot 目录不存在`

**解决方法**:
- 创建目录: `sudo mkdir -p /var/www/html`
- 设置权限: `sudo chmod 755 /var/www/html`
- 确保 acme.sh 有写入权限

### 4. DNS API 认证失败

**错误信息**: `Invalid response` 或 `Authentication failed`

**解决方法**:
- 检查 API 凭证是否正确
- 确认 API Token 有足够的权限
- 检查 IP 白名单限制（如果有）
- 验证 Account ID 或 Zone ID 是否正确

### 5. 频率限制

**错误信息**: `too many certificates` 或 `rate limit`

**解决方法**:
- Let's Encrypt 限制：每周每域名最多 50 个证书
- 等待一周后重试
- 使用测试环境验证配置（系统会自动先测试）

### 6. 测试申请失败

**错误信息**: `测试申请失败`

**解决方法**:
- 系统会先用测试服务器验证配置
- 检查上述所有配置项
- 查看详细错误日志
- 修复问题后重新申请

---

## 最佳实践

### 1. 使用 DNS 验证

- 更安全（不需要开放端口 80）
- 支持通配符证书
- 适用于内网服务器

### 2. 限制 API 权限

- 使用最小权限原则
- Cloudflare 使用 API Token 而非 Global Key
- 定期轮换 API 凭证

### 3. 启用自动续期

- 证书默认启用自动续期
- 系统会在证书过期前 30 天自动续期
- 续期成功后自动部署到关联节点

### 4. 监控证书状态

- 定期检查证书列表
- 关注过期时间
- 查看续期日志

### 5. 备份证书

- 证书存储在 `/path/to/certs/{domain}/` 目录
- 定期备份证书和私钥
- 私钥权限为 0600（仅所有者可读写）

---

## API 调用示例

### HTTP 验证

```bash
curl -X POST http://your-server/api/certificates/apply \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-token" \
  -d '{
    "domain": "example.com",
    "email": "admin@example.com",
    "provider": "letsencrypt",
    "method": "http",
    "webroot": "/var/www/html"
  }'
```

### DNS 验证（Cloudflare）

```bash
curl -X POST http://your-server/api/certificates/apply \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-token" \
  -d '{
    "domain": "example.com",
    "email": "admin@example.com",
    "provider": "letsencrypt",
    "method": "dns",
    "dns_provider": "dns_cf",
    "dns_env": {
      "CF_Token": "your-api-token",
      "CF_Account_ID": "your-account-id"
    }
  }'
```

---

## 支持的 DNS 提供商

系统支持 170+ DNS 提供商，常用的包括：

| 提供商 | dns_provider | 所需环境变量 |
|--------|--------------|--------------|
| Cloudflare | `dns_cf` | `CF_Token`, `CF_Account_ID` |
| 阿里云 | `dns_ali` | `Ali_Key`, `Ali_Secret` |
| 腾讯云 | `dns_tencent` | `Tencent_SecretId`, `Tencent_SecretKey` |
| DNSPod | `dns_dp` | `DP_Id`, `DP_Key` |
| AWS Route53 | `dns_aws` | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` |
| Google Cloud | `dns_gcloud` | 使用 gcloud 认证 |
| Azure DNS | `dns_azure` | `AZUREDNS_SUBSCRIPTIONID`, `AZUREDNS_TENANTID` 等 |
| DigitalOcean | `dns_dgon` | `DO_API_KEY` |
| Linode | `dns_linode_v4` | `LINODE_V4_API_KEY` |
| Vultr | `dns_vultr` | `VULTR_API_KEY` |

完整列表请参考: https://github.com/acmesh-official/acme.sh/wiki/dnsapi

---

## 常见问题

### Q: 证书申请需要多长时间？

A: 通常 1-5 分钟。DNS 验证可能需要等待 DNS 记录传播（2-15 分钟）。

### Q: 可以申请多个域名的证书吗？

A: 可以，在申请时指定多个域名即可。

### Q: 证书会自动续期吗？

A: 是的，系统会在证书过期前 30 天自动续期。

### Q: 如何申请通配符证书？

A: 使用 DNS 验证方式，域名填写 `*.example.com`。

### Q: 测试申请失败怎么办？

A: 系统会先用测试服务器验证配置，失败后不会继续正式申请。检查错误信息并修复配置。

### Q: 私钥文件在哪里？

A: 证书和私钥存储在 `{certDir}/{domain}/` 目录下，私钥文件为 `privkey.pem`。

---

## 技术支持

如遇到问题，请提供以下信息：

1. 域名
2. 验证方式（HTTP/DNS）
3. DNS 提供商（如使用 DNS 验证）
4. 错误信息
5. 系统日志

---

## 参考资料

- [Let's Encrypt 官方文档](https://letsencrypt.org/docs/)
- [acme.sh 官方文档](https://github.com/acmesh-official/acme.sh)
- [acme.sh DNS API 文档](https://github.com/acmesh-official/acme.sh/wiki/dnsapi)
- [Cloudflare API 文档](https://developers.cloudflare.com/api/)
