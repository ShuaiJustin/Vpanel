# V Panel 运维指南

## 概述

本指南提供 V Panel 的日常运维操作说明，包括监控、备份、日志管理、故障排查等。

---

## 日常运维任务

### 1. 健康检查

**每日检查**:
```bash
# 检查所有组件
./scripts/health-check.sh all

# 检查 Panel
./scripts/health-check.sh panel

# 检查 Agent
./scripts/health-check.sh agent
```

**检查项**:
- ✅ 进程运行状态
- ✅ HTTP 端点响应
- ✅ 服务健康状态
- ✅ 日志错误数量
- ✅ 系统资源使用

**告警阈值**:
- CPU 使用率 > 80%
- 内存使用率 > 90%
- 磁盘使用率 > 85%
- 错误日志 > 10 条/小时

---

### 2. 备份管理

**自动备份**:
```bash
# 设置每日自动备份
(crontab -l 2>/dev/null; echo "0 3 * * * cd /path/to/vpanel && ./scripts/backup.sh all") | crontab -
```

**手动备份**:
```bash
# 备份所有内容
./scripts/backup.sh all

# 只备份数据库
DB_PASSWORD=xxx ./scripts/backup.sh database

# 只备份配置
./scripts/backup.sh config
```

**备份验证**:
```bash
# 列出备份
./scripts/backup.sh list

# 验证备份完整性
gunzip -t backups/database/vpanel_db_*.sql.gz
```

**备份策略**:
- 每日全量备份
- 保留 7 天备份
- 每周备份归档到远程存储
- 每月测试恢复流程

---

### 3. 日志管理

**日志轮转**:
```bash
# 设置自动轮转
./scripts/log-rotate.sh setup

# 手动轮转
./scripts/log-rotate.sh rotate

# 清理旧日志
./scripts/log-rotate.sh clean
```

**日志分析**:
```bash
# 分析日志
./scripts/log-rotate.sh analyze

# 查看错误日志
tail -f logs/vpanel.log | grep ERROR

# 统计错误
grep ERROR logs/vpanel.log | wc -l
```

**日志位置**:
- Panel: `logs/vpanel.log`
- Agent: `journalctl -u vpanel-agent`
- Xray: `/var/log/xray/error.log`

---

### 4. 性能监控

**系统资源**:
```bash
# CPU 和内存
top -p $(pgrep vpanel)

# 磁盘 I/O
iostat -x 1

# 网络连接
netstat -an | grep ESTABLISHED | wc -l
```

**数据库性能**:
```bash
# 连接数
psql -U vpanel -d vpanel -c "SELECT count(*) FROM pg_stat_activity;"

# 慢查询
psql -U vpanel -d vpanel -c "SELECT query, calls, total_time FROM pg_stat_statements ORDER BY total_time DESC LIMIT 10;"

# 数据库大小
psql -U vpanel -d vpanel -c "SELECT pg_size_pretty(pg_database_size('vpanel'));"
```

**应用性能**:
```bash
# API 响应时间
curl -w "@curl-format.txt" -o /dev/null -s http://localhost:8080/health

# 创建 curl-format.txt
cat > curl-format.txt <<EOF
time_namelookup:  %{time_namelookup}\n
time_connect:  %{time_connect}\n
time_starttransfer:  %{time_starttransfer}\n
time_total:  %{time_total}\n
EOF
```

---

## 故障排查

### 1. Panel 无法启动

**检查步骤**:
```bash
# 1. 检查配置文件
cat configs/config.yaml

# 2. 检查数据库连接
psql -h localhost -U vpanel -d vpanel

# 3. 检查端口占用
lsof -i :8080

# 4. 查看日志
tail -100 logs/vpanel.log

# 5. 检查权限
ls -la vpanel
```

**常见问题**:
- 数据库连接失败 → 检查数据库配置和状态
- 端口被占用 → 修改配置或停止占用进程
- 配置文件错误 → 验证 YAML 格式
- 权限不足 → 检查文件权限

---

### 2. Agent 无法连接 Panel

**检查步骤**:
```bash
# 1. 检查 Agent 状态
systemctl status vpanel-agent

# 2. 查看 Agent 日志
journalctl -u vpanel-agent -n 50

# 3. 测试网络连接
curl -v https://panel.example.com/health

# 4. 检查 Token
cat /etc/vpanel/agent.yaml | grep token

# 5. 检查防火墙
iptables -L -n | grep 8080
```

**常见问题**:
- Token 错误 → 重新生成 Token
- 网络不通 → 检查防火墙和路由
- SSL 证书错误 → 更新证书或禁用验证
- Panel 地址错误 → 修正配置

---

### 3. Xray 无法启动

**检查步骤**:
```bash
# 1. 检查 Xray 安装
which xray
xray version

# 2. 验证配置
xray -test -config /etc/xray/config.json

# 3. 查看日志
tail -100 /var/log/xray/error.log

# 4. 检查端口
netstat -tlnp | grep xray

# 5. 手动启动测试
xray -config /etc/xray/config.json
```

**常见问题**:
- 配置格式错误 → 使用 `-test` 验证
- 端口冲突 → 修改端口配置
- 证书问题 → 检查证书路径和权限
- 权限不足 → 使用 root 运行

---

### 4. 代理连接失败

**检查步骤**:
```bash
# 1. 检查代理状态
curl http://localhost:8080/api/proxies

# 2. 检查 Xray 配置
cat /etc/xray/config.json | jq '.inbounds'

# 3. 测试端口
telnet localhost <proxy-port>

# 4. 检查防火墙
iptables -L -n | grep <proxy-port>

# 5. 查看 Xray 日志
tail -f /var/log/xray/access.log
```

**常见问题**:
- 端口未开放 → 配置防火墙规则
- UUID 错误 → 检查客户端配置
- 协议不匹配 → 确认协议设置
- 网络限制 → 检查 ISP 限制

---

## 安全加固

### 1. 系统安全

**防火墙配置**:
```bash
# 只允许必要端口
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp    # SSH
ufw allow 8080/tcp  # Panel
ufw allow 443/tcp   # HTTPS
ufw enable
```

**SSH 安全**:
```bash
# 禁用密码登录
sudo sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config
sudo systemctl restart sshd

# 使用密钥认证
ssh-keygen -t ed25519
ssh-copy-id user@server
```

**自动更新**:
```bash
# Ubuntu/Debian
sudo apt install unattended-upgrades
sudo dpkg-reconfigure -plow unattended-upgrades
```

---

### 2. 应用安全

**Token 管理**:
```bash
# 定期轮换 Token
# 在 Panel 管理界面重新生成节点 Token

# 限制 Token 权限
# 为不同节点使用不同 Token
```

**数据库安全**:
```bash
# 使用强密码
# 限制数据库访问 IP
# 定期备份

# PostgreSQL 配置
sudo vim /etc/postgresql/*/main/pg_hba.conf
# 只允许本地连接
host    vpanel    vpanel    127.0.0.1/32    md5
```

**HTTPS 配置**:
```bash
# 使用 Let's Encrypt
sudo certbot --nginx -d panel.example.com

# 强制 HTTPS
# 在 nginx 配置中添加重定向
```

---

### 3. 监控告警

**设置告警**:
```bash
# 创建监控脚本
cat > /usr/local/bin/vpanel-monitor.sh <<'EOF'
#!/bin/bash
# 检查服务状态
if ! systemctl is-active --quiet vpanel-agent; then
    echo "Alert: vpanel-agent is down" | mail -s "VPanel Alert" admin@example.com
fi

# 检查磁盘空间
disk_usage=$(df / | tail -1 | awk '{print $5}' | sed 's/%//')
if [ $disk_usage -gt 85 ]; then
    echo "Alert: Disk usage is ${disk_usage}%" | mail -s "VPanel Alert" admin@example.com
fi
EOF

chmod +x /usr/local/bin/vpanel-monitor.sh

# 添加定时任务
(crontab -l; echo "*/5 * * * * /usr/local/bin/vpanel-monitor.sh") | crontab -
```

---

## 性能优化

### 1. 数据库优化

**索引优化**:
```sql
-- 检查缺失的索引
SELECT schemaname, tablename, attname, n_distinct, correlation
FROM pg_stats
WHERE schemaname = 'public'
ORDER BY n_distinct DESC;

-- 添加索引
CREATE INDEX CONCURRENTLY idx_traffic_created_at ON traffic(created_at);
```

**连接池配置**:
```yaml
# configs/config.yaml
database:
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m
```

**定期维护**:
```bash
# 分析表
psql -U vpanel -d vpanel -c "ANALYZE;"

# 清理死元组
psql -U vpanel -d vpanel -c "VACUUM ANALYZE;"
```

---

### 2. 应用优化

**缓存配置**:
```yaml
# 启用配置缓存
cache:
  enabled: true
  ttl: 5m
```

**并发控制**:
```yaml
# 限制并发请求
server:
  max_concurrent_requests: 100
  timeout: 30s
```

---

### 3. 系统优化

**文件描述符**:
```bash
# 增加限制
sudo vim /etc/security/limits.conf
* soft nofile 65535
* hard nofile 65535

# 重启生效
```

**内核参数**:
```bash
# 优化网络
sudo vim /etc/sysctl.conf
net.core.somaxconn = 1024
net.ipv4.tcp_max_syn_backlog = 2048
net.ipv4.tcp_tw_reuse = 1

# 应用配置
sudo sysctl -p
```

---

## 升级和维护

### 1. 版本升级

**升级前准备**:
```bash
# 1. 备份数据
./scripts/backup.sh all

# 2. 查看变更日志
cat CHANGELOG.md

# 3. 测试环境验证
# 在测试环境先升级测试
```

**升级步骤**:
```bash
# 1. 停止服务
systemctl stop vpanel-agent

# 2. 备份当前版本
cp vpanel vpanel.backup
cp vpanel-agent vpanel-agent.backup

# 3. 下载新版本
wget https://github.com/xxx/vpanel/releases/download/v1.1.0/vpanel
wget https://github.com/xxx/vpanel/releases/download/v1.1.0/vpanel-agent

# 4. 运行迁移
./vpanel migrate

# 5. 启动服务
systemctl start vpanel-agent

# 6. 验证
./scripts/health-check.sh all
```

**回滚**:
```bash
# 如果升级失败
systemctl stop vpanel-agent
cp vpanel.backup vpanel
cp vpanel-agent.backup vpanel-agent
./scripts/backup.sh restore backups/database/xxx.sql.gz
systemctl start vpanel-agent
```

---

### 2. 定期维护

**每日任务**:
- ✅ 健康检查
- ✅ 日志分析
- ✅ 备份验证

**每周任务**:
- ✅ 性能分析
- ✅ 安全扫描
- ✅ 备份归档

**每月任务**:
- ✅ 系统更新
- ✅ 证书检查
- ✅ 恢复演练
- ✅ 容量规划

---

## 容量规划

### 1. 资源需求

**最小配置**:
- CPU: 2 核
- 内存: 2GB
- 磁盘: 20GB
- 网络: 10Mbps

**推荐配置**:
- CPU: 4 核
- 内存: 4GB
- 磁盘: 50GB SSD
- 网络: 100Mbps

**高负载配置**:
- CPU: 8 核
- 内存: 8GB
- 磁盘: 100GB SSD
- 网络: 1Gbps

---

### 2. 扩展策略

**垂直扩展**:
- 增加 CPU 和内存
- 升级到 SSD
- 增加网络带宽

**水平扩展**:
- 增加节点数量
- 负载均衡
- 数据库读写分离

---

## 应急响应

### 1. 服务中断

**响应流程**:
1. 确认问题范围
2. 通知相关人员
3. 启动应急预案
4. 恢复服务
5. 问题分析
6. 预防措施

**应急联系**:
- 技术负责人: xxx
- 运维负责人: xxx
- 紧急热线: xxx

---

### 2. 数据恢复

**恢复流程**:
```bash
# 1. 停止服务
systemctl stop vpanel-agent

# 2. 恢复数据库
./scripts/backup.sh restore backups/database/latest.sql.gz

# 3. 恢复配置
./scripts/backup.sh restore backups/config/latest.tar.gz

# 4. 验证数据
./scripts/verify-migration.sh

# 5. 启动服务
systemctl start vpanel-agent

# 6. 验证功能
./scripts/health-check.sh all
```

---

## 相关文档

- [脚本使用指南](./SCRIPTS-GUIDE.md)
- [已知问题](./KNOWN-ISSUES.md)
- [快速开始](./quick-start-xray.md)
- [远程部署指南](./remote-deploy-guide.md)

---

**最后更新**: 2026-01-19
