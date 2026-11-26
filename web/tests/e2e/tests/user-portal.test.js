import { BrowserHelper } from '../helpers/browser.js';
import { AuthHelper } from '../helpers/auth.js';

/**
 * 用户门户测试
 */
describe('用户门户测试', () => {
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

  test('应该能够访问用户门户首页', async () => {
    await browser.goto('/user');
    await browser.screenshot('user-portal-home');
    
    // 验证跳转到用户门户
    const url = browser.page.url();
    expect(url).toContain('/user');
  });

  test('注册直达页应遵循当前注册设置', async () => {
    const settings = browser.page.waitForResponse(response =>
      new URL(response.url()).pathname.endsWith('/api/portal/auth/oauth/providers')
    );
    await browser.goto('/user/register');
    const response = await settings;
    expect(response.ok()).toBe(true);
    const { registration_open } = await response.json();
    expect(typeof registration_open).toBe('boolean');
    await browser.waitForElement(registration_open ? '.register-form' : '.register-card [role="status"]');
    await browser.screenshot('user-register-page');
    expect(await browser.exists('.register-form')).toBe(registration_open);
    if (!registration_open) {
      expect(await browser.getText('.register-card [role="status"]')).toContain('当前未开放注册');
    }
  });

  test('应该显示登录页面', async () => {
    await browser.goto('/user/login');
    await browser.screenshot('user-login-page');
    
    // 验证登录表单
    const hasUsername = await browser.exists('input[type="text"]');
    const hasPassword = await browser.exists('input[type="password"]');
    
    expect(hasUsername).toBe(true);
    expect(hasPassword).toBe(true);
  });
});
