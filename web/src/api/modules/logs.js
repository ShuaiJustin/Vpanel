/**
 * 日志管理 API
 * 处理日志的查询、删除、清理和导出操作
 */
import api from '../base'

export const logsApi = {
  /**
   * 获取日志列表
   * @param {Object} params - 查询参数
   * @param {number} params.page - 页码
   * @param {number} params.page_size - 每页数量
   * @param {string} params.level - 日志级别
   * @param {string} params.min_level - 最小日志级别
   * @param {string} params.source - 日志来源
   * @param {number} params.user_id - 用户 ID
   * @param {string} params.start_time - 开始时间 (RFC3339)
   * @param {string} params.end_time - 结束时间 (RFC3339)
   * @param {string} params.keyword - 搜索关键词
   * @param {string} params.request_id - 请求 ID
   * @returns {Promise<Object>} 日志列表响应
   */
  getLogs: (params) => api.get('/logs', { params }),

  /**
   * 获取单条日志
   * @param {number|string} id - 日志 ID
   * @returns {Promise<Object>} 日志详情
   */
  getLog: (id) => api.get(`/logs/${id}`),

  /**
   * 删除日志
   * @param {Object} filter - 删除过滤条件
   * @param {string} filter.level - 日志级别
   * @param {string} filter.min_level - 最小日志级别
   * @param {string} filter.source - 日志来源
   * @param {number} filter.user_id - 用户 ID
   * @param {string} filter.start_time - 开始时间 (RFC3339)
   * @param {string} filter.end_time - 结束时间 (RFC3339)
   * @param {string} filter.keyword - 搜索关键词
   * @param {string} filter.request_id - 请求 ID
   * @returns {Promise<Object>} 删除结果
   */
  deleteLogs: (filter) => api.delete('/logs', { data: filter }),

  /**
   * 清理过期日志
   * @param {Object} options - 清理选项
   * @param {number} options.retention_days - 保留天数 (默认 30)
   * @returns {Promise<Object>} 清理结果
   */
  cleanup: (options = {}) => api.post('/logs/cleanup', options, { silent: true }),

  /**
   * 导出日志
   * @param {Object} params - 导出参数
   * @param {string} params.format - 导出格式 ('json' | 'csv')
   * @param {string} params.level - 日志级别
   * @param {string} params.min_level - 最小日志级别
   * @param {string} params.source - 日志来源
   * @param {number} params.user_id - 用户 ID
   * @param {string} params.start_time - 开始时间 (RFC3339)
   * @param {string} params.end_time - 结束时间 (RFC3339)
   * @param {string} params.keyword - 搜索关键词
   * @param {string} params.request_id - 请求 ID
   * @returns {Promise<Blob>} 导出文件
   */
  exportLogs: (params) => api.get('/logs/export', {
    params,
    responseType: 'blob'
  })
}

export default logsApi
