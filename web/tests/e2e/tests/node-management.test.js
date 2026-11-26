import { BrowserHelper } from '../helpers/browser.js';
import { AuthHelper } from '../helpers/auth.js';

/**
 * 节点管理测试
 */
describe('节点管理测试', () => {
  let browser;
  let auth;

  beforeAll(async () => {
    browser = new BrowserHelper();
    await browser.launch();
    auth = new AuthHelper(browser);
    
    // 登录管理后台
    await auth.loginAsAdmin();
  });

  afterAll(async () => {
    await browser.close();
  });

  test('应该能够访问节点管理页面', async () => {
    // 导航到节点管理
    await browser.goto('/admin/nodes');
    await browser.screenshot('nodes-page');
    
    // 验证页面元素
    const url = browser.page.url();
    expect(url).toContain('/admin/nodes');
  });

  test('应该显示节点列表', async () => {
    await browser.goto('/admin/nodes');
    
    // 等待表格加载
    await browser.waitForElement('table, .el-table, .node-list');
    await browser.screenshot('nodes-list');
    
    const hasTable = await browser.exists('table, .el-table');
    expect(hasTable).toBe(true);
  });

  test('应该能够打开添加节点对话框', async () => {
    await browser.goto('/admin/nodes');
    
    // 查找并点击添加按钮
    const addButtonSelectors = [
      'button:has-text("添加")',
      'button:has-text("新增")',
      '.add-button',
      '.el-button--primary',
    ];
    
    for (const selector of addButtonSelectors) {
      if (await browser.exists(selector)) {
        await browser.click(selector);
        break;
      }
    }
    
    await browser.wait(500);
    await browser.screenshot('add-node-dialog');
    
    // 验证对话框出现
    const hasDialog = await browser.exists('.el-dialog, .dialog, [role="dialog"]');
    expect(hasDialog).toBe(true);
  });
});
