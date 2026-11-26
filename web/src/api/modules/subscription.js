/**
 * 订阅相关 API
 * 处理订阅链接获取、重新生成、内容获取等操作
 */
import api from '../base'

export const subscriptionApi = {
  /**
   * 获取当前用户的订阅链接
   * @returns {Promise<Object>} 订阅链接信息
   */
  getLink: () => api.get('/subscription/link'),

  /**
   * 获取当前用户的订阅详细信息
   * @returns {Promise<Object>} 订阅详细信息
   */
  getInfo: () => api.get('/subscription/info'),

  /**
   * 重新生成订阅令牌
   * @returns {Promise<Object>} 新的订阅信息
   */
  regenerate: () => api.post('/subscription/regenerate'),

  /**
   * 获取订阅内容（通过令牌）
   * @param {string} token - 订阅令牌
   * @param {Object} params - 可选参数
   * @param {string} params.format - 格式 (v2rayn, clash, clashmeta, shadowrocket, surge, quantumultx, singbox)
   * @param {string} params.protocols - 协议过滤 (逗号分隔)
   * @returns {Promise<string>} 订阅内容
   */
  getContent: (token, params = {}) => api.get(`/subscription/${token}`, { params }),

  // 管理员 API
  admin: {
    /**
     * 获取所有订阅列表（管理员）
     * @param {Object} params - 查询参数
     * @param {number} params.page - 页码
     * @param {number} params.page_size - 每页数量
     * @param {number} params.user_id - 用户ID过滤
     * @returns {Promise<Object>} 订阅列表
     */
    list: (params = {}) => api.get('/admin/subscriptions', { params }),

    /**
     * 撤销用户订阅（管理员）
     * @param {number} userId - 用户ID
     * @returns {Promise<void>}
     */
    revoke: (userId) => api.delete(`/admin/subscriptions/${userId}`),

    /**
     * 重置订阅访问统计（管理员）
     * @param {number} userId - 用户ID
     * @returns {Promise<void>}
     */
    resetStats: (userId) => api.post(`/admin/subscriptions/${userId}/reset-stats`)
  }
}

export default subscriptionApi
