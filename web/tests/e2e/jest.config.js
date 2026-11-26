/**
 * Jest 配置用于 Puppeteer E2E 测试
 */
export default {
  testEnvironment: 'node',
  testMatch: ['**/tests/e2e/tests/**/*.test.js'],
  testPathIgnorePatterns: ['/node_modules/', '/src/'],
  testTimeout: 60000, // 60 秒超时
  verbose: true,
  bail: false, // 不在第一个失败时停止
  maxWorkers: 1, // 串行运行测试
  transform: {},
  extensionsToTreatAsEsm: ['.js'],
  moduleNameMapper: {
    '^(\\.{1,2}/.*)\\.js$': '$1',
  },
};
