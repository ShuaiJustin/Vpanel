# Puppeteer E2E 测试

使用 Puppeteer 进行端到端自动化测试。

## 安装依赖

```bash
npm install
```

## 运行测试

### 基本用法

```bash
# 运行所有测试（无头模式）
npm run test:e2e

# 运行所有测试（有头模式，可以看到浏览器）
npm run test:e2e:headed

# 运行特定测试文件
npm run test:e2e tests/admin-login.test.js

# 运行特定测试套件
npm run test:e2e tests/node-management.test.js
```

### 使用 Node 直接运行

```bash
# 无头模式
node tests/e2e/run-tests.js

# 有头模式
node tests/e2e/run-tests.js --headed

# 运行特定测试
node tests/e2e/run-tests.js tests/admin-login.test.js
```

## 环境变量

在运行测试前，可以设置以下环境变量：

```bash
# 基础 URL
export BASE_URL=http://localhost:8080

# 管理员凭证
export ADMIN_USER=admin
export ADMIN_PASS=admin123

# 测试用户凭证
export TEST_USER=testuser
export TEST_PASS=test123

# 浏览器模式
export HEADLESS=false  # 显示浏览器
export SLOW_MO=100     # 减慢操作速度（毫秒）
export DEVTOOLS=true   # 打开开发者工具

# 截图
export SCREENSHOT=true  # 启用截图
```

## 测试结构

```
tests/e2e/
├── puppeteer.config.js    # Puppeteer 配置
├── jest.config.js         # Jest 配置
├── run-tests.js           # 测试运行器
├── helpers/               # 辅助函数
│   ├── browser.js         # 浏览器操作封装
│   └── auth.js            # 认证辅助函数
├── tests/                 # 测试用例
│   ├── admin-login.test.js
│   ├── user-portal.test.js
│   ├── subscription.test.js
│   └── node-management.test.js
└── screenshots/           # 测试截图（自动生成）
```

## 编写测试

### 基本测试模板

```javascript
import { BrowserHelper } from '../helpers/browser.js';
import { AuthHelper } from '../helpers/auth.js';

describe('测试套件名称', () => {
  let browser;
  let auth;

  beforeAll(async () => {
    browser = new BrowserHelper();
    await browser.launch();
    auth = new AuthHelper(browser);
  });

  afterAll(async () => {
    await browser.close();
  });

  test('测试用例描述', async () => {
    // 导航到页面
    await browser.goto('/path');
    
    // 执行操作
    await browser.click('button');
    await browser.type('input', 'text');
    
    // 截图
    await browser.screenshot('test-name');
    
    // 断言
    const text = await browser.getText('.element');
    expect(text).toBe('Expected Text');
  });
});
```

### 常用操作

```javascript
// 导航
await browser.goto('/admin/dashboard');

// 等待元素
await browser.waitForElement('.loading');

// 点击
await browser.click('button.submit');

// 输入文本
await browser.type('input[name="username"]', 'admin');

// 获取文本
const text = await browser.getText('.title');

// 检查元素是否存在
const exists = await browser.exists('.error-message');

// 截图
await browser.screenshot('page-state');

// 等待
await browser.wait(1000); // 等待 1 秒

// 执行 JavaScript
const result = await browser.evaluate(() => {
  return document.title;
});
```

### 认证操作

```javascript
// 管理员登录
await auth.loginAsAdmin();

// 普通用户登录
await auth.loginAsUser();

// 自定义登录
await auth.login('username', 'password', '/login');

// 登出
await auth.logout();

// 检查登录状态
const isLoggedIn = await auth.isLoggedIn();
```

## 调试技巧

### 1. 使用有头模式

```bash
npm run test:e2e:headed
```

### 2. 减慢操作速度

```bash
SLOW_MO=500 npm run test:e2e:headed
```

### 3. 打开开发者工具

```bash
DEVTOOLS=true npm run test:e2e:headed
```

### 4. 查看截图

测试运行时会自动截图，保存在 `tests/e2e/screenshots/` 目录。

### 5. 单独运行失败的测试

```bash
npm run test:e2e tests/admin-login.test.js
```

## 最佳实践

1. **使用有意义的选择器**：优先使用 data-testid 属性
2. **等待元素加载**：使用 `waitForElement` 而不是固定延迟
3. **截图记录**：在关键步骤截图，便于调试
4. **独立测试**：每个测试应该独立运行，不依赖其他测试
5. **清理状态**：在 `afterEach` 或 `afterAll` 中清理测试数据

## 常见问题

### 测试超时

增加超时时间：

```javascript
test('长时间运行的测试', async () => {
  // ...
}, 120000); // 120 秒
```

### 元素找不到

使用更宽松的选择器或增加等待时间：

```javascript
await browser.waitForElement('.element', 10000);
```

### 浏览器启动失败

确保系统已安装必要的依赖：

```bash
# Ubuntu/Debian
sudo apt-get install -y chromium-browser

# macOS
brew install chromium
```

## CI/CD 集成

### GitHub Actions

```yaml
- name: Run E2E Tests
  run: |
    npm run test:e2e
  env:
    BASE_URL: http://localhost:8080
    HEADLESS: true
```

### GitLab CI

```yaml
e2e-tests:
  script:
    - npm install
    - npm run test:e2e
  variables:
    BASE_URL: http://localhost:8080
    HEADLESS: "true"
```
