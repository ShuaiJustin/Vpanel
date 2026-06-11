# Changelog

All notable changes to V Panel will be documented in this file.

## [Unreleased]

### Added
- Agent 自动更新功能 - 远程触发节点 Agent 升级
- SSH 元数据管理 - 保存和获取节点 SSH 连接信息
- 订阅链接智能检测 - 支持更多反向代理头，自动过滤本地地址
- 生产部署检查清单文档
- 订阅链接配置详细指南

### Fixed
- 节点集群概览页面在中等屏幕（1280px-1440px）的布局问题
- 节点详情页面在中等屏幕的布局混乱
- 登录界面垂直居中和间距优化

### Changed
- README 重构为标准 GitHub 项目格式
- 完善 .env.example 配置说明

## [1.0.0] - 2026-06-11

### 初始发布

#### 核心功能
- 多协议支持：VLESS、VMess、Trojan、Shadowsocks
- 远程节点管理：SSH 配置下发、Agent 心跳监控
- 用户管理：流量统计、配额管理、订阅链接生成
- HTTPS 证书：Let's Encrypt 自动申请和续期
- 流量监控：实时统计、历史数据分析

#### 商业化功能
- 套餐系统：灵活配置、订单管理
- 营销工具：优惠券、礼品卡、试用、邀请返佣
- 支付集成：支付宝、微信支付
- 数据分析：用户统计、收入报表

#### 运维功能
- 操作审计：完整的日志记录
- 安全控制：IP 限制、JWT 认证、CSRF 保护
- 自动备份：数据库定期备份
- 健康检查：节点状态监控

#### 技术栈
- 后端：Go 1.21+ / Gin / SQLite
- 前端：Vue 3 / Element Plus / Vite
- 部署：Docker / Docker Compose / Nginx

---

格式说明：
- `[Unreleased]` - 未发布的开发中功能
- `[1.0.0]` - 已发布版本号
- `Added` - 新增功能
- `Changed` - 功能变更
- `Deprecated` - 即将废弃
- `Removed` - 已移除功能
- `Fixed` - 问题修复
- `Security` - 安全更新

[Unreleased]: https://github.com/ShuaiJustin/Vpanel/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/ShuaiJustin/Vpanel/releases/tag/v1.0.0
