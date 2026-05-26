# V Panel 文档

这里是 V Panel 的运维 / 部署文档。**新手先看根目录 [README.md](../README.md)** 了解快速部署。

## 文档列表

| 文档 | 适用场景 |
|---|---|
| [QUICK-REFERENCE.md](./QUICK-REFERENCE.md) | 常用命令速查 |
| [OPERATIONS-GUIDE.md](./OPERATIONS-GUIDE.md) | 日常运维（备份、日志、监控、故障排查） |
| [certificate-guide.md](./certificate-guide.md) | Let's Encrypt 证书申请详细配置（含各家 DNS provider 凭证获取） |
| [xray-config-guide.md](./xray-config-guide.md) | Xray 节点配置生成、代理协议示例、传输层选项 |
| [NODE-AGENT-GUIDE.md](./NODE-AGENT-GUIDE.md) | 节点 Agent 的安装、配置、systemd 管理 |
| [remote-deploy-guide.md](./remote-deploy-guide.md) | 通过 SSH 一键远程部署 Agent 到节点服务器 |

## 我想……

- **部署主面板** → 看根目录 [README.md](../README.md)
- **添加远程节点** → [remote-deploy-guide.md](./remote-deploy-guide.md)（推荐，全自动）或 [NODE-AGENT-GUIDE.md](./NODE-AGENT-GUIDE.md)（手动）
- **申请 HTTPS 证书** → [certificate-guide.md](./certificate-guide.md)
- **改 Xray 节点配置** → [xray-config-guide.md](./xray-config-guide.md)
- **看日志 / 备份 / 排障** → [OPERATIONS-GUIDE.md](./OPERATIONS-GUIDE.md)
- **忘了某个命令** → [QUICK-REFERENCE.md](./QUICK-REFERENCE.md)
