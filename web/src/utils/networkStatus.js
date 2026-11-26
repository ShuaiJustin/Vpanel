/**
 * 网络状态监控
 * 提供网络状态检测、离线指示器和自动重试功能
 */

import { ref, readonly } from 'vue'
import { ElNotification } from 'element-plus'

// 网络状态
const isOnline = ref(navigator.onLine)
const lastOnlineTime = ref(Date.now())
const connectionType = ref(getConnectionType())

// 离线通知实例
let offlineNotification = null

/**
 * 获取连接类型
 * @returns {string}
 */
function getConnectionType() {
  if ('connection' in navigator) {
    return navigator.connection.effectiveType || 'unknown'
  }
  return 'unknown'
}

/**
 * 处理上线事件
 */
function handleOnline() {
  isOnline.value = true
  lastOnlineTime.value = Date.now()
  connectionType.value = getConnectionType()
  
  // 关闭离线通知
  if (offlineNotification) {
    offlineNotification.close()
    offlineNotification = null
  }
  
  // 显示恢复通知
  ElNotification({
    title: '网络已恢复',
    message: '网络连接已恢复，正在同步数据...',
    type: 'success',
    duration: 3000
  })
  
  // 触发自定义事件
  window.dispatchEvent(new CustomEvent('network-online'))
}

/**
 * 处理离线事件
 */
function handleOffline() {
  isOnline.value = false
  
  // 显示持久的离线通知
  offlineNotification = ElNotification({
    title: '网络已断开',
    message: '网络连接已断开，部分功能可能不可用',
    type: 'warning',
    duration: 0, // 不自动关闭
    showClose: true,
    onClose: () => {
      offlineNotification = null
    }
  })
  
  // 触发自定义事件
  window.dispatchEvent(new CustomEvent('network-offline'))
}

/**
 * 处理连接变化事件
 */
function handleConnectionChange() {
  connectionType.value = getConnectionType()
  
  // 触发自定义事件
  window.dispatchEvent(new CustomEvent('network-change', {
    detail: { connectionType: connectionType.value }
  }))
}

// 初始化事件监听
function initNetworkListeners() {
  window.addEventListener('online', handleOnline)
  window.addEventListener('offline', handleOffline)
  
  if ('connection' in navigator) {
    navigator.connection.addEventListener('change', handleConnectionChange)
  }
}

// 清理事件监听
function cleanupNetworkListeners() {
  window.removeEventListener('online', handleOnline)
  window.removeEventListener('offline', handleOffline)
  
  if ('connection' in navigator) {
    navigator.connection.removeEventListener('change', handleConnectionChange)
  }
}

/**
 * 检查网络连接
 * @returns {Promise<boolean>}
 */
async function checkConnection() {
  try {
    const response = await fetch('/api/health', {
      method: 'HEAD',
      cache: 'no-cache'
    })
    return response.ok
  } catch {
    return false
  }
}

/**
 * 等待网络恢复
 * @param {number} timeout 超时时间（毫秒）
 * @returns {Promise<boolean>}
 */
function waitForOnline(timeout = 30000) {
  return new Promise((resolve) => {
    if (isOnline.value) {
      resolve(true)
      return
    }
    
    const timeoutId = setTimeout(() => {
      window.removeEventListener('online', onOnline)
      resolve(false)
    }, timeout)
    
    function onOnline() {
      clearTimeout(timeoutId)
      window.removeEventListener('online', onOnline)
      resolve(true)
    }
    
    window.addEventListener('online', onOnline)
  })
}

/**
 * 带重试的请求
 * @param {Function} requestFn 请求函数
 * @param {Object} options 选项
 * @returns {Promise}
 */
async function requestWithRetry(requestFn, options = {}) {
  const {
    maxRetries = 3,
    retryDelay = 1000,
    retryOnOffline = true
  } = options
  
  let lastError
  
  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      // 如果离线且需要等待
      if (!isOnline.value && retryOnOffline) {
        const online = await waitForOnline(retryDelay * 2)
        if (!online) {
          throw new Error('Network offline')
        }
      }
      
      return await requestFn()
    } catch (error) {
      lastError = error
      
      // 如果是最后一次尝试，抛出错误
      if (attempt === maxRetries) {
        throw error
      }
      
      // 等待后重试
      await new Promise(resolve => 
        setTimeout(resolve, retryDelay * Math.pow(2, attempt))
      )
    }
  }
  
  throw lastError
}

// 初始化
initNetworkListeners()

// 导出
export {
  isOnline,
  lastOnlineTime,
  connectionType,
  checkConnection,
  waitForOnline,
  requestWithRetry,
  initNetworkListeners,
  cleanupNetworkListeners
}

export default {
  isOnline: readonly(isOnline),
  lastOnlineTime: readonly(lastOnlineTime),
  connectionType: readonly(connectionType),
  checkConnection,
  waitForOnline,
  requestWithRetry
}
