/**
 * API 基础配置和工具函数
 * 提供统一的 HTTP 客户端配置、错误处理和请求拦截
 */
import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '@/router'
import { cancelManager, deduplicator } from '@/utils/requestManager'
import { isNoEntitlementError, normalizeBackendErrorMessage } from '@/utils/entitlement'

// 错误码到本地化消息的映射
const ERROR_MESSAGES = {
  VALIDATION_ERROR: '输入数据验证失败',
  UNAUTHORIZED: '未授权访问，请重新登录',
  FORBIDDEN: '没有权限执行此操作',
  NOT_FOUND: '请求的资源不存在',
  CONFLICT: '资源冲突，请检查数据',
  RATE_LIMIT_EXCEEDED: '请求过于频繁，请稍后再试',
  INTERNAL_ERROR: '服务器内部错误',
  DATABASE_ERROR: '数据库操作失败',
  CACHE_ERROR: '缓存服务异常',
  XRAY_ERROR: 'Xray 服务异常',
  NETWORK_ERROR: '网络连接失败',
  TIMEOUT_ERROR: '请求超时',
  PAYMENT_ERROR: '创建支付失败，请稍后重试',
  PAYMENT_METHOD_UNAVAILABLE: '当前支付方式暂不可用，请选择其他支付方式',
  INSUFFICIENT_BALANCE: '余额不足，请先充值后再支付',
  ORDER_NOT_PAYABLE: '当前订单状态不支持继续支付，请返回订单页刷新后重试',
  RECHARGE_UNAVAILABLE: '当前未开启在线充值，请联系管理员或使用礼品卡充值',
  UNKNOWN_ERROR: '未知错误'
}

// HTTP 状态码到错误码的映射
const STATUS_TO_ERROR_CODE = {
  400: 'VALIDATION_ERROR',
  401: 'UNAUTHORIZED',
  403: 'FORBIDDEN',
  404: 'NOT_FOUND',
  409: 'CONFLICT',
  429: 'RATE_LIMIT_EXCEEDED',
  500: 'INTERNAL_ERROR',
  502: 'INTERNAL_ERROR',
  503: 'INTERNAL_ERROR',
  504: 'TIMEOUT_ERROR'
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
 * 生成唯一的请求追踪 ID
 * @returns {string} 请求 ID
 */
export function generateRequestId() {
  const timestamp = Date.now().toString(36)
  const random = Math.random().toString(36).substring(2, 8)
  return `REQ-${timestamp}-${random}`.toUpperCase()
}

/**
 * 获取错误消息
 * @param {string} code 错误码
 * @param {string} defaultMessage 默认消息
 * @returns {string} 本地化错误消息
 */
export function getErrorMessage(code, defaultMessage) {
  return ERROR_MESSAGES[code] || defaultMessage || ERROR_MESSAGES.UNKNOWN_ERROR
}

/**
 * 格式化 API 错误
 * @param {Error} error Axios 错误对象
 * @returns {Object} 格式化的错误对象
 */
export function formatApiError(error) {
  const errorId = generateErrorId()
  
  if (error.response) {
    const { status, data } = error.response
    const code =
      data?.code ||
      data?.error?.code ||
      STATUS_TO_ERROR_CODE[status] ||
      'UNKNOWN_ERROR'
    const message = normalizeBackendErrorMessage(
      data?.message ||
      data?.error?.message ||
      (typeof data?.error === 'string' ? data.error : '') ||
      getErrorMessage(code)
    ) || getErrorMessage(code)
    
    return {
      errorId,
      code,
      message,
      details: data?.details || data?.error?.details || {},
      requestId: data?.request_id,
      status,
      timestamp: new Date().toISOString()
    }
  }
  
  if (error.request) {
    return {
      errorId,
      code: 'NETWORK_ERROR',
      message: getErrorMessage('NETWORK_ERROR'),
      details: {},
      status: 0,
      timestamp: new Date().toISOString()
    }
  }
  
  return {
    errorId,
    code: 'UNKNOWN_ERROR',
    message: error.message || getErrorMessage('UNKNOWN_ERROR'),
    details: {},
    status: 0,
    timestamp: new Date().toISOString()
  }
}

// 创建 axios 实例
// baseURL honors panel_base_path when the panel is mounted under a prefix.
// The backend injects window.__VPANEL_BASE__ into index.html at boot.
const __panelBase = (typeof window !== 'undefined' && window.__VPANEL_BASE__) || ''
const api = axios.create({
  baseURL: import.meta.env.VITE_APP_API_URL || (__panelBase + '/api'),
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 请求拦截器
api.interceptors.request.use(
  config => {
    // 优先使用用户门户令牌，其次使用管理后台令牌
    const userToken = sessionStorage.getItem('userToken') || localStorage.getItem('userToken')
    const adminToken = sessionStorage.getItem('token') || localStorage.getItem('token')
    const token = userToken || adminToken
    
    if (token) {
      config.headers['Authorization'] = `Bearer ${token}`
    }
    
    // 添加请求ID用于追踪
    if (!config.headers['X-Request-ID']) {
      config.headers['X-Request-ID'] = generateRequestId()
    }
    
    // 支持请求取消
    if (config.cancelKey) {
      const controller = cancelManager.getController(config.cancelKey, config.cancelPrevious !== false)
      config.signal = controller.signal
    }
    
    return config
  },
  error => Promise.reject(error)
)

// 用于防止重复显示相同错误消息
let lastErrorMessage = ''
let lastErrorTime = 0

// 响应拦截器
api.interceptors.response.use(
  response => {
    // 清理取消控制器
    if (response.config.cancelKey) {
      cancelManager.remove(response.config.cancelKey)
    }
    return response.data
  },
  error => {
    // 清理取消控制器
    if (error.config?.cancelKey) {
      cancelManager.remove(error.config.cancelKey)
    }
    
    // 如果是取消的请求，不显示错误
    if (axios.isCancel(error) || error.name === 'AbortError') {
      return Promise.reject({ cancelled: true })
    }
    
    const formattedError = formatApiError(error)
    const currentPathname = typeof window !== 'undefined' ? window.location.pathname : ''
    const isUserPortalEntitlementError = currentPathname.startsWith('/user') && isNoEntitlementError(formattedError)

    if (isUserPortalEntitlementError) {
      formattedError.silent = true
      formattedError.isEntitlementError = true
    }
    
    // 保留原始响应数据（用于获取详细错误信息，如部署日志）
    if (error.response?.data) {
      formattedError.response = {
        data: error.response.data
      }
    }
    
    // 处理特定状态码
    if (error.response?.status === 401) {
      // 根据当前路径决定清除哪个令牌和跳转到哪个登录页
      const currentPath = window.location.pathname
      
      // 避免在登录页重复跳转和清理
      const isLoginPage = currentPath.includes('/login')
      
      if (!isLoginPage) {
        if (currentPath.startsWith('/user')) {
          // 用户门户 - 清除用户令牌，跳转到用户登录页
          const userToken = sessionStorage.getItem('userToken') || localStorage.getItem('userToken')
          const adminToken = sessionStorage.getItem('token') || localStorage.getItem('token')
          sessionStorage.removeItem('userToken')
          localStorage.removeItem('userToken')
          sessionStorage.removeItem('userInfo')
          localStorage.removeItem('userInfo')
          if (userToken && adminToken && userToken === adminToken) {
            sessionStorage.removeItem('token')
            localStorage.removeItem('token')
            sessionStorage.removeItem('adminUserInfo')
            localStorage.removeItem('adminUserInfo')
            sessionStorage.removeItem('adminRole')
            localStorage.removeItem('adminRole')
          }
          
          // 使用 replace 避免产生历史记录
          router.replace('/user/login')
        } else if (currentPath.startsWith('/admin')) {
          // 管理后台 - 清除管理员令牌，跳转到管理员登录页
          sessionStorage.removeItem('token')
          localStorage.removeItem('token')
          sessionStorage.removeItem('adminRole')
          localStorage.removeItem('adminRole')
          sessionStorage.removeItem('adminUserInfo')
          localStorage.removeItem('adminUserInfo')
          
          // 当前没有独立的后台登录页，统一回到用户登录页并保留目标地址
          router.replace('/user/login?redirect=' + encodeURIComponent(currentPath))
        } else {
          // 其他路径 - 默认清除所有令牌，跳转到用户登录页
          sessionStorage.removeItem('token')
          localStorage.removeItem('token')
          sessionStorage.removeItem('userToken')
          localStorage.removeItem('userToken')
          sessionStorage.removeItem('adminRole')
          localStorage.removeItem('adminRole')
          sessionStorage.removeItem('adminUserInfo')
          localStorage.removeItem('adminUserInfo')
          
          router.replace('/user/login')
        }
      }
      
      // 401 错误完全静默处理，不显示任何消息，直接返回
      // 标记为 silent 避免后续处理显示错误
      return Promise.reject({ ...formattedError, silent: true, code: 'UNAUTHORIZED' })
    }
    
    // 只在非静默模式下显示错误消息
    // 401 错误已经在上面标记为 silent，这里会被跳过
    if (!error.config?.silent && !formattedError.silent) {
      const isUserPortalPath = currentPathname.startsWith('/user')
      const displayMessage = isUserPortalPath
        ? formattedError.message
        : `${formattedError.message} (${formattedError.errorId})`
      const now = Date.now()
      
      // 防止 1 秒内显示相同的错误消息
      if (displayMessage !== lastErrorMessage || now - lastErrorTime > 1000) {
        ElMessage.error(displayMessage)
        lastErrorMessage = displayMessage
        lastErrorTime = now
      }
    }
    
    // 返回格式化的错误
    return Promise.reject(formattedError)
  }
)

/**
 * 创建可取消的请求
 * @param {string} cancelKey 取消标识
 * @param {Object} config 请求配置
 * @returns {Promise}
 */
export function cancellableRequest(cancelKey, config) {
  return api({
    ...config,
    cancelKey
  })
}

/**
 * 创建去重请求
 * @param {Object} config 请求配置
 * @returns {Promise}
 */
export function deduplicatedRequest(config) {
  const key = deduplicator.generateKey(
    config.method || 'GET',
    config.url,
    config.params || config.data || {}
  )
  return deduplicator.execute(key, () => api(config))
}

/**
 * 取消指定请求
 * @param {string} cancelKey 取消标识
 */
export function cancelRequest(cancelKey) {
  cancelManager.cancel(cancelKey)
}

/**
 * 取消所有请求
 */
export function cancelAllRequests() {
  cancelManager.cancelAll()
}

export default api
