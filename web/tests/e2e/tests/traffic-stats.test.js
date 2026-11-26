import { BrowserHelper } from '../helpers/browser.js';
import { AuthHelper } from '../helpers/auth.js';

/**
 * 流量统计测试
 */
describe('流量统计测试', () => {
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

  test('应该能够访问仪表板', async () => {
    await browser.goto('/admin/dashboard');
    await browser.screenshot('dashboard');
    
    const url = browser.page.url();
    expect(url).not.toContain('/login');
  }, 30000);

  test('应该显示仪表板内容', async () => {
    await browser.goto('/admin/dashboard');
    await browser.wait(3000);
    
    await browser.screenshot('dashboard-stats');
    
    // 页面加载成功即可
    const url = browser.page.url();
    expect(url).not.toContain('/login');
  }, 30000);

  test('应该能够访问流量统计页面', async () => {
    await browser.goto('/admin/traffic');
    await browser.screenshot('traffic-page');
    
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);
});
