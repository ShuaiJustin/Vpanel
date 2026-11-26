#!/usr/bin/env node

/**
 * Puppeteer E2E 测试运行器
 */
import { spawn } from 'child_process';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';
import fs from 'fs';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

// 确保截图目录存在
const screenshotDir = join(__dirname, 'screenshots');
if (!fs.existsSync(screenshotDir)) {
  fs.mkdirSync(screenshotDir, { recursive: true });
}

// 解析命令行参数
const args = process.argv.slice(2);
const headless = !args.includes('--headed');
const testPattern = args.find(arg => !arg.startsWith('--')) || 'tests/e2e/tests';

console.log('🚀 启动 Puppeteer E2E 测试...\n');
console.log(`📋 测试模式: ${headless ? '无头模式' : '有头模式'}`);
console.log(`📁 测试文件: ${testPattern}\n`);

// 设置环境变量
const env = {
  ...process.env,
  HEADLESS: headless.toString(),
  NODE_ENV: 'test',
};

// 运行测试
const testProcess = spawn('node', [
  '--experimental-vm-modules',
  'node_modules/jest/bin/jest.js',
  testPattern,
  '--runInBand',
  '--verbose',
], {
  cwd: join(__dirname, '../..'),
  env,
  stdio: 'inherit',
});

testProcess.on('exit', (code) => {
  if (code === 0) {
    console.log('\n✅ 所有测试通过！');
  } else {
    console.log('\n❌ 测试失败');
    process.exit(code);
  }
});
