# Contributing to V Panel

感谢你考虑为 V Panel 做出贡献！

## 如何贡献

### 报告 Bug

如果你发现了 bug，请在 GitHub Issues 中提交，并包含以下信息：

1. **环境信息**
   - 操作系统和版本
   - Docker 版本
   - V Panel 版本或 commit hash
   - 浏览器版本（前端问题）

2. **复现步骤**
   - 详细的操作步骤
   - 预期行为
   - 实际行为

3. **日志**
   - Panel 日志：`docker logs vpanel-v-panel-1`
   - 浏览器控制台错误（前端问题）

### 提交功能建议

在 GitHub Issues 中提交功能建议时，请说明：
- 功能的使用场景
- 预期的用户体验
- 可能的实现方案（可选）

### 提交 Pull Request

1. **Fork 仓库**
   ```bash
   git clone https://github.com/YOUR_USERNAME/Vpanel.git
   cd Vpanel
   ```

2. **创建分支**
   ```bash
   git checkout -b feature/your-feature-name
   # 或
   git checkout -b fix/your-bug-fix
   ```

3. **本地开发**
   ```bash
   # 后端开发
   go run ./cmd/v
   
   # 前端开发（新终端）
   cd web
   npm install
   npm run dev
   ```

4. **代码规范**
   - **后端 (Go)**
     - 遵循 Go 标准命名规范
     - 使用 `gofmt` 格式化代码
     - 运行 `go vet ./...` 检查
     - 添加必要的注释
   
   - **前端 (Vue)**
     - 遵循 Vue 3 Composition API 风格
     - 使用 ESLint 规范：`npm run lint`
     - 组件命名使用 PascalCase
     - 添加必要的注释

5. **测试**
   ```bash
   # 后端测试
   go test ./...
   
   # 前端测试
   cd web
   npm test
   ```

6. **提交规范**
   
   使用语义化提交信息：
   ```
   <type>: <subject>
   
   <body>
   ```
   
   **Type 类型：**
   - `feat`: 新功能
   - `fix`: Bug 修复
   - `docs`: 文档更新
   - `style`: 代码格式（不影响功能）
   - `refactor`: 重构（不改变功能）
   - `perf`: 性能优化
   - `test`: 测试相关
   - `chore`: 构建/工具配置
   
   **示例：**
   ```
   feat: 添加节点健康检查功能
   
   - 新增健康检查 API 接口
   - 添加前端健康状态显示
   - 支持自定义检查间隔
   ```

7. **推送并创建 PR**
   ```bash
   git push origin feature/your-feature-name
   ```
   
   在 GitHub 上创建 Pull Request，并说明：
   - 改动的内容和原因
   - 相关的 Issue 编号（如有）
   - 测试情况
   - 截图或 GIF（UI 改动）

### PR 审核流程

1. 自动检查：代码格式、测试通过
2. 人工审核：代码质量、功能正确性
3. 合并：审核通过后合并到 main 分支

## 开发指南

### 项目结构

```
Vpanel/
├── cmd/                 # 主程序入口
│   ├── v/              # Panel 主程序
│   └── agent/          # Agent 程序
├── internal/           # 内部包
│   ├── api/           # API 路由和 handlers
│   ├── auth/          # 认证服务
│   ├── database/      # 数据库操作
│   ├── node/          # 节点管理
│   └── proxy/         # 代理配置
├── web/               # 前端项目
│   ├── src/
│   │   ├── api/      # API 调用
│   │   ├── views/    # 页面组件
│   │   ├── components/ # 公共组件
│   │   └── stores/   # Pinia 状态管理
│   └── public/
├── deployments/       # 部署配置
├── docs/             # 文档
└── configs/          # 配置文件示例
```

### 调试技巧

**后端调试**
```bash
# 开启详细日志
V_LOG_LEVEL=debug go run ./cmd/v

# 使用 Delve 调试器
dlv debug ./cmd/v
```

**前端调试**
- 使用 Vue DevTools 浏览器扩展
- 查看 Network 面板的 API 请求
- Console 查看日志

### 数据库变更

数据库迁移使用 GORM AutoMigrate，在 `internal/database/repository/*.go` 中定义模型即可自动创建表。

**注意**：不使用 SQL 迁移文件，所有表结构由 Go 结构体定义。

## 行为准则

- 尊重所有贡献者
- 欢迎建设性的批评和建议
- 专注于对项目最有利的方案
- 友好、专业的沟通

## 许可证

提交 PR 即表示你同意将代码以 MIT 许可证贡献给本项目。

## 获取帮助

- 查看文档：[docs/](docs/)
- 提交 Issue：描述你的问题
- 查看已有 Issue：可能已有解决方案

---

再次感谢你的贡献！🎉
