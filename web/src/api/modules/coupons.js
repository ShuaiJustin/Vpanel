/**
 * 优惠券相关 API
 * 处理优惠券验证、管理等操作
 */
import api from '../base'

export const couponsApi = {
  /**
   * 验证优惠券
   * @param {Object} data - 验证数据
   * @param {string} data.code - 优惠券码
   * @param {number} data.plan_id - 套餐ID
   * @param {number} data.amount - 订单金额
   * @returns {Promise<Object>} 优惠券信息和折扣金额
   */
  validate: (data, config = {}) => api.post('/coupons/validate', data, config),

  // 管理员 API
  admin: {
    /**
     * 获取优惠券列表（管理员）
     * @param {Object} params - 查询参数
     * @param {number} params.page - 页码
     * @param {number} params.page_size - 每页数量
     * @param {string} params.status - 状态过滤 (active, expired, depleted)
     * @returns {Promise<Object>} 优惠券列表
     */
    list: (params = {}) => api.get('/admin/coupons', { params }),

    /**
     * 获取优惠券详情（管理员）
     * @param {number} id - 优惠券ID
     * @returns {Promise<Object>} 优惠券详情
     */
    get: (id) => api.get(`/admin/coupons/${id}`),

    /**
     * 创建优惠券（管理员）
     * @param {Object} data - 优惠券数据
     * @param {string} data.code - 优惠券码（可选，不填自动生成）
     * @param {string} data.type - 类型 (fixed, percent)
     * @param {number} data.value - 优惠值（固定金额为分，百分比为0-100）
     * @param {number} data.min_amount - 最低消费金额
     * @param {number} data.max_discount - 最大折扣金额（百分比类型）
     * @param {number} data.total_count - 总数量
     * @param {number} data.per_user_limit - 每用户限制
     * @param {string} data.start_time - 开始时间
     * @param {string} data.end_time - 结束时间
     * @param {Array<number>} data.plan_ids - 适用套餐ID列表
     * @returns {Promise<Object>} 创建的优惠券
     */
    create: (data) => api.post('/admin/coupons', data),

    /**
     * 更新优惠券（管理员）
     * @param {number} id - 优惠券ID
     * @param {Object} data - 优惠券数据
     * @returns {Promise<Object>} 更新后的优惠券
     */
    update: (id, data) => api.put(`/admin/coupons/${id}`, data),

    /**
     * 删除优惠券（管理员）
     * @param {number} id - 优惠券ID
     * @returns {Promise<void>}
     */
    delete: (id) => api.delete(`/admin/coupons/${id}`),

    /**
     * 批量生成优惠券码（管理员）
     * @param {Object} data - 生成数据
     * @param {number} data.coupon_id - 优惠券ID
     * @param {number} data.count - 生成数量
     * @param {string} data.prefix - 前缀（可选）
     * @returns {Promise<Object>} 生成的优惠券码列表
     */
    batchGenerate: (data) => api.post('/admin/coupons/batch', data),

    /**
     * 获取优惠券使用记录（管理员）
     * @param {number} id - 优惠券ID
     * @param {Object} params - 查询参数
     * @returns {Promise<Object>} 使用记录列表
     */
    getUsage: (id, params = {}) => api.get(`/admin/coupons/${id}/usage`, { params })
  }
}

export default couponsApi
