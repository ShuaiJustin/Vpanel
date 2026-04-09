# V Panel 节点Agent部署指南

## 架构说明

V Panel 采用 **中心化管理 + 分布式节点** 的架构：

```
┌─────────────────┐
│   Panel Server  │  ← 中心管理面板（你当前运行的）
│   (主控端)      │
└────────┬────────┘
         │ HTTP/HTTPS
         │ 心跳 + 配置同步
         │
    ┌────┴────┬────────┬────────┐
    │         │        │        │
┌───▼───┐ ┌──▼───┐ ┌──▼───┐ ┌──▼───┐
│ Node1 │ │ Node2│ │ Node3│ │ Node4│  ← 节点Agent（需要部署）
│ Agent │ │ Agent│ │ Agent│ │ Agent│
└───┬───┘ └──┬───┘ └──┬───┘ └──┬───┘
    │        │        │        │
┌───▼───┐ ┌──▼───┐ ┌──▼───┐ ┌──▼───┐
│ Xray  │ │ Xray │ │ Xray │ │ Xray │  ← 实际的代理服务
└───────┘ └──────┘ └──────┘ └──────┘
```

## 通信机制

### 1. 节点注册
- Agent启动时向Panel注册
- 提供节点信息（名称、系统信息等）
- Panel分配节点ID和验证token

### 2. 心跳机制
- Agent每30秒向Panel发送心跳
- 上报节点状态（CPU、内存、流量等）
- 接收Panel下发的命令

### 3. 配置同步
- Panel修改配置后推送到Agent
- Agent更新Xray配置并重启服务
- 支持热更新和版本回滚

### 4. 命令执行
- Panel可以远程执行命令：
  - 重启Xray
  - 更新配置
  - 查看日志
  - 系统诊断

## 节点Agent部署步骤

### 步骤1: 在Panel中创建节点

1. 登录管理后台
2. 进入 **节点管理** → **添加节点**
3. 填写节点信息：
   - 节点名称：如 "香港节点1"
   - 节点地址：节点服务器的IP或域名
   - 端口：Xray监听端口
4. 点击保存，系统会生成 **节点Token**
5. **复制并保存Token**（后面配置Agent需要）

### 步骤2: 在节点服务器上部署Agent

#### 2.1 下载并编译Agent

```bash
# 在节点服务器上
git clone <your-repo-url>
cd V

# 编译Agent
go build -o vpanel-agent ./cmd/agent/main.go
```

#### 2.2 创建配置文件

```bash
# 复制配置模板
cp configs/agent.yaml.example configs/agent.yaml

# 编辑配置
vim configs/agent.yaml
```

配置示例：
```yaml
# Panel Server配置
panel:
  url: "https://your-panel-domain.com"  # Panel地址
  tls_skip_verify: false                 # 生产环境设为false
  connect_timeout: 30s
  reconnect_interval: 10s
  max_reconnect_delay: 5m

# 节点配置
node:
  name: "香港节点1"                      # 节点名称
  token: "your-node-token-here"         # 从Panel获取的Token

# Xray配置
xray:
  binary_path: "/usr/local/bin/xray"    # Xray二进制路径
  config_path: "/usr/local/etc/xray/config.json"  # 托管标准路径；历史手工节点仍兼容 /etc/xray/config.json
  backup_dir: "/var/backups/xray"       # 配置备份目录

# 健康检查服务
health:
  host: "0.0.0.0"
  port: 18443                             # 健康检查端口（Agent 监听端口）

# 日志配置
log:
  level: "info"
  format: "json"
  output: "/var/log/vpanel-agent.log"
```

#### 2.3 安装Xray（如果未安装）

```bash
# 使用官方脚本安装
bash <(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)

# 或手动下载
wget https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-64.zip
unzip Xray-linux-64.zip
mv xray /usr/local/bin/
chmod +x /usr/local/bin/xray
```

#### 2.4 启动Agent

```bash
# 前台运行（测试）
./vpanel-agent -config configs/agent.yaml

# 后台运行
nohup ./vpanel-agent -config configs/agent.yaml > agent.log 2>&1 &

# 使用systemd（推荐）
sudo cp deployments/systemd/vpanel-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable vpanel-agent
sudo systemctl start vpanel-agent
```

### 步骤3: 验证连接

#### 3.1 检查Agent日志
```bash
tail -f /var/log/vpanel-agent.log

# 应该看到类似输出：
# {"level":"info","msg":"registered with panel","node_id":1}
# {"level":"info","msg":"heartbeat sent successfully","node_id":1}
```

#### 3.2 在Panel中查看
1. 进入 **节点管理**
2. 查看节点状态应该显示 **在线**
3. 可以看到节点的实时指标（CPU、内存等）

#### 3.3 测试健康检查
```bash
curl http://localhost:18443/health
# 应该返回：{"status":"healthy","registered":true,"node_id":1}
```

## 常见问题

### Q1: Agent无法连接到Panel
**检查：**
- Panel地址是否正确
- 防火墙是否开放
- Token是否正确
- TLS证书是否有效

### Q2: 节点显示离线
**检查：**
- Agent进程是否运行：`ps aux | grep vpanel-agent`
- 查看Agent日志：`tail -f /var/log/vpanel-agent.log`
- 网络连接：`curl -v https://your-panel-domain.com/health`

### Q3: Xray无法启动
**检查：**
- Xray是否已安装：`xray version`
- 配置文件路径是否正确
- 端口是否被占用：`netstat -tlnp | grep <port>`

### Q4: 配置同步失败
**检查：**
- Agent是否有写入权限
- Xray配置目录是否存在
- 查看Agent日志中的错误信息

### Q5: 节点在线但流量不上报
**检查：**
- Agent 配置里的 Xray 路径：`grep config_path /etc/vpanel/agent.yaml`
- Xray 实际运行路径：`ps -eo pid=,comm=,args= | grep xray`
- Xray Stats API 是否可用：`/usr/local/bin/xray api statsquery --server=127.0.0.1:<api_port>`
- Agent 流量诊断：`curl http://127.0.0.1:18443/traffic/status`

**历史手工节点巡检步骤：**
1. 核对 `/etc/vpanel/agent.yaml` 中的 `xray.config_path` 是否已经收口到 `/usr/local/etc/xray/config.json`
2. 核对 `ps` 输出里的 Xray `-config` 或 `-c` 实际参数
3. 如果 Agent 配置路径与 Xray 实际运行路径不一致，优先以运行中的配置路径为准修正 Agent 配置
4. 确认 `curl http://127.0.0.1:18443/traffic/status` 返回 `healthy_idle` 或 `healthy_collecting`

## 多节点部署

对于多个节点，重复以上步骤：

1. 在Panel中为每个节点创建记录并获取Token
2. 在每台节点服务器上部署Agent
3. 使用对应的Token配置每个Agent

## 安全建议

1. **使用HTTPS**: Panel必须使用HTTPS（生产环境）
2. **Token保密**: 节点Token相当于密码，不要泄露
3. **防火墙**: 只开放必要的端口
4. **定期更新**: 及时更新Agent和Xray版本
5. **日志监控**: 定期检查Agent日志

## API端点

Agent与Panel通信的API端点：

- `POST /api/node/register` - 节点注册
- `POST /api/node/heartbeat` - 心跳上报
- `GET /api/node/:id/config` - 获取配置
- `POST /api/node/command/result` - 命令结果上报

## 监控指标

Agent会上报以下指标：
- CPU使用率
- 内存使用率
- 磁盘使用率
- 网络流量（入/出）
- Xray运行状态
- 活跃连接数
- 节点在线时长

## 下一步

部署完成后，你可以：
1. 在Panel中查看节点实时状态
2. 远程管理Xray配置
3. 监控节点性能
4. 查看流量统计
5. 执行远程命令
