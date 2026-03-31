/**
 * 余额相关 API
 * 处理余额查询、充值、交易历史等操作
 */
import api from '../base'

export const balanceApi = {
  /**
   * 获取当前用户余额
   * @returns {Promise<Object>} 余额信息
   */
  get: () => api.get('/balance'),

  /**
   * 创建充值订单
   * @param {Object} data - 充值数据
   * @param {number} data.amount - 充值金额（分）
   * @param {string} data.method - 支付方式
   * @returns {Promise<Object>} 充值订单与支付信息
   */
  recharge: (data) => api.post('/balance/recharge', data, { silent: true }),

  /**
   * 获取充值订单状态
   * @param {string} orderNo - 充值订单号
   * @returns {Promise<Object>} 充值订单状态
   */
  getRechargeStatus: (orderNo) => api.get(`/balance/recharge/status/${orderNo}`),

  /**
   * 获取交易历史
   * @param {Object} params - 查询参数
   * @param {number} params.page - 页码
   * @param {number} params.page_size - 每页数量
   * @param {string} params.type - 交易类型过滤
   * @returns {Promise<Object>} 交易历史列表
   */
  getTransactions: (params = {}) => api.get('/balance/transactions', { params }),

  admin: {
    /**
     * 获取充值订单列表（管理员）
     * @param {Object} params - 查询参数
     * @returns {Promise<Object>} 充值订单列表
     */
    listRechargeOrders: (params = {}) => api.get('/admin/balance/recharge-orders', { params }),

    /**
     * 调整用户余额（管理员）
     * @param {Object} data - 调整数据
     * @param {number} data.user_id - 用户ID
     * @param {number} data.amount - 调整金额（正数增加，负数减少）
     * @param {string} data.reason - 调整原因
     * @returns {Promise<Object>} 调整结果
     */
    adjust: (data) => api.post('/admin/balance/adjust', data)
  }
}

export default balanceApi
