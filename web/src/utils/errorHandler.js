/**
 * 前端错误处理工具
 * 提供统一的错误处理、错误码映射和用户友好消息
 */

import { ElMessage, ElMessageBox, ElNotification } from 'element-plus'

// 错误码到本地化消息的映射
const ERROR_CODE_MESSAGES = {
  // 通用错误
  VALIDATION_ERROR: '输入数据验证失败，请检查填写的内容',
  UNAUTHORIZED: '登录已过期，请重新登录',
  FORBIDDEN: '您没有权限执行此操作',
  NOT_FOUND: '请求的资源不存在',
  CONFLICT: '数据冲突，该资源可能已被修改',
  RATE_LIMIT_EXCEEDED: '操作过于频繁，请稍后再试',
  INTERNAL_ERROR: '服务器内部错误，请稍后重试',
  DATABASE_ERROR: '数据库操作失败，请稍后重试',
  CACHE_ERROR: '缓存服务异常',
  XRAY_ERROR: 'Xray 服务异常',
  NETWORK_ERROR: '网络连接失败，请检查网络',
  TIMEOUT_ERROR: '请求超时，请稍后重试',
  UNKNOWN_ERROR: '发生未知错误',
  
  // 认证相关
  INVALID_CREDENTIALS: '用户名或密码错误',
  ACCOUNT_DISABLED: '账户已被禁用',
  ACCOUNT_EXPIRED: '账户已过期',
  TOKEN_EXPIRED: '登录已过期，请重新登录',
  TOKEN_INVALID: '无效的登录凭证',
  PASSWORD_WEAK: '密码强度不足',
  
  // 用户相关
  USER_NOT_FOUND: '用户不存在',
  USERNAME_EXISTS: '用户名已存在',
  EMAIL_EXISTS: '邮箱已被使用',
  EMAIL_INVALID: '邮箱格式不正确',
  TRAFFIC_EXCEEDED: '流量已用尽',
  
  // 代理相关
  PROXY_NOT_FOUND: '代理不存在',
  PORT_CONFLICT: '端口已被占用',
  PROTOCOL_INVALID: '无效的协议类型',
  CONFIG_INVALID: '配置格式错误',
  
  // 角色相关
  ROLE_NOT_FOUND: '角色不存在',
  ROLE_SYSTEM_PROTECTED: '系统角色不可修改',
  PERMISSION_INVALID: '无效的权限',
  
  // Xray 相关
  XRAY_NOT_RUNNING: 'Xray 服务未运行',
  XRAY_CONFIG_INVALID: 'Xray 配置无效',
  XRAY_RESTART_FAILED: 'Xray 重启失败'
}

// 错误严重程度
const ERROR_SEVERITY = {
  VALIDATION_ERROR: 'warning',
  UNAUTHORIZED: 'error',
  FORBIDDEN: 'warning',
  NOT_FOUND: 'warning',
  CONFLICT: 'warning',
  RATE_LIMIT_EXCEEDED: 'warning',
  INTERNAL_ERROR: 'error',
  DATABASE_ERROR: 'error',
  NETWORK_ERROR: 'error',
  TIMEOUT_ERROR: 'warning'
}

/**
 * 生成唯一的错误 ID
 * @returns {string} 错误 ID
 */
export function generateErrorId() {
  const timestamp = Date.now().toString(36)
  const random = Math.random().toString(36).substring(2, 8)
  return `ERR-${timestamp}-${random}`.toUpperCase()
}

/**
 * 获取错误消息
 * @param {string} code 错误码
 * @param {string} defaultMessage 默认消息
 * @returns {string} 本地化错误消息
 */
export function getErrorMessage(code, defaultMessage) {
  return ERROR_CODE_MESSAGES[code] || defaultMessage || ERROR_CODE_MESSAGES.UNKNOWN_ERROR
}

/**
 * 获取错误严重程度
 * @param {string} code 错误码
 * @returns {string} 严重程度
 */
export function getErrorSeverity(code) {
  return ERROR_SEVERITY[code] || 'error'
}

/**
 * 格式化字段验证错误
 * @param {Object} details 错误详情
 * @returns {string} 格式化的错误消息
 */
export function formatValidationErrors(details) {
  if (!details || !details.fields) {
    return '输入数据验证失败'
  }
  
  const errors = Object.entries(details.fields)
    .map(([field, message]) => `${field}: ${message}`)
    .join('\n')
  
  return errors || '输入数据验证失败'
}

/**
 * 显示错误消息
 * @param {Object} error 错误对象
 * @param {Object} options 选项
 */
export function showError(error, options = {}) {
  const {
    showId = true,
    duration = 5000,
    type = 'message' // 'message', 'notification', 'modal'
  } = options
  
  const errorId = error.errorId || generateErrorId()
  const message = getErrorMessage(error.code, error.message)
  const severity = getErrorSeverity(error.code)
  
  let displayMessage = message
  if (showId) {
    displayMessage = `${message}\n错误ID: ${errorId}`
  }
  
  switch (type) {
    case 'notification':
      ElNotification({
        title: '错误',
        message: displayMessage,
        type: severity,
        duration
      })
      break
      
    case 'modal':
      ElMessageBox.alert(displayMessage, '错误', {
        type: severity,
        confirmButtonText: '确定'
      })
      break
      
    default:
      ElMessage({
        message: displayMessage,
        type: severity,
        duration,
        showClose: true
      })
  }
  
  return errorId
}

/**
 * 显示验证错误
 * @param {Object} error 错误对象
 */
export function showValidationError(error) {
  if (error.code === 'VALIDATION_ERROR' && error.details?.fields) {
    const message = formatValidationErrors(error.details)
    ElMessage({
      message,
      type: 'warning',
      duration: 5000,
      showClose: true
    })
  } else {
    showError(error)
  }
}

/**
 * 显示网络错误
 * @param {boolean} isOffline 是否离线
 */
export function showNetworkError(isOffline = false) {
  const message = isOffline 
    ? '网络连接已断开，请检查网络设置'
    : '网络请求失败，请稍后重试'
  
  ElNotification({
    title: '网络错误',
    message,
    type: 'error',
    duration: 0, // 不自动关闭
    showClose: true
  })
}

/**
 * 显示会话过期提示
 * @param {Function} onRelogin 重新登录回调
 */
export function showSessionExpired(onRelogin) {
  ElMessageBox.confirm(
    '您的登录已过期，请重新登录',
    '会话过期',
    {
      confirmButtonText: '重新登录',
      cancelButtonText: '取消',
      type: 'warning'
    }
  ).then(() => {
    if (onRelogin) {
      onRelogin()
    }
  }).catch(() => {
    // 用户取消
  })
}

/**
 * 错误队列管理器
 * 防止同时显示过多错误消息
 */
class ErrorQueue {
  constructor(maxSize = 3, interval = 1000) {
    this.queue = []
    this.maxSize = maxSize
    this.interval = interval
    this.processing = false
    this.displayedCount = 0
  }
  
  /**
   * 添加错误到队列
   * @param {Object} error 错误对象
   * @param {Object} options 显示选项
   */
  add(error, options) {
    // 如果队列已满，移除最旧的错误
    if (this.queue.length >= this.maxSize) {
      this.queue.shift()
    }
    
    // 检查是否有重复错误
    const isDuplicate = this.queue.some(
      item => item.error.code === error.code && item.error.message === error.message
    )
    
    if (!isDuplicate) {
      this.queue.push({ error, options, timestamp: Date.now() })
      this.process()
    }
  }
  
  /**
   * 处理队列中的错误
   */
  process() {
    if (this.processing || this.queue.length === 0) {
      return
    }
    
    this.processing = true
    const { error, options } = this.queue.shift()
    showError(error, options)
    this.displayedCount++
    
    setTimeout(() => {
      this.processing = false
      this.process()
    }, this.interval)
  }
  
  /**
   * 清空队列
   */
  clear() {
    this.queue = []
    this.processing = false
  }
  
  /**
   * 获取队列长度
   * @returns {number} 队列长度
   */
  get length() {
    return this.queue.length
  }
  
  /**
   * 检查队列是否为空
   * @returns {boolean} 是否为空
   */
  get isEmpty() {
    return this.queue.length === 0
  }
  
  /**
   * 获取已显示的错误数量
   * @returns {number} 已显示数量
   */
  get displayed() {
    return this.displayedCount
  }
  
  /**
   * 重置统计
   */
  resetStats() {
    this.displayedCount = 0
  }
}
    
// 导出错误队列实例
export const errorQueue = new ErrorQueue()

/**
 * 全局错误处理器
 * @param {Error} error 错误对象
 * @param {Object} vm Vue 实例
 * @param {string} info 错误信息
 */
export function globalErrorHandler(error, vm, info) {
  const errorId = generateErrorId()
  
  console.error(`[${errorId}] Vue Error:`, error)
  console.error('Component:', vm?.$options?.name || 'Unknown')
  console.error('Info:', info)
  
  // 上报错误到后端（可选）
  reportError({
    errorId,
    message: error.message,
    stack: error.stack,
    component: vm?.$options?.name,
    info
  })
  
  // 显示用户友好的错误消息
  ElNotification({
    title: '应用错误',
    message: `发生了一个错误，错误ID: ${errorId}`,
    type: 'error',
    duration: 0
  })
}

/**
 * 错误上报队列
 * 批量上报错误以减少网络请求
 */
const errorReportQueue = []
let reportTimer = null
const REPORT_BATCH_SIZE = 10
const REPORT_INTERVAL = 5000 // 5秒

/**
 * 上报错误到后端
 * @param {Object} errorData 错误数据
 */
export async function reportError(errorData) {
  // 添加通用信息
  const enrichedData = {
    ...errorData,
    url: window.location.href,
    userAgent: navigator.userAgent,
    timestamp: new Date().toISOString(),
    sessionId: getSessionId(),
    viewport: {
      width: window.innerWidth,
      height: window.innerHeight
    }
  }
  
  // 开发环境只打印日志
  if (import.meta.env.DEV) {
    console.log('Error report (dev mode):', enrichedData)
    return
  }
  
  // 添加到队列
  errorReportQueue.push(enrichedData)
  
  // 如果队列满了，立即发送
  if (errorReportQueue.length >= REPORT_BATCH_SIZE) {
    flushErrorReports()
  } else if (!reportTimer) {
    // 否则设置定时器
    reportTimer = setTimeout(flushErrorReports, REPORT_INTERVAL)
  }
}

/**
 * 获取会话 ID
 * @returns {string} 会话 ID
 */
function getSessionId() {
  let sessionId = sessionStorage.getItem('error_session_id')
  if (!sessionId) {
    sessionId = generateErrorId()
    sessionStorage.setItem('error_session_id', sessionId)
  }
  return sessionId
}

/**
 * 发送错误报告
 */
async function flushErrorReports() {
  if (reportTimer) {
    clearTimeout(reportTimer)
    reportTimer = null
  }
  
  if (errorReportQueue.length === 0) {
    return
  }
  
  const errors = errorReportQueue.splice(0, REPORT_BATCH_SIZE)
  
  try {
    await fetch('/api/errors/report', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        errors,
        batchId: generateErrorId(),
        reportedAt: new Date().toISOString()
      })
    })
  } catch (e) {
    console.error('Failed to report errors:', e)
    // 失败时将错误放回队列（最多保留 50 条）
    if (errorReportQueue.length < 50) {
      errorReportQueue.unshift(...errors)
    }
  }
}

// 页面卸载时发送剩余错误
if (typeof window !== 'undefined') {
  window.addEventListener('beforeunload', () => {
    if (errorReportQueue.length > 0 && !import.meta.env.DEV) {
      // 使用 sendBeacon 确保数据发送
      const data = JSON.stringify({
        errors: errorReportQueue,
        batchId: generateErrorId(),
        reportedAt: new Date().toISOString()
      })
      navigator.sendBeacon('/api/errors/report', data)
    }
  })
}

export default {
  generateErrorId,
  getErrorMessage,
  getErrorSeverity,
  formatValidationErrors,
  showError,
  showValidationError,
  showNetworkError,
  showSessionExpired,
  errorQueue,
  globalErrorHandler,
  reportError
}
