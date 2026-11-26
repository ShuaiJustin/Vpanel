/**
 * 套餐相关 API
 * 处理套餐列表、详情、管理等操作
 */
import api from '../base'

export const plansApi = {
  /**
   * 获取可用套餐列表（用户端）
   * @returns {Promise<Object>} 套餐列表
   */
  list: () => api.get('/plans'),

  /**
   * 获取套餐详情
   * @param {number} id - 套餐ID
   * @returns {Promise<Object>} 套餐详情
   */
  get: (id) => api.get(`/plans/${id}`),

  /**
   * 比较套餐
   * @param {Array<number>} ids - 套餐ID列表
   * @returns {Promise<Object>} 套餐比较结果
   */
  compare: (ids) => api.get('/plans/compare', { params: { ids: ids.join(',') } }),

  // 管理员 API
  admin: {
    /**
     * 获取所有套餐列表（管理员）
     * @param {Object} params - 查询参数
     * @param {number} params.page - 页码
     * @param {number} params.page_size - 每页数量
     * @param {boolean} params.include_inactive - 是否包含禁用套餐
     * @returns {Promise<Object>} 套餐列表
     */
    list: (params = {}) => api.get('/admin/plans', { params }),

    /**
     * 创建套餐
     * @param {Object} data - 套餐数据
     * @param {string} data.name - 套餐名称
     * @param {string} data.description - 套餐描述
     * @param {number} data.price - 价格（分）
     * @param {number} data.duration - 时长（天）
     * @param {number} data.traffic_limit - 流量限制（字节）
     * @param {string} data.plan_type - 套餐类型
     * @param {string} data.reset_cycle - 流量重置周期
     * @param {number} data.ip_limit - IP / 设备限制
     * @param {Array<string>} data.features - 功能列表
     * @param {number} data.sort_order - 排序
     * @param {boolean} data.is_active - 是否启用
     * @param {boolean} data.is_recommended - 是否推荐
     * @returns {Promise<Object>} 创建的套餐
     */
    create: (data) => api.post('/admin/plans', data),

    /**
     * 更新套餐
     * @param {number} id - 套餐ID
     * @param {Object} data - 套餐数据
     * @returns {Promise<Object>} 更新后的套餐
     */
    update: (id, data) => api.put(`/admin/plans/${id}`, data),

    /**
     * 删除套餐
     * @param {number} id - 套餐ID
     * @returns {Promise<void>}
     */
    delete: (id) => api.delete(`/admin/plans/${id}`),

    /**
     * 启用/禁用套餐
     * @param {number} id - 套餐ID
     * @param {boolean} active - 是否启用
     * @returns {Promise<Object>} 更新后的套餐
     */
    setActive: (id, active) => api.put(`/admin/plans/${id}/status`, { is_active: active })
  }
}

export default plansApi
