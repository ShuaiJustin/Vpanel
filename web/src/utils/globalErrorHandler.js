/**
 * 全局错误处理器
 * 捕获未处理的 JavaScript 错误和 Promise 拒绝
 */

import { ElNotification } from 'element-plus'
import { generateErrorId, reportError } from './errorHandler'

// 错误队列，防止重复显示
const errorCache = new Map()
const ERROR_CACHE_TTL = 5000 // 5秒内相同错误不重复显示

/**
 * 生成错误指纹
 * @param {Error} error 错误对象
 * @returns {string} 错误指纹
 */
function getErrorFingerprint(error) {
  return `${error.name}:${error.message}:${error.stack?.split('\n')[1] || ''}`
}

/**
 * 检查是否应该显示错误
 * @param {Error} error 错误对象
 * @returns {boolean} 是否显示
 */
function shouldShowError(error) {
  const fingerprint = getErrorFingerprint(error)
  const now = Date.now()
  
  // 清理过期缓存
  for (const [key, timestamp] of errorCache.entries()) {
    if (now - timestamp > ERROR_CACHE_TTL) {
      errorCache.delete(key)
    }
  }
  
  // 检查是否重复
  if (errorCache.has(fingerprint)) {
    return false
  }
  
  errorCache.set(fingerprint, now)
  return true
}

/**
 * 格式化错误堆栈
 * @param {string} stack 错误堆栈
 * @returns {string} 格式化后的堆栈
 */
function formatStack(stack) {
  if (!stack) return ''
  
  return stack
    .split('\n')
    .slice(0, 5) // 只取前5行
    .map(line => line.trim())
    .join('\n')
}

/**
 * 处理 Vue 错误
 * @param {Error} error 错误对象
 * @param {Object} vm Vue 实例
 * @param {string} info 错误信息
 */
export function handleVueError(error, vm, info) {
  const errorId = generateErrorId()
  
  console.error(`[${errorId}] Vue Error:`, error)
  console.error('Component:', vm?.$options?.name || vm?.$options?.__name || 'Unknown')
  console.error('Info:', info)
  
  if (!shouldShowError(error)) {
    return
  }
  
  // 上报错误
  reportError({
    errorId,
    type: 'vue',
    message: error.message,
    stack: error.stack,
    component: vm?.$options?.name || vm?.$options?.__name,
    info
  })
  
  // 显示用户友好的错误消息
  ElNotification({
    title: '应用错误',
    message: `发生了一个错误\n错误ID: ${errorId}`,
    type: 'error',
    duration: 0,
    showClose: true
  })
}

/**
 * 处理未捕获的 JavaScript 错误
 * @param {ErrorEvent} event 错误事件
 */
export function handleGlobalError(event) {
  const { error, message, filename, lineno, colno } = event
  const errorId = generateErrorId()
  
  console.error(`[${errorId}] Uncaught Error:`, error || message)
  console.error('Location:', `${filename}:${lineno}:${colno}`)
  
  // 忽略某些错误
  if (shouldIgnoreError(message)) {
    return
  }
  
  const errorObj = error || new Error(message)
  if (!shouldShowError(errorObj)) {
    return
  }
  
  // 上报错误
  reportError({
    errorId,
    type: 'uncaught',
    message: message,
    stack: error?.stack,
    filename,
    lineno,
    colno
  })
  
  // 显示用户友好的错误消息
  ElNotification({
    title: '应用错误',
    message: `发生了一个错误\n错误ID: ${errorId}`,
    type: 'error',
    duration: 0,
    showClose: true
  })
}

/**
 * 处理未处理的 Promise 拒绝
 * @param {PromiseRejectionEvent} event 拒绝事件
 */
export function handleUnhandledRejection(event) {
  const { reason } = event
  const errorId = generateErrorId()
  
  console.error(`[${errorId}] Unhandled Promise Rejection:`, reason)

  // 已在 API 拦截器中处理的预期错误（如 401）不再上报
  if (reason?.silent || reason?.code === 'UNAUTHORIZED' || reason?.status === 401) {
    return
  }
  
  // 忽略某些错误
  const message = reason?.message || String(reason)
  if (shouldIgnoreError(message)) {
    return
  }
  
  const errorObj = reason instanceof Error ? reason : new Error(message)
  if (!shouldShowError(errorObj)) {
    return
  }
  
  // 上报错误
  reportError({
    errorId,
    type: 'unhandledRejection',
    message: message,
    stack: reason?.stack
  })
  
  // 显示用户友好的错误消息
  ElNotification({
    title: '请求错误',
    message: `操作失败\n错误ID: ${errorId}`,
    type: 'error',
    duration: 5000,
    showClose: true
  })
}

/**
 * 判断是否应该忽略错误
 * @param {string} message 错误消息
 * @returns {boolean} 是否忽略
 */
function shouldIgnoreError(message) {
  if (!message) return false
  
  const ignorePatterns = [
    // 网络错误（通常已有专门处理）
    'Network Error',
    'Failed to fetch',
    'Load failed',
    // 取消的请求
    'canceled',
    'aborted',
    'CanceledError',
    // 已由路由/拦截器处理的鉴权错误
    '未授权访问，请重新登录',
    'unauthorized',
    // ResizeObserver 错误（浏览器 bug）
    'ResizeObserver loop',
    // 脚本加载错误
    'Script error',
    // 扩展相关错误
    'Extension context invalidated'
  ]
  
  return ignorePatterns.some(pattern => 
    message.toLowerCase().includes(pattern.toLowerCase())
  )
}

/**
 * 安装全局错误处理器
 * @param {Object} app Vue 应用实例
 */
export function installGlobalErrorHandler(app) {
  // Vue 错误处理
  app.config.errorHandler = handleVueError
  
  // Vue 警告处理（仅开发环境）
  if (import.meta.env.DEV) {
    app.config.warnHandler = (msg, vm, trace) => {
      console.warn('[Vue Warning]:', msg)
      if (trace) {
        console.warn('Trace:', trace)
      }
    }
  }
  
  // 全局 JavaScript 错误
  window.addEventListener('error', handleGlobalError)
  
  // 未处理的 Promise 拒绝
  window.addEventListener('unhandledrejection', handleUnhandledRejection)
  
  console.log('[GlobalErrorHandler] Installed')
}

/**
 * 卸载全局错误处理器
 */
export function uninstallGlobalErrorHandler() {
  window.removeEventListener('error', handleGlobalError)
  window.removeEventListener('unhandledrejection', handleUnhandledRejection)
  console.log('[GlobalErrorHandler] Uninstalled')
}

export default {
  handleVueError,
  handleGlobalError,
  handleUnhandledRejection,
  installGlobalErrorHandler,
  uninstallGlobalErrorHandler
}
