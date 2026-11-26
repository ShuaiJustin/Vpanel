import puppeteer from 'puppeteer';
import config from '../puppeteer.config.js';

/**
 * 浏览器辅助类
 */
export class BrowserHelper {
  constructor() {
    this.browser = null;
    this.page = null;
  }

  /**
   * 启动浏览器
   */
  async launch() {
    this.browser = await puppeteer.launch(config.browser);
    this.page = await this.browser.newPage();
    
    // 设置视口大小
    await this.page.setViewport({ width: 1920, height: 1080 });
    
    // 设置默认超时
    this.page.setDefaultNavigationTimeout(config.timeout.navigation);
    this.page.setDefaultTimeout(config.timeout.element);
    
    return this.page;
  }

  /**
   * 关闭浏览器
   */
  async close() {
    if (this.browser) {
      await this.browser.close();
    }
  }

  /**
   * 导航到指定路径
   */
  async goto(path) {
    const url = `${config.baseURL}${path}`;
    await this.page.goto(url, { waitUntil: 'networkidle2' });
  }

  /**
   * 截图
   */
  async screenshot(name) {
    if (config.screenshot.enabled) {
      const path = `${config.screenshot.path}/${name}-${Date.now()}.png`;
      await this.page.screenshot({ 
        path, 
        fullPage: config.screenshot.fullPage 
      });
      console.log(`📸 截图已保存: ${path}`);
    }
  }

  /**
   * 等待元素出现
   */
  async waitForElement(selector, timeout = config.timeout.element) {
    return await this.page.waitForSelector(selector, { timeout });
  }

  /**
   * 点击元素
   */
  async click(selector) {
    await this.waitForElement(selector);
    await this.page.click(selector);
  }

  /**
   * 输入文本
   */
  async type(selector, text) {
    await this.waitForElement(selector);
    await this.page.type(selector, text);
  }

  /**
   * 获取文本内容
   */
  async getText(selector) {
    await this.waitForElement(selector);
    return await this.page.$eval(selector, el => el.textContent);
  }

  /**
   * 等待导航完成
   */
  async waitForNavigation() {
    await this.page.waitForNavigation({ waitUntil: 'networkidle2' });
  }

  /**
   * 执行 JavaScript
   */
  async evaluate(fn, ...args) {
    return await this.page.evaluate(fn, ...args);
  }

  /**
   * 检查元素是否存在
   */
  async exists(selector) {
    try {
      await this.page.waitForSelector(selector, { timeout: 1000 });
      return true;
    } catch {
      return false;
    }
  }

  /**
   * 等待指定时间
   */
  async wait(ms) {
    await new Promise(resolve => setTimeout(resolve, ms));
  }
}
