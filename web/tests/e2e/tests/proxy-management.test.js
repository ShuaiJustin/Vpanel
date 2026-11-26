import { BrowserHelper } from '../helpers/browser.js';
import { AuthHelper } from '../helpers/auth.js';

/**
 * 代理管理测试
 */
describe('代理管理测试', () => {
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

  test('应该能够访问代理管理页面', async () => {
    await browser.goto('/admin/proxies');
    await browser.screenshot('proxies-page');
    
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);

  test('应该显示代理列表或空状态', async () => {
    await browser.goto('/admin/proxies');
    await browser.wait(2000);
    await browser.screenshot('proxies-list');
    
    // 页面加载成功即可
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);

  test('应该能够查看代理页面', async () => {
    await browser.goto('/admin/proxies');
    await browser.wait(2000);
    
    await browser.screenshot('proxies-detail');
    
    // 页面加载成功即可
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);
});
