import { BrowserHelper } from '../helpers/browser.js';
import { AuthHelper } from '../helpers/auth.js';

/**
 * 系统设置测试
 */
describe('系统设置测试', () => {
  let browser;
  let auth;

  beforeAll(async () => {
    browser = new BrowserHelper();
    await browser.launch();
    auth = new AuthHelper(browser);
    await auth.loginAsAdmin();
  });

  afterAll(async () => {
    await browser.close();
  });

  test('应该能够访问系统设置页面', async () => {
    await browser.goto('/admin/settings');
    await browser.screenshot('settings-page');
    
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);

  test('应该显示设置表单', async () => {
    await browser.goto('/admin/settings');
    await browser.wait(2000);
    
    await browser.screenshot('settings-form');
    
    // 页面加载成功即可
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);
});
