/**
 * 表单验证规则工具
 * 提供常用的验证规则和自定义验证器
 */

/**
 * 必填验证
 * @param {string} message 错误消息
 * @returns {Object} 验证规则
 */
export const required = (message = '此字段为必填项') => ({
  required: true,
  message,
  trigger: ['blur', 'change']
})

/**
 * 最小长度验证
 * @param {number} min 最小长度
 * @param {string} message 错误消息
 * @returns {Object} 验证规则
 */
export const minLength = (min, message) => ({
  min,
  message: message || `长度不能少于 ${min} 个字符`,
  trigger: ['blur', 'change']
})

/**
 * 最大长度验证
 * @param {number} max 最大长度
 * @param {string} message 错误消息
 * @returns {Object} 验证规则
 */
export const maxLength = (max, message) => ({
  max,
  message: message || `长度不能超过 ${max} 个字符`,
  trigger: ['blur', 'change']
})

/**
 * 长度范围验证
 * @param {number} min 最小长度
 * @param {number} max 最大长度
 * @param {string} message 错误消息
 * @returns {Object} 验证规则
 */
export const lengthRange = (min, max, message) => ({
  min,
  max,
  message: message || `长度应在 ${min} 到 ${max} 个字符之间`,
  trigger: ['blur', 'change']
})

/**
 * 邮箱验证
 * @param {string} message 错误消息
 * @returns {Object} 验证规则
 */
export const email = (message = '请输入有效的邮箱地址') => ({
  type: 'email',
  message,
  trigger: ['blur', 'change']
})

/**
 * 正则表达式验证
 * @param {RegExp} pattern 正则表达式
 * @param {string} message 错误消息
 * @returns {Object} 验证规则
 */
export const pattern = (regex, message = '格式不正确') => ({
  pattern: regex,
  message,
  trigger: ['blur', 'change']
})

/**
 * 用户名验证（字母、数字、下划线）
 * @param {string} message 错误消息
 * @returns {Object} 验证规则
 */
export const username = (message = '用户名只能包含字母、数字和下划线') => ({
  pattern: /^[a-zA-Z0-9_]+$/,
  message,
  trigger: ['blur', 'change']
})

/**
 * 密码强度验证
 * @param {Object} options 选项
 * @returns {Object} 验证规则
 */
export const password = (options = {}) => {
  const {
    minLength: min = 8,
    requireUppercase = true,
    requireLowercase = true,
    requireNumber = true,
    requireSpecial = false
  } = options
  
  return {
    validator: (rule, value, callback) => {
      if (!value) {
        callback()
        return
      }
      
      const errors = []
      
      if (value.length < min) {
        errors.push(`至少 ${min} 个字符`)
      }
      if (requireUppercase && !/[A-Z]/.test(value)) {
        errors.push('至少一个大写字母')
      }
      if (requireLowercase && !/[a-z]/.test(value)) {
        errors.push('至少一个小写字母')
      }
      if (requireNumber && !/\d/.test(value)) {
        errors.push('至少一个数字')
      }
      if (requireSpecial && !/[!@#$%^&*(),.?":{}|<>]/.test(value)) {
        errors.push('至少一个特殊字符')
      }
      
      if (errors.length > 0) {
        callback(new Error(`密码需要: ${errors.join(', ')}`))
      } else {
        callback()
      }
    },
    trigger: ['blur', 'change']
  }
}

/**
 * 确认密码验证
 * @param {Function} getPassword 获取密码的函数
 * @param {string} message 错误消息
 * @returns {Object} 验证规则
 */
export const confirmPassword = (getPassword, message = '两次输入的密码不一致') => ({
  validator: (rule, value, callback) => {
    if (value !== getPassword()) {
      callback(new Error(message))
    } else {
      callback()
    }
  },
  trigger: ['blur', 'change']
})

/**
 * 端口号验证
 * @param {string} message 错误消息
 * @returns {Object} 验证规则
 */
export const port = (message = '请输入有效的端口号 (1-65535)') => ({
  validator: (rule, value, callback) => {
    if (!value) {
      callback()
      return
    }
    const num = parseInt(value, 10)
    if (isNaN(num) || num < 1 || num > 65535) {
      callback(new Error(message))
    } else {
      callback()
    }
  },
  trigger: ['blur', 'change']
})

/**
 * IP 地址验证
 * @param {string} message 错误消息
 * @returns {Object} 验证规则
 */
export const isValidIPv4Address = (value) =>
  typeof value === 'string' && /^(?:0|[1-9]\d{0,2})(?:\.(?:0|[1-9]\d{0,2})){3}$/.test(value) &&
  value.split('.').every(part => Number(part) <= 255)

export const isValidIPv6Address = (value) => {
  if (typeof value !== 'string' || !value.includes(':') || !/^[0-9a-f:.]+$/i.test(value)) return false
  try {
    // The browser's IPv6 parser handles compression and embedded IPv4 consistently.
    return new URL(`http://[${value}]/`).hostname.startsWith('[')
  } catch {
    return false
  }
}

export const ipAddress = (message = '请输入有效的 IP 地址') => ({
  validator: (rule, value, callback) => {
    if (!value || isValidIPv4Address(value) || isValidIPv6Address(value)) callback()
    else callback(new Error(message))
  },
  trigger: ['blur', 'change']
})

/**
 * URL 验证
 * @param {string} message 错误消息
 * @returns {Object} 验证规则
 */
export const url = (message = '请输入有效的 URL') => ({
  type: 'url',
  message,
  trigger: ['blur', 'change']
})

/**
 * 数字范围验证
 * @param {number} min 最小值
 * @param {number} max 最大值
 * @param {string} message 错误消息
 * @returns {Object} 验证规则
 */
export const numberRange = (min, max, message) => ({
  validator: (rule, value, callback) => {
    if (value === '' || value === null || value === undefined) {
      callback()
      return
    }
    const num = Number(value)
    if (isNaN(num) || num < min || num > max) {
      callback(new Error(message || `数值应在 ${min} 到 ${max} 之间`))
    } else {
      callback()
    }
  },
  trigger: ['blur', 'change']
})

/**
 * 整数验证
 * @param {string} message 错误消息
 * @returns {Object} 验证规则
 */
export const integer = (message = '请输入整数') => ({
  validator: (rule, value, callback) => {
    if (value === '' || value === null || value === undefined) {
      callback()
      return
    }
    if (!Number.isInteger(Number(value))) {
      callback(new Error(message))
    } else {
      callback()
    }
  },
  trigger: ['blur', 'change']
})

/**
 * 自定义异步验证
 * @param {Function} validator 验证函数 (value) => Promise<boolean | string>
 * @param {string} message 默认错误消息
 * @returns {Object} 验证规则
 */
export const asyncValidator = (validator, message = '验证失败') => ({
  validator: async (rule, value, callback) => {
    try {
      const result = await validator(value)
      if (result === true) {
        callback()
      } else if (typeof result === 'string') {
        callback(new Error(result))
      } else {
        callback(new Error(message))
      }
    } catch (e) {
      callback(new Error(e.message || message))
    }
  },
  trigger: 'blur'
})

/**
 * 组合多个验证规则
 * @param  {...Object} rules 验证规则
 * @returns {Array} 规则数组
 */
export const combine = (...rules) => rules

/**
 * 常用验证规则预设
 */
export const presets = {
  // 用户名：必填，3-20字符，字母数字下划线
  username: combine(
    required('请输入用户名'),
    lengthRange(3, 20, '用户名长度应在 3-20 个字符之间'),
    username()
  ),
  
  // 邮箱：必填，有效邮箱格式
  email: combine(
    required('请输入邮箱'),
    email()
  ),
  
  // 密码：必填，至少8字符，包含大小写和数字
  password: combine(
    required('请输入密码'),
    password({ minLength: 8 })
  ),
  
  // 端口：必填，有效端口号
  port: combine(
    required('请输入端口号'),
    port()
  )
}

export default {
  required,
  minLength,
  maxLength,
  lengthRange,
  email,
  pattern,
  username,
  password,
  confirmPassword,
  port,
  ipAddress,
  url,
  numberRange,
  integer,
  asyncValidator,
  combine,
  presets
}
