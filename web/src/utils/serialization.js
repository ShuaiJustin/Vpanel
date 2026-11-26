/**
 * 数据序列化工具
 * 提供统一的数据序列化和反序列化功能
 */

/**
 * 安全的 JSON 序列化
 * 处理循环引用和特殊类型
 * @param {any} data 要序列化的数据
 * @param {Object} options 选项
 * @returns {string} JSON 字符串
 */
export function serialize(data, options = {}) {
  const { pretty = false, replacer = null } = options
  
  const seen = new WeakSet()
  
  const customReplacer = function(key, value) {
    // 获取原始值（this[key] 是原始值，value 可能已被 toJSON 转换）
    const originalValue = this[key]
    
    // 处理循环引用
    if (typeof originalValue === 'object' && originalValue !== null) {
      if (seen.has(originalValue)) {
        return '[Circular]'
      }
      seen.add(originalValue)
    }
    
    // 处理特殊类型 - 使用原始值检查
    if (originalValue instanceof Date) {
      // Handle invalid dates
      if (isNaN(originalValue.getTime())) {
        return { __type: 'InvalidDate' }
      }
      return { __type: 'Date', value: originalValue.toISOString() }
    }
    
    if (originalValue instanceof RegExp) {
      return { __type: 'RegExp', source: originalValue.source, flags: originalValue.flags }
    }
    
    if (originalValue instanceof Map) {
      return { __type: 'Map', entries: Array.from(originalValue.entries()) }
    }
    
    if (originalValue instanceof Set) {
      return { __type: 'Set', values: Array.from(originalValue.values()) }
    }
    
    // 处理 undefined（JSON 默认会忽略）
    if (originalValue === undefined) {
      return { __type: 'undefined' }
    }
    
    // 处理 NaN 和 Infinity
    if (typeof originalValue === 'number') {
      if (Number.isNaN(originalValue)) {
        return { __type: 'NaN' }
      }
      if (!Number.isFinite(originalValue)) {
        return { __type: 'Infinity', negative: originalValue < 0 }
      }
    }
    
    // 应用自定义 replacer
    if (replacer) {
      return replacer(key, value)
    }
    
    return value
  }
  
  const space = pretty ? 2 : undefined
  return JSON.stringify(data, customReplacer, space)
}

/**
 * 安全的 JSON 反序列化
 * 恢复特殊类型
 * @param {string} json JSON 字符串
 * @param {Object} options 选项
 * @returns {any} 反序列化后的数据
 */
export function deserialize(json, options = {}) {
  const { reviver = null } = options
  
  const customReviver = (key, value) => {
    // 恢复特殊类型
    if (value && typeof value === 'object' && value.__type) {
      switch (value.__type) {
        case 'Date':
          return new Date(value.value)
        case 'InvalidDate':
          return new Date(NaN)
        case 'RegExp':
          return new RegExp(value.source, value.flags)
        case 'Map':
          return new Map(value.entries)
        case 'Set':
          return new Set(value.values)
        case 'undefined':
          return undefined
        case 'NaN':
          return NaN
        case 'Infinity':
          return value.negative ? -Infinity : Infinity
      }
    }
    
    // 应用自定义 reviver
    if (reviver) {
      return reviver(key, value)
    }
    
    return value
  }
  
  return JSON.parse(json, customReviver)
}

/**
 * 深度克隆对象
 * 使用序列化/反序列化实现
 * @param {any} data 要克隆的数据
 * @returns {any} 克隆后的数据
 */
export function deepClone(data) {
  return deserialize(serialize(data))
}

/**
 * 比较两个值是否相等
 * 支持特殊类型比较
 * @param {any} a 第一个值
 * @param {any} b 第二个值
 * @returns {boolean} 是否相等
 */
export function isEqual(a, b) {
  // 处理基本类型
  if (a === b) return true
  
  // 处理 NaN
  if (Number.isNaN(a) && Number.isNaN(b)) return true
  
  // 处理 null/undefined
  if (a == null || b == null) return a === b
  
  // 处理不同类型
  const typeA = Object.prototype.toString.call(a)
  const typeB = Object.prototype.toString.call(b)
  if (typeA !== typeB) return false
  
  // 处理 Date
  if (a instanceof Date && b instanceof Date) {
    // Handle invalid dates
    if (isNaN(a.getTime()) && isNaN(b.getTime())) return true
    return a.getTime() === b.getTime()
  }
  
  // 处理 RegExp
  if (a instanceof RegExp && b instanceof RegExp) {
    return a.source === b.source && a.flags === b.flags
  }
  
  // 处理 Map
  if (a instanceof Map && b instanceof Map) {
    if (a.size !== b.size) return false
    for (const [key, value] of a) {
      if (!b.has(key) || !isEqual(value, b.get(key))) return false
    }
    return true
  }
  
  // 处理 Set
  if (a instanceof Set && b instanceof Set) {
    if (a.size !== b.size) return false
    for (const value of a) {
      if (!b.has(value)) return false
    }
    return true
  }
  
  // 处理数组
  if (Array.isArray(a) && Array.isArray(b)) {
    if (a.length !== b.length) return false
    return a.every((item, index) => isEqual(item, b[index]))
  }
  
  // 处理对象
  if (typeof a === 'object' && typeof b === 'object') {
    const keysA = Object.keys(a)
    const keysB = Object.keys(b)
    
    if (keysA.length !== keysB.length) return false
    
    return keysA.every(key => isEqual(a[key], b[key]))
  }
  
  return false
}

/**
 * 序列化为 URL 查询字符串
 * @param {Object} params 参数对象
 * @returns {string} 查询字符串
 */
export function serializeParams(params) {
  const searchParams = new URLSearchParams()
  
  for (const [key, value] of Object.entries(params)) {
    if (value === null || value === undefined) {
      continue
    }
    
    if (Array.isArray(value)) {
      value.forEach(v => searchParams.append(key, String(v)))
    } else if (typeof value === 'object') {
      searchParams.append(key, JSON.stringify(value))
    } else {
      searchParams.append(key, String(value))
    }
  }
  
  return searchParams.toString()
}

/**
 * 反序列化 URL 查询字符串
 * @param {string} queryString 查询字符串
 * @returns {Object} 参数对象
 */
export function deserializeParams(queryString) {
  const searchParams = new URLSearchParams(queryString)
  const result = {}
  
  for (const [key, value] of searchParams.entries()) {
    // 尝试解析 JSON
    try {
      const parsed = JSON.parse(value)
      if (typeof parsed === 'object') {
        result[key] = parsed
        continue
      }
    } catch {
      // 不是 JSON，继续
    }
    
    // 处理数组
    if (result[key] !== undefined) {
      if (!Array.isArray(result[key])) {
        result[key] = [result[key]]
      }
      result[key].push(value)
    } else {
      result[key] = value
    }
  }
  
  return result
}

export default {
  serialize,
  deserialize,
  deepClone,
  isEqual,
  serializeParams,
  deserializeParams
}
