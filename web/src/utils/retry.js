/**
 * 重试工具
 * 提供指数退避重试逻辑
 */

/**
 * 默认重试配置
 */
const DEFAULT_CONFIG = {
  maxRetries: 3,
  baseDelay: 1000,
  maxDelay: 30000,
  backoffFactor: 2,
  retryCondition: (error) => {
    // 默认重试网络错误和服务器错误
    if (error.code === 'NETWORK_ERROR') return true
    if (error.status >= 500) return true
    if (error.status === 429) return true // Rate limit
    return false
  }
}

/**
 * 计算退避延迟
 * @param {number} attempt 当前尝试次数
 * @param {Object} config 配置
 * @returns {number} 延迟毫秒数
 */
function calculateDelay(attempt, config) {
  const delay = config.baseDelay * Math.pow(config.backoffFactor, attempt)
  // 添加随机抖动 (±10%)
  const jitter = delay * 0.1 * (Math.random() * 2 - 1)
  return Math.min(delay + jitter, config.maxDelay)
}

/**
 * 延迟执行
 * @param {number} ms 毫秒数
 * @returns {Promise}
 */
function delay(ms) {
  return new Promise(resolve => setTimeout(resolve, ms))
}

/**
 * 带重试的异步函数执行
 * @param {Function} fn 要执行的异步函数
 * @param {Object} options 配置选项
 * @returns {Promise}
 */
export async function withRetry(fn, options = {}) {
  const config = { ...DEFAULT_CONFIG, ...options }
  let lastError
  
  for (let attempt = 0; attempt <= config.maxRetries; attempt++) {
    try {
      return await fn()
    } catch (error) {
      lastError = error
      
      // 检查是否应该重试
      if (attempt < config.maxRetries && config.retryCondition(error)) {
        const waitTime = calculateDelay(attempt, config)
        
        // 触发重试回调
        if (config.onRetry) {
          config.onRetry(error, attempt + 1, waitTime)
        }
        
        await delay(waitTime)
      } else {
        break
      }
    }
  }
  
  throw lastError
}

/**
 * 创建带重试的函数
 * @param {Function} fn 原始函数
 * @param {Object} options 配置选项
 * @returns {Function} 带重试的函数
 */
export function createRetryableFunction(fn, options = {}) {
  return (...args) => withRetry(() => fn(...args), options)
}

/**
 * 重试装饰器（用于类方法）
 * @param {Object} options 配置选项
 * @returns {Function} 装饰器函数
 */
export function retryable(options = {}) {
  return function(target, propertyKey, descriptor) {
    const originalMethod = descriptor.value
    
    descriptor.value = function(...args) {
      return withRetry(() => originalMethod.apply(this, args), options)
    }
    
    return descriptor
  }
}

/**
 * API 请求重试配置
 */
export const API_RETRY_CONFIG = {
  maxRetries: 3,
  baseDelay: 1000,
  maxDelay: 10000,
  backoffFactor: 2,
  retryCondition: (error) => {
    // 不重试认证错误
    if (error.status === 401 || error.status === 403) return false
    // 不重试验证错误
    if (error.status === 400) return false
    // 不重试资源不存在
    if (error.status === 404) return false
    // 重试网络错误
    if (error.code === 'NETWORK_ERROR') return true
    // 重试服务器错误
    if (error.status >= 500) return true
    // 重试限流
    if (error.status === 429) return true
    return false
  }
}

export default {
  withRetry,
  createRetryableFunction,
  retryable,
  API_RETRY_CONFIG
}
