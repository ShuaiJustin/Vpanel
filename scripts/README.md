# V Panel 脚本说明

## 目录结构

```
scripts/
├── build/                  # 构建
│   ├── build.sh            # 编译前后端
│   ├── build-agent.sh      # 编译 Agent 多平台二进制
│   └── docker-build.sh     # Docker 镜像构建
├── deploy/                 # 部署与启动
│   ├── start.sh            # 面板启动/停止/重启
│   ├── start-agent.sh      # Agent 启动/停止
│   ├── quick-deploy.sh     # 一键部署
│   ├── remote-install.sh   # 远程节点 Agent 安装
│   ├── install-agent.sh    # 本地 Agent 安装
│   └── install-xray.sh     # Xray 核心安装
├── ops/                    # 运维
│   ├── backup.sh           # 数据备份与恢复
│   ├── health-check.sh     # 健康检查
│   └── log-rotate.sh       # 日志轮转
├── db/                     # 数据库
│   ├── switch-to-mysql.sh  # 迁移至 MySQL
│   └── switch-to-postgres.sh # 迁移至 PostgreSQL
└── dev/                    # 开发
    ├── dev-setup.sh        # 开发环境初始化
    └── cleanup-repo.sh     # 仓库清理
```

## 快速上手

```bash
# 构建
./scripts/build/build.sh all

# 启动
./scripts/deploy/start.sh start

# Docker 部署（推荐）
./vpanel.sh

# 备份
./scripts/ops/backup.sh
```
