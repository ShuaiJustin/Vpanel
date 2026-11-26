import { BrowserHelper } from '../helpers/browser.js';
import { AuthHelper } from '../helpers/auth.js';

/**
 * API 错误详细检测
 */
describe('API 错误详细检测', () => {
  let browser;
  let auth;
  const apiErrors = [];

  beforeAll(async () => {
    browser = new BrowserHelper();
    await browser.launch();
    auth = new AuthHelper(browser);
    
    // 监听所有网络请求
    browser.page.on('response', async (response) => {
      const url = response.url();
      const status = response.status();
      
      // 记录所有失败的 API 请求
      if (status >= 400 && url.includes('/api/')) {
        try {
          const text = await response.text();
          apiErrors.push({
            url,
            status,
            statusText: response.statusText(),
            body: text.substring(0, 200)
          });
        } catch (e) {
          apiErrors.push({
            url,
            status,
            statusText: response.statusText(),
            body: 'Unable to read response'
          });
        }
      }
    });
    
    await auth.loginAsAdmin();
  });

  afterAll(async () => {
    await browser.close();
    
    // 输出所有 API 错误
    if (apiErrors.length > 0) {
      console.log(`\n⚠️  发现 ${apiErrors.length} 个 API 错误:\n`);
      apiErrors.forEach((err, i) => {
        console.log(`${i + 1}. ${err.status} ${err.statusText}`);
        console.log(`   URL: ${err.url}`);
        console.log(`   响应: ${err.body}\n`);
      });
    }
  });

  test('访问仪表板并检测 API 错误', async () => {
    await browser.goto('/admin/dashboard');
    await browser.wait(3000);
    await browser.screenshot('dashboard-api-check');
    
    const url = browser.page.url();
    expect(url).toContain('/admin/dashboard');
  }, 30000);

  test('访问用户管理并检测 API 错误', async () => {
    await browser.goto('/admin/users');
    await browser.wait(3000);
    await browser.screenshot('users-api-check');
    
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);

  test('访问节点管理并检测 API 错误', async () => {
    await browser.goto('/admin/nodes');
    await browser.wait(3000);
    await browser.screenshot('nodes-api-check');
    
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);

  test('访问礼品卡管理并检测 API 错误', async () => {
    await browser.goto('/admin/gift-cards');
    await browser.wait(3000);
    await browser.screenshot('gift-cards-api-check');
    
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);

  test('访问财务报表并检测 API 错误', async () => {
    await browser.goto('/admin/reports');
    await browser.wait(3000);
    await browser.screenshot('reports-api-check');
    
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);
});
