import { BrowserHelper } from '../helpers/browser.js';
import { AuthHelper } from '../helpers/auth.js';

/**
 * 商业化功能测试
 */
describe('商业化功能测试', () => {
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

  test('应该能够访问套餐管理页面', async () => {
    await browser.goto('/admin/plans');
    await browser.screenshot('plans-page');
    
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);

  test('应该能够访问订单管理页面', async () => {
    await browser.goto('/admin/orders');
    await browser.screenshot('orders-page');
    
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);

  test('应该能够访问优惠券管理页面', async () => {
    await browser.goto('/admin/coupons');
    await browser.screenshot('coupons-page');
    
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);

  test('应该能够访问邀请管理页面', async () => {
    await browser.goto('/admin/invites');
    await browser.screenshot('invites-page');
    
    const url = browser.page.url();
    expect(url).toContain('/admin');
  }, 30000);
});
