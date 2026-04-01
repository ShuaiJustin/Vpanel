/**
 * 用户前台统计 API 模块
 */
import api from '@/api/base'

const BASE_URL = '/portal/stats'

/**
 * 获取流量统计
 * @param {Object} [params] - 查询参数
 * @param {string} [params.period] - 时间周期 (day, week, month)
 * @param {string} [params.start_date] - 开始日期
 * @param {string} [params.end_date] - 结束日期
 * @returns {Promise}
 */
export function getTrafficStats(params = {}) {
  return api.get(`${BASE_URL}/traffic`, { params })
}

/**
 * 获取使用统计（按节点/协议）
 * @param {Object} [params] - 查询参数
 * @param {string} [params.group_by] - 分组方式 (node, protocol)
 * @param {string} [params.period] - 时间周期
 * @returns {Promise}
 */
export function getUsageStats(params = {}) {
  return api.get(`${BASE_URL}/usage`, { params })
}

/**
 * 导出统计数据
 * @param {Object} [params] - 查询参数
 * @param {string} [params.format] - 导出格式 (csv, json)
 * @param {string} [params.period] - 时间周期
 * @param {string} [params.start_date] - 开始日期
 * @param {string} [params.end_date] - 结束日期
 * @returns {Promise}
 */
export function exportStats(params = {}) {
  return api.get(`${BASE_URL}/export`, { 
    params,
    responseType: 'blob'
  })
}

export default {
  getTrafficStats,
  getUsageStats,
  exportStats
}
