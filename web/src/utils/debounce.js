/**
 * 防抖和节流工具
 * 用于优化搜索输入和频繁触发的事件
 */

/**
 * 防抖函数
 * 在指定时间内多次调用只执行最后一次
 * @param {Function} fn 要执行的函数
 * @param {number} delay 延迟时间（毫秒）
 * @param {Object} options 选项
 * @returns {Function} 防抖后的函数
 */
export function debounce(fn, delay = 300, options = {}) {
  const { leading = false, trailing = true } = options
  let timeoutId = null
  let lastArgs = null
  let lastThis = null
  let result
  let lastCallTime = null

  function invokeFunc() {
    const args = lastArgs
    const thisArg = lastThis

    lastArgs = null
    lastThis = null
    result = fn.apply(thisArg, args)
    return result
  }
  
  function startTimer(pendingFunc, wait) {
    return setTimeout(pendingFunc, wait)
  }
  
  function cancelTimer() {
    if (timeoutId !== null) {
      clearTimeout(timeoutId)
      timeoutId = null
    }
  }
  
  function trailingEdge() {
    timeoutId = null
    
    if (trailing && lastArgs) {
      return invokeFunc()
    }
    
    lastArgs = null
    lastThis = null
    return result
  }
  
  function leadingEdge() {
    timeoutId = startTimer(() => trailingEdge(), delay)

    return leading ? invokeFunc() : result
  }
  
  function debounced(...args) {
    const time = Date.now()
    const isInvoking = lastCallTime === null
    
    lastArgs = args
    lastThis = this
    lastCallTime = time
    
    if (isInvoking) {
      return leadingEdge()
    }
    
    // 重置定时器
    cancelTimer()
    timeoutId = startTimer(() => trailingEdge(), delay)
    
    return result
  }
  
  debounced.cancel = function() {
    cancelTimer()
    lastArgs = null
    lastThis = null
    lastCallTime = null
  }
  
  debounced.flush = function() {
    if (timeoutId === null) {
      return result
    }
    return trailingEdge()
  }
  
  debounced.pending = function() {
    return timeoutId !== null
  }
  
  return debounced
}

/**
 * 节流函数
 * 在指定时间内只执行一次
 * @param {Function} fn 要执行的函数
 * @param {number} wait 等待时间（毫秒）
 * @param {Object} options 选项
 * @returns {Function} 节流后的函数
 */
export function throttle(fn, wait = 300, options = {}) {
  const { leading = true, trailing = true } = options
  let timeoutId = null
  let lastArgs = null
  let lastThis = null
  let result
  let lastInvokeTime = 0
  
  function invokeFunc(time) {
    const args = lastArgs
    const thisArg = lastThis
    
    lastArgs = null
    lastThis = null
    lastInvokeTime = time
    result = fn.apply(thisArg, args)
    return result
  }
  
  function shouldInvoke(time) {
    const timeSinceLastInvoke = time - lastInvokeTime
    return lastInvokeTime === 0 || timeSinceLastInvoke >= wait
  }
  
  function trailingEdge(time) {
    timeoutId = null
    
    if (trailing && lastArgs) {
      return invokeFunc(time)
    }
    
    lastArgs = null
    lastThis = null
    return result
  }
  
  function throttled(...args) {
    const time = Date.now()
    const isInvoking = shouldInvoke(time)
    
    lastArgs = args
    lastThis = this
    
    if (isInvoking) {
      if (timeoutId === null) {
        if (leading) {
          return invokeFunc(time)
        }
        lastInvokeTime = time
      }
    }
    
    if (timeoutId === null && trailing) {
      const remaining = wait - (time - lastInvokeTime)
      timeoutId = setTimeout(() => trailingEdge(Date.now()), Math.max(remaining, 0))
    }
    
    return result
  }
  
  throttled.cancel = function() {
    if (timeoutId !== null) {
      clearTimeout(timeoutId)
      timeoutId = null
    }
    lastInvokeTime = 0
    lastArgs = null
    lastThis = null
  }
  
  throttled.flush = function() {
    if (timeoutId === null) {
      return result
    }
    return trailingEdge(Date.now())
  }
  
  throttled.pending = function() {
    return timeoutId !== null
  }
  
  return throttled
}

/**
 * 创建防抖的搜索函数
 * @param {Function} searchFn 搜索函数
 * @param {number} delay 延迟时间
 * @returns {Object} 包含 search 和 cancel 方法的对象
 */
export function createDebouncedSearch(searchFn, delay = 300) {
  const debouncedFn = debounce(searchFn, delay)
  
  return {
    search: debouncedFn,
    cancel: debouncedFn.cancel,
    flush: debouncedFn.flush,
    pending: debouncedFn.pending
  }
}

/**
 * Vue 组合式 API 的防抖 hook
 * @param {Function} fn 要执行的函数
 * @param {number} delay 延迟时间
 * @param {Object} options 选项
 * @returns {Object} 防抖函数和控制方法
 */
export function useDebounceFn(fn, delay = 300, options = {}) {
  const debouncedFn = debounce(fn, delay, options)
  
  return {
    run: debouncedFn,
    cancel: debouncedFn.cancel,
    flush: debouncedFn.flush,
    pending: debouncedFn.pending
  }
}

/**
 * Vue 组合式 API 的节流 hook
 * @param {Function} fn 要执行的函数
 * @param {number} wait 等待时间
 * @param {Object} options 选项
 * @returns {Object} 节流函数和控制方法
 */
export function useThrottleFn(fn, wait = 300, options = {}) {
  const throttledFn = throttle(fn, wait, options)
  
  return {
    run: throttledFn,
    cancel: throttledFn.cancel,
    flush: throttledFn.flush,
    pending: throttledFn.pending
  }
}

export default {
  debounce,
  throttle,
  createDebouncedSearch,
  useDebounceFn,
  useThrottleFn
}
