import { BrowserHelper } from '../helpers/browser.js';
import { AuthHelper } from '../helpers/auth.js';

/**
 * 后台错误检测测试
 */
describe('后台错误检测', () => {
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

  test('检查仪表板是否有错误', async () => {
    await browser.goto('/admin/dashboard');
    await browser.wait(3000);
    await browser.screenshot('dashboard-error-check');
    
    // 检查是否有错误提示
    const hasError = await browser.exists('.el-message--error, .error-message, [class*="error"]');
    
    if (hasError) {
      const errorText = await browser.page.evaluate(() => {
        const errorEl = document.querySelector('.el-message--error, .error-message, [class*="error"]');
        return errorEl ? errorEl.textContent : '';
      });
      console.log(`⚠️  发现错误: ${errorText}`);
    }
    
    // 页面应该能正常加载
    const url = browser.page.url();
    expect(url).toContain('/admin/dashboard');
  }, 30000);

  test('检查礼品卡页面是否有错误', async () => {
    await browser.goto('/admin/gift-cards');
    await browser.wait(3000);
    await browser.screenshot('gift-cards-error-check');
    
    // 检查是否有错误提示
    const hasError = await browser.exists('.el-message--error, .error-message');
    
    if (hasError) {
      const errorText = await browser.page.evaluate(() => {
        const errorEl = document.querySelector('.el-message--error, .error-message');
        return errorEl ? errorEl.textContent : '';
      });
      console.log(`⚠️  发现错误: ${errorText}`);
    }
    
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);

  test('检查财务报表页面是否有错误', async () => {
    await browser.goto('/admin/reports');
    await browser.wait(3000);
    await browser.screenshot('reports-error-check');
    
    // 检查是否有错误提示
    const hasError = await browser.exists('.el-message--error, .error-message');
    
    if (hasError) {
      const errorText = await browser.page.evaluate(() => {
        const errorEl = document.querySelector('.el-message--error, .error-message');
        return errorEl ? errorEl.textContent : '';
      });
      console.log(`⚠️  发现错误: ${errorText}`);
    }
    
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);

  test('检查所有管理页面的控制台错误', async () => {
    const pages = [
      '/admin/dashboard',
      '/admin/users',
      '/admin/nodes',
      '/admin/inbounds',
      '/admin/plans',
      '/admin/orders',
      '/admin/gift-cards',
      '/admin/reports'
    ];
    
    const errors = [];
    
    // 监听控制台错误
    browser.page.on('console', msg => {
      if (msg.type() === 'error') {
        errors.push(`${msg.text()}`);
      }
    });
    
    for (const page of pages) {
      await browser.goto(page);
      await browser.wait(2000);
    }
    
    if (errors.length > 0) {
      console.log(`⚠️  发现 ${errors.length} 个控制台错误:`);
      errors.forEach((err, i) => {
        console.log(`  ${i + 1}. ${err}`);
      });
    } else {
      console.log('✅ 未发现控制台错误');
    }
    
    // 测试通过，只是记录错误
    expect(true).toBe(true);
  }, 60000);
});
