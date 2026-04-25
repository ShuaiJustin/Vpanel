# 远程自动部署 Agent 指南

## 功能概述

V Panel 现在支持通过 SSH 远程自动部署 Agent 到节点服务器，无需手动登录服务器执行命令。

## 功能特性

✅ **一键部署** - 输入 IP、用户名、密码即可自动部署
✅ **自动安装 Xray** - 自动下载并安装 Xray
✅ **自动配置** - 自动生成配置文件和 systemd 服务
✅ **连接测试** - 部署前可测试 SSH 连接
✅ **部署日志** - 实时查看部署过程和日志
✅ **手动脚本** - 也可下载脚本手动执行

## 使用方法

### 方法 1: 通过管理后台一键部署

1. **创建节点**
   ```
   进入"节点管理" → 点击"添加节点"
   填写节点信息（名称、地址、端口）
   ```

2. **远程部署**
   ```
   在节点列表中，点击"远程部署"按钮
   填写 SSH 连接信息：
   - 服务器 IP
   - SSH 端口（默认 22）
   - 用户名（建议 root）
   - 密码或私钥
   ```

3. **开始部署**
   ```
   点击"测试连接"验证 SSH 连接
   点击"开始部署"执行自动部署
   查看部署日志和进度
   ```

4. **验证部署**
   ```
   部署完成后，节点状态会自动变为"在线"
   可以在节点详情中查看 Agent 状态
   ```

### 方法 2: 下载脚本手动部署

1. **获取部署脚本**
   ```bash
   # 在节点管理页面，点击"下载部署脚本"
   # 或通过 API 获取
   curl -H "Authorization: Bearer <token>" \
     https://panel.example.com/api/admin/nodes/1/deploy/script \
     -o install-agent.sh
   ```

2. **上传并执行**
   ```bash
   # 上传到目标服务器
   scp install-agent.sh root@node-server:/root/
   
   # 登录服务器执行
   ssh root@node-server
   chmod +x install-agent.sh
   ./install-agent.sh
   ```

## API 接口

### 1. 远程部署 Agent

```
POST /api/admin/nodes/:id/deploy
```

请求体：
```json
{
  "host": "192.168.1.100",
  "port": 22,
  "username": "root",
  "password": "your-password"
}
```

或使用私钥：
```json
{
  "host": "192.168.1.100",
  "port": 22,
  "username": "root",
  "private_key": "-----BEGIN RSA PRIVATE KEY-----\n..."
}
```

响应：
```json
{
  "success": true,
  "message": "Agent 部署成功",
  "steps": [
    "连接到远程服务器...",
    "检查系统要求...",
    "安装依赖...",
    "下载并安装 Agent...",
    "安装 Xray...",
    "配置 Agent...",
    "启动 Agent 服务...",
    "验证安装..."
  ],
  "logs": "✓ SSH 连接成功\n✓ 系统检查完成\n..."
}
```

### 2. 测试 SSH 连接

```
POST /api/admin/nodes/test-connection
```

请求体：
```json
{
  "host": "192.168.1.100",
  "port": 22,
  "username": "root",
  "password": "your-password"
}
```

响应：
```json
{
  "success": true,
  "message": "Connection successful"
}
```

### 3. 获取部署脚本

```
GET /api/admin/nodes/:id/deploy/script
```

响应：Shell 脚本文件

## 部署流程

```
1. SSH 连接
   ↓
2. 检查系统要求
   - 操作系统信息
   - 内存和磁盘空间
   ↓
3. 安装依赖
   - curl, wget, unzip
   - systemctl
   ↓
4. 创建目录
   - /etc/vpanel
   - /var/log/vpanel
   ↓
5. 安装 Xray
   - 使用官方安装脚本
   - 验证安装
   ↓
6. 配置 Agent
   - 生成配置文件
   - 创建 systemd 服务
   ↓
7. 启动服务
   - 启用自动启动
   - 启动 Agent
   ↓
8. 验证安装
   - 检查服务状态
   - 验证配置文件
```

## 系统要求

### 目标服务器

- **操作系统**: Linux (Ubuntu, Debian, CentOS, RHEL)
- **内存**: 最少 512MB
- **磁盘**: 最少 1GB 可用空间
- **网络**: 能访问 Panel 服务器；Xray 下载默认会依次尝试 GitHub 和内置镜像
- **权限**: root 或 sudo 权限

### SSH 要求

- SSH 服务已启动
- 端口 22 或自定义端口开放
- 支持密码或密钥认证
- 防火墙允许 SSH 连接

## 安全建议

### 1. 使用密钥认证

```bash
# 生成 SSH 密钥对
ssh-keygen -t rsa -b 4096 -C "vpanel-deploy"

# 复制公钥到目标服务器
ssh-copy-id root@node-server

# 在部署时使用私钥
```

### 2. 限制 SSH 访问

```bash
# 只允许特定 IP 访问 SSH
# /etc/ssh/sshd_config
AllowUsers root@panel-server-ip
```

### 3. 使用临时密码

```bash
# 部署完成后立即修改密码
passwd root
```

### 4. 防火墙配置

```bash
# 只开放必要端口
ufw allow 22/tcp    # SSH
ufw allow 443/tcp   # HTTPS
ufw enable
```

## 故障排查

### SSH 连接失败

**问题**: Connection failed: dial tcp: i/o timeout

**解决**:
1. 检查服务器 IP 是否正确
2. 检查 SSH 端口是否正确
3. 检查防火墙规则
4. 验证网络连通性

```bash
# 测试连接
ping node-server
telnet node-server 22
```

### 认证失败

**问题**: Connection failed: ssh: unable to authenticate

**解决**:
1. 验证用户名和密码
2. 检查 SSH 配置是否允许密码认证
3. 尝试使用密钥认证

```bash
# 检查 SSH 配置
cat /etc/ssh/sshd_config | grep PasswordAuthentication
```

### 依赖安装失败

**问题**: 依赖安装失败: apt-get update failed

**解决**:
1. 检查网络连接
2. 更新软件源
3. 手动安装依赖

```bash
# 手动更新
apt-get update
apt-get install -y curl wget unzip
```

### Xray 安装失败

**问题**: Xray 安装失败: download failed

**解决**:
1. 检查 GitHub 访问
2. 使用代理或镜像
3. 手动下载安装

```bash
# 页面下载的部署脚本支持覆盖 GitHub 加速镜像，多个地址用空格分隔
export XRAY_GITHUB_PROXY_PREFIXES="https://gh.llkk.cc https://gh-proxy.com https://ghproxy.net https://hub.gitmirror.com"
# 默认镜像优先；如需官方 GitHub 优先，可设置为 0
export XRAY_GITHUB_PROXY_FIRST=1

# 手动安装 Xray
bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install
```

### Agent 启动失败

**问题**: Agent 服务启动失败

**解决**:
1. 检查配置文件
2. 查看服务日志
3. 验证 Panel URL 和 Token

```bash
# 查看服务状态
systemctl status vpanel-agent

# 查看日志
journalctl -u vpanel-agent -n 50

# 测试配置
cat /etc/vpanel/agent.yaml
```

## 手动部署步骤

如果自动部署失败，可以手动执行以下步骤：

### 1. 安装依赖

```bash
# Ubuntu/Debian
apt-get update
apt-get install -y curl wget unzip

# CentOS/RHEL
yum install -y curl wget unzip
```

### 2. 安装 Xray

```bash
# 如果目标机器访问 GitHub 不稳定，优先使用页面下载的部署脚本；
# 该脚本会按官方源和 XRAY_GITHUB_PROXY_PREFIXES 镜像顺序自动重试。
bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install
```

### 3. 创建配置

```bash
mkdir -p /etc/vpanel

cat > /etc/vpanel/agent.yaml <<EOF
panel:
  url: "https://panel.example.com"
  token: "your-node-token"

xray:
  binary_path: "/usr/local/bin/xray"
  config_path: "/usr/local/etc/xray/config.json"

sync:
  interval: 5m

health:
  port: 18443
EOF
```

### 4. 创建服务

```bash
cat > /etc/systemd/system/vpanel-agent.service <<EOF
[Unit]
Description=V Panel Agent
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/vpanel-agent -config /etc/vpanel/agent.yaml
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
```

### 5. 启动服务

```bash
systemctl daemon-reload
systemctl enable vpanel-agent
systemctl start vpanel-agent
systemctl status vpanel-agent
```

## 验证部署

### 1. 检查服务状态

```bash
systemctl status vpanel-agent
```

应该显示 `active (running)`

### 2. 查看日志

```bash
journalctl -u vpanel-agent -f
```

应该看到连接成功和心跳日志

### 3. 检查 Panel

在 Panel 管理后台，节点状态应该显示"在线"

### 4. 测试配置同步

```bash
# 在 Panel 中创建代理
# 查看节点配置是否更新
cat /etc/xray/config.json
```

## 最佳实践

1. **使用密钥认证** - 比密码更安全
2. **限制 SSH 访问** - 只允许必要的 IP
3. **定期更新** - 保持 Agent 和 Xray 最新
4. **监控日志** - 及时发现问题
5. **备份配置** - 定期备份重要配置

## 相关文档

- [Xray 配置指南](./xray-config-guide.md)
- [Agent 部署指南](./NODE-AGENT-GUIDE.md)
- [快速开始](./quick-start-xray.md)
