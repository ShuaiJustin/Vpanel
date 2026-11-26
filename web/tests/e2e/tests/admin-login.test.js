import { BrowserHelper } from '../helpers/browser.js';
import { AuthHelper } from '../helpers/auth.js';

/**
 * 管理员登录测试
 */
describe('管理员登录测试', () => {
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

  test('应该成功登录管理后台', async () => {
    // 执行登录
    await auth.loginAsAdmin();
    
    // 验证登录成功 - 检查 URL 已经改变（不再是登录页）
    const url = browser.page.url();
    expect(url).not.toContain('/login');
    
    console.log(`✅ 登录后 URL: ${url}`);
    
    // 截图验证
    await browser.screenshot('admin-dashboard');
  }, 30000); // 30秒超时

  test('应该显示管理员用户信息', async () => {
    // 等待用户信息加载
    await browser.waitForElement('.user-info, .user-avatar, .header, .el-header');
    
    // 验证用户名显示
    const hasUsername = await browser.exists('.username, .user-name, .el-dropdown');
    expect(hasUsername).toBe(true);
    
    await browser.screenshot('admin-user-info');
  }, 30000);

  test('应该能够访问用户管理页面', async () => {
    // 尝试导航到用户管理页面
    await browser.goto('/admin/users');
    await browser.wait(2000);
    
    // 验证页面跳转
    const url = browser.page.url();
    expect(url).toContain('/admin');
    
    await browser.screenshot('admin-users-page');
  }, 30000);
});
