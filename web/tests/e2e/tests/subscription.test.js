import { BrowserHelper } from '../helpers/browser.js';
import { AuthHelper } from '../helpers/auth.js';

/**
 * 订阅系统测试
 */
describe('订阅系统测试', () => {
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

  test('应该能够访问订阅管理页面', async () => {
    await browser.goto('/admin/subscriptions');
    await browser.screenshot('subscriptions-page');
    
    const url = browser.page.url();
    expect(url).toContain('/admin/subscription');
  });

  test('应该显示订阅链接', async () => {
    await browser.goto('/admin/subscriptions');
    
    // 等待订阅链接元素
    const hasLink = await browser.exists('.subscription-link, input[readonly]');
    expect(hasLink).toBe(true);
    
    await browser.screenshot('subscription-link');
  });
});
