/**
 * 套餐升降级相关 API
 * 处理套餐升级、降级、计算价格差异等操作
 */
import api from '../base'

export const planChangeApi = {
  /** 获取服务端确认的当前套餐及购买时计价信息。 */
  getCurrentPlan: () => api.get('/plan-change/current', { silent: true }),
  /**
   * 计算套餐变更价格
   * @param {Object} data - 变更数据
   * @param {number} [data.current_plan_id] - 当前套餐ID（仅作为过期页面校验，实际归属由服务端确认）
   * @param {number} data.new_plan_id - 新套餐ID
   * @returns {Promise<Object>} 价格计算结果
   */
  calculate: (data) => api.post('/plan-change/calculate', data),

  /**
   * 执行套餐升级
   * @param {Object} data - 升级数据
   * @param {number} [data.current_plan_id] - 当前套餐ID（仅校验，不能覆盖服务端归属）
   * @param {number} data.new_plan_id - 新套餐ID
   * @returns {Promise<Object>} 升级结果
   */
  upgrade: (data) => api.post('/plan-change/upgrade', data),

  /**
   * 预约套餐降级
   * @param {Object} data - 降级数据
   * @param {number} [data.current_plan_id] - 当前套餐ID（仅校验，不能覆盖服务端归属）
   * @param {number} data.new_plan_id - 新套餐ID
   * @returns {Promise<Object>} 降级预约结果
   */
  downgrade: (data) => api.post('/plan-change/downgrade', data),

  /**
   * 获取待执行的降级
   * @returns {Promise<Object>} 待执行降级信息
   */
  getPendingDowngrade: () => api.get('/plan-change/downgrade'),

  /**
   * 取消待执行的降级
   * @returns {Promise<void>}
   */
  cancelDowngrade: () => api.delete('/plan-change/downgrade')
}

export default planChangeApi
