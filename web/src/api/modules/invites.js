/**
 * 邀请相关 API
 * 处理邀请码、推荐、佣金等操作
 */
import api from '../base'

export const invitesApi = {
  /**
   * 获取当前用户的邀请码
   * @returns {Promise<Object>} 邀请码信息
   */
  getCode: () => api.get('/invite/code'),

  /**
   * 获取推荐列表
   * @param {Object} params - 查询参数
   * @param {number} params.page - 页码
   * @param {number} params.page_size - 每页数量
   * @returns {Promise<Object>} 推荐列表
   */
  getReferrals: (params = {}) => api.get('/invite/referrals', { params }),

  /**
   * 获取邀请统计
   * @returns {Promise<Object>} 邀请统计
   */
  getStats: () => api.get('/invite/stats'),

  /**
   * 获取佣金列表
   * @param {Object} params - 查询参数
   * @param {number} params.page - 页码
   * @param {number} params.page_size - 每页数量
   * @param {string} params.status - 状态过滤 (pending, confirmed, cancelled)
   * @returns {Promise<Object>} 佣金列表
   */
  getCommissions: (params = {}) => api.get('/invite/commissions', { params }),

  /**
   * 获取佣金收益汇总
   * @returns {Promise<Object>} 收益汇总
   */
  getEarnings: () => api.get('/invite/earnings'),

  // 管理员 API
  admin: {
    /**
     * 获取邀请统计（管理员）
     * @returns {Promise<Object>} 全局邀请统计
     */
    getStats: () => api.get('/admin/invites/stats'),

    /**
     * 获取推荐网络（管理员）
     * @param {Object} params - 查询参数
     * @param {number} params.page - 页码
     * @param {number} params.page_size - 每页数量
     * @param {number} params.user_id - 用户ID过滤
     * @returns {Promise<Object>} 推荐网络
     */
    getReferralNetwork: (params = {}) => api.get('/admin/invites/network', { params }),

    /**
     * 获取佣金列表（管理员）
     * @param {Object} params - 查询参数
     * @returns {Promise<Object>} 佣金列表
     */
    getCommissions: (params = {}) => api.get('/admin/invites/commissions', { params }),

    /**
     * 确认佣金（管理员）
     * @param {number} id - 佣金ID
     * @returns {Promise<Object>} 确认后的佣金
     */
    confirmCommission: (id) => api.post(`/admin/invites/commissions/${id}/confirm`),

    /**
     * 取消佣金（管理员）
     * @param {number} id - 佣金ID
     * @param {string} reason - 取消原因
     * @returns {Promise<Object>} 取消后的佣金
     */
    cancelCommission: (id, reason) => api.post(`/admin/invites/commissions/${id}/cancel`, { reason }),

    /**
     * 更新佣金配置（管理员）
     * @param {Object} data - 配置数据
     * @param {number} data.rate - 佣金比例
     * @param {number} data.settlement_delay - 结算延迟（天）
     * @param {number} data.min_withdraw - 最低提现金额
     * @returns {Promise<Object>} 更新后的配置
     */
    updateConfig: (data) => api.put('/admin/invites/config', data)
  }
}

export default invitesApi
