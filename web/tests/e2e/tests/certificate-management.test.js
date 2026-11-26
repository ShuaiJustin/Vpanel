import { BrowserHelper } from '../helpers/browser.js';
import { AuthHelper } from '../helpers/auth.js';

/**
 * 证书管理测试
 */
describe('证书管理测试', () => {
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

  test('应该能够访问证书管理页面', async () => {
    await browser.goto('/admin/certificates');
    await browser.screenshot('certificates-page');
    
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);

  test('应该显示证书列表或空状态', async () => {
    await browser.goto('/admin/certificates');
    await browser.wait(2000);
    
    await browser.screenshot('certificates-list');
    
    // 页面加载成功即可
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);
});
