/**
 * 状态颜色工具
 * 提供统一的状态颜色映射和辅助函数
 */

// Element Plus 标签类型
export const TAG_TYPES = {
  SUCCESS: 'success',
  WARNING: 'warning',
  DANGER: 'danger',
  INFO: 'info',
  PRIMARY: 'primary'
}

// 颜色值映射
export const COLORS = {
  success: '#67c23a',
  warning: '#e6a23c',
  danger: '#f56c6c',
  info: '#909399',
  primary: '#409eff',
  // 扩展颜色
  green: '#67c23a',
  yellow: '#e6a23c',
  red: '#f56c6c',
  gray: '#909399',
  blue: '#409eff',
  orange: '#ff9800',
  purple: '#9c27b0'
}

// 代理状态配置
export const PROXY_STATUS = {
  running: {
    type: TAG_TYPES.SUCCESS,
    label: '运行中',
    color: COLORS.success
  },
  stopped: {
    type: TAG_TYPES.DANGER,
    label: '已停止',
    color: COLORS.danger
  },
  starting: {
    type: TAG_TYPES.WARNING,
    label: '启动中',
    color: COLORS.warning
  },
  stopping: {
    type: TAG_TYPES.WARNING,
    label: '停止中',
    color: COLORS.warning
  },
  error: {
    type: TAG_TYPES.DANGER,
    label: '错误',
    color: COLORS.danger
  },
  disabled: {
    type: TAG_TYPES.INFO,
    label: '已禁用',
    color: COLORS.info
  }
}

// 用户状态配置
export const USER_STATUS = {
  active: {
    type: TAG_TYPES.SUCCESS,
    label: '启用',
    color: COLORS.success
  },
  inactive: {
    type: TAG_TYPES.DANGER,
    label: '禁用',
    color: COLORS.danger
  },
  locked: {
    type: TAG_TYPES.WARNING,
    label: '已锁定',
    color: COLORS.warning
  },
  expired: {
    type: TAG_TYPES.DANGER,
    label: '已过期',
    color: COLORS.danger
  },
  pending: {
    type: TAG_TYPES.WARNING,
    label: '待审核',
    color: COLORS.warning
  }
}

// 协议类型颜色
export const PROTOCOL_COLORS = {
  vmess: { type: TAG_TYPES.PRIMARY, color: COLORS.blue },
  vless: { type: TAG_TYPES.SUCCESS, color: COLORS.green },
  trojan: { type: TAG_TYPES.WARNING, color: COLORS.orange },
  shadowsocks: { type: TAG_TYPES.DANGER, color: COLORS.purple },
  socks: { type: TAG_TYPES.INFO, color: COLORS.gray },
  http: { type: TAG_TYPES.INFO, color: COLORS.gray },
  dokodemo: { type: TAG_TYPES.WARNING, color: COLORS.yellow }
}

// 健康状态配置
export const HEALTH_STATUS = {
  healthy: {
    type: TAG_TYPES.SUCCESS,
    label: '健康',
    color: COLORS.success
  },
  unhealthy: {
    type: TAG_TYPES.DANGER,
    label: '不健康',
    color: COLORS.danger
  },
  degraded: {
    type: TAG_TYPES.WARNING,
    label: '降级',
    color: COLORS.warning
  },
  unknown: {
    type: TAG_TYPES.INFO,
    label: '未知',
    color: COLORS.info
  }
}

// 日志级别颜色
export const LOG_LEVEL_COLORS = {
  debug: { type: TAG_TYPES.INFO, color: COLORS.gray },
  info: { type: TAG_TYPES.PRIMARY, color: COLORS.blue },
  warn: { type: TAG_TYPES.WARNING, color: COLORS.warning },
  warning: { type: TAG_TYPES.WARNING, color: COLORS.warning },
  error: { type: TAG_TYPES.DANGER, color: COLORS.danger },
  fatal: { type: TAG_TYPES.DANGER, color: COLORS.danger }
}

/**
 * 获取代理状态标签类型
 * @param {boolean|string} status 状态值
 * @returns {string} Element Plus 标签类型
 */
export function getProxyStatusType(status) {
  if (typeof status === 'boolean') {
    return status ? TAG_TYPES.SUCCESS : TAG_TYPES.DANGER
  }
  const config = PROXY_STATUS[status]
  return config ? config.type : TAG_TYPES.INFO
}

/**
 * 获取代理状态标签
 * @param {boolean|string} status 状态值
 * @returns {string} 状态标签文本
 */
export function getProxyStatusLabel(status) {
  if (typeof status === 'boolean') {
    return status ? '运行中' : '已停止'
  }
  const config = PROXY_STATUS[status]
  return config ? config.label : status
}

/**
 * 获取用户状态标签类型
 * @param {boolean|string} status 状态值
 * @returns {string} Element Plus 标签类型
 */
export function getUserStatusType(status) {
  if (typeof status === 'boolean') {
    return status ? TAG_TYPES.SUCCESS : TAG_TYPES.DANGER
  }
  const config = USER_STATUS[status]
  return config ? config.type : TAG_TYPES.INFO
}

/**
 * 获取用户状态标签
 * @param {boolean|string} status 状态值
 * @returns {string} 状态标签文本
 */
export function getUserStatusLabel(status) {
  if (typeof status === 'boolean') {
    return status ? '启用' : '禁用'
  }
  const config = USER_STATUS[status]
  return config ? config.label : status
}

/**
 * 获取协议类型标签类型
 * @param {string} protocol 协议名称
 * @returns {string} Element Plus 标签类型
 */
export function getProtocolType(protocol) {
  const config = PROTOCOL_COLORS[protocol?.toLowerCase()]
  return config ? config.type : TAG_TYPES.INFO
}

/**
 * 获取健康状态标签类型
 * @param {string} status 健康状态
 * @returns {string} Element Plus 标签类型
 */
export function getHealthStatusType(status) {
  const config = HEALTH_STATUS[status?.toLowerCase()]
  return config ? config.type : TAG_TYPES.INFO
}

/**
 * 获取日志级别标签类型
 * @param {string} level 日志级别
 * @returns {string} Element Plus 标签类型
 */
export function getLogLevelType(level) {
  const config = LOG_LEVEL_COLORS[level?.toLowerCase()]
  return config ? config.type : TAG_TYPES.INFO
}

/**
 * 根据百分比获取状态类型
 * @param {number} percent 百分比值 (0-100)
 * @param {Object} thresholds 阈值配置
 * @returns {string} Element Plus 标签类型
 */
export function getPercentStatusType(percent, thresholds = { warning: 70, danger: 90 }) {
  if (percent >= thresholds.danger) {
    return TAG_TYPES.DANGER
  }
  if (percent >= thresholds.warning) {
    return TAG_TYPES.WARNING
  }
  return TAG_TYPES.SUCCESS
}

/**
 * 根据百分比获取颜色
 * @param {number} percent 百分比值 (0-100)
 * @param {Object} thresholds 阈值配置
 * @returns {string} 颜色值
 */
export function getPercentColor(percent, thresholds = { warning: 70, danger: 90 }) {
  if (percent >= thresholds.danger) {
    return COLORS.danger
  }
  if (percent >= thresholds.warning) {
    return COLORS.warning
  }
  return COLORS.success
}

/**
 * 获取布尔状态标签类型
 * @param {boolean} value 布尔值
 * @param {boolean} inverse 是否反转 (true=danger, false=success)
 * @returns {string} Element Plus 标签类型
 */
export function getBooleanStatusType(value, inverse = false) {
  if (inverse) {
    return value ? TAG_TYPES.DANGER : TAG_TYPES.SUCCESS
  }
  return value ? TAG_TYPES.SUCCESS : TAG_TYPES.DANGER
}

export default {
  TAG_TYPES,
  COLORS,
  PROXY_STATUS,
  USER_STATUS,
  PROTOCOL_COLORS,
  HEALTH_STATUS,
  LOG_LEVEL_COLORS,
  getProxyStatusType,
  getProxyStatusLabel,
  getUserStatusType,
  getUserStatusLabel,
  getProtocolType,
  getHealthStatusType,
  getLogLevelType,
  getPercentStatusType,
  getPercentColor,
  getBooleanStatusType
}
