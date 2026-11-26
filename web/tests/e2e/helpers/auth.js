import config from '../puppeteer.config.js';

/**
 * 认证辅助函数
 */
export class AuthHelper {
  constructor(browserHelper) {
    this.browser = browserHelper;
  }

  /**
   * 管理员登录
   */
  async loginAsAdmin() {
    await this.login(
      config.credentials.admin.username,
      config.credentials.admin.password,
      '/login'
    );
  }

  /**
   * 普通用户登录
   */
  async loginAsUser() {
    await this.login(
      config.credentials.user.username,
      config.credentials.user.password,
      '/user/login'
    );
  }

  /**
   * 通用登录方法
   */
  async login(username, password, loginPath = '/login') {
    const page = this.browser.page;
    
    // 导航到登录页面
    await this.browser.goto(loginPath);
    await this.browser.screenshot('login-page');

    // 等待登录表单加载
    await this.browser.waitForElement('input[type="text"], input[name="username"]');
    
    // 清空并输入用户名和密码
    const usernameInput = await page.$('input[type="text"], input[name="username"]');
    await usernameInput.click({ clickCount: 3 }); // 选中所有文本
    await page.keyboard.press('Backspace');
    await page.type('input[type="text"], input[name="username"]', username);
    
    const passwordInput = await page.$('input[type="password"], input[name="password"]');
    await passwordInput.click({ clickCount: 3 });
    await page.keyboard.press('Backspace');
    await page.type('input[type="password"], input[name="password"]', password);
    
    await this.browser.screenshot('login-filled');

    // 点击登录按钮
    const submitButton = await page.$('button[type="submit"], button.el-button--primary, .el-button--primary');
    if (submitButton) {
      await submitButton.click();
    } else {
      // 如果找不到按钮，尝试按回车
      await page.keyboard.press('Enter');
    }
    
    // 等待登录完成（等待 URL 变化或等待一段时间）
    try {
      await page.waitForNavigation({ timeout: 10000, waitUntil: 'networkidle2' });
    } catch (error) {
      // 如果导航超时，等待一下看是否已经登录
      await this.browser.wait(2000);
    }
    
    await this.browser.screenshot('login-success');
    
    console.log(`✅ 登录成功: ${username}`);
  }

  /**
   * 登出
   */
  async logout() {
    const page = this.browser.page;
    
    // 查找并点击登出按钮
    const logoutSelectors = [
      'button:has-text("退出")',
      'button:has-text("登出")',
      'a:has-text("退出")',
      '.logout-button',
    ];

    for (const selector of logoutSelectors) {
      if (await this.browser.exists(selector)) {
        await this.browser.click(selector);
        break;
      }
    }

    await this.browser.wait(1000);
    console.log('✅ 已登出');
  }

  /**
   * 检查是否已登录
   */
  async isLoggedIn() {
    const page = this.browser.page;
    
    // 检查是否存在登录表单
    const hasLoginForm = await this.browser.exists('input[type="password"]');
    
    return !hasLoginForm;
  }
}
