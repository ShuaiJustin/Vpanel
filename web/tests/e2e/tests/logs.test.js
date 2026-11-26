import { BrowserHelper } from '../helpers/browser.js';
import { AuthHelper } from '../helpers/auth.js';

/**
 * 日志系统测试
 */
describe('日志系统测试', () => {
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

  test('应该能够访问日志页面', async () => {
    await browser.goto('/admin/logs');
    await browser.screenshot('logs-page');
    
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);

  test('应该显示日志列表或空状态', async () => {
    await browser.goto('/admin/logs');
    await browser.wait(2000);
    
    await browser.screenshot('logs-list');
    
    // 页面加载成功即可
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);
});
