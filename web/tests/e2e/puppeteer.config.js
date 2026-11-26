/**
 * Puppeteer 测试配置
 */
export default {
  // 测试基础 URL
  baseURL: process.env.BASE_URL || 'http://localhost:8080',
  
  // 浏览器配置
  browser: {
    headless: process.env.HEADLESS !== 'false', // 默认无头模式
    slowMo: parseInt(process.env.SLOW_MO) || 0, // 减慢操作速度（毫秒）
    devtools: process.env.DEVTOOLS === 'true', // 是否打开开发者工具
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--disable-dev-shm-usage',
      '--disable-gpu',
    ],
  },
  
  // 默认超时时间
  timeout: {
    navigation: 30000, // 页面导航超时
    element: 5000,     // 元素查找超时
    action: 3000,      // 操作超时
  },
  
  // 截图配置
  screenshot: {
    enabled: process.env.SCREENSHOT !== 'false',
    path: './tests/e2e/screenshots',
    fullPage: true,
  },
  
  // 视频录制配置
  video: {
    enabled: process.env.VIDEO === 'true',
    path: './tests/e2e/videos',
  },
  
  // 测试用户凭证
  credentials: {
    admin: {
      username: process.env.ADMIN_USER || 'admin',
      password: process.env.ADMIN_PASS || 'admin123',
    },
    user: {
      username: process.env.TEST_USER || 'testuser',
      password: process.env.TEST_PASS || 'test123',
    },
  },
};
