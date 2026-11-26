/**
 * 统计数据 API
 * 处理仪表盘统计、用户统计、协议统计等数据查询
 */
import api from '../base'

export const statsApi = {
  /**
   * 获取仪表盘统计
   * @returns {Promise<Object>} 仪表盘统计数据
   */
  getDashboard: () => api.get('/stats/dashboard'),

  // 兼容旧调用方
  getDashboardStats: () => api.get('/stats/dashboard'),

  /**
   * 获取用户统计
   * @param {Object} params - 查询参数
   * @param {string} params.period - 时间段 (today, week, month, year)
   * @param {string} params.startDate - 开始日期
   * @param {string} params.endDate - 结束日期
   * @returns {Promise<Object>} 用户统计数据
   */
  getUserStats: (params) => api.get('/stats/user', { params }),

  /**
   * 获取协议统计
   * @param {Object} params - 查询参数
   * @returns {Promise<Object>} 协议统计数据
   */
  getProtocolStats: (params) => api.get('/stats/protocol', { params }),

  /**
   * 获取详细统计
   * @param {Object} params - 查询参数
   * @returns {Promise<Object>} 详细统计数据
   */
  getDetailedStats: (params) => api.get('/stats/detailed', { params }),

  /**
   * 获取流量统计
   * @param {Object} params - 查询参数
   * @param {string} params.period - 时间段
   * @param {string} params.granularity - 粒度 (hourly, daily)
   * @returns {Promise<Object>} 流量统计数据
   */
  getTrafficStats: (params) => api.get('/stats/traffic', { params })
}

export default statsApi
