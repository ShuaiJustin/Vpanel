/**
 * 礼品卡相关 API
 * 处理礼品卡兑换、查询等操作
 */
import api from '../base'

export const giftCardsApi = {
  /**
   * 兑换礼品卡
   * @param {Object} data - 兑换数据
   * @param {string} data.code - 礼品卡码
   * @returns {Promise<Object>} 兑换结果
   */
  redeem: (data) => api.post('/gift-cards/redeem', data),

  /**
   * 验证礼品卡
   * @param {Object} data - 验证数据
   * @param {string} data.code - 礼品卡码
   * @returns {Promise<Object>} 礼品卡信息
   */
  validate: (data) => api.post('/gift-cards/validate', data),

  /**
   * 获取用户已兑换的礼品卡列表
   * @param {Object} params - 查询参数
   * @param {number} params.page - 页码
   * @param {number} params.page_size - 每页数量
   * @returns {Promise<Object>} 礼品卡列表
   */
  list: (params = {}) => api.get('/gift-cards', { params }),

  // 管理员 API
  admin: {
    /**
     * 获取礼品卡列表（管理员）
     * @param {Object} params - 查询参数
     * @param {number} params.page - 页码
     * @param {number} params.page_size - 每页数量
     * @param {string} params.status - 状态过滤 (active, redeemed, expired, disabled)
     * @param {string} params.batch_id - 批次ID过滤
     * @returns {Promise<Object>} 礼品卡列表
     */
    list: (params = {}) => api.get('/admin/gift-cards', { params }),

    /**
     * 获取礼品卡详情（管理员）
     * @param {number} id - 礼品卡ID
     * @returns {Promise<Object>} 礼品卡详情
     */
    get: (id) => api.get(`/admin/gift-cards/${id}`),

    /**
     * 批量创建礼品卡（管理员）
     * @param {Object} data - 创建数据
     * @param {number} data.count - 数量 (1-1000)
     * @param {number} data.value - 面值（分）
     * @param {string} data.expires_at - 过期时间（可选，ISO 8601格式）
     * @param {string} data.prefix - 前缀（可选）
     * @returns {Promise<Object>} 创建的礼品卡列表
     */
    createBatch: (data) => api.post('/admin/gift-cards/batch', data),

    /**
     * 更新礼品卡状态（管理员）
     * @param {number} id - 礼品卡ID
     * @param {Object} data - 状态数据
     * @param {string} data.status - 状态 (active, disabled)
     * @returns {Promise<void>}
     */
    setStatus: (id, data) => api.put(`/admin/gift-cards/${id}/status`, data),

    /**
     * 删除礼品卡（管理员）
     * @param {number} id - 礼品卡ID
     * @returns {Promise<void>}
     */
    delete: (id) => api.delete(`/admin/gift-cards/${id}`),

    /**
     * 获取礼品卡统计（管理员）
     * @returns {Promise<Object>} 统计数据
     */
    getStats: () => api.get('/admin/gift-cards/stats'),

    /**
     * 获取批次统计（管理员）
     * @param {string} batchId - 批次ID
     * @returns {Promise<Object>} 批次统计数据
     */
    getBatchStats: (batchId) => api.get(`/admin/gift-cards/batch/${batchId}/stats`)
  }
}

export default giftCardsApi
