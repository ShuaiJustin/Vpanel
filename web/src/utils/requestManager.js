/**
 * 请求管理器
 * 提供请求取消、去重和队列管理功能
 */

/**
 * 请求取消管理器
 * 管理 AbortController 实例，支持按 key 取消请求
 */
class RequestCancelManager {
  constructor() {
    this.controllers = new Map()
  }

  /**
   * 创建或获取取消控制器
   * @param {string} key 请求标识
   * @param {boolean} cancelPrevious 是否取消之前的同 key 请求
   * @returns {AbortController} 取消控制器
   */
  getController(key, cancelPrevious = true) {
    if (cancelPrevious && this.controllers.has(key)) {
      this.cancel(key)
    }
    
    const controller = new AbortController()
    this.controllers.set(key, controller)
    return controller
  }

  /**
   * 取消指定请求
   * @param {string} key 请求标识
   */
  cancel(key) {
    const controller = this.controllers.get(key)
    if (controller) {
      controller.abort()
      this.controllers.delete(key)
    }
  }

  /**
   * 取消所有请求
   */
  cancelAll() {
    for (const controller of this.controllers.values()) {
      controller.abort()
    }
    this.controllers.clear()
  }

  /**
   * 移除控制器（请求完成后调用）
   * @param {string} key 请求标识
   */
  remove(key) {
    this.controllers.delete(key)
  }

  /**
   * 检查请求是否已取消
   * @param {string} key 请求标识
   * @returns {boolean}
   */
  isCancelled(key) {
    const controller = this.controllers.get(key)
    return controller ? controller.signal.aborted : false
  }
}

/**
 * 请求去重管理器
 * 防止相同请求同时发送多次
 */
class RequestDeduplicator {
  constructor() {
    this.pendingRequests = new Map()
  }

  /**
   * 生成请求 key
   * @param {string} method HTTP 方法
   * @param {string} url 请求 URL
   * @param {Object} params 请求参数
   * @returns {string} 请求 key
   */
  generateKey(method, url, params = {}) {
    const sortedParams = JSON.stringify(params, Object.keys(params).sort())
    return `${method}:${url}:${sortedParams}`
  }

  /**
   * 执行去重请求
   * @param {string} key 请求 key
   * @param {Function} requestFn 请求函数
   * @returns {Promise} 请求结果
   */
  async execute(key, requestFn) {
    // 如果已有相同请求在进行中，返回该请求的 Promise
    if (this.pendingRequests.has(key)) {
      return this.pendingRequests.get(key)
    }

    // 创建新请求
    const promise = requestFn()
      .finally(() => {
        this.pendingRequests.delete(key)
      })

    this.pendingRequests.set(key, promise)
    return promise
  }

  /**
   * 检查是否有待处理的请求
   * @param {string} key 请求 key
   * @returns {boolean}
   */
  isPending(key) {
    return this.pendingRequests.has(key)
  }

  /**
   * 清除所有待处理请求
   */
  clear() {
    this.pendingRequests.clear()
  }
}

/**
 * 请求队列管理器
 * 支持离线请求队列和自动重试
 */
class RequestQueue {
  constructor(options = {}) {
    this.queue = []
    this.maxRetries = options.maxRetries || 3
    this.retryDelay = options.retryDelay || 1000
    this.isOnline = navigator.onLine
    this.processing = false
    this.storageKey = 'request_queue'

    // 监听网络状态变化
    window.addEventListener('online', () => this.handleOnline())
    window.addEventListener('offline', () => this.handleOffline())

    // 从存储恢复队列
    this.restore()
  }

  /**
   * 添加请求到队列
   * @param {Object} request 请求配置
   * @returns {Promise} 请求结果
   */
  add(request) {
    return new Promise((resolve, reject) => {
      const queueItem = {
        id: Date.now() + Math.random().toString(36).substring(2),
        request,
        retries: 0,
        resolve,
        reject,
        createdAt: new Date().toISOString()
      }

      this.queue.push(queueItem)
      this.save()

      if (this.isOnline) {
        this.process()
      }
    })
  }

  /**
   * 处理队列
   */
  async process() {
    if (this.processing || !this.isOnline || this.queue.length === 0) {
      return
    }

    this.processing = true

    while (this.queue.length > 0 && this.isOnline) {
      const item = this.queue[0]

      try {
        const result = await this.executeRequest(item.request)
        item.resolve(result)
        this.queue.shift()
        this.save()
      } catch (error) {
        if (this.shouldRetry(error) && item.retries < this.maxRetries) {
          item.retries++
          await this.delay(this.retryDelay * Math.pow(2, item.retries - 1))
        } else {
          item.reject(error)
          this.queue.shift()
          this.save()
        }
      }
    }

    this.processing = false
  }

  /**
   * 执行请求
   * @param {Object} request 请求配置
   * @returns {Promise}
   */
  async executeRequest(request) {
    const { method, url, data, headers } = request
    const response = await fetch(url, {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...headers
      },
      body: data ? JSON.stringify(data) : undefined
    })

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`)
    }

    return response.json()
  }

  /**
   * 判断是否应该重试
   * @param {Error} error 错误对象
   * @returns {boolean}
   */
  shouldRetry(error) {
    // 网络错误或服务器错误应该重试
    return error.message.includes('network') ||
           error.message.includes('HTTP 5') ||
           error.message.includes('HTTP 429')
  }

  /**
   * 延迟
   * @param {number} ms 毫秒数
   * @returns {Promise}
   */
  delay(ms) {
    return new Promise(resolve => setTimeout(resolve, ms))
  }

  /**
   * 处理上线事件
   */
  handleOnline() {
    this.isOnline = true
    this.process()
  }

  /**
   * 处理离线事件
   */
  handleOffline() {
    this.isOnline = false
  }

  /**
   * 保存队列到存储
   */
  save() {
    try {
      const data = this.queue.map(item => ({
        id: item.id,
        request: item.request,
        retries: item.retries,
        createdAt: item.createdAt
      }))
      localStorage.setItem(this.storageKey, JSON.stringify(data))
    } catch (e) {
      console.error('Failed to save request queue:', e)
    }
  }

  /**
   * 从存储恢复队列
   */
  restore() {
    try {
      const data = localStorage.getItem(this.storageKey)
      if (data) {
        const items = JSON.parse(data)
        // 只恢复最近 24 小时内的请求
        const cutoff = Date.now() - 24 * 60 * 60 * 1000
        this.queue = items
          .filter(item => new Date(item.createdAt).getTime() > cutoff)
          .map(item => ({
            ...item,
            resolve: () => {},
            reject: () => {}
          }))
      }
    } catch (e) {
      console.error('Failed to restore request queue:', e)
    }
  }

  /**
   * 获取队列长度
   * @returns {number}
   */
  get length() {
    return this.queue.length
  }

  /**
   * 清空队列
   */
  clear() {
    this.queue.forEach(item => item.reject(new Error('Queue cleared')))
    this.queue = []
    this.save()
  }
}

// 创建单例实例
export const cancelManager = new RequestCancelManager()
export const deduplicator = new RequestDeduplicator()
export const requestQueue = new RequestQueue()

/**
 * 创建可取消的请求
 * @param {string} key 请求标识
 * @param {Function} requestFn 请求函数（接收 signal 参数）
 * @returns {Promise}
 */
export function createCancellableRequest(key, requestFn) {
  const controller = cancelManager.getController(key)
  
  return requestFn(controller.signal)
    .finally(() => {
      cancelManager.remove(key)
    })
}

/**
 * 创建去重请求
 * @param {string} method HTTP 方法
 * @param {string} url 请求 URL
 * @param {Object} params 请求参数
 * @param {Function} requestFn 请求函数
 * @returns {Promise}
 */
export function createDeduplicatedRequest(method, url, params, requestFn) {
  const key = deduplicator.generateKey(method, url, params)
  return deduplicator.execute(key, requestFn)
}

/**
 * 取消请求
 * @param {string} key 请求标识
 */
export function cancelRequest(key) {
  cancelManager.cancel(key)
}

/**
 * 取消所有请求
 */
export function cancelAllRequests() {
  cancelManager.cancelAll()
}

export default {
  cancelManager,
  deduplicator,
  requestQueue,
  createCancellableRequest,
  createDeduplicatedRequest,
  cancelRequest,
  cancelAllRequests
}
