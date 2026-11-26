/**
 * 代理管理 API
 * 处理代理的增删改查、启停、统计等操作
 */
import api from '../base'

export const proxiesApi = {
  /**
   * 获取代理列表
   * @param {Object} params - 查询参数
   * @param {number} params.page - 页码
   * @param {number} params.pageSize - 每页数量
   * @param {string} params.protocol - 协议类型
   * @param {boolean} params.enabled - 是否启用
   * @returns {Promise<Object>} 代理列表
   */
  list: (params) => api.get('/proxies', { params }),

  /**
   * 获取单个代理
   * @param {number|string} id - 代理 ID
   * @returns {Promise<Object>} 代理信息
   */
  get: (id) => api.get(`/proxies/${id}`),

  /**
   * 创建代理
   * @param {Object} data - 代理数据
   * @returns {Promise<Object>} 创建的代理
   */
  create: (data) => api.post('/proxies', data),

  /**
   * 更新代理
   * @param {number|string} id - 代理 ID
   * @param {Object} data - 代理数据
   * @returns {Promise<Object>} 更新后的代理
   */
  update: (id, data) => api.put(`/proxies/${id}`, data),

  /**
   * 删除代理
   * @param {number|string} id - 代理 ID
   * @returns {Promise<void>}
   */
  delete: (id) => api.delete(`/proxies/${id}`),

  /**
   * 启动代理
   * @param {number|string} id - 代理 ID
   * @returns {Promise<void>}
   */
  start: (id) => api.post(`/proxies/${id}/start`),

  /**
   * 停止代理
   * @param {number|string} id - 代理 ID
   * @returns {Promise<void>}
   */
  stop: (id) => api.post(`/proxies/${id}/stop`),

  /**
   * 获取代理统计
   * @param {number|string} id - 代理 ID
   * @returns {Promise<Object>} 代理统计信息
   */
  getStats: (id) => api.get(`/proxies/${id}/stats`),

  /**
   * 批量启用代理
   * @param {Array<number|string>} ids - 代理 ID 列表
   * @returns {Promise<void>}
   */
  batchEnable: (ids) => api.post('/proxies/batch', { operation: 'enable', ids }),

  /**
   * 批量禁用代理
   * @param {Array<number|string>} ids - 代理 ID 列表
   * @returns {Promise<void>}
   */
  batchDisable: (ids) => api.post('/proxies/batch', { operation: 'disable', ids }),

  /**
   * 批量删除代理
   * @param {Array<number|string>} ids - 代理 ID 列表
   * @returns {Promise<void>}
   */
  batchDelete: (ids) => api.post('/proxies/batch', { operation: 'delete', ids }),

  /**
   * 生成分享链接
   * @param {number|string} id - 代理 ID
   * @returns {Promise<Object>} 分享链接
   */
  generateLink: (id) => api.get(`/proxies/${id}/link`)
}

export default proxiesApi
