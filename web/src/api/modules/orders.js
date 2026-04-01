/**
 * 订单相关 API
 * 处理订单创建、查询、取消等操作
 */
import api from '../base'

export const ordersApi = {
  /**
   * 创建订单
   * @param {Object} data - 订单数据
   * @param {number} data.plan_id - 套餐ID
   * @param {string} data.coupon_code - 预留字段，当前下单流程未启用
   * @returns {Promise<Object>} 创建的订单
   */
  create: (data) => api.post('/orders', data),

  /**
   * 获取用户订单列表
   * @param {Object} params - 查询参数
   * @param {number} params.page - 页码
   * @param {number} params.page_size - 每页数量
   * @param {string} params.status - 状态过滤
   * @returns {Promise<Object>} 订单列表
   */
  list: (params = {}) => api.get('/orders', { params }),

  /**
   * 获取订单详情
   * @param {number} id - 订单ID
   * @returns {Promise<Object>} 订单详情
   */
  get: (id) => api.get(`/orders/${id}`),

  /**
   * 根据订单号获取订单详情
   * @param {string} orderNo - 订单号
   * @returns {Promise<Object>} 订单详情
   */
  getByOrderNo: (orderNo) => api.get(`/orders/by-order-no/${orderNo}`),

  /**
   * 取消订单
   * @param {number} id - 订单ID
   * @returns {Promise<Object>} 取消后的订单
   */
  cancel: (id) => api.post(`/orders/${id}/cancel`),

  // 管理员 API
  admin: {
    /**
     * 获取所有订单列表（管理员）
     * @param {Object} params - 查询参数
     * @param {number} params.page - 页码
     * @param {number} params.page_size - 每页数量
     * @param {string} params.status - 状态过滤
     * @param {number} params.user_id - 用户ID过滤
     * @param {string} params.start_date - 开始日期
     * @param {string} params.end_date - 结束日期
     * @returns {Promise<Object>} 订单列表
     */
    list: (params = {}) => api.get('/admin/orders', { params }),

    /**
     * 获取订单详情（管理员）
     * @param {number} id - 订单ID
     * @returns {Promise<Object>} 订单详情
     */
    get: (id) => api.get(`/admin/orders/${id}`),

    /**
     * 更新订单状态（管理员）
     * @param {number} id - 订单ID
     * @param {string} status - 新状态
     * @returns {Promise<Object>} 更新后的订单
     */
    updateStatus: (id, status) => api.put(`/admin/orders/${id}/status`, { status }),

    /**
     * 处理退款（管理员）
     * @param {number} id - 订单ID
     * @param {Object} data - 退款数据
     * @param {number} data.amount - 退款金额（可选，不填为全额退款）
     * @param {string} data.reason - 退款原因
     * @returns {Promise<Object>} 退款结果
     */
    refund: (id, data) => api.post(`/admin/orders/${id}/refund`, data)
  }
}

export default ordersApi
