#!/usr/bin/env node

/**
 * 快速测试脚本 - 验证 Puppeteer 设置是否正确
 */
import puppeteer from 'puppeteer';

console.log('🚀 开始快速测试...\n');

(async () => {
  let browser;
  
  try {
    // 启动浏览器
    console.log('📦 正在启动浏览器...');
    browser = await puppeteer.launch({
      headless: process.argv.includes('--headed') ? false : true,
      args: ['--no-sandbox', '--disable-setuid-sandbox'],
    });
    console.log('✅ 浏览器启动成功\n');

    // 创建新页面
    const page = await browser.newPage();
    await page.setViewport({ width: 1920, height: 1080 });

    // 测试 1: 访问首页
    console.log('🔍 测试 1: 访问首页');
    const baseURL = process.env.BASE_URL || 'http://localhost:8080';
    console.log(`   URL: ${baseURL}`);
    
    try {
      await page.goto(baseURL, { waitUntil: 'networkidle2', timeout: 10000 });
      const title = await page.title();
      console.log(`   ✅ 成功访问，页面标题: ${title}\n`);
    } catch (error) {
      console.log(`   ❌ 访问失败: ${error.message}`);
      console.log(`   💡 提示: 请确保应用正在运行在 ${baseURL}\n`);
    }

    // 测试 2: 访问登录页
    console.log('🔍 测试 2: 访问登录页');
    try {
      await page.goto(`${baseURL}/login`, { waitUntil: 'networkidle2', timeout: 10000 });
      
      // 检查登录表单元素
      const hasUsername = await page.$('input[type="text"], input[name="username"]') !== null;
      const hasPassword = await page.$('input[type="password"]') !== null;
      
      console.log(`   用户名输入框: ${hasUsername ? '✅' : '❌'}`);
      console.log(`   密码输入框: ${hasPassword ? '✅' : '❌'}`);
      
      if (hasUsername && hasPassword) {
        console.log('   ✅ 登录页面检查通过\n');
      } else {
        console.log('   ⚠️  登录页面元素不完整\n');
      }
    } catch (error) {
      console.log(`   ❌ 访问失败: ${error.message}\n`);
    }

    // 测试 3: 截图功能
    console.log('🔍 测试 3: 截图功能');
    try {
      const screenshotPath = './tests/e2e/screenshots/quick-test.png';
      await page.screenshot({ path: screenshotPath, fullPage: true });
      console.log(`   ✅ 截图已保存: ${screenshotPath}\n`);
    } catch (error) {
      console.log(`   ❌ 截图失败: ${error.message}\n`);
    }

    console.log('🎉 快速测试完成！\n');
    console.log('📋 下一步:');
    console.log('   1. 运行完整测试: npm run test:e2e');
    console.log('   2. 运行特定测试: npm run test:e2e tests/basic-check.test.js');
    console.log('   3. 有头模式运行: npm run test:e2e:headed\n');

  } catch (error) {
    console.error('❌ 测试失败:', error.message);
    process.exit(1);
  } finally {
    if (browser) {
      await browser.close();
    }
  }
})();
